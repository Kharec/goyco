package services

import (
	"fmt"

	"goyco/internal/config"
	"goyco/internal/database"
)

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
