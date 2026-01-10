package handlers

import (
	"errors"
	"net/http"
	"strings"

	"goyco/internal/database"
	"goyco/internal/dto"
	"goyco/internal/repositories"
	"goyco/internal/security"
	"goyco/internal/services"
	"goyco/internal/validation"

	"github.com/go-chi/chi/v5"
)

type AuthServiceInterface interface {
	Login(username, password string) (*services.AuthResult, error)
	Register(username, email, password string) (*services.RegistrationResult, error)
	ConfirmEmail(token string) (*database.User, error)
	ResendVerificationEmail(email string) error
	RequestPasswordReset(usernameOrEmail string) error
	ResetPassword(token, newPassword string) error
	UpdateEmail(userID uint, email string) (*database.User, error)
	UpdateUsername(userID uint, username string) (*database.User, error)
	UpdatePassword(userID uint, currentPassword, newPassword string) (*database.User, error)
	RequestAccountDeletion(userID uint) error
	ConfirmAccountDeletionWithPosts(token string, deletePosts bool) error
	RefreshAccessToken(refreshToken string) (*services.AuthResult, error)
	RevokeRefreshToken(refreshToken string) error
	RevokeAllUserTokens(userID uint) error
	InvalidateAllSessions(userID uint) error
	GetAdminEmail() string
	VerifyToken(tokenString string) (uint, error)
	GetUserIDFromDeletionToken(token string) (uint, error)
	UserHasPosts(userID uint) (bool, int64, error)
}

type AuthHandler struct {
	authService AuthServiceInterface
	userRepo    repositories.UserRepository
}

type AuthResponse = CommonResponse

func NewAuthHandler(authService AuthServiceInterface, userRepo repositories.UserRepository) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		userRepo:    userRepo,
	}
}

// @Summary Login user
// @Description Authenticate user with username and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Login credentials"
// @Success 200 {object} AuthResponse "Authentication successful"
// @Failure 400 {object} AuthResponse "Invalid request data or validation failed"
// @Failure 401 {object} AuthResponse "Invalid credentials"
// @Failure 403 {object} AuthResponse "Account is locked"
// @Failure 500 {object} AuthResponse "Internal server error"
// @Router /api/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	req, ok := GetValidatedDTO[dto.LoginRequest](r)
	if !ok {
		SendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}

	username := security.SanitizeUsername(req.Username)
	password := strings.TrimSpace(req.Password)

	result, err := h.authService.Login(username, password)
	if !HandleServiceError(w, err, "Authentication failed", http.StatusInternalServerError) {
		return
	}

	responseDTO := dto.ToAuthResponseDTO(result)
	SendSuccessResponse(w, "Authentication successful", responseDTO)
}

// @Summary Register a new user
// @Description Register a new user with username, email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RegisterRequest true "Registration data"
// @Success 201 {object} AuthResponse "Registration successful"
// @Failure 400 {object} AuthResponse "Invalid request data or validation failed"
// @Failure 409 {object} AuthResponse "Username or email already exists"
// @Failure 500 {object} AuthResponse "Internal server error"
// @Router /api/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	req, ok := GetValidatedDTO[dto.RegisterRequest](r)
	if !ok {
		SendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.Password)

	username = security.SanitizeUsername(username)

	result, err := h.authService.Register(username, email, password)
	if err != nil {
		var validationErr *validation.ValidationError
		if errors.As(err, &validationErr) {
			SendErrorResponse(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !HandleServiceError(w, err, "Registration failed", http.StatusInternalServerError) {
			return
		}
	}

	responseDTO := dto.ToRegistrationResponseDTO(result.User, result.VerificationSent)
	SendCreatedResponse(w, "Registration successful. Check your email to confirm your account.", responseDTO)
}

// @Summary Confirm email address
// @Description Confirm user email with verification token
// @Tags auth
// @Accept json
// @Produce json
// @Param token query string true "Email verification token"
// @Success 200 {object} AuthResponse "Email confirmed successfully"
// @Failure 400 {object} AuthResponse "Invalid or missing token"
// @Failure 500 {object} AuthResponse "Internal server error"
// @Router /api/auth/confirm [get]
func (h *AuthHandler) ConfirmEmail(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		SendErrorResponse(w, "Verification token is required", http.StatusBadRequest)
		return
	}

	user, err := h.authService.ConfirmEmail(token)
	if !HandleServiceError(w, err, "Unable to verify email", http.StatusInternalServerError) {
		return
	}

	userDTO := dto.ToUserDTO(user)
	SendSuccessResponse(w, "Email confirmed successfully", userDTO)
}

// @Summary Resend verification email
// @Description Send a new verification email to the provided address
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.ResendVerificationRequest true "Email address"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} AuthResponse
// @Failure 404 {object} AuthResponse
// @Failure 409 {object} AuthResponse
// @Failure 429 {object} AuthResponse
// @Failure 503 {object} AuthResponse
// @Failure 500 {object} AuthResponse
// @Router /api/auth/resend-verification [post]
func (h *AuthHandler) ResendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	req, ok := GetValidatedDTO[dto.ResendVerificationRequest](r)
	if !ok {
		SendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(req.Email)

	if email == "" {
		SendErrorResponse(w, "Email address is required", http.StatusBadRequest)
		return
	}

	err := h.authService.ResendVerificationEmail(email)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCredentials):
			SendErrorResponse(w, "No account found with this email address", http.StatusNotFound)
		case errors.Is(err, services.ErrInvalidEmail):
			SendErrorResponse(w, "Invalid email address format", http.StatusBadRequest)
		case errors.Is(err, services.ErrEmailSenderUnavailable):
			SendErrorResponse(w, "We couldn't send the verification email. Try again later.", http.StatusServiceUnavailable)
		case err.Error() == "email already verified":
			SendErrorResponse(w, "This email address is already verified", http.StatusConflict)
		case err.Error() == "verification email sent recently, please wait before requesting another":
			SendErrorResponse(w, "Please wait 5 minutes before requesting another verification email", http.StatusTooManyRequests)
		default:
			SendErrorResponse(w, "Unable to resend verification email", http.StatusInternalServerError)
		}
		return
	}

	responseDTO := dto.MessageResponseDTO{
		Message: "Check your inbox for the verification link",
	}
	SendSuccessResponse(w, "Verification email sent successfully", responseDTO)
}

// @Summary Get current user profile
// @Description Retrieve the authenticated user's profile information
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} AuthResponse "User profile retrieved successfully"
// @Failure 401 {object} AuthResponse "Authentication required"
// @Failure 404 {object} AuthResponse "User not found"
// @Router /api/auth/me [get]
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := RequireAuth(w, r)
	if !ok {
		return
	}

	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		SendErrorResponse(w, "User not found", http.StatusNotFound)
		return
	}

	userDTO := dto.ToUserDTO(user)
	SendSuccessResponse(w, "User profile fetched", userDTO)
}

// @Summary Request a password reset
// @Description Send a password reset email using a username or email
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.ForgotPasswordRequest true "Username or email"
// @Success 200 {object} AuthResponse "Password reset email sent if account exists"
// @Failure 400 {object} AuthResponse "Invalid request data"
// @Router /api/auth/forgot-password [post]
func (h *AuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	req, ok := GetValidatedDTO[dto.ForgotPasswordRequest](r)
	if !ok {
		SendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}

	usernameOrEmail := strings.TrimSpace(req.UsernameOrEmail)

	if usernameOrEmail == "" {
		SendErrorResponse(w, "Username or email is required", http.StatusBadRequest)
		return
	}

	if err := h.authService.RequestPasswordReset(usernameOrEmail); err != nil {
	}

	SendSuccessResponse(w, "If an account with that username or email exists, we've sent a password reset link.", dto.EmptyResponseDTO{})
}

// @Summary Reset password
// @Description Reset a user's password using a reset token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.ResetPasswordRequest true "Password reset data"
// @Success 200 {object} AuthResponse "Password reset successfully"
// @Failure 400 {object} AuthResponse "Invalid or expired token, or validation failed"
// @Failure 500 {object} AuthResponse "Internal server error"
// @Router /api/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	req, ok := GetValidatedDTO[dto.ResetPasswordRequest](r)
	if !ok {
		SendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}

	token := strings.TrimSpace(req.Token)
	newPassword := strings.TrimSpace(req.NewPassword)

	if token == "" {
		SendErrorResponse(w, "Token is required", http.StatusBadRequest)
		return
	}

	if err := h.authService.ResetPassword(token, newPassword); err != nil {
		switch {
		case strings.Contains(err.Error(), "expired"):
			SendErrorResponse(w, "The reset link has expired. Please request a new one.", http.StatusBadRequest)
		case strings.Contains(err.Error(), "invalid"):
			SendErrorResponse(w, "The reset link is invalid. Please request a new one.", http.StatusBadRequest)
		default:
			SendErrorResponse(w, "Unable to reset password. Please try again later.", http.StatusInternalServerError)
		}
		return
	}

	SendSuccessResponse(w, "Password reset successfully. You can now sign in with your new password.", dto.EmptyResponseDTO{})
}

// @Summary Update email address
// @Description Update the authenticated user's email address
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UpdateEmailRequest true "New email address"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} AuthResponse
// @Failure 401 {object} AuthResponse
// @Failure 409 {object} AuthResponse
// @Failure 503 {object} AuthResponse
// @Failure 500 {object} AuthResponse
// @Router /api/auth/email [put]
func (h *AuthHandler) UpdateEmail(w http.ResponseWriter, r *http.Request) {
	userID, ok := RequireAuth(w, r)
	if !ok {
		return
	}

	req, ok := GetValidatedDTO[dto.UpdateEmailRequest](r)
	if !ok {
		SendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(req.Email)

	user, err := h.authService.UpdateEmail(userID, email)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrEmailTaken):
			SendErrorResponse(w, "That email is already in use. Choose another one.", http.StatusConflict)
		case errors.Is(err, services.ErrEmailSenderUnavailable):
			SendErrorResponse(w, "We couldn't send the confirmation email. Try again later.", http.StatusServiceUnavailable)
		case errors.Is(err, services.ErrInvalidEmail):
			SendErrorResponse(w, "Invalid email address", http.StatusBadRequest)
		default:
			SendErrorResponse(w, "We couldn't update your email right now.", http.StatusInternalServerError)
		}
		return
	}

	userDTO := dto.ToUserDTO(user)
	SendSuccessResponse(w, "Email updated. Check your inbox to confirm the new address.", userDTO)
}

// @Summary Update username
// @Description Update the authenticated user's username
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UpdateUsernameRequest true "New username"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} AuthResponse
// @Failure 401 {object} AuthResponse
// @Failure 409 {object} AuthResponse
// @Failure 500 {object} AuthResponse
// @Router /api/auth/username [put]
func (h *AuthHandler) UpdateUsername(w http.ResponseWriter, r *http.Request) {
	userID, ok := RequireAuth(w, r)
	if !ok {
		return
	}

	req, ok := GetValidatedDTO[dto.UpdateUsernameRequest](r)
	if !ok {
		SendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(req.Username)

	user, err := h.authService.UpdateUsername(userID, username)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrUsernameTaken):
			SendErrorResponse(w, "That username is already taken. Try another one.", http.StatusConflict)
		default:
			SendErrorResponse(w, "We couldn't update your username right now.", http.StatusInternalServerError)
		}
		return
	}

	userDTO := dto.ToUserDTO(user)
	SendSuccessResponse(w, "Username updated successfully.", userDTO)
}

// @Summary Update password
// @Description Update the authenticated user's password
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UpdatePasswordRequest true "Password update data"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} AuthResponse
// @Failure 401 {object} AuthResponse
// @Failure 500 {object} AuthResponse
// @Router /api/auth/password [put]
func (h *AuthHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := RequireAuth(w, r)
	if !ok {
		return
	}

	req, ok := GetValidatedDTO[dto.UpdatePasswordRequest](r)
	if !ok {
		SendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}

	currentPassword := strings.TrimSpace(req.CurrentPassword)
	newPassword := strings.TrimSpace(req.NewPassword)

	user, err := h.authService.UpdatePassword(userID, currentPassword, newPassword)
	if err != nil {
		if strings.Contains(err.Error(), "current password is incorrect") {
			SendErrorResponse(w, "Current password is incorrect", http.StatusBadRequest)
		} else {
			SendErrorResponse(w, "We couldn't update your password right now.", http.StatusInternalServerError)
		}
		return
	}

	userDTO := dto.ToUserDTO(user)
	SendSuccessResponse(w, "Password updated successfully.", userDTO)
}

// @Summary Request account deletion
// @Description Initiate the deletion process for the authenticated user's account
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} AuthResponse "Deletion email sent"
// @Failure 401 {object} AuthResponse "Authentication required"
// @Failure 503 {object} AuthResponse "Email delivery unavailable"
// @Failure 500 {object} AuthResponse "Internal server error"
// @Router /api/auth/account [delete]
func (h *AuthHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := RequireAuth(w, r)
	if !ok {
		return
	}

	err := h.authService.RequestAccountDeletion(userID)
	if err != nil {
		if errors.Is(err, services.ErrEmailSenderUnavailable) {
			SendErrorResponse(w, "Account deletion isn't available right now because email delivery is disabled.", http.StatusServiceUnavailable)
		} else {
			SendErrorResponse(w, "We couldn't start the deletion process right now.", http.StatusInternalServerError)
		}
		return
	}

	SendSuccessResponse(w, "Check your inbox for a confirmation link to finish deleting your account.", dto.EmptyResponseDTO{})
}

// @Summary Confirm account deletion
// @Description Confirm account deletion using the provided token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.ConfirmAccountDeletionRequest true "Account deletion data"
// @Success 200 {object} AuthResponse "Account deleted successfully"
// @Failure 400 {object} AuthResponse "Invalid or expired token"
// @Failure 503 {object} AuthResponse "Email delivery unavailable"
// @Failure 500 {object} AuthResponse "Internal server error"
// @Router /api/auth/account/confirm [post]
func (h *AuthHandler) ConfirmAccountDeletion(w http.ResponseWriter, r *http.Request) {
	req, ok := GetValidatedDTO[dto.ConfirmAccountDeletionRequest](r)
	if !ok {
		SendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}

	token := strings.TrimSpace(req.Token)

	if token == "" {
		SendErrorResponse(w, "Deletion token is required", http.StatusBadRequest)
		return
	}

	if err := h.authService.ConfirmAccountDeletionWithPosts(token, req.DeletePosts); err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDeletionToken):
			SendErrorResponse(w, "This deletion link is invalid or has expired.", http.StatusBadRequest)
		case errors.Is(err, services.ErrEmailSenderUnavailable):
			SendErrorResponse(w, "Account deletion isn't available right now because email delivery is disabled.", http.StatusServiceUnavailable)
		case errors.Is(err, services.ErrDeletionEmailFailed):
			responseDTO := dto.AccountDeletionResponseDTO{
				PostsDeleted: req.DeletePosts,
			}
			SendSuccessResponse(w, "Your account has been deleted, but we couldn't send the confirmation email.", responseDTO)
		default:
			SendErrorResponse(w, "We couldn't confirm the deletion right now.", http.StatusInternalServerError)
		}
		return
	}

	responseDTO := dto.AccountDeletionResponseDTO{
		PostsDeleted: req.DeletePosts,
	}
	SendSuccessResponse(w, "Your account has been deleted.", responseDTO)
}

// @Summary Logout user
// @Description Logout the authenticated user and invalidate their session
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} AuthResponse "Logged out successfully"
// @Failure 401 {object} AuthResponse "Authentication required"
// @Router /api/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	SendSuccessResponse(w, "Logged out successfully", dto.EmptyResponseDTO{})
}

// @Summary Refresh access token
// @Description Use a refresh token to get a new access token. The refresh token is rotated on success, and the previous refresh token becomes invalid.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RefreshTokenRequest true "Refresh token data"
// @Success 200 {object} AuthResponse "Token refreshed successfully"
// @Failure 400 {object} AuthResponse "Invalid request body or missing refresh token"
// @Failure 401 {object} AuthResponse "Invalid or expired refresh token"
// @Failure 403 {object} AuthResponse "Account is locked"
// @Failure 500 {object} AuthResponse "Internal server error"
// @Router /api/auth/refresh [post]
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	req, ok := GetValidatedDTO[dto.RefreshTokenRequest](r)
	if !ok {
		SendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.RefreshToken == "" {
		SendErrorResponse(w, "Refresh token is required", http.StatusBadRequest)
		return
	}

	result, err := h.authService.RefreshAccessToken(req.RefreshToken)
	if !HandleServiceError(w, err, "Token refresh failed", http.StatusInternalServerError) {
		return
	}

	responseDTO := dto.ToAuthResponseDTO(result)
	SendSuccessResponse(w, "Token refreshed successfully", responseDTO)
}

// @Summary Revoke refresh token
// @Description Revoke a specific refresh token. This endpoint allows authenticated users to invalidate a specific refresh token, preventing its future use.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.RevokeTokenRequest true "Token revocation data"
// @Success 200 {object} AuthResponse "Token revoked successfully"
// @Failure 400 {object} AuthResponse "Invalid request body or missing refresh token"
// @Failure 401 {object} AuthResponse "Invalid or expired access token"
// @Failure 500 {object} AuthResponse "Internal server error"
// @Router /api/auth/revoke [post]
func (h *AuthHandler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	req, ok := GetValidatedDTO[dto.RevokeTokenRequest](r)
	if !ok {
		SendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.RefreshToken == "" {
		SendErrorResponse(w, "Refresh token is required", http.StatusBadRequest)
		return
	}

	err := h.authService.RevokeRefreshToken(req.RefreshToken)
	if err != nil {
		SendErrorResponse(w, "Failed to revoke token", http.StatusInternalServerError)
		return
	}

	SendSuccessResponse(w, "Token revoked successfully", dto.EmptyResponseDTO{})
}

// @Summary Revoke all user tokens
// @Description Revoke all refresh tokens for the authenticated user. This endpoint allows users to invalidate all their refresh tokens at once, effectively logging them out from all devices.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} AuthResponse "All tokens revoked successfully"
// @Failure 401 {object} AuthResponse "Invalid or expired access token"
// @Failure 500 {object} AuthResponse "Internal server error"
// @Router /api/auth/revoke-all [post]
func (h *AuthHandler) RevokeAllTokens(w http.ResponseWriter, r *http.Request) {
	userID, ok := RequireAuth(w, r)
	if !ok {
		return
	}

	err := h.authService.RevokeAllUserTokens(userID)
	if err != nil {
		SendErrorResponse(w, "Failed to revoke tokens", http.StatusInternalServerError)
		return
	}

	SendSuccessResponse(w, "All tokens revoked successfully", dto.EmptyResponseDTO{})
}

func (h *AuthHandler) MountRoutes(r chi.Router, config RouteModuleConfig) {
	if config.GeneralRateLimit != nil {
		rateLimited := config.GeneralRateLimit(r)
		rateLimited.Post("/auth/refresh", WithValidation[dto.RefreshTokenRequest](config.ValidationMiddleware, h.RefreshToken))
		rateLimited.Get("/auth/confirm", h.ConfirmEmail)
		rateLimited.Post("/auth/resend-verification", WithValidation[dto.ResendVerificationRequest](config.ValidationMiddleware, h.ResendVerificationEmail))
	} else {
		r.Post("/auth/refresh", WithValidation[dto.RefreshTokenRequest](config.ValidationMiddleware, h.RefreshToken))
		r.Get("/auth/confirm", h.ConfirmEmail)
		r.Post("/auth/resend-verification", WithValidation[dto.ResendVerificationRequest](config.ValidationMiddleware, h.ResendVerificationEmail))
	}

	if config.AuthRateLimit != nil {
		rateLimited := config.AuthRateLimit(r)
		rateLimited.Post("/auth/register", WithValidation[dto.RegisterRequest](config.ValidationMiddleware, h.Register))
		rateLimited.Post("/auth/login", WithValidation[dto.LoginRequest](config.ValidationMiddleware, h.Login))
		rateLimited.Post("/auth/forgot-password", WithValidation[dto.ForgotPasswordRequest](config.ValidationMiddleware, h.RequestPasswordReset))
		rateLimited.Post("/auth/reset-password", WithValidation[dto.ResetPasswordRequest](config.ValidationMiddleware, h.ResetPassword))
		rateLimited.Post("/auth/account/confirm", WithValidation[dto.ConfirmAccountDeletionRequest](config.ValidationMiddleware, h.ConfirmAccountDeletion))
	} else {
		r.Post("/auth/register", WithValidation[dto.RegisterRequest](config.ValidationMiddleware, h.Register))
		r.Post("/auth/login", WithValidation[dto.LoginRequest](config.ValidationMiddleware, h.Login))
		r.Post("/auth/forgot-password", WithValidation[dto.ForgotPasswordRequest](config.ValidationMiddleware, h.RequestPasswordReset))
		r.Post("/auth/reset-password", WithValidation[dto.ResetPasswordRequest](config.ValidationMiddleware, h.ResetPassword))
		r.Post("/auth/account/confirm", WithValidation[dto.ConfirmAccountDeletionRequest](config.ValidationMiddleware, h.ConfirmAccountDeletion))
	}

	protected := r
	if config.AuthMiddleware != nil {
		protected = r.With(config.AuthMiddleware)
	}
	if config.GeneralRateLimit != nil {
		protected = config.GeneralRateLimit(protected)
	}

	protected.Get("/auth/me", h.Me)
	protected.Post("/auth/logout", h.Logout)
	protected.Post("/auth/revoke", WithValidation[dto.RevokeTokenRequest](config.ValidationMiddleware, h.RevokeToken))
	protected.Post("/auth/revoke-all", h.RevokeAllTokens)
	protected.Put("/auth/email", WithValidation[dto.UpdateEmailRequest](config.ValidationMiddleware, h.UpdateEmail))
	protected.Put("/auth/username", WithValidation[dto.UpdateUsernameRequest](config.ValidationMiddleware, h.UpdateUsername))
	protected.Put("/auth/password", WithValidation[dto.UpdatePasswordRequest](config.ValidationMiddleware, h.UpdatePassword))
	protected.Delete("/auth/account", h.DeleteAccount)
}
