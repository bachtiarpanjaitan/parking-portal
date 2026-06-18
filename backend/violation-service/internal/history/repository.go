package history

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/parking-portal/backend/pkg/errs"
)

// Filter holds the list query options.
type Filter struct {
	MemberID *uuid.UUID
	From     *string // ISO 8601
	To       *string
	Page     int
	PageSize int
}

// Repository is the persistence interface for the history view.
type Repository interface {
	List(ctx context.Context, f Filter) ([]Entry, int, error)
}

type pgRepo struct{ db *pgxpool.Pool }

func NewPGRepository(db *pgxpool.Pool) Repository { return &pgRepo{db: db} }

// baseSelectFragment is the immutable prefix of the SELECT.
//
// NOTE: After migration 0011 the `scenario` column on `payments` was removed
// in favor of Midtrans columns. We now select `midtrans_transaction_status`
// (capture/settlement/pending/deny/cancel/expire/refund) as the closest
// equivalent of the old "scenario" field.
const baseSelectFragment = `
		SELECT v.id, v.member_id, v.license_plate, v.violation_type,
		       v.location, v.violation_timestamp, v.fine_amount, v.photo_url,
		       v.rule_version_id, frv.version_number,
		       COALESCE(i.id::text, ''), COALESCE(i.status, ''), COALESCE(i.amount, 0),
		       COALESCE(p.status, ''), COALESCE(p.midtrans_transaction_status, ''),
		       v.calculation_snapshot
		FROM violations v
		JOIN fine_rule_versions frv ON frv.id = v.rule_version_id
		LEFT JOIN invoices i ON i.violation_id = v.id
		LEFT JOIN LATERAL (
			SELECT status, midtrans_transaction_status FROM payments
			WHERE invoice_id = i.id
			ORDER BY created_at DESC LIMIT 1
		) p ON true
		WHERE 1=1`

// buildWhere appends the dynamic WHERE clauses for member_id / from / to.
// It returns the SQL fragment (starting with " AND ...") and the args slice
// matching the order of $N placeholders.
func buildWhere(f Filter) (string, []any) {
	var sb strings.Builder
	var args []any
	if f.MemberID != nil {
		args = append(args, *f.MemberID)
		sb.WriteString(fmt.Sprintf(" AND v.member_id = $%d", len(args)))
	}
	if f.From != nil {
		args = append(args, *f.From)
		sb.WriteString(fmt.Sprintf(" AND v.violation_timestamp >= $%d::timestamptz", len(args)))
	}
	if f.To != nil {
		args = append(args, *f.To)
		sb.WriteString(fmt.Sprintf(" AND v.violation_timestamp <  $%d::timestamptz", len(args)))
	}
	return sb.String(), args
}

// List runs the join query for the history view. Sorted by violation_timestamp desc.
func (r *pgRepo) List(ctx context.Context, f Filter) ([]Entry, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 20
	}

	whereSQL, whereArgs := buildWhere(f)
	listSQL := baseSelectFragment + whereSQL +
		" ORDER BY v.violation_timestamp DESC" +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(whereArgs)+1, len(whereArgs)+2)
	listArgs := append([]any{}, whereArgs...)
	listArgs = append(listArgs, f.PageSize, (f.Page-1)*f.PageSize)

	rows, err := r.db.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, "list history", err)
	}
	defer rows.Close()
	out := make([]Entry, 0, f.PageSize)
	for rows.Next() {
		var e Entry
		var snapRaw []byte
		if err := rows.Scan(
			&e.ViolationID, &e.MemberID, &e.LicensePlate, &e.ViolationType,
			&e.Location, &e.ViolationTS, &e.FineAmount, &e.PhotoURL,
			&e.RuleVersionID, &e.RuleVersionNumber,
			&e.InvoiceID, &e.InvoiceStatus, &e.InvoiceAmount,
			&e.PaymentStatus, &e.PaymentTxStatus,
			&snapRaw,
		); err != nil {
			return nil, 0, errs.Wrap(errs.CodeInternal, "scan history row", err)
		}
		_ = json.Unmarshal(snapRaw, &e.CalculationSnapshot)
		out = append(out, e)
	}

	countSQL := "SELECT count(*) FROM violations v " +
		"JOIN fine_rule_versions frv ON frv.id = v.rule_version_id " +
		"LEFT JOIN invoices i ON i.violation_id = v.id WHERE 1=1" + whereSQL
	var total int
	if err := r.db.QueryRow(ctx, countSQL, whereArgs...).Scan(&total); err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, "count history", err)
	}
	return out, total, nil
}
