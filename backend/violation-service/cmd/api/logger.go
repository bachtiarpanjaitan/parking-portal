package main

import "go.uber.org/zap"

// loggerNew re-exports the shared logger constructor. We do this so the
// main package doesn't import pkg/logger directly (cleaner dependency graph).
func loggerNew(env string) (*zap.Logger, error) {
	return zap.NewDevelopment() // good enough for the slice
}
