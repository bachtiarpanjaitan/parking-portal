// Package users holds user records (officer + member).
// See .ai/DATABASE_MAPPING.md → users and .ai/MODELS.md.
package users

import (
	"time"

	"github.com/google/uuid"
)

// Role is the user role. Mirrors auth.Role.
type Role string

const (
	RoleOfficer Role = "OFFICER"
	RoleMember  Role = "MEMBER"
)

// User is the domain entity. PasswordHash is NEVER sent over the API
// (it lives in a separate DTO used only for login).
type User struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserWithPassword is an internal type used only by the auth service
// after it loads the user for credential validation. The hash never leaves
// the server.
type UserWithPassword struct {
	User
	PasswordHash string
}
