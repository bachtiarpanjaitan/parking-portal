package users

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/parking-portal/backend/pkg/errs"
)

// Repository is the persistence interface for users.
type Repository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByEmailWithPassword(ctx context.Context, email string) (*UserWithPassword, error)
	List(ctx context.Context, q string, limit, offset int) ([]User, int, error)
}

type pgRepo struct{ db *pgxpool.Pool }

func NewPGRepository(db *pgxpool.Pool) Repository { return &pgRepo{db: db} }

// findByEmailBase is the internal helper that includes the password hash.
// Public methods project to User (without the hash) or return UserWithPassword.
func (r *pgRepo) findByEmailBase(ctx context.Context, email string, withPassword bool) (any, error) {
	q := `SELECT id, name, email, role, created_at, updated_at,
		COALESCE(password_hash, '') FROM users WHERE email = $1`
	var u UserWithPassword
	err := r.db.QueryRow(ctx, q, email).Scan(
		&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt, &u.PasswordHash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.CodeNotFound, "user not found")
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "find user by email", err)
	}
	if withPassword {
		return &u, nil
	}
	return &u.User, nil
}

func (r *pgRepo) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	const q = `SELECT id, name, email, role, created_at, updated_at
		FROM users WHERE id = $1`
	var u User
	err := r.db.QueryRow(ctx, q, id).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.CodeNotFound, "user not found")
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "find user", err)
	}
	return &u, nil
}

func (r *pgRepo) FindByEmail(ctx context.Context, email string) (*User, error) {
	v, err := r.findByEmailBase(ctx, email, false)
	if err != nil {
		return nil, err
	}
	return v.(*User), nil
}

func (r *pgRepo) FindByEmailWithPassword(ctx context.Context, email string) (*UserWithPassword, error) {
	v, err := r.findByEmailBase(ctx, email, true)
	if err != nil {
		return nil, err
	}
	return v.(*UserWithPassword), nil
}

func (r *pgRepo) List(ctx context.Context, q string, limit, offset int) ([]User, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const sqlList = `
		SELECT id, name, email, role, created_at, updated_at
		FROM users
		WHERE role = 'MEMBER'
		  AND ($1 = '' OR name ILIKE '%' || $1 || '%' OR email ILIKE '%' || $1 || '%')
		ORDER BY name
		LIMIT $2 OFFSET $3`
	const sqlCount = `
		SELECT count(*)
		FROM users
		WHERE role = 'MEMBER'
		  AND ($1 = '' OR name ILIKE '%' || $1 || '%' OR email ILIKE '%' || $1 || '%')`

	rows, err := r.db.Query(ctx, sqlList, q, limit, offset)
	if err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, "list users", err)
	}
	defer rows.Close()
	items := make([]User, 0, limit)
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, errs.Wrap(errs.CodeInternal, "scan user", err)
		}
		items = append(items, u)
	}
	var total int
	if err := r.db.QueryRow(ctx, sqlCount, q).Scan(&total); err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, "count users", err)
	}
	return items, total, nil
}
