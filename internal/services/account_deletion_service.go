package services

import (
	"errors"
	"fmt"
	"log"
	"time"

	"goyco/internal/database"
	"goyco/internal/repositories"

	"gorm.io/gorm"
)

type AccountDeletionService struct {
	userRepo     repositories.UserRepository
	postRepo     repositories.PostRepository
	deletionRepo repositories.AccountDeletionRepository
	emailService *EmailService
}

func NewAccountDeletionService(userRepo repositories.UserRepository, postRepo repositories.PostRepository, deletionRepo repositories.AccountDeletionRepository, emailService *EmailService) *AccountDeletionService {
	return &AccountDeletionService{
		userRepo:     userRepo,
		postRepo:     postRepo,
		deletionRepo: deletionRepo,
		emailService: emailService,
	}
}

func (s *AccountDeletionService) GetUserIDFromDeletionToken(token string) (uint, error) {
	trimmed := TrimString(token)
	if trimmed == "" {
		return 0, ErrInvalidDeletionToken
	}

	if s.deletionRepo == nil {
		return 0, fmt.Errorf("account deletion repository not configured")
	}

	hashed := HashVerificationToken(trimmed)
	req, err := s.deletionRepo.GetByTokenHash(hashed)
	if err != nil {
		if IsRecordNotFound(err) {
			return 0, ErrInvalidDeletionToken
		}
		return 0, fmt.Errorf("lookup deletion request: %w", err)
	}

	if time.Now().After(req.ExpiresAt) {
		if delErr := s.deletionRepo.DeleteByID(req.ID); delErr != nil {
			log.Printf("Failed to delete expired deletion request %d: %v", req.ID, delErr)
		}
		return 0, ErrInvalidDeletionToken
	}

	return req.UserID, nil
}

func (s *AccountDeletionService) RequestAccountDeletion(userID uint) error {
	if userID == 0 {
		return fmt.Errorf("invalid user identifier")
	}

	if s.deletionRepo == nil {
		return fmt.Errorf("account deletion repository not configured")
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		if IsRecordNotFound(err) {
			return ErrUserNotFound
		}
		return fmt.Errorf("load user: %w", err)
	}

	if err := s.deletionRepo.DeleteByUserID(userID); err != nil {
		return fmt.Errorf("clear existing deletion requests: %w", err)
	}

	token, hash, err := generateVerificationToken()
	if err != nil {
		return err
	}

	req := &database.AccountDeletionRequest{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(time.Duration(deletionTokenExpirationHours) * time.Hour),
	}

	if err := s.deletionRepo.Create(req); err != nil {
		return fmt.Errorf("create deletion request: %w", err)
	}

	if err := s.emailService.SendAccountDeletionEmail(user, token); err != nil {
		if delErr := s.deletionRepo.DeleteByID(req.ID); delErr != nil {
			log.Printf("Failed to cleanup deletion request %d after email failure: %v", req.ID, delErr)
		}
		return fmt.Errorf("send deletion confirmation email: %w", err)
	}

	return nil
}

func (s *AccountDeletionService) validateAndGetDeletionRequest(token string) (*database.AccountDeletionRequest, *database.User, error) {
	trimmed := TrimString(token)
	if trimmed == "" {
		return nil, nil, ErrInvalidDeletionToken
	}

	if s.deletionRepo == nil {
		return nil, nil, fmt.Errorf("account deletion repository not configured")
	}

	hashed := HashVerificationToken(trimmed)
	req, err := s.deletionRepo.GetByTokenHash(hashed)
	if err != nil {
		if IsRecordNotFound(err) {
			return nil, nil, ErrInvalidDeletionToken
		}
		return nil, nil, fmt.Errorf("lookup deletion request: %w", err)
	}

	if time.Now().After(req.ExpiresAt) {
		if delErr := s.deletionRepo.DeleteByID(req.ID); delErr != nil {
			log.Printf("Failed to delete expired deletion request %d: %v", req.ID, delErr)
		}
		return nil, nil, ErrInvalidDeletionToken
	}

	user, err := s.userRepo.GetByID(req.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if delErr := s.deletionRepo.DeleteByID(req.ID); delErr != nil {
				log.Printf("Failed to delete orphaned deletion request %d: %v", req.ID, delErr)
			}
			return nil, nil, ErrInvalidDeletionToken
		}
		return nil, nil, fmt.Errorf("load user: %w", err)
	}

	return req, user, nil
}

func (s *AccountDeletionService) ConfirmAccountDeletion(token string) error {
	return s.ConfirmAccountDeletionWithPosts(token, true)
}

func (s *AccountDeletionService) ConfirmAccountDeletionWithPosts(token string, deletePosts bool) error {
	req, user, err := s.validateAndGetDeletionRequest(token)
	if err != nil {
		return err
	}

	if !deletePosts {
		if err := s.userRepo.SoftDeleteWithPosts(user.ID); err != nil {
			return fmt.Errorf("soft delete user with posts: %w", err)
		}
	} else {
		if err := s.userRepo.HardDelete(user.ID); err != nil {
			return fmt.Errorf("delete user: %w", err)
		}
	}

	if err := s.deletionRepo.DeleteByID(req.ID); err != nil {
		return fmt.Errorf("clear deletion request: %w", err)
	}

	if err := s.emailService.SendAccountDeletionNotificationEmail(user, deletePosts); err != nil {
		log.Printf("Failed to send account deletion notification email for user %d: %v", user.ID, err)
		return fmt.Errorf("%w: %w", ErrDeletionEmailFailed, err)
	}

	return nil
}
