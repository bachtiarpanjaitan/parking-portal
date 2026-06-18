// Package config loads the payment-service's environment configuration.
// See .ai/ENVVAR_CONFIG.md (Midtrans section) and ADR-012.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully-resolved configuration.
type Config struct {
	AppEnv  string
	AppName string
	AppPort string

	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string
	DBMaxConns int32

	JWTSecret        string
	JWTExpirationHrs int

	// Violation Service (we call it to fetch invoices + update their status)
	ViolationServiceURL string

	// RabbitMQ
	RabbitMQURL      string
	RabbitMQExchange string

	// Midtrans (see ADR-012)
	MidtransServerKey       string
	MidtransEnv             string // "sandbox" or "production"
	MidtransEnabledMethods  []string
	MidtransNotificationURL string
	MidtransReturnURL       string
	MidtransHTTPTimeout     time.Duration
}

// Load reads env vars and returns a Config or an error.
func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:  getenv("APP_ENV", "development"),
		AppName: getenv("APP_NAME", "Parking Violation Portal"),
		AppPort: getenv("APP_PORT", "8082"),

		DBHost:     getenv("DB_HOST", "localhost"),
		DBPort:     getenv("DB_PORT", "5432"),
		DBName:     getenv("DB_NAME", "parking_portal"),
		DBUser:     getenv("DB_USER", "postgres"),
		DBPassword: getenv("DB_PASSWORD", "postgres"),

		JWTSecret: os.Getenv("JWT_SECRET"),

		ViolationServiceURL: getenv("VIOLATION_SERVICE_URL", "http://localhost:8081"),

		RabbitMQURL:      getenv("RABBITMQ_URL", ""),
		RabbitMQExchange: getenv("RABBITMQ_EXCHANGE", "parking.events"),

		MidtransServerKey:       getenv("MIDTRANS_SERVER_KEY", ""),
		MidtransEnv:             getenv("MIDTRANS_ENV", "sandbox"),
		MidtransNotificationURL: getenv("MIDTRANS_NOTIFICATION_URL", ""),
		MidtransReturnURL:       getenv("MIDTRANS_RETURN_URL", ""),
	}

	// Parse comma-separated enabled methods
	raw := getenv("MIDTRANS_ENABLED_METHODS", "qris,gopay")
	for _, m := range strings.Split(raw, ",") {
		m = strings.TrimSpace(m)
		if m != "" {
			cfg.MidtransEnabledMethods = append(cfg.MidtransEnabledMethods, m)
		}
	}
	if len(cfg.MidtransEnabledMethods) == 0 {
		cfg.MidtransEnabledMethods = []string{"qris", "gopay"}
	}

	cfg.DBMaxConns = int32(getenvInt("DB_MAX_CONNS", 10))
	cfg.JWTExpirationHrs = getenvInt("JWT_EXPIRATION_HOURS", 24)
	cfg.MidtransHTTPTimeout = time.Duration(getenvInt("MIDTRANS_HTTP_TIMEOUT_SECONDS", 10)) * time.Second

	// Validation
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 bytes")
	}
	if cfg.MidtransServerKey == "" {
		return nil, fmt.Errorf("MIDTRANS_SERVER_KEY is required")
	}
	return cfg, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}
