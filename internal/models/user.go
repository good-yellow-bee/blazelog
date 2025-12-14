package models

import (
	"time"
)

// Role represents a user's permission level.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// User represents a system user with RBAC.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Never expose in JSON
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewUser creates a new User with initialized timestamps.
func NewUser(username, email string, role Role) *User {
	now := time.Now()
	return &User{
		Username:  username,
		Email:     email,
		Role:      role,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsAdmin returns true if user has admin role.
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// CanWrite returns true if user can modify resources.
func (u *User) CanWrite() bool {
	return u.Role == RoleAdmin || u.Role == RoleOperator
}

// ParseRole converts a string to Role.
func ParseRole(s string) Role {
	switch s {
	case "admin":
		return RoleAdmin
	case "operator":
		return RoleOperator
	default:
		return RoleViewer
	}
}
