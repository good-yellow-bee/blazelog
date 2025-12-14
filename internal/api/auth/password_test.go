package auth

import (
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantOK   bool
	}{
		// Valid passwords
		{"valid complex", "MyP@ssw0rd123!", true},
		{"valid minimal", "Abcdefgh123!", true},
		{"valid special chars", "Test123!@#$%^", true},

		// Too short
		{"too short", "Ab1!", false},
		{"exactly 11", "Abcdefgh12!", false},

		// Missing uppercase
		{"no uppercase", "abcdefgh123!", false},

		// Missing lowercase
		{"no lowercase", "ABCDEFGH123!", false},

		// Missing digit
		{"no digit", "Abcdefghijk!", false},

		// Missing special
		{"no special", "Abcdefgh1234", false},

		// Edge cases
		{"empty", "", false},
		{"spaces only", "            ", false},
		{"unicode", "Abcdefgh123!\u00E9", true}, // unicode letter counts as lowercase
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePassword(tc.password)
			got := err == nil
			if got != tc.wantOK {
				t.Errorf("ValidatePassword(%q) error=%v, want valid=%v", tc.password, err, tc.wantOK)
			}
		})
	}
}

func TestValidatePasswordOrError(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"valid", "MyP@ssw0rd123!", false},
		{"too short", "Ab1!", true},
		{"no uppercase", "abcdefgh123!", true},
		{"no lowercase", "ABCDEFGH123!", true},
		{"no digit", "Abcdefghijk!", true},
		{"no special", "Abcdefgh1234", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePasswordOrError(tc.password)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidatePasswordOrError(%q) error = %v, wantErr %v", tc.password, err, tc.wantErr)
			}
		})
	}
}

func TestValidatePasswordOrError_Messages(t *testing.T) {
	tests := []struct {
		password    string
		wantContain string
	}{
		{"short", "at least 12"},
		{"abcdefgh123!", "uppercase"},
		{"ABCDEFGH123!", "lowercase"},
		{"Abcdefghijk!", "digit"},
		{"Abcdefgh1234", "special"},
	}

	for _, tc := range tests {
		t.Run(tc.wantContain, func(t *testing.T) {
			err := ValidatePasswordOrError(tc.password)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantContain) {
				t.Errorf("error %q should contain %q", err.Error(), tc.wantContain)
			}
		})
	}
}
