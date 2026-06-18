// Package main is the entrypoint for the notification worker.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/parking-portal/backend/notification-worker/internal/config"
	"github.com/parking-portal/backend/notification-worker/internal/consumer"
	"github.com/parking-portal/backend/notification-worker/internal/notifications"
	"github.com/parking-portal/backend/notification-worker/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	log.Printf("[main] %s starting (concurrency=%d)", cfg.AppName, cfg.WorkerConcurrency)

	// Postgres pool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("pgx parse: %v", err)
	}
	poolCfg.MaxConns = cfg.DBMaxConns
	poolCfg.HealthCheckPeriod = 30 * time.Second
	db, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		log.Fatalf("pgx connect: %v", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		log.Fatalf("pgx ping: %v", err)
	}
	log.Printf("[main] postgres connected")

	// RabbitMQ consumer
	c, err := consumer.New(cfg.RabbitMQURL, cfg.RabbitMQExchange, cfg.RabbitMQQueue)
	if err != nil {
		log.Fatalf("rabbitmq: %v", err)
	}
	defer c.Close()
	log.Printf("[main] rabbitmq consumer ready (exchange=%s queue=%s)", cfg.RabbitMQExchange, cfg.RabbitMQQueue)

	// Notifications service
	repo := notifications.NewPGRepository(db)
	svc := notifications.NewService(repo)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Printf("[main] shutdown signal received")
		cancel()
	}()

	if err := worker.Run(ctx, c, svc, cfg.WorkerConcurrency); err != nil {
		log.Fatalf("worker: %v", err)
	}
	log.Printf("[main] bye")
}
