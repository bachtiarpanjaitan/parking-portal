// Package config loads the violation-service's environment configuration.
// See .ai/ENVVAR_CONFIG.md.
package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/parking-portal/backend/pkg/dotenv"
)

// Config is the fully-resolved configuration for the violation service.
type Config struct {
	AppEnv  string
	AppName string
	AppPort string

	DBHost            string
	DBPort            string
	DBName            string
	DBUser            string
	DBPassword        string
	DBMaxConns        int32
	DBConnMaxLifetime time.Duration

	JWTSecret        string
	JWTExpirationHrs int

	RabbitMQURL      string
	RabbitMQExchange string

	StoragePath     string
	MaxUploadSizeMB int
	PublicUploadURL string
}

// Load reads env vars and returns a Config or an error if a required one is missing.
//
// On startup, the project-root .env file is auto-loaded (walking up from the
// current working directory) so `go run ./violation-service/cmd/api` works
// out of the box. Existing shell / docker-compose env vars always win.
func Load() (*Config, error) {
	if path, err := dotenv.AutoLoad(); err != nil {
		log.Printf("config: dotenv load %s: %v", path, err)
	}

	cfg := &Config{
		AppEnv:  getenv("APP_ENV", "development"),
		AppName: getenv("APP_NAME", "Parking Violation Portal"),
		AppPort: getenv("APP_PORT", "8081"),

		DBHost:     getenv("DB_HOST", "localhost"),
		DBPort:     getenv("DB_PORT", "5432"),
		DBName:     getenv("DB_NAME", "parking_portal"),
		DBUser:     getenv("DB_USER", "postgres"),
		DBPassword: getenv("DB_PASSWORD", "postgres"),
		JWTSecret:  os.Getenv("JWT_SECRET"),

		RabbitMQURL:      getenv("RABBITMQ_URL", ""),
		RabbitMQExchange: getenv("RABBITMQ_EXCHANGE", "parking.events"),

		StoragePath:     getenv("STORAGE_PATH", "./storage"),
		MaxUploadSizeMB: getenvInt("MAX_UPLOAD_SIZE_MB", 5),
		PublicUploadURL: getenv("PUBLIC_UPLOAD_URL", "/uploads"),
	}

	maxConns := getenvInt("DB_MAX_CONNS", 10)
	cfg.DBMaxConns = int32(maxConns)

	cfg.JWTExpirationHrs = getenvInt("JWT_EXPIRATION_HOURS", 24)

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
