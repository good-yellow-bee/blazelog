package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/storage"
)

// TokenService handles refresh token operations.
type TokenService struct {
	storage storage.Storage
	ttl     time.Duration
}

// NewTokenService creates a new token service.
func NewTokenService(store storage.Storage, ttl time.Duration) *TokenService {
	return &TokenService{
		storage: store,
		ttl:     ttl,
	}
}

// CreateRefreshToken creates and stores a new refresh token for the user.
// Returns the plaintext token to send to the client.
func (s *TokenService) CreateRefreshToken(ctx context.Context, userID string) (string, error) {
	token, plainToken, err := models.NewRefreshToken(userID, s.ttl)
	if err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}

	if err := s.storage.Tokens().Create(ctx, token); err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}

	return plainToken, nil
}

// ValidateRefreshToken validates a refresh token and returns the associated user.
func (s *TokenService) ValidateRefreshToken(ctx context.Context, plainToken string) (*models.User, error) {
	tokenHash := models.HashToken(plainToken)

	token, err := s.storage.Tokens().GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("lookup refresh token: %w", err)
	}
	if token == nil {
		return nil, fmt.Errorf("token not found")
	}

	if !token.IsValid() {
		return nil, fmt.Errorf("token expired or revoked")
	}

	// Get user
	user, err := s.storage.Users().GetByID(ctx, token.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// RevokeRefreshToken revokes a refresh token.
func (s *TokenService) RevokeRefreshToken(ctx context.Context, plainToken string) error {
	tokenHash := models.HashToken(plainToken)
	return s.storage.Tokens().RevokeByTokenHash(ctx, tokenHash)
}

// RevokeAllUserTokens revokes all refresh tokens for a user.
func (s *TokenService) RevokeAllUserTokens(ctx context.Context, userID string) error {
	return s.storage.Tokens().RevokeAllForUser(ctx, userID)
}

// RotateRefreshToken revokes the old token and creates a new one.
// Returns the new plaintext token.
func (s *TokenService) RotateRefreshToken(ctx context.Context, oldPlainToken string, userID string) (string, error) {
	// Revoke old token
	if err := s.RevokeRefreshToken(ctx, oldPlainToken); err != nil {
		// Log but don't fail - we still want to issue new token
	}

	// Create new token
	return s.CreateRefreshToken(ctx, userID)
}

// CleanupExpiredTokens removes expired tokens from storage.
func (s *TokenService) CleanupExpiredTokens(ctx context.Context) (int64, error) {
	return s.storage.Tokens().DeleteExpired(ctx)
}

// TTL returns the refresh token time-to-live.
func (s *TokenService) TTL() time.Duration {
	return s.ttl
}
