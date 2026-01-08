package services

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"goyco/internal/database"
	"goyco/internal/repositories"
)

type SessionService struct {
	jwtService *JWTService
	userRepo   repositories.UserRepository
}

func NewSessionService(jwtService *JWTService, userRepo repositories.UserRepository) *SessionService {
	return &SessionService{
		jwtService: jwtService,
		userRepo:   userRepo,
	}
}

func (s *SessionService) Login(username, password string) (*AuthResult, error) {
	trimmedUsername := TrimString(username)
	if trimmedUsername == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := s.userRepo.GetByUsername(trimmedUsername)
	if err != nil {
		if IsRecordNotFound(err) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("lookup user: %w", err)
	}
	if !user.EmailVerified {
		return nil, ErrEmailNotVerified
	}

	if user.Locked {
		return nil, ErrAccountLocked
	}

	if compareErr := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); compareErr != nil {
		return nil, ErrInvalidCredentials
	}

	return s.issueAuthResult(user)
}

func (s *SessionService) VerifyToken(tokenString string) (uint, error) {
	return s.jwtService.VerifyAccessToken(tokenString)
}

func (s *SessionService) issueAuthResult(user *database.User) (*AuthResult, error) {
	accessToken, err := s.jwtService.GenerateAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(user)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	return &AuthResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         sanitizeUser(user),
	}, nil
}

func (s *SessionService) RefreshAccessToken(refreshToken string) (*AuthResult, error) {
	accessToken, newRefreshToken, err := s.jwtService.RefreshAccessTokenWithRotation(refreshToken)
	if err != nil {
		return nil, err
	}

	userID, err := s.jwtService.VerifyAccessToken(accessToken)
	if err != nil {
		return nil, fmt.Errorf("verify new access token: %w", err)
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("lookup user: %w", err)
	}

	return &AuthResult{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		User:         sanitizeUser(user),
	}, nil
}

func (s *SessionService) RevokeRefreshToken(refreshToken string) error {
	return s.jwtService.RevokeRefreshToken(refreshToken)
}

func (s *SessionService) RevokeAllUserTokens(userID uint) error {
	return s.jwtService.RevokeAllRefreshTokens(userID)
}

func (s *SessionService) InvalidateAllSessions(userID uint) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("load user: %w", err)
	}

	user.SessionVersion++
	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("update session version: %w", err)
	}

	if err := s.jwtService.RevokeAllRefreshTokens(userID); err != nil {
		return fmt.Errorf("revoke refresh tokens: %w", err)
	}

	return nil
}

func (s *SessionService) CleanupExpiredTokens() error {
	return s.jwtService.CleanupExpiredTokens()
}
