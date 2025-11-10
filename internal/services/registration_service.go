package services

import (
	"fmt"
	"time"

	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/repositories"
	"goyco/internal/validation"
)

type RegistrationService struct {
	userRepo     repositories.UserRepository
	emailService *EmailService
	config       *config.Config
}

func NewRegistrationService(userRepo repositories.UserRepository, emailService *EmailService, config *config.Config) *RegistrationService {
	return &RegistrationService{
		userRepo:     userRepo,
		emailService: emailService,
		config:       config,
	}
}

func (s *RegistrationService) Register(username, email, password string) (*RegistrationResult, error) {
	trimmedUsername := TrimString(username)
	if err := validation.ValidateUsername(trimmedUsername); err != nil {
		return nil, err
	}

	if err := validation.ValidatePassword(password); err != nil {
		return nil, err
	}

	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return nil, err
	}

	userCheck, err := s.userRepo.GetByUsername(trimmedUsername)
	if err == nil {
		if userCheck != nil {
			return nil, ErrUsernameTaken
		}
	} else if !IsRecordNotFound(err) {
		if handled := HandleUniqueConstraintError(err); handled != err {
			return nil, handled
		}
		return nil, fmt.Errorf("lookup user: %w", err)
	}

	emailCheck, err := s.userRepo.GetByEmail(normalizedEmail)
	if err == nil {
		if emailCheck != nil {
			return nil, ErrEmailTaken
		}
	} else if !IsRecordNotFound(err) {
		if handled := HandleUniqueConstraintError(err); handled != err {
			return nil, handled
		}
		return nil, fmt.Errorf("lookup email: %w", err)
	}

	hashedPassword, err := HashPassword(password, s.config.App.BcryptCost)
	if err != nil {
		return nil, err
	}

	token, hashedToken, err := generateVerificationToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user := &database.User{
		Username:                trimmedUsername,
		Email:                   normalizedEmail,
		Password:                string(hashedPassword),
		EmailVerified:           false,
		EmailVerificationToken:  hashedToken,
		EmailVerificationSentAt: &now,
	}

	if err := s.userRepo.Create(user); err != nil {
		if handled := HandleUniqueConstraintErrorWithMessage(err); handled != err {
			return nil, handled
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	if err := s.emailService.SendVerificationEmail(user, token); err != nil {
		if deleteErr := s.userRepo.HardDelete(user.ID); deleteErr != nil {
			return nil, fmt.Errorf("verification email failed and user cleanup failed: email=%w, cleanup=%v", err, deleteErr)
		}
		return nil, fmt.Errorf("verification email failed: %w", err)
	}

	return &RegistrationResult{
		User:             sanitizeUser(user),
		VerificationSent: true,
	}, nil
}

func (s *RegistrationService) ConfirmEmail(token string) (*database.User, error) {
	trimmed := TrimString(token)
	if trimmed == "" {
		return nil, ErrInvalidVerificationToken
	}

	hashed := HashVerificationToken(trimmed)
	user, err := s.userRepo.GetByVerificationToken(hashed)
	if err != nil {
		if IsRecordNotFound(err) {
			return nil, ErrInvalidVerificationToken
		}
		return nil, fmt.Errorf("lookup verification token: %w", err)
	}

	if user.EmailVerified {
		return sanitizeUser(user), nil
	}

	now := time.Now()
	user.EmailVerified = true
	user.EmailVerifiedAt = &now
	user.EmailVerificationToken = ""
	user.EmailVerificationSentAt = nil

	if err := s.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	return sanitizeUser(user), nil
}

func (s *RegistrationService) ResendVerificationEmail(email string) error {
	email = TrimString(email)
	if err := validation.ValidateEmail(email); err != nil {
		return ErrInvalidEmail
	}

	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		if IsRecordNotFound(err) {
			return ErrInvalidCredentials
		}
		return fmt.Errorf("lookup user: %w", err)
	}

	if user.EmailVerified {
		return fmt.Errorf("email already verified")
	}

	if user.EmailVerificationSentAt != nil && time.Since(*user.EmailVerificationSentAt) < 5*time.Minute {
		return fmt.Errorf("verification email sent recently, please wait before requesting another")
	}

	token, hash, err := generateVerificationToken()
	if err != nil {
		return err
	}

	now := time.Now()
	user.EmailVerificationToken = hash
	user.EmailVerificationSentAt = &now

	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	if err := s.emailService.SendResendVerificationEmail(user, token); err != nil {
		return fmt.Errorf("send verification email: %w", err)
	}

	return nil
}
