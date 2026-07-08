package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func GenerateVerificationToken() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate email verification token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}
