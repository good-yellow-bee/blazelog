// Package users provides user management API endpoints.
package users

import (
	"regexp"
	"strings"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

var (
	usernameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{2,31}$`)
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

// ValidationError contains validation error details.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ValidateUsername validates a username.
func ValidateUsername(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return &ValidationError{Field: "username", Message: "username is required"}
	}
	if len(username) < 3 {
		return &ValidationError{Field: "username", Message: "username must be at least 3 characters"}
	}
	if len(username) > 32 {
		return &ValidationError{Field: "username", Message: "username must be at most 32 characters"}
	}
	if !usernameRegex.MatchString(username) {
		return &ValidationError{Field: "username", Message: "username must start with a letter and contain only letters, numbers, underscores, or hyphens"}
	}
	return nil
}

// ValidateEmail validates an email address.
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return &ValidationError{Field: "email", Message: "email is required"}
	}
	if len(email) > 255 {
		return &ValidationError{Field: "email", Message: "email must be at most 255 characters"}
	}
	if !emailRegex.MatchString(email) {
		return &ValidationError{Field: "email", Message: "invalid email format"}
	}
	return nil
}

// ValidateRole validates a role string.
func ValidateRole(role string) (models.Role, error) {
	role = strings.TrimSpace(strings.ToLower(role))
	switch role {
	case "admin":
		return models.RoleAdmin, nil
	case "operator":
		return models.RoleOperator, nil
	case "viewer":
		return models.RoleViewer, nil
	default:
		return "", &ValidationError{Field: "role", Message: "role must be one of: admin, operator, viewer"}
	}
}
