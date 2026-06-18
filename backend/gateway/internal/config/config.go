// Package config loads the API gateway's environment configuration.
// See .ai/ENVVAR_CONFIG.md.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config is the fully-resolved configuration for the gateway.
type Config struct {
	AppEnv  string
	AppName string
	AppPort string

	// JWT shared with backend services.
	JWTSecret        string
	JWTExpirationHrs int

	// Backend service URLs (in-process or Docker).
	ViolationServiceURL string
	PaymentServiceURL   string

	// HTTP client tuning.
	UpstreamTimeoutSeconds int
}

func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:  getenv("APP_ENV", "development"),
		AppName: getenv("APP_NAME", "API Gateway"),
		AppPort: getenv("APP_PORT", "8080"),

		JWTSecret: os.Getenv("JWT_SECRET"),

		ViolationServiceURL: getenv("VIOLATION_SERVICE_URL", "http://localhost:8081"),
		PaymentServiceURL:   getenv("PAYMENT_SERVICE_URL", "http://localhost:8082"),
	}

	cfg.JWTExpirationHrs = getenvInt("JWT_EXPIRATION_HOURS", 24)
	cfg.UpstreamTimeoutSeconds = getenvInt("UPSTREAM_TIMEOUT_SECONDS", 30)

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 bytes")
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
