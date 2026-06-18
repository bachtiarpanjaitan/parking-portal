// Package logger creates the project's structured logger (zap).
// All services share this so log lines have a consistent format.
package logger

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New returns a zap.Logger configured for the given env ("development" |
// "staging" | "production"). In dev it uses console output; otherwise JSON.
func New(env string) (*zap.Logger, error) {
	var cfg zap.Config
	if strings.EqualFold(env, "production") {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "ts"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	cfg.InitialFields = map[string]any{
		"service": os.Getenv("APP_NAME"),
		"env":     env,
	}
	return cfg.Build()
}
