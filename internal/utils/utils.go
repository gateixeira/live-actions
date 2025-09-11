package utils

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

// Contains checks if a string is present in a slice
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GenerateCSRFToken generates a random token for CSRF protection
func GenerateCSRFToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// CookieName is the name of the CSRF cookie
const CookieName = "csrf_token"

// HeaderName is the name of the CSRF header
const HeaderName = "X-CSRF-Token"

// FormatDuration converts seconds to a human-readable duration string
func FormatDuration(seconds float64) string {
	if seconds == 0 {
		return "0s"
	}
	duration := time.Duration(seconds * float64(time.Second))
	return duration.String()
}
