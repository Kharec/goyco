package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"goyco/internal/config"
	"goyco/internal/repositories"
)

func TrimString(s string) string {
	return strings.TrimSpace(s)
}

const (
	DefaultBcryptCost = 10
)

func HashPassword(password string, cost int) (string, error) {
	if cost <= 0 {
		cost = DefaultBcryptCost
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hashedPassword), nil
}

func IsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func HandleUniqueConstraintError(err error) error {
	if err == nil {
		return nil
	}

	var pqErr *pq.Error
	if !errors.As(err, &pqErr) || pqErr.Code != "23505" {
		return err
	}

	constraintLower := strings.ToLower(pqErr.Constraint)
	errMsgLower := strings.ToLower(pqErr.Message)

	if strings.Contains(constraintLower, "username") ||
		strings.Contains(errMsgLower, "username") ||
		strings.Contains(errMsgLower, "users_username_key") ||
		strings.Contains(errMsgLower, "users.username") {
		return ErrUsernameTaken
	}

	if strings.Contains(constraintLower, "email") ||
		strings.Contains(errMsgLower, "email") ||
		strings.Contains(errMsgLower, "users_email_key") ||
		strings.Contains(errMsgLower, "users.email") {
		return ErrEmailTaken
	}

	return ErrUsernameTaken
}

func HandleUniqueConstraintErrorWithMessage(err error) error {
	if err == nil {
		return nil
	}

	if handled := HandleUniqueConstraintError(err); handled != err {
		return handled
	}

	errMsg := err.Error()
	errMsgLower := strings.ToLower(errMsg)

	isUniqueError := strings.Contains(errMsgLower, "duplicate key") ||
		strings.Contains(errMsgLower, "unique constraint") ||
		strings.Contains(errMsgLower, "violates unique constraint") ||
		strings.Contains(errMsgLower, "unique constraint failed") ||
		strings.Contains(errMsgLower, "constraint failed") ||
		(strings.Contains(errMsgLower, "constraint") && strings.Contains(errMsgLower, "unique"))

	if !isUniqueError {
		return err
	}

	if strings.Contains(errMsgLower, "username") ||
		strings.Contains(errMsgLower, "users_username_key") ||
		strings.Contains(errMsgLower, "users.username") ||
		strings.Contains(errMsg, "username") ||
		strings.Contains(errMsg, "users_username_key") ||
		strings.Contains(errMsg, "users.username") {
		return ErrUsernameTaken
	}

	if strings.Contains(errMsgLower, "email") ||
		strings.Contains(errMsgLower, "users_email_key") ||
		strings.Contains(errMsgLower, "users.email") ||
		strings.Contains(errMsg, "email") ||
		strings.Contains(errMsg, "users_email_key") ||
		strings.Contains(errMsg, "users.email") {
		return ErrEmailTaken
	}

	return ErrUsernameTaken
}

func NewAuthFacadeForTest(cfg *config.Config, userRepo repositories.UserRepository, postRepo repositories.PostRepository, deletionRepo repositories.AccountDeletionRepository, refreshRepo repositories.RefreshTokenRepositoryInterface, emailSender EmailSender) (*AuthFacade, error) {
	emailService, err := NewEmailService(cfg, emailSender)
	if err != nil {
		return nil, fmt.Errorf("create email service: %w", err)
	}

	jwtService := NewJWTService(&cfg.JWT, userRepo, refreshRepo)

	registrationService := NewRegistrationService(userRepo, emailService, cfg)
	passwordResetService := NewPasswordResetService(userRepo, emailService)
	deletionService := NewAccountDeletionService(userRepo, postRepo, deletionRepo, emailService)
	sessionService := NewSessionService(jwtService, userRepo)
	userManagementService := NewUserManagementService(userRepo, postRepo, emailService)

	authFacade := NewAuthFacade(
		registrationService,
		passwordResetService,
		deletionService,
		sessionService,
		userManagementService,
		cfg,
	)

	return authFacade, nil
}
