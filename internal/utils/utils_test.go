package utils

import (
	"testing"
)

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{
			name:     "item exists in slice",
			slice:    []string{"self-hosted", "linux", "x64"},
			item:     "linux",
			expected: true,
		},
		{
			name:     "item does not exist in slice",
			slice:    []string{"self-hosted", "linux", "x64"},
			item:     "windows",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			item:     "test",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Contains(tt.slice, tt.item)
			if result != tt.expected {
				t.Errorf("Contains() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGenerateCSRFToken(t *testing.T) {
	// Test token generation and uniqueness
	token1, err1 := GenerateCSRFToken()
	if err1 != nil {
		t.Errorf("GenerateCSRFToken() error = %v", err1)
		return
	}

	token2, err2 := GenerateCSRFToken()
	if err2 != nil {
		t.Errorf("GenerateCSRFToken() error = %v", err2)
		return
	}

	// Check that tokens are not empty
	if token1 == "" {
		t.Error("GenerateCSRFToken() returned empty token")
	}

	if token2 == "" {
		t.Error("GenerateCSRFToken() returned empty token")
	}

	// Check that two consecutive tokens are different
	if token1 == token2 {
		t.Error("GenerateCSRFToken() returned identical tokens")
	}
}
