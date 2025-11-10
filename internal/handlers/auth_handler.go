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

type AuthTokensResponse struct {
	Success bool             `json:"success" example:"true"`
	Message string           `json:"message" example:"Authentication successful"`
	Data    AuthTokensDetail `json:"data"`
}

type AuthTokensDetail struct {
	AccessToken  string          `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	RefreshToken string          `json:"refresh_token" example:"f94d4ddc7d9b4fcb9d3a2c44c400b780"`
	User         AuthUserSummary `json:"user"`
}

type AuthUserSummary struct {
	ID            uint   `json:"id" example:"42"`
	Username      string `json:"username" example:"janedoe"`
	Email         string `json:"email" example:"jane@example.com"`
	EmailVerified bool   `json:"email_verified" example:"true"`
	Locked        bool   `json:"locked" example:"false"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CreatePostRequest struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

type ResendVerificationRequest struct {
	Email string `json:"email"`
}

type ForgotPasswordRequest struct {
	UsernameOrEmail string `json:"username_or_email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type UpdateEmailRequest struct {
	Email string `json:"email"`
}

type UpdateUsernameRequest struct {
	Username string `json:"username"`
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type ConfirmAccountDeletionRequest struct {
	Token       string `json:"token"`
	DeletePosts bool   `json:"delete_posts"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." binding:"required"`
}

type RevokeTokenRequest struct {
	RefreshToken string `json:"refresh_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." binding:"required"`
}

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
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} AuthTokensResponse "Authentication successful"
// @Failure 400 {object} AuthResponse "Invalid request data or validation failed"
// @Failure 401 {object} AuthResponse "Invalid credentials"
// @Failure 403 {object} AuthResponse "Account is locked"
// @Failure 500 {object} AuthResponse "Internal server error"
// @Router /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if !DecodeJSONRequest(w, r, &req) {
		return
	}

	username := security.SanitizeUsername(req.Username)
	password := strings.TrimSpace(req.Password)

	if username == "" || password == "" {
		SendErrorResponse(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	if err := validation.ValidatePassword(password); err != nil {
		SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.authService.Login(username, password)
	if !HandleServiceError(w, err, "Authentication failed", http.StatusInternalServerError) {
		return
	}

	SendSuccessResponse(w, "Authentication successful", result)
}

// @Summary Register a new user
// @Description Register a new user with username, email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration data"
// @Success 201 {object} AuthResponse "Registration successful"
// @Failure 400 {object} AuthResponse "Invalid request data or validation failed"
// @Failure 409 {object} AuthResponse "Username or email already exists"
// @Failure 500 {object} AuthResponse "Internal server error"
// @Router /auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if !DecodeJSONRequest(w, r, &req) {
		return
	}

	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.Password)

	if username == "" || email == "" || password == "" {
		SendErrorResponse(w, "Username, email, and password are required", http.StatusBadRequest)
		return
	}

	username = security.SanitizeUsername(username)
	if err := validation.ValidateUsername(username); err != nil {
		SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validation.ValidateEmail(email); err != nil {
		SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validation.ValidatePassword(password); err != nil {
		SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

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

	userData := map[string]any{
		"id":             result.User.ID,
		"username":       result.User.Username,
		"email":          result.User.Email,
		"email_verified": result.User.EmailVerified,
		"created_at":     result.User.CreatedAt,
		"updated_at":     result.User.UpdatedAt,
		"deleted_at":     result.User.DeletedAt,
	}

	responseData := map[string]any{
		"user":              userData,
		"verification_sent": result.VerificationSent,
	}

	SendCreatedResponse(w, "Registration successful. Check your email to confirm your account.", responseData)
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
// @Router /auth/confirm [get]
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
	SendSuccessResponse(w, "Email confirmed successfully", map[string]any{
		"user": userDTO,
	})
}

// @Summary Resend verification email
// @Description Send a new verification email to the provided address
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ResendVerificationRequest true "Email address"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} AuthResponse
// @Failure 404 {object} AuthResponse
// @Failure 409 {object} AuthResponse
// @Failure 429 {object} AuthResponse
// @Failure 503 {object} AuthResponse
// @Failure 500 {object} AuthResponse
// @Router /auth/resend-verification [post]
func (h *AuthHandler) ResendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}

	if !DecodeJSONRequest(w, r, &req) {
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

	SendSuccessResponse(w, "Verification email sent successfully", map[string]any{
		"message": "Check your inbox for the verification link",
	})
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
// @Router /auth/me [get]
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
// @Param request body ForgotPasswordRequest true "Username or email"
// @Success 200 {object} AuthResponse "Password reset email sent if account exists"
// @Failure 400 {object} AuthResponse "Invalid request data"
// @Router /auth/forgot-password [post]
func (h *AuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UsernameOrEmail string `json:"username_or_email"`
	}

	if !DecodeJSONRequest(w, r, &req) {
		return
	}

	usernameOrEmail := strings.TrimSpace(req.UsernameOrEmail)
	if usernameOrEmail == "" {
		SendErrorResponse(w, "Username or email is required", http.StatusBadRequest)
		return
	}

	if err := h.authService.RequestPasswordReset(usernameOrEmail); err != nil {
	}

	SendSuccessResponse(w, "If an account with that username or email exists, we've sent a password reset link.", nil)
}

// @Summary Reset password
// @Description Reset a user's password using a reset token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ResetPasswordRequest true "Password reset data"
// @Success 200 {object} AuthResponse "Password reset successfully"
// @Failure 400 {object} AuthResponse "Invalid or expired token, or validation failed"
// @Failure 500 {object} AuthResponse "Internal server error"
// @Router /auth/reset-password [post]
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}

	if !DecodeJSONRequest(w, r, &req) {
		return
	}

	token := strings.TrimSpace(req.Token)
	newPassword := strings.TrimSpace(req.NewPassword)

	if token == "" {
		SendErrorResponse(w, "Reset token is required", http.StatusBadRequest)
		return
	}

	if newPassword == "" {
		SendErrorResponse(w, "New password is required", http.StatusBadRequest)
		return
	}

	if len(newPassword) < 8 {
		SendErrorResponse(w, "Password must be at least 8 characters long", http.StatusBadRequest)
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

	SendSuccessResponse(w, "Password reset successfully. You can now sign in with your new password.", nil)
}

// @Summary Update email address
// @Description Update the authenticated user's email address
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateEmailRequest true "New email address"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} AuthResponse
// @Failure 401 {object} AuthResponse
// @Failure 409 {object} AuthResponse
// @Failure 503 {object} AuthResponse
// @Failure 500 {object} AuthResponse
// @Router /auth/email [put]
func (h *AuthHandler) UpdateEmail(w http.ResponseWriter, r *http.Request) {
	userID, ok := RequireAuth(w, r)
	if !ok {
		return
	}

	var req struct {
		Email string `json:"email"`
	}

	if !DecodeJSONRequest(w, r, &req) {
		return
	}

	email := strings.TrimSpace(req.Email)
	if err := validation.ValidateEmail(email); err != nil {
		SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

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
	SendSuccessResponse(w, "Email updated. Check your inbox to confirm the new address.", map[string]any{
		"user": userDTO,
	})
}

// @Summary Update username
// @Description Update the authenticated user's username
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateUsernameRequest true "New username"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} AuthResponse
// @Failure 401 {object} AuthResponse
// @Failure 409 {object} AuthResponse
// @Failure 500 {object} AuthResponse
// @Router /auth/username [put]
func (h *AuthHandler) UpdateUsername(w http.ResponseWriter, r *http.Request) {
	userID, ok := RequireAuth(w, r)
	if !ok {
		return
	}

	var req struct {
		Username string `json:"username"`
	}

	if !DecodeJSONRequest(w, r, &req) {
		return
	}

	username := strings.TrimSpace(req.Username)
	if err := validation.ValidateUsername(username); err != nil {
		SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

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
	SendSuccessResponse(w, "Username updated successfully.", map[string]any{
		"user": userDTO,
	})
}

// @Summary Update password
// @Description Update the authenticated user's password
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdatePasswordRequest true "Password update data"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} AuthResponse
// @Failure 401 {object} AuthResponse
// @Failure 500 {object} AuthResponse
// @Router /auth/password [put]
func (h *AuthHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := RequireAuth(w, r)
	if !ok {
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if !DecodeJSONRequest(w, r, &req) {
		return
	}

	currentPassword := strings.TrimSpace(req.CurrentPassword)
	newPassword := strings.TrimSpace(req.NewPassword)

	if currentPassword == "" {
		SendErrorResponse(w, "Current password is required", http.StatusBadRequest)
		return
	}

	if err := validation.ValidatePassword(newPassword); err != nil {
		SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

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
	SendSuccessResponse(w, "Password updated successfully.", map[string]any{
		"user": userDTO,
	})
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
// @Router /auth/account [delete]
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

	SendSuccessResponse(w, "Check your inbox for a confirmation link to finish deleting your account.", nil)
}

// @Summary Confirm account deletion
// @Description Confirm account deletion using the provided token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ConfirmAccountDeletionRequest true "Account deletion data"
// @Success 200 {object} AuthResponse "Account deleted successfully"
// @Failure 400 {object} AuthResponse "Invalid or expired token"
// @Failure 503 {object} AuthResponse "Email delivery unavailable"
// @Failure 500 {object} AuthResponse "Internal server error"
// @Router /auth/account/confirm [post]
func (h *AuthHandler) ConfirmAccountDeletion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		DeletePosts bool   `json:"delete_posts"`
	}

	if !DecodeJSONRequest(w, r, &req) {
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
			SendSuccessResponse(w, "Your account has been deleted, but we couldn't send the confirmation email.", map[string]any{
				"posts_deleted": req.DeletePosts,
			})
		default:
			SendErrorResponse(w, "We couldn't confirm the deletion right now.", http.StatusInternalServerError)
		}
		return
	}

	SendSuccessResponse(w, "Your account has been deleted.", map[string]any{
		"posts_deleted": req.DeletePosts,
	})
}

// @Summary Logout user
// @Description Logout the authenticated user and invalidate their session
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} AuthResponse "Logged out successfully"
// @Failure 401 {object} AuthResponse "Authentication required"
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	SendSuccessResponse(w, "Logged out successfully", nil)
}

// @Summary Refresh access token
// @Description Use a refresh token to get a new access token. This endpoint allows clients to obtain a new access token using a valid refresh token without requiring user credentials.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RefreshTokenRequest true "Refresh token data"
// @Success 200 {object} AuthTokensResponse "Token refreshed successfully"
// @Failure 400 {object} AuthResponse "Invalid request body or missing refresh token"
// @Failure 401 {object} AuthResponse "Invalid or expired refresh token"
// @Failure 403 {object} AuthResponse "Account is locked"
// @Failure 500 {object} AuthResponse "Internal server error"
// @Router /auth/refresh [post]
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenRequest

	if !DecodeJSONRequest(w, r, &req) {
		return
	}

	if strings.TrimSpace(req.RefreshToken) == "" {
		SendErrorResponse(w, "Refresh token is required", http.StatusBadRequest)
		return
	}

	result, err := h.authService.RefreshAccessToken(req.RefreshToken)
	if !HandleServiceError(w, err, "Token refresh failed", http.StatusInternalServerError) {
		return
	}

	SendSuccessResponse(w, "Token refreshed successfully", result)
}

// @Summary Revoke refresh token
// @Description Revoke a specific refresh token. This endpoint allows authenticated users to invalidate a specific refresh token, preventing its future use.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body RevokeTokenRequest true "Token revocation data"
// @Success 200 {object} AuthResponse "Token revoked successfully"
// @Failure 400 {object} AuthResponse "Invalid request body or missing refresh token"
// @Failure 401 {object} AuthResponse "Invalid or expired access token"
// @Failure 500 {object} AuthResponse "Internal server error"
// @Router /auth/revoke [post]
func (h *AuthHandler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	var req RevokeTokenRequest

	if !DecodeJSONRequest(w, r, &req) {
		return
	}

	if strings.TrimSpace(req.RefreshToken) == "" {
		SendErrorResponse(w, "Refresh token is required", http.StatusBadRequest)
		return
	}

	err := h.authService.RevokeRefreshToken(req.RefreshToken)
	if err != nil {
		SendErrorResponse(w, "Failed to revoke token", http.StatusInternalServerError)
		return
	}

	SendSuccessResponse(w, "Token revoked successfully", nil)
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
// @Router /auth/revoke-all [post]
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

	SendSuccessResponse(w, "All tokens revoked successfully", nil)
}

func (h *AuthHandler) MountRoutes(r chi.Router, config RouteModuleConfig) {
	if config.GeneralRateLimit != nil {
		rateLimited := config.GeneralRateLimit(r)
		rateLimited.Post("/auth/refresh", h.RefreshToken)
		rateLimited.Get("/auth/confirm", h.ConfirmEmail)
		rateLimited.Post("/auth/resend-verification", h.ResendVerificationEmail)
	} else {
		r.Post("/auth/refresh", h.RefreshToken)
		r.Get("/auth/confirm", h.ConfirmEmail)
		r.Post("/auth/resend-verification", h.ResendVerificationEmail)
	}

	if config.AuthRateLimit != nil {
		rateLimited := config.AuthRateLimit(r)
		rateLimited.Post("/auth/register", h.Register)
		rateLimited.Post("/auth/login", h.Login)
		rateLimited.Post("/auth/forgot-password", h.RequestPasswordReset)
		rateLimited.Post("/auth/reset-password", h.ResetPassword)
		rateLimited.Post("/auth/account/confirm", h.ConfirmAccountDeletion)
	} else {
		r.Post("/auth/register", h.Register)
		r.Post("/auth/login", h.Login)
		r.Post("/auth/forgot-password", h.RequestPasswordReset)
		r.Post("/auth/reset-password", h.ResetPassword)
		r.Post("/auth/account/confirm", h.ConfirmAccountDeletion)
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
	protected.Post("/auth/revoke", h.RevokeToken)
	protected.Post("/auth/revoke-all", h.RevokeAllTokens)
	protected.Put("/auth/email", h.UpdateEmail)
	protected.Put("/auth/username", h.UpdateUsername)
	protected.Put("/auth/password", h.UpdatePassword)
	protected.Delete("/auth/account", h.DeleteAccount)
}
