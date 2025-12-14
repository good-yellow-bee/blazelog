package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"
)

// RefreshToken represents a JWT refresh token stored in the database.
type RefreshToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"` // SHA-256 hash of the actual token
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	Revoked   bool      `json:"revoked"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// NewRefreshToken creates a new RefreshToken with generated token.
// Returns the token model and the plaintext token to send to the client.
func NewRefreshToken(userID string, ttl time.Duration) (*RefreshToken, string, error) {
	// Generate 32 random bytes
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, "", err
	}

	// Encode as base64url
	plainToken := base64.RawURLEncoding.EncodeToString(tokenBytes)

	// Hash for storage
	hash := sha256.Sum256([]byte(plainToken))
	tokenHash := base64.RawURLEncoding.EncodeToString(hash[:])

	now := time.Now()
	return &RefreshToken{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
		Revoked:   false,
	}, plainToken, nil
}

// HashToken creates a SHA-256 hash of a plaintext token for lookup.
func HashToken(plainToken string) string {
	hash := sha256.Sum256([]byte(plainToken))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// IsExpired returns true if the token has expired.
func (t *RefreshToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid returns true if the token is not revoked and not expired.
func (t *RefreshToken) IsValid() bool {
	return !t.Revoked && !t.IsExpired()
}
