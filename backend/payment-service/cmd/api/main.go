// Package main is the entrypoint for the payment-service.
// It wires: config → logger → postgres pool → midtrans client → gin engine
// → register payments routes → graceful shutdown.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/parking-portal/backend/payment-service/internal/config"
	"github.com/parking-portal/backend/payment-service/internal/database"
	"github.com/parking-portal/backend/payment-service/internal/events"
	"github.com/parking-portal/backend/payment-service/internal/invoices"
	"github.com/parking-portal/backend/payment-service/internal/midtrans"
	"github.com/parking-portal/backend/payment-service/internal/payments"
	"github.com/parking-portal/backend/pkg/auth"
	"github.com/parking-portal/backend/pkg/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	zl, _ := zap.NewDevelopment()
	defer zl.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Postgres pool
	db, err := database.NewPostgres(ctx, cfg, zl)
	if err != nil {
		zl.Fatal("postgres", zap.Error(err))
	}
	defer db.Close()

	// Auth signer (also used to mint service-to-service JWT for violation-service)
	signer, err := auth.NewSigner(cfg.JWTSecret, cfg.JWTExpirationHrs, cfg.AppName)
	if err != nil {
		zl.Fatal("auth signer", zap.Error(err))
	}

	// Service-to-service JWT for calling the violation-service.
	// We mint a long-lived OFFICER-role token used only for service-to-service
	// calls. (In production we'd add a dedicated "service" role or a
	// X-Service-Token bypass in the violation-service's auth middleware.)
	// Using the seeded officer's user_id so the violation-service's
	// `middleware.UserID` returns a valid FK target.
	serviceToken, _ := signer.Sign(
		uuid.MustParse("11111111-1111-1111-1111-111111111111"), // officer
		auth.RoleOfficer,
	)

	// Midtrans client
	mid := midtrans.NewClient(
		cfg.MidtransServerKey,
		cfg.MidtransEnv,
		cfg.MidtransNotificationURL,
		cfg.MidtransReturnURL,
		cfg.MidtransEnabledMethods,
		cfg.MidtransHTTPTimeout,
	)
	zl.Info("midtrans client ready",
		zap.String("env", cfg.MidtransEnv),
		zap.Bool("mock", mid.IsMock()),
		zap.Strings("enabled_methods", mid.EnabledMethods()),
	)

	// Invoices client (HTTP to violation-service)
	invClient := invoices.NewClient(cfg.ViolationServiceURL, serviceToken)

	// Events publisher (optional)
	var pub *events.Publisher
	if cfg.RabbitMQURL != "" {
		p, err := events.NewPublisher(cfg.RabbitMQURL, cfg.RabbitMQExchange, zl)
		if err != nil {
			zl.Warn("rabbitmq unavailable, events will be dropped", zap.Error(err))
		} else {
			pub = p
			defer pub.Close()
		}
	}

	// Payments
	payRepo := payments.NewPGRepository(db)
	paySvc := payments.NewService(payRepo, invClient, mid, pub)
	payH := payments.NewHandler(paySvc, zl)

	// Gin engine
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(
		middleware.RequestID(),
		middleware.Recovery(zl),
		middleware.CORS(),
		middleware.Logger(zl),
	)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "payment-service"})
	})

	// Webhook is public (no JWT) — Midtrans is the caller
	// All other routes require JWT
	authMW := middleware.Auth(signer)
	v1 := r.Group("/api/v1")
	v1.Use(authMW)

	payH.Register(v1, r)

	// Start server
	srv := &http.Server{Addr: ":" + cfg.AppPort, Handler: r}
	go func() {
		zl.Info("payment-service listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zl.Fatal("listen", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	zl.Info("shutting down...")
	shutCtx, c2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer c2()
	_ = srv.Shutdown(shutCtx)
	fmt.Println("bye")
}
