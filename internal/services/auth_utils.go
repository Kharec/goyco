package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/mail"
	"strings"

	"goyco/internal/database"
)

const (
	defaultTokenExpirationHours  = 24
	verificationTokenBytes       = 32
	deletionTokenExpirationHours = 24
)

func normalizeEmail(email string) (string, error) {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return "", fmt.Errorf("email is required")
	}

	parsed, err := mail.ParseAddress(trimmed)
	if err != nil {
		return "", ErrInvalidEmail
	}

	return strings.ToLower(parsed.Address), nil
}

func generateVerificationToken() (string, string, error) {
	buf := make([]byte, verificationTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate verification token: %w", err)
	}

	token := hex.EncodeToString(buf)
	hashed := HashVerificationToken(token)
	return token, hashed, nil
}

func HashVerificationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func sanitizeUser(user *database.User) *database.User {
	if user == nil {
		return nil
	}

	copy := *user
	copy.Password = ""
	copy.EmailVerificationToken = ""
	return &copy
}
