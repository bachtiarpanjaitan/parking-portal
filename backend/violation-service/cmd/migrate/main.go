// Package main runs the SQL migrations in order.
//
// Usage:
//
//	go run ./cmd/migrate
//	go run ./cmd/migrate -dir ./migrations
//
// Env (see .ai/ENVVAR_CONFIG.md): DB_HOST DB_PORT DB_NAME DB_USER DB_PASSWORD
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	dir := flag.String("dir", "./migrations", "directory containing *.sql files (lexicographic order)")
	flag.Parse()

	dsn := buildDSN()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	if err := runMigrations(ctx, conn, *dir); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	fmt.Println("✅ migrations applied")
}

func buildDSN() string {
	host := env("DB_HOST", "localhost")
	port := env("DB_PORT", "5432")
	name := env("DB_NAME", "parking_portal")
	user := env("DB_USER", "postgres")
	pass := env("DB_PASSWORD", "postgres")
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, name)
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func runMigrations(ctx context.Context, conn *pgx.Conn, dir string) error {
	// First, ensure the schema_migrations table exists (idempotent).
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(32) PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)

	applied := map[string]bool{}
	rows, err := conn.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("select applied: %w", err)
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()

	for _, f := range files {
		version := strings.TrimSuffix(f, ".sql")
		if applied[version] {
			continue
		}
		path := filepath.Join(dir, f)
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		fmt.Printf("→ applying %s\n", version)
		// Run as a single batch; CREATE INDEX CONCURRENTLY is not used here
		// because the migrator runs outside a transaction.
		if _, err := conn.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("apply %s: %w", version, err)
		}
		if _, err := conn.Exec(ctx,
			"INSERT INTO schema_migrations(version) VALUES ($1)", version,
		); err != nil {
			return fmt.Errorf("record %s: %w", version, err)
		}
	}
	return nil
}
