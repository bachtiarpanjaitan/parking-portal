// Package auth provides JWT signing and verification using golang-jwt.
// The API Gateway signs tokens on login; every HTTP service verifies them.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Role is the user's role claim.
type Role string

const (
	RoleOfficer Role = "OFFICER"
	RoleMember  Role = "MEMBER"
)

// Claims is the JWT payload. sub = user id, role = OFFICER|MEMBER.
type Claims struct {
	UserID uuid.UUID `json:"sub"`
	Role   Role      `json:"role"`
	jwt.RegisteredClaims
}

// Signer issues JWTs.
type Signer struct {
	secret []byte
	expiry time.Duration
	issuer string
}

// NewSigner creates a Signer. secret must be >= 32 bytes (validated).
func NewSigner(secret string, expiryHours int, issuer string) (*Signer, error) {
	if len(secret) < 32 {
		return nil, errors.New("jwt secret must be at least 32 bytes")
	}
	if expiryHours <= 0 {
		expiryHours = 24
	}
	return &Signer{
		secret: []byte(secret),
		expiry: time.Duration(expiryHours) * time.Hour,
		issuer: issuer,
	}, nil
}

// Sign builds a JWT for the given user.
func (s *Signer) Sign(userID uuid.UUID, role Role) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.expiry)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.secret)
}

// Verify parses and validates a token string. Returns the claims on success.
func (s *Signer) Verify(tokenStr string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return c, nil
}
