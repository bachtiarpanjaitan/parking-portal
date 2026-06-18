// Package auth handles password-based login. Replaces the email-only mock
// (the old ADR-006 was revised to ADR-006-R1: password + bcrypt).
// The handler is mounted on the gateway in production, but is also available
// on the violation-service for local development.
package auth

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/parking-portal/backend/pkg/auth"
	"github.com/parking-portal/backend/pkg/errs"
	"github.com/parking-portal/backend/violation-service/internal/users"
)

// LoginRequest is the POST /auth/login body.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=1"`
}

// LoginResponse is the successful response. The token is a signed JWT.
type LoginResponse struct {
	Token string     `json:"token"`
	User  users.User `json:"user"`
}

// Service validates credentials and issues a JWT.
type Service struct {
	users  users.Repository
	signer *auth.Signer
}

func NewService(u users.Repository, s *auth.Signer) *Service {
	return &Service{users: u, signer: s}
}

// Login looks up the user by email, compares the bcrypt password, and
// returns a signed JWT on success. All failure cases collapse to
// UNAUTHORIZED to avoid leaking whether the email exists.
func (s *Service) Login(email, password string) (LoginResponse, error) {
	u, err := s.users.FindByEmailWithPassword(nilCtx(), email)
	if err != nil {
		// If not found, return UNAUTHORIZED (don't leak that the email is missing).
		if ae, ok := errs.AsAppError(err); ok && ae.ErrCode == errs.CodeNotFound {
			return LoginResponse{}, errs.New(errs.CodeUnauthorized, "invalid email or password")
		}
		return LoginResponse{}, err
	}
	if u.PasswordHash == "" {
		// User exists but has no password set (shouldn't happen after seeding).
		return LoginResponse{}, errs.New(errs.CodeUnauthorized, "invalid email or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return LoginResponse{}, errs.New(errs.CodeUnauthorized, "invalid email or password")
	}
	tok, err := s.signer.Sign(u.ID, auth.Role(u.Role))
	if err != nil {
		return LoginResponse{}, errs.Wrap(errs.CodeInternal, "sign token", err)
	}
	return LoginResponse{Token: tok, User: u.User}, nil
}
