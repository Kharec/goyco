package services

import (
	"errors"

	"goyco/internal/database"
)

var (
	ErrInvalidCredentials       = errors.New("invalid credentials")
	ErrInvalidToken             = errors.New("invalid or expired token")
	ErrUsernameTaken            = errors.New("username already exists")
	ErrEmailTaken               = errors.New("email already exists")
	ErrInvalidEmail             = errors.New("invalid email address")
	ErrPasswordTooShort         = errors.New("password too short")
	ErrEmailNotVerified         = errors.New("email not verified")
	ErrAccountLocked            = errors.New("account is locked")
	ErrInvalidVerificationToken = errors.New("invalid verification token")
	ErrEmailSenderUnavailable   = errors.New("email sender not configured")
	ErrDeletionEmailFailed      = errors.New("account deletion email failed")
	ErrInvalidDeletionToken     = errors.New("invalid account deletion token")
	ErrUserNotFound             = errors.New("user not found")
	ErrDeletionRequestNotFound  = errors.New("deletion request not found")
)

type AuthResult struct {
	AccessToken  string         `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	RefreshToken string         `json:"refresh_token" example:"a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0"`
	User         *database.User `json:"user"`
}

type RegistrationResult struct {
	User             *database.User `json:"user"`
	VerificationSent bool           `json:"verification_sent"`
}
