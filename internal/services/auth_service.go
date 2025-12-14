package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"goyco/internal/config"
	"goyco/internal/database"
)

const (
	defaultTokenExpirationHours  = 24
	verificationTokenBytes       = 32
	deletionTokenExpirationHours = 24
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

type AuthFacade struct {
	registrationService   *RegistrationService
	passwordResetService  *PasswordResetService
	deletionService       *AccountDeletionService
	sessionService        *SessionService
	userManagementService *UserManagementService
	config                *config.Config
}

func NewAuthFacade(
	registrationService *RegistrationService,
	passwordResetService *PasswordResetService,
	deletionService *AccountDeletionService,
	sessionService *SessionService,
	userManagementService *UserManagementService,
	config *config.Config,
) *AuthFacade {
	return &AuthFacade{
		registrationService:   registrationService,
		passwordResetService:  passwordResetService,
		deletionService:       deletionService,
		sessionService:        sessionService,
		userManagementService: userManagementService,
		config:                config,
	}
}

func (f *AuthFacade) Register(username, email, password string) (*RegistrationResult, error) {
	return f.registrationService.Register(username, email, password)
}

func (f *AuthFacade) Login(username, password string) (*AuthResult, error) {
	return f.sessionService.Login(username, password)
}

func (f *AuthFacade) VerifyToken(tokenString string) (uint, error) {
	return f.sessionService.VerifyToken(tokenString)
}

func (f *AuthFacade) ConfirmEmail(token string) (*database.User, error) {
	return f.registrationService.ConfirmEmail(token)
}

func (f *AuthFacade) ResendVerificationEmail(email string) error {
	return f.registrationService.ResendVerificationEmail(email)
}

func (f *AuthFacade) RequestPasswordReset(usernameOrEmail string) error {
	return f.passwordResetService.RequestPasswordReset(usernameOrEmail)
}

func (f *AuthFacade) ResetPassword(token, newPassword string) error {
	user, err := f.passwordResetService.GetUserByResetToken(token)
	if err != nil {
		return err
	}

	if err := f.passwordResetService.ResetPassword(token, newPassword); err != nil {
		return err
	}

	if err := f.sessionService.InvalidateAllSessions(user.ID); err != nil {
		return fmt.Errorf("invalidate sessions: %w", err)
	}

	return nil
}

func (f *AuthFacade) UpdateUsername(userID uint, username string) (*database.User, error) {
	return f.userManagementService.UpdateUsername(userID, username)
}

func (f *AuthFacade) UpdateEmail(userID uint, email string) (*database.User, error) {
	return f.userManagementService.UpdateEmail(userID, email)
}

func (f *AuthFacade) UpdatePassword(userID uint, currentPassword, newPassword string) (*database.User, error) {
	user, err := f.userManagementService.UpdatePassword(userID, currentPassword, newPassword)
	if err != nil {
		return nil, err
	}
	if err := f.sessionService.InvalidateAllSessions(userID); err != nil {
		return nil, fmt.Errorf("invalidate sessions: %w", err)
	}
	return user, nil
}

func (f *AuthFacade) RequestAccountDeletion(userID uint) error {
	return f.deletionService.RequestAccountDeletion(userID)
}

func (f *AuthFacade) ConfirmAccountDeletion(token string) error {
	return f.deletionService.ConfirmAccountDeletion(token)
}

func (f *AuthFacade) ConfirmAccountDeletionWithPosts(token string, deletePosts bool) error {
	return f.deletionService.ConfirmAccountDeletionWithPosts(token, deletePosts)
}

func (f *AuthFacade) GetUserIDFromDeletionToken(token string) (uint, error) {
	return f.deletionService.GetUserIDFromDeletionToken(token)
}

func (f *AuthFacade) RefreshAccessToken(refreshToken string) (*AuthResult, error) {
	return f.sessionService.RefreshAccessToken(refreshToken)
}

func (f *AuthFacade) RevokeRefreshToken(refreshToken string) error {
	return f.sessionService.RevokeRefreshToken(refreshToken)
}

func (f *AuthFacade) RevokeAllUserTokens(userID uint) error {
	return f.sessionService.RevokeAllUserTokens(userID)
}

func (f *AuthFacade) InvalidateAllSessions(userID uint) error {
	return f.sessionService.InvalidateAllSessions(userID)
}

func (f *AuthFacade) CleanupExpiredTokens() error {
	return f.sessionService.CleanupExpiredTokens()
}

func (f *AuthFacade) GetAdminEmail() string {
	return f.config.App.AdminEmail
}

func (f *AuthFacade) UserHasPosts(userID uint) (bool, int64, error) {
	return f.userManagementService.UserHasPosts(userID)
}
