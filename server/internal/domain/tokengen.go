package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const TokenPrefix = "memorix_"

// GenerateToken creates a cryptographically random API token
// in the format "memorix_" + 32 hex characters.
func GenerateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return TokenPrefix + hex.EncodeToString(b), nil
}
