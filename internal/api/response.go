package api

import (
	"encoding/json"
	"net/http"
)

// Response is a standard API response wrapper.
type Response struct {
	Data  any    `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
}

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := Response{Data: data}
	json.NewEncoder(w).Encode(resp)
}

// JSONError writes a JSON error response.
func JSONError(w http.ResponseWriter, err *Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Status)

	resp := Response{Error: err}
	json.NewEncoder(w).Encode(resp)
}

// Created writes a 201 Created response.
func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, data)
}

// OK writes a 200 OK response.
func OK(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, data)
}

// NoContent writes a 204 No Content response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// LoginResponse is returned on successful login.
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds
	TokenType    string `json:"token_type"`
}

// UserResponse is a user without sensitive fields.
type UserResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// PaginatedResponse wraps a list with pagination info.
type PaginatedResponse struct {
	Items      any   `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	TotalPages int   `json:"total_pages"`
}
