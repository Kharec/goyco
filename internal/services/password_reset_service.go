package services

import (
	"fmt"
	"time"

	"goyco/internal/database"
	"goyco/internal/repositories"
	"goyco/internal/validation"
)

type PasswordResetService struct {
	userRepo     repositories.UserRepository
	emailService *EmailService
}

func NewPasswordResetService(userRepo repositories.UserRepository, emailService *EmailService) *PasswordResetService {
	return &PasswordResetService{
		userRepo:     userRepo,
		emailService: emailService,
	}
}

func (s *PasswordResetService) RequestPasswordReset(usernameOrEmail string) error {
	trimmed := TrimString(usernameOrEmail)
	if trimmed == "" {
		return fmt.Errorf("username or email is required")
	}

	var user *database.User
	var err error

	normalized, emailErr := normalizeEmail(trimmed)
	if emailErr == nil {
		user, err = s.userRepo.GetByEmail(normalized)
		if err != nil && !IsRecordNotFound(err) {
			return fmt.Errorf("lookup user by email: %w", err)
		}
	}

	if user == nil {
		user, err = s.userRepo.GetByUsername(trimmed)
		if err != nil {
			if IsRecordNotFound(err) {
				return nil
			}
			return fmt.Errorf("lookup user by username: %w", err)
		}
	}

	token, hashed, err := generateVerificationToken()
	if err != nil {
		return err
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(defaultTokenExpirationHours) * time.Hour)

	user.PasswordResetToken = hashed
	user.PasswordResetSentAt = &now
	user.PasswordResetExpiresAt = &expiresAt

	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	if err := s.emailService.SendPasswordResetEmail(user, token); err != nil {
		user.PasswordResetToken = ""
		user.PasswordResetSentAt = nil
		user.PasswordResetExpiresAt = nil
		_ = s.userRepo.Update(user)
		return fmt.Errorf("send password reset email: %w", err)
	}

	return nil
}

func (s *PasswordResetService) GetUserByResetToken(token string) (*database.User, error) {
	trimmed := TrimString(token)
	if trimmed == "" {
		return nil, fmt.Errorf("reset token is required")
	}

	hashed := HashVerificationToken(trimmed)
	user, err := s.userRepo.GetByPasswordResetToken(hashed)
	if err != nil {
		if IsRecordNotFound(err) {
			return nil, fmt.Errorf("invalid or expired reset token")
		}
		return nil, fmt.Errorf("lookup reset token: %w", err)
	}

	if user.PasswordResetExpiresAt == nil || time.Now().After(*user.PasswordResetExpiresAt) {
		return nil, fmt.Errorf("invalid or expired reset token")
	}

	return user, nil
}

func (s *PasswordResetService) ResetPassword(token, newPassword string) error {
	if err := validation.ValidatePassword(newPassword); err != nil {
		return err
	}

	user, err := s.GetUserByResetToken(token)
	if err != nil {
		hashed := HashVerificationToken(TrimString(token))
		expiredUser, lookupErr := s.userRepo.GetByPasswordResetToken(hashed)
		if lookupErr == nil && expiredUser != nil {
			if expiredUser.PasswordResetExpiresAt == nil || time.Now().After(*expiredUser.PasswordResetExpiresAt) {
				expiredUser.PasswordResetToken = ""
				expiredUser.PasswordResetSentAt = nil
				expiredUser.PasswordResetExpiresAt = nil
				_ = s.userRepo.Update(expiredUser)
			}
		}
		return err
	}

	hashedPassword, err := HashPassword(newPassword, DefaultBcryptCost)
	if err != nil {
		return err
	}

	user.Password = string(hashedPassword)
	user.PasswordResetToken = ""
	user.PasswordResetSentAt = nil
	user.PasswordResetExpiresAt = nil

	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	return nil
}
