package handlers

import (
	"errors"
	"net/http"

	"goyco/internal/dto"
	"goyco/internal/repositories"
	"goyco/internal/validation"

	"github.com/go-chi/chi/v5"
)

type UserHandler struct {
	userRepo    repositories.UserRepository
	authService AuthServiceInterface
}

func NewUserHandler(userRepo repositories.UserRepository, authService AuthServiceInterface) *UserHandler {
	return &UserHandler{
		userRepo:    userRepo,
		authService: authService,
	}
}

type UserResponse = CommonResponse

// @Summary List users
// @Description Retrieve a paginated list of users
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Number of users to return" default(20)
// @Param offset query int false "Number of users to skip" default(0)
// @Success 200 {object} UserResponse "Users retrieved successfully"
// @Failure 401 {object} UserResponse "Authentication required"
// @Failure 500 {object} UserResponse "Internal server error"
// @Router /api/users [get]
func (h *UserHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	users, err := h.userRepo.GetAll(limit, offset)
	if err != nil {
		SendErrorResponse(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}

	responseDTO := dto.ToSanitizedUserListDTO(users, limit, offset)
	SendSuccessResponse(w, "Users retrieved successfully", responseDTO)
}

// @Summary Get user
// @Description Retrieve a specific user by ID
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} UserResponse "User retrieved successfully"
// @Failure 400 {object} UserResponse "Invalid user ID"
// @Failure 401 {object} UserResponse "Authentication required"
// @Failure 404 {object} UserResponse "User not found"
// @Failure 500 {object} UserResponse "Internal server error"
// @Router /api/users/{id} [get]
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := ParseUintParam(w, r, "id", "User")
	if !ok {
		return
	}

	user, err := h.userRepo.GetByID(userID)
	if !HandleRepoError(w, err, "User") {
		return
	}

	userDTO := dto.ToSanitizedUserDTO(user)

	SendSuccessResponse(w, "User retrieved successfully", userDTO)
}

// @Summary Create user
// @Description Create a new user account
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.RegisterRequest true "User data"
// @Success 201 {object} UserResponse "User created successfully"
// @Failure 400 {object} UserResponse "Invalid request data or validation failed"
// @Failure 401 {object} UserResponse "Authentication required"
// @Failure 409 {object} UserResponse "Username or email already exists"
// @Failure 500 {object} UserResponse "Internal server error"
// @Router /api/users [post]
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	req, ok := GetValidatedDTO[dto.RegisterRequest](r)
	if !ok {
		SendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := validation.ValidateUsername(req.Username); err != nil {
		SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validation.ValidateEmail(req.Email); err != nil {
		SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validation.ValidatePassword(req.Password); err != nil {
		SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.authService.Register(req.Username, req.Email, req.Password)
	if err != nil {
		var validationErr *validation.ValidationError
		if errors.As(err, &validationErr) {
			SendErrorResponse(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !HandleServiceError(w, err, "Failed to create user", http.StatusInternalServerError) {
			return
		}
	}

	responseDTO := dto.ToRegistrationResponseDTO(result.User, result.VerificationSent)
	SendCreatedResponse(w, "User created successfully. Verification email sent.", responseDTO)
}

// @Summary Get user posts
// @Description Retrieve posts created by a specific user
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param limit query int false "Number of posts to return" default(20)
// @Param offset query int false "Number of posts to skip" default(0)
// @Success 200 {object} UserResponse "User posts retrieved successfully"
// @Failure 400 {object} UserResponse "Invalid user ID or pagination parameters"
// @Failure 401 {object} UserResponse "Authentication required"
// @Failure 500 {object} UserResponse "Internal server error"
// @Router /api/users/{id}/posts [get]
func (h *UserHandler) GetUserPosts(w http.ResponseWriter, r *http.Request) {
	userID, ok := ParseUintParam(w, r, "id", "User")
	if !ok {
		return
	}

	limit, offset := parsePagination(r)

	posts, err := h.userRepo.GetPosts(userID, limit, offset)
	if err != nil {
		SendErrorResponse(w, "Failed to fetch user posts", http.StatusInternalServerError)
		return
	}

	responseDTO := dto.ToPostListDTO(posts, limit, offset)
	SendSuccessResponse(w, "User posts retrieved successfully", responseDTO)
}

func (h *UserHandler) MountRoutes(r chi.Router, config RouteModuleConfig) {
	protected := r
	if config.AuthMiddleware != nil {
		protected = r.With(config.AuthMiddleware)
	}
	if config.GeneralRateLimit != nil {
		protected = config.GeneralRateLimit(protected)
	}

	protected.Get("/users", h.GetUsers)
	protected.Post("/users", WithValidation[dto.RegisterRequest](config.ValidationMiddleware, h.CreateUser))
	protected.Get("/users/{id}", h.GetUser)
	protected.Get("/users/{id}/posts", h.GetUserPosts)
}
