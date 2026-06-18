// Package config loads the notification-worker's environment configuration.
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

// Config is the fully-resolved configuration for the worker.
type Config struct {
	AppName string

	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string
	DBMaxConns int32

	RabbitMQURL      string
	RabbitMQExchange string
	RabbitMQQueue    string // queue name to consume from

	WorkerConcurrency int
	WorkerRetryCount  int
}

// Load reads env vars and returns a Config or an error.
func Load() (*Config, error) {
	if path, err := dotenv.AutoLoad(); err != nil {
		log.Printf("config: dotenv load %s: %v", path, err)
	}

	cfg := &Config{
		AppName: getenv("APP_NAME", "Notification Worker"),

		DBHost:     getenv("DB_HOST", "localhost"),
		DBPort:     getenv("DB_PORT", "5432"),
		DBName:     getenv("DB_NAME", "parking_portal"),
		DBUser:     getenv("DB_USER", "postgres"),
		DBPassword: getenv("DB_PASSWORD", "postgres"),

		RabbitMQURL:      getenv("RABBITMQ_URL", "amqp://localhost:5672/"),
		RabbitMQExchange: getenv("RABBITMQ_EXCHANGE", "parking.events"),
		RabbitMQQueue:    getenv("RABBITMQ_NOTIFICATION_QUEUE", "notification.queue"),
	}
	fmt.Println(cfg)
	cfg.DBMaxConns = int32(getenvInt("DB_MAX_CONNS", 5))
	cfg.WorkerConcurrency = getenvInt("WORKER_CONCURRENCY", 1)
	cfg.WorkerRetryCount = getenvInt("WORKER_RETRY_COUNT", 3)

	if cfg.RabbitMQURL == "" {
		return nil, fmt.Errorf("RABBITMQ_URL is required")
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

// unused import guard
var _ = time.Second
