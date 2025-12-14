// Package auth provides authentication and authorization functionality.
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// Claims represents the JWT claims for access tokens.
type Claims struct {
	jwt.RegisteredClaims
	UserID   string      `json:"uid"`
	Username string      `json:"usr"`
	Role     models.Role `json:"role"`
}

// JWTService handles JWT token generation and validation.
type JWTService struct {
	secret []byte
	ttl    time.Duration
	issuer string
}

// NewJWTService creates a new JWT service.
func NewJWTService(secret []byte, ttl time.Duration) *JWTService {
	return &JWTService{
		secret: secret,
		ttl:    ttl,
		issuer: "blazelog",
	}
}

// GenerateToken creates a new JWT access token for the given user.
func (s *JWTService) GenerateToken(user *models.User) (string, error) {
	now := time.Now()
	expiresAt := now.Add(s.ttl)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ValidateToken validates a JWT token and returns the claims.
func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Verify issuer
	if claims.Issuer != s.issuer {
		return nil, fmt.Errorf("invalid issuer")
	}

	return claims, nil
}

// TTL returns the token time-to-live duration.
func (s *JWTService) TTL() time.Duration {
	return s.ttl
}

// TTLSeconds returns the token TTL in seconds.
func (s *JWTService) TTLSeconds() int {
	return int(s.ttl.Seconds())
}
