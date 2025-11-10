package services

import (
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"goyco/internal/database"
	"goyco/internal/repositories"
	"goyco/internal/validation"
)

type UserManagementService struct {
	userRepo     repositories.UserRepository
	postRepo     repositories.PostRepository
	emailService *EmailService
}

func NewUserManagementService(userRepo repositories.UserRepository, postRepo repositories.PostRepository, emailService *EmailService) *UserManagementService {
	return &UserManagementService{
		userRepo:     userRepo,
		postRepo:     postRepo,
		emailService: emailService,
	}
}

func (s *UserManagementService) UpdateUsername(userID uint, newUsername string) (*database.User, error) {
	trimmed := TrimString(newUsername)
	if err := validation.ValidateUsername(trimmed); err != nil {
		return nil, err
	}

	existing, err := s.userRepo.GetByUsernameIncludingDeleted(trimmed)
	if err == nil && existing.ID != userID {
		return nil, ErrUsernameTaken
	}
	if err != nil && !IsRecordNotFound(err) {
		return nil, fmt.Errorf("lookup username: %w", err)
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}

	if user.Username == trimmed {
		return sanitizeUser(user), nil
	}

	user.Username = trimmed
	if err := s.userRepo.Update(user); err != nil {
		if handled := HandleUniqueConstraintError(err); handled != err {
			return nil, handled
		}
		return nil, fmt.Errorf("update user: %w", err)
	}

	return sanitizeUser(user), nil
}

func (s *UserManagementService) UpdatePassword(userID uint, currentPassword, newPassword string) (*database.User, error) {
	if err := validation.ValidatePassword(newPassword); err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentPassword)); err != nil {
		return nil, fmt.Errorf("current password is incorrect")
	}

	hashedPassword, err := HashPassword(newPassword, DefaultBcryptCost)
	if err != nil {
		return nil, err
	}

	user.Password = string(hashedPassword)
	if err := s.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("update password: %w", err)
	}

	return sanitizeUser(user), nil
}

func (s *UserManagementService) UpdateEmail(userID uint, newEmail string) (*database.User, error) {
	normalized, err := normalizeEmail(newEmail)
	if err != nil {
		return nil, err
	}

	existing, err := s.userRepo.GetByEmail(normalized)
	if err == nil && existing.ID != userID {
		return nil, ErrEmailTaken
	}
	if err != nil && !IsRecordNotFound(err) {
		return nil, fmt.Errorf("lookup email: %w", err)
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}

	if user.Email == normalized {
		return sanitizeUser(user), nil
	}

	previousEmail := user.Email
	previousVerified := user.EmailVerified
	previousVerifiedAt := user.EmailVerifiedAt
	previousToken := user.EmailVerificationToken
	previousSentAt := user.EmailVerificationSentAt

	token, hashed, err := generateVerificationToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user.Email = normalized
	user.EmailVerified = false
	user.EmailVerifiedAt = nil
	user.EmailVerificationToken = hashed
	user.EmailVerificationSentAt = &now

	if err := s.userRepo.Update(user); err != nil {
		if handled := HandleUniqueConstraintError(err); handled != err {
			return nil, handled
		}
		return nil, fmt.Errorf("update user: %w", err)
	}

	if err := s.emailService.SendEmailChangeVerificationEmail(user, token); err != nil {
		user.Email = previousEmail
		user.EmailVerified = previousVerified
		user.EmailVerifiedAt = previousVerifiedAt
		user.EmailVerificationToken = previousToken
		user.EmailVerificationSentAt = previousSentAt
		_ = s.userRepo.Update(user)
		return nil, err
	}

	return sanitizeUser(user), nil
}

func (s *UserManagementService) UserHasPosts(userID uint) (bool, int64, error) {
	if s.postRepo == nil {
		return false, 0, fmt.Errorf("post repository not configured")
	}

	count, err := s.postRepo.CountByUserID(userID)
	if err != nil {
		return false, 0, fmt.Errorf("count user posts: %w", err)
	}

	return count > 0, count, nil
}
