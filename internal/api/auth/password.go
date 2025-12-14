package auth

import (
	"errors"
	"strings"
	"unicode"
)

// PasswordValidationError contains details about password validation failure.
type PasswordValidationError struct {
	Messages []string
}

func (e *PasswordValidationError) Error() string {
	return strings.Join(e.Messages, "; ")
}

// ValidatePassword checks if a password meets complexity requirements.
// Requirements:
// - Minimum 12 characters
// - At least 1 uppercase letter (A-Z)
// - At least 1 lowercase letter (a-z)
// - At least 1 digit (0-9)
// - At least 1 special character
func ValidatePassword(password string) error {
	var messages []string

	if len(password) < 12 {
		messages = append(messages, "password must be at least 12 characters")
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasDigit   bool
		hasSpecial bool
	)

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case isSpecialChar(r):
			hasSpecial = true
		}
	}

	if !hasUpper {
		messages = append(messages, "password must contain at least 1 uppercase letter")
	}
	if !hasLower {
		messages = append(messages, "password must contain at least 1 lowercase letter")
	}
	if !hasDigit {
		messages = append(messages, "password must contain at least 1 digit")
	}
	if !hasSpecial {
		messages = append(messages, "password must contain at least 1 special character (!@#$%^&*...)")
	}

	if len(messages) > 0 {
		return &PasswordValidationError{Messages: messages}
	}

	return nil
}

// isSpecialChar returns true if the rune is a special character.
func isSpecialChar(r rune) bool {
	specials := "!@#$%^&*()-_=+[]{}|;:',.<>?/`~\"\\"
	return strings.ContainsRune(specials, r)
}

// ValidatePasswordOrError returns an error suitable for API responses.
func ValidatePasswordOrError(password string) error {
	if err := ValidatePassword(password); err != nil {
		var validErr *PasswordValidationError
		if errors.As(err, &validErr) {
			return errors.New(validErr.Messages[0]) // Return first message for API
		}
		return err
	}
	return nil
}
