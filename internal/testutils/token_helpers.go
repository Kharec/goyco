package testutils

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"goyco/internal/config"
	"goyco/internal/database"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type TokenClaims struct {
	UserID         uint   `json:"sub"`
	Username       string `json:"username"`
	SessionVersion uint   `json:"session_version"`
	TokenType      string `json:"type"`
	KeyID          string `json:"kid,omitempty"`
	jwt.RegisteredClaims
}

type TokenOption func(*TokenClaims, *config.JWTConfig) string

func GenerateTestToken(t *testing.T, user *database.User, cfg *config.JWTConfig, opts ...TokenOption) string {
	t.Helper()

	claims := TokenClaims{
		UserID:         user.ID,
		Username:       user.Username,
		SessionVersion: user.SessionVersion,
		TokenType:      TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.Issuer,
			Audience:  []string{cfg.Audience},
			Subject:   fmt.Sprint(user.ID),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}

	secret := cfg.Secret
	for _, opt := range opts {
		if opt != nil {
			secret = opt(&claims, cfg)
			if secret == "" {
				secret = cfg.Secret
			}
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	return tokenString
}

func WithExpiredToken(claims *TokenClaims, cfg *config.JWTConfig) string {
	claims.IssuedAt = jwt.NewNumericDate(time.Now().Add(-2 * time.Hour))
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-1 * time.Hour))
	return cfg.Secret
}

func WithTamperedSecret(claims *TokenClaims, cfg *config.JWTConfig) string {
	return cfg.Secret + "-tampered"
}

func WithWrongIssuer(claims *TokenClaims, cfg *config.JWTConfig) string {
	claims.Issuer = "wrong-issuer"
	return cfg.Secret
}

func WithWrongAudience(claims *TokenClaims, cfg *config.JWTConfig) string {
	claims.Audience = []string{"wrong-audience"}
	return cfg.Secret
}
