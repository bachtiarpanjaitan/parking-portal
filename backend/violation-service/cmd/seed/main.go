// Package main seeds the demo data defined in .ai/SEED_DATA.md.
//
// Usage:
//
//	go run ./cmd/seed
//
// Idempotent: re-running is safe (uses ON CONFLICT DO NOTHING).
// Default password for all 3 demo users is "password123" (bcrypt-hashed).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// Fixed UUIDs (matches .ai/SEED_DATA.md).
var (
	OfficerID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	MemberID  = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	Member2ID = uuid.MustParse("33333333-3333-3333-3333-333333333333")

	RuleV1ID = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	V1 = uuid.MustParse("10000001-0000-0000-0000-000000000001")
	V2 = uuid.MustParse("10000002-0000-0000-0000-000000000002")
	V3 = uuid.MustParse("10000003-0000-0000-0000-000000000003")
	V4 = uuid.MustParse("10000004-0000-0000-0000-000000000004")

	I1 = uuid.MustParse("20000001-0000-0000-0000-000000000001")
	I2 = uuid.MustParse("20000002-0000-0000-0000-000000000002")
	I3 = uuid.MustParse("20000003-0000-0000-0000-000000000003")
	I4 = uuid.MustParse("20000004-0000-0000-0000-000000000004")

	P1 = uuid.MustParse("30000001-0000-0000-0000-000000000001")
	P3 = uuid.MustParse("30000003-0000-0000-0000-000000000003")
)

// DefaultPassword is the demo password for all 3 seeded users.
// In production, this would be set per-user at creation time and never stored in plaintext.
const DefaultPassword = "password123"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dsn := buildDSN()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	if err := run(ctx, conn); err != nil {
		log.Fatalf("seed: %v", err)
	}
	fmt.Println("✅ seed complete")
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

func run(ctx context.Context, conn *pgx.Conn) error {
	now := time.Now().UTC()

	// Hash the default password once.
	hash, err := bcrypt.GenerateFromPassword([]byte(DefaultPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	// ---- users (now with password_hash) ----
	users := []struct {
		id    uuid.UUID
		name  string
		email string
		role  string
	}{
		{OfficerID, "Officer User", "officer@example.com", "OFFICER"},
		{MemberID, "Member User", "member@example.com", "MEMBER"},
		{Member2ID, "Member Two", "member2@example.com", "MEMBER"},
	}
	for _, u := range users {
		// ON CONFLICT (id) DO UPDATE so re-seeding updates the password hash too
		// (this is important: the previous seed didn't have password_hash).
		if _, err := conn.Exec(ctx, `
			INSERT INTO users (id, name, email, role, password_hash, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $6)
			ON CONFLICT (id) DO UPDATE
			SET password_hash = EXCLUDED.password_hash,
			    updated_at    = EXCLUDED.updated_at
		`, u.id, u.name, u.email, u.role, string(hash), now); err != nil {
			return fmt.Errorf("upsert user %s: %w", u.email, err)
		}
	}

	// ---- fine_rule_versions (V1 active) ----
	if _, err := conn.Exec(ctx, `
		INSERT INTO fine_rule_versions (id, version_number, is_active, published_at, created_by, created_at, updated_at)
		VALUES ($1, 1, true, '2026-01-01T00:00:00Z', $2, $3, $3)
		ON CONFLICT (id) DO NOTHING
	`, RuleV1ID, OfficerID, now); err != nil {
		return fmt.Errorf("insert rule version: %w", err)
	}

	// ---- fine_rule_details (4 rows) ----
	rules := []struct {
		id, vid     uuid.UUID
		vtype       string
		base        float64
		day, night  float64
		r0, r1, r2p float64
	}{
		{uuid.New(), RuleV1ID, "expired_meter", 50000, 1.0, 1.5, 1.0, 1.5, 2.0},
		{uuid.New(), RuleV1ID, "no_parking_zone", 150000, 1.0, 1.5, 1.0, 1.5, 2.0},
		{uuid.New(), RuleV1ID, "blocking_hydrant", 250000, 1.0, 1.5, 1.0, 1.5, 2.0},
		{uuid.New(), RuleV1ID, "disabled_spot", 500000, 1.0, 1.5, 1.0, 1.5, 2.0},
	}
	for _, r := range rules {
		if _, err := conn.Exec(ctx, `
			INSERT INTO fine_rule_details
				(id, rule_version_id, violation_type, base_amount,
				 day_multiplier, night_multiplier, repeat_0, repeat_1, repeat_2_plus,
				 created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)
			ON CONFLICT (rule_version_id, violation_type) DO NOTHING
		`, r.id, r.vid, r.vtype, r.base, r.day, r.night, r.r0, r.r1, r.r2p, now); err != nil {
			return fmt.Errorf("insert rule detail %s: %w", r.vtype, err)
		}
	}

	// ---- violations (4) ----
	violations := []struct {
		id    uuid.UUID
		ts    time.Time
		vtype string
		loc   string
		plate string
		fine  float64
		snap  any
	}{
		{V1, mustTime("2026-03-20T10:00:00Z"), "no_parking_zone", "Jl. Sudirman, Jakarta", "B1234XYZ", 150000,
			snap(RuleV1ID, 1, "no_parking_zone", 150000, 1.0, "DAY", 1.0, 0, 150000, "2026-03-20T10:00:00Z")},
		{V2, mustTime("2026-05-01T23:30:00Z"), "blocking_hydrant", "Jl. Thamrin, Jakarta", "B1234XYZ", 750000,
			snap(RuleV1ID, 1, "blocking_hydrant", 250000, 1.5, "NIGHT", 2.0, 1, 750000, "2026-05-01T23:30:00Z")},
		{V3, mustTime("2026-05-30T08:00:00Z"), "disabled_spot", "Jl. Gatot Subroto, Jakarta", "B1234XYZ", 500000,
			snap(RuleV1ID, 1, "disabled_spot", 500000, 1.0, "DAY", 1.0, 0, 500000, "2026-05-30T08:00:00Z")},
		{V4, mustTime("2026-06-10T10:00:00Z"), "expired_meter", "Jl. Kuningan, Jakarta", "B1234XYZ", 100000,
			snap(RuleV1ID, 1, "expired_meter", 50000, 1.0, "DAY", 2.0, 2, 100000, "2026-06-10T10:00:00Z")},
	}
	for _, v := range violations {
		snapJSON, _ := json.Marshal(v.snap)
		if _, err := conn.Exec(ctx, `
			INSERT INTO violations
				(id, member_id, rule_version_id, license_plate, violation_type,
				 location, violation_timestamp, photo_url, fine_amount,
				 calculation_snapshot, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,$11)
			ON CONFLICT (id) DO NOTHING
		`, v.id, MemberID, RuleV1ID, v.plate, v.vtype, v.loc, v.ts,
			fmt.Sprintf("/uploads/violations/seed-%s.jpg", v.id.String()[:8]),
			v.fine, string(snapJSON), now); err != nil {
			return fmt.Errorf("insert violation %s: %w", v.id, err)
		}
	}

	// ---- invoices ----
	invoices := []struct {
		id     uuid.UUID
		vid    uuid.UUID
		amount float64
		status string
	}{
		{I1, V1, 150000, "PAID"},
		{I2, V2, 750000, "PENDING"},
		{I3, V3, 500000, "FAILED"},
		{I4, V4, 100000, "PENDING"},
	}
	for _, inv := range invoices {
		if _, err := conn.Exec(ctx, `
			INSERT INTO invoices (id, violation_id, amount, status, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$5)
			ON CONFLICT (id) DO NOTHING
		`, inv.id, inv.vid, inv.amount, inv.status, now); err != nil {
			return fmt.Errorf("insert invoice %s: %w", inv.id, err)
		}
	}

	// ---- payments (V1 paid success, V3 failed) ----
	payments := []struct {
		id   uuid.UUID
		iid  uuid.UUID
		amt  float64
		tx   string
		stat string
		sc   string
	}{
		{P1, I1, 150000, "trx_seed_001", "PAID", "success"},
		{P3, I3, 500000, "trx_seed_003", "FAILED", "failed"},
	}
	for _, p := range payments {
		if _, err := conn.Exec(ctx, `
			INSERT INTO payments
				(id, invoice_id, amount, transaction_id, status, scenario, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$7)
			ON CONFLICT (id) DO NOTHING
		`, p.id, p.iid, p.amt, p.tx, p.stat, p.sc, now); err != nil {
			return fmt.Errorf("insert payment %s: %w", p.id, err)
		}
	}
	return nil
}

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		log.Fatalf("bad time %q: %v", s, err)
	}
	return t.UTC()
}

func snap(rvID uuid.UUID, ver int, vtype string, base, timeMult float64, window string,
	repMult float64, priorUnpaid int, calcFine float64, calcAt string) map[string]any {
	return map[string]any{
		"rule_version_id":     rvID,
		"rule_version_number": ver,
		"violation_type":      vtype,
		"base_amount":         base,
		"time_multiplier":     timeMult,
		"time_window":         window,
		"repeat_multiplier":   repMult,
		"prior_unpaid_count":  priorUnpaid,
		"calculated_fine":     calcFine,
		"calculated_at":       calcAt,
	}
}
