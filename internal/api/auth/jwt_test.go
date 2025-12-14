package auth

import (
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

func TestJWTService_GenerateAndValidate(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	ttl := 15 * time.Minute
	svc := NewJWTService(secret, ttl)

	user := &models.User{
		ID:       "user-123",
		Username: "testuser",
		Role:     models.RoleAdmin,
	}

	// Generate token
	token, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// Validate token
	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.UserID != user.ID {
		t.Errorf("UserID = %q, want %q", claims.UserID, user.ID)
	}
	if claims.Username != user.Username {
		t.Errorf("Username = %q, want %q", claims.Username, user.Username)
	}
	if claims.Role != user.Role {
		t.Errorf("Role = %q, want %q", claims.Role, user.Role)
	}
}

func TestJWTService_InvalidToken(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	ttl := 15 * time.Minute
	svc := NewJWTService(secret, ttl)

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"garbage", "not-a-jwt-token"},
		{"wrong-segments", "a.b"},
		{"invalid-signature", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1aWQiOiJ0ZXN0In0.invalid"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.ValidateToken(tc.token)
			if err == nil {
				t.Error("expected error for invalid token")
			}
		})
	}
}

func TestJWTService_DifferentSecret(t *testing.T) {
	ttl := 15 * time.Minute
	svc1 := NewJWTService([]byte("secret-one-32-bytes-long!!!!!!!"), ttl)
	svc2 := NewJWTService([]byte("secret-two-32-bytes-long!!!!!!!"), ttl)

	user := &models.User{
		ID:       "user-123",
		Username: "testuser",
		Role:     models.RoleViewer,
	}

	token, err := svc1.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Token signed with svc1 should fail validation with svc2
	_, err = svc2.ValidateToken(token)
	if err == nil {
		t.Error("expected error validating token with different secret")
	}
}

func TestJWTService_ExpiredToken(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	ttl := 1 * time.Millisecond // Very short TTL
	svc := NewJWTService(secret, ttl)

	user := &models.User{
		ID:       "user-123",
		Username: "testuser",
		Role:     models.RoleOperator,
	}

	token, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	_, err = svc.ValidateToken(token)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestJWTService_TTLSeconds(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")
	ttl := 15 * time.Minute
	svc := NewJWTService(secret, ttl)

	got := svc.TTLSeconds()
	want := 900 // 15 * 60
	if got != want {
		t.Errorf("TTLSeconds() = %d, want %d", got, want)
	}
}
