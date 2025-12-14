package api

import "net/http"

// Error represents an API error response.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *Error) Error() string {
	return e.Message
}

// Common error codes
const (
	ErrCodeUnauthorized     = "UNAUTHORIZED"
	ErrCodeForbidden        = "FORBIDDEN"
	ErrCodeNotFound         = "NOT_FOUND"
	ErrCodeBadRequest       = "BAD_REQUEST"
	ErrCodeConflict         = "CONFLICT"
	ErrCodeInternalError    = "INTERNAL_ERROR"
	ErrCodeRateLimited      = "RATE_LIMITED"
	ErrCodeAccountLocked    = "ACCOUNT_LOCKED"
	ErrCodeValidationFailed = "VALIDATION_FAILED"
)

// Standard errors
var (
	ErrUnauthorized = &Error{
		Code:    ErrCodeUnauthorized,
		Message: "Invalid credentials",
		Status:  http.StatusUnauthorized,
	}

	ErrInvalidToken = &Error{
		Code:    ErrCodeUnauthorized,
		Message: "Invalid or expired token",
		Status:  http.StatusUnauthorized,
	}

	ErrForbidden = &Error{
		Code:    ErrCodeForbidden,
		Message: "Access denied",
		Status:  http.StatusForbidden,
	}

	ErrNotFound = &Error{
		Code:    ErrCodeNotFound,
		Message: "Resource not found",
		Status:  http.StatusNotFound,
	}

	ErrUserNotFound = &Error{
		Code:    ErrCodeNotFound,
		Message: "User not found",
		Status:  http.StatusNotFound,
	}

	ErrInternalServer = &Error{
		Code:    ErrCodeInternalError,
		Message: "Internal server error",
		Status:  http.StatusInternalServerError,
	}

	ErrRateLimited = &Error{
		Code:    ErrCodeRateLimited,
		Message: "Too many requests",
		Status:  http.StatusTooManyRequests,
	}

	ErrAccountLocked = &Error{
		Code:    ErrCodeAccountLocked,
		Message: "Account temporarily locked due to too many failed attempts",
		Status:  http.StatusTooManyRequests,
	}
)

// NewBadRequest creates a bad request error with custom message.
func NewBadRequest(message string) *Error {
	return &Error{
		Code:    ErrCodeBadRequest,
		Message: message,
		Status:  http.StatusBadRequest,
	}
}

// NewValidationError creates a validation error with custom message.
func NewValidationError(message string) *Error {
	return &Error{
		Code:    ErrCodeValidationFailed,
		Message: message,
		Status:  http.StatusBadRequest,
	}
}

// NewConflict creates a conflict error with custom message.
func NewConflict(message string) *Error {
	return &Error{
		Code:    ErrCodeConflict,
		Message: message,
		Status:  http.StatusConflict,
	}
}

// NewNotFound creates a not found error with custom message.
func NewNotFound(message string) *Error {
	return &Error{
		Code:    ErrCodeNotFound,
		Message: message,
		Status:  http.StatusNotFound,
	}
}
