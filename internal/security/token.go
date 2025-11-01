package security

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateSecureToken generates a cryptographically secure random token
func GenerateSecureToken(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("token length must be positive")
	}
	
	// Generate random bytes
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	
	// Encode to base64 and return
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateInternalTaskToken generates a token specifically for internal tasks
func GenerateInternalTaskToken() (string, error) {
	// Generate 32-byte token (256 bits) for strong security
	return GenerateSecureToken(32)
}