package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"goyco/internal/database"
	"goyco/internal/dto"
	"goyco/internal/middleware"
	"goyco/internal/services"
	"goyco/internal/validation"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type CommonResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

type PaginationData struct {
	Count  int `json:"count"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type VoteCookieData struct {
	Type      database.VoteType `json:"type"`
	Timestamp int64             `json:"timestamp"`
}

func sendResponse(w http.ResponseWriter, statusCode int, success bool, message string, data any, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := CommonResponse{
		Success: success,
		Message: message,
		Data:    data,
		Error:   errMsg,
	}

	json.NewEncoder(w).Encode(response)
}

func SendSuccessResponse(w http.ResponseWriter, message string, data any) {
	sendResponse(w, http.StatusOK, true, message, data, "")
}

func SendCreatedResponse(w http.ResponseWriter, message string, data any) {
	sendResponse(w, http.StatusCreated, true, message, data, "")
}

func SendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	sendResponse(w, statusCode, false, "", nil, message)
}

func DecodeJSONRequest(w http.ResponseWriter, r *http.Request, req any) bool {
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		SendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return false
	}
	return true
}

func GetClientIP(r *http.Request) string {
	return middleware.GetSecureClientIP(r)
}

const (
	CookieMaxAgeDays        = 30
	SecondsPerDay           = 86400
	DefaultPaginationLimit  = 20
	DefaultPaginationOffset = 0
)

func SetVoteCookie(w http.ResponseWriter, r *http.Request, postID uint, voteType database.VoteType) {
	cookieName := fmt.Sprintf("vote_%d", postID)
	cookieValue := fmt.Sprintf("%s:%d", voteType, time.Now().Unix())

	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    cookieValue,
		Path:     "/",
		MaxAge:   SecondsPerDay * CookieMaxAgeDays,
		HttpOnly: true,
		Secure:   middleware.IsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	}

	http.SetCookie(w, cookie)
}

func GetVoteCookie(r *http.Request, postID uint) string {
	cookieName := fmt.Sprintf("vote_%d", postID)
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func ClearVoteCookie(w http.ResponseWriter, postID uint) {
	cookieName := fmt.Sprintf("vote_%d", postID)
	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
}

func SanitizeUser(user *database.User) dto.SanitizedUserDTO {
	if user == nil {
		return dto.SanitizedUserDTO{}
	}
	return dto.ToSanitizedUserDTO(user)
}

func SanitizeUsers(users []database.User) []dto.SanitizedUserDTO {
	return dto.ToSanitizedUserDTOs(users)
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit = DefaultPaginationLimit
	offset = DefaultPaginationOffset

	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	return limit, offset
}

func ValidateRedirectURL(redirectURL string) string {
	redirectURL = strings.TrimSpace(redirectURL)
	if redirectURL == "" || len(redirectURL) > 512 {
		return ""
	}

	if !strings.HasPrefix(redirectURL, "/") || strings.HasPrefix(redirectURL, "//") {
		return ""
	}

	parsed, err := url.Parse(redirectURL)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.User != nil || parsed.Path == "" {
		return ""
	}

	path := parsed.EscapedPath()
	if path == "" {
		path = parsed.Path
	}

	validated := path
	if parsed.RawQuery != "" {
		validated += "?" + parsed.RawQuery
	}
	if parsed.Fragment != "" {
		validated += "#" + parsed.Fragment
	}

	return validated
}

func ParseUintParam(w http.ResponseWriter, r *http.Request, paramName, entityName string) (uint, bool) {
	str := chi.URLParam(r, paramName)
	if str == "" {
		SendErrorResponse(w, entityName+" ID is required", http.StatusBadRequest)
		return 0, false
	}
	id, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		SendErrorResponse(w, "Invalid "+entityName+" ID", http.StatusBadRequest)
		return 0, false
	}
	return uint(id), true
}

func RequireAuth(w http.ResponseWriter, r *http.Request) (uint, bool) {
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == 0 {
		SendErrorResponse(w, "Authentication required", http.StatusUnauthorized)
		return 0, false
	}
	return userID, true
}

func NewVoteContext(r *http.Request) services.VoteContext {
	return services.VoteContext{
		UserID:    middleware.GetUserIDFromContext(r.Context()),
		IPAddress: GetClientIP(r),
		UserAgent: r.UserAgent(),
	}
}

func HandleRepoError(w http.ResponseWriter, err error, entityName string) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		SendErrorResponse(w, entityName+" not found", http.StatusNotFound)
	} else {
		SendErrorResponse(w, "Failed to retrieve "+entityName, http.StatusInternalServerError)
	}
	return false
}

var AuthErrorMapping = []struct {
	err  error
	msg  string
	code int
}{
	{services.ErrInvalidCredentials, "Invalid username or password", http.StatusUnauthorized},
	{services.ErrEmailNotVerified, "Please confirm your email before logging in", http.StatusForbidden},
	{services.ErrAccountLocked, "Your account has been locked. Please contact us for assistance.", http.StatusForbidden},
	{services.ErrUsernameTaken, "Username is already taken", http.StatusConflict},
	{services.ErrEmailTaken, "Email is already registered", http.StatusConflict},
	{services.ErrInvalidEmail, "Invalid email address", http.StatusBadRequest},
	{services.ErrPasswordTooShort, "Password must be at least 8 characters", http.StatusBadRequest},
	{services.ErrInvalidVerificationToken, "Invalid or expired verification token", http.StatusBadRequest},
	{services.ErrRefreshTokenExpired, "Refresh token has expired", http.StatusUnauthorized},
	{services.ErrRefreshTokenInvalid, "Invalid refresh token", http.StatusUnauthorized},
	{services.ErrInvalidDeletionToken, "This deletion link is invalid or has expired.", http.StatusBadRequest},
	{services.ErrDeletionRequestNotFound, "Deletion request not found", http.StatusBadRequest},
	{services.ErrUserNotFound, "User not found", http.StatusNotFound},
	{services.ErrEmailSenderUnavailable, "Email service is unavailable. Please try again later.", http.StatusServiceUnavailable},
}

func HandleServiceError(w http.ResponseWriter, err error, defaultMsg string, defaultCode int) bool {
	if err == nil {
		return true
	}

	for _, mapping := range AuthErrorMapping {
		if err == mapping.err || errors.Is(err, mapping.err) {
			SendErrorResponse(w, mapping.msg, mapping.code)
			return false
		}
	}

	errMsg := err.Error()
	for _, mapping := range AuthErrorMapping {
		if mapping.err.Error() == errMsg {
			SendErrorResponse(w, mapping.msg, mapping.code)
			return false
		}
	}

	SendErrorResponse(w, defaultMsg, defaultCode)
	return false
}

func GetValidatedDTO[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	dtoVal := middleware.GetValidatedDTOFromContext(r.Context())
	dtoTypeInContext := middleware.GetDTOTypeFromContext(r.Context())

	var dto *T
	needsValidation := false

	if dtoVal != nil {
		var ok bool
		dto, ok = dtoVal.(*T)
		if !ok {
			return nil, false
		}
		if dtoTypeInContext == nil {
			needsValidation = true
		}
	} else {
		var decoded T
		if err := json.NewDecoder(r.Body).Decode(&decoded); err != nil {
			SendErrorResponse(w, "Invalid JSON", http.StatusBadRequest)
			return nil, false
		}
		dto = &decoded
		needsValidation = true
	}

	if needsValidation {
		if err := validation.ValidateStruct(dto); err != nil {
			var errorMessages []string
			if structErr, ok := err.(*validation.StructValidationError); ok {
				errorMessages = make([]string, len(structErr.Errors))
				for i, fieldError := range structErr.Errors {
					errorMessages[i] = fieldError.Message
				}
			} else {
				errorMessages = []string{err.Error()}
			}

			errorMsg := strings.Join(errorMessages, "; ")
			SendErrorResponse(w, errorMsg, http.StatusBadRequest)
			return nil, false
		}
	}

	return dto, true
}

func WithValidation[T any](validationMiddleware func(http.Handler) http.Handler, handler http.HandlerFunc) http.HandlerFunc {
	if validationMiddleware == nil {
		return handler
	}
	var zero T
	dtoType := reflect.TypeOf(zero)
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.SetDTOTypeInContext(r.Context(), dtoType)
		validationMiddleware(handler).ServeHTTP(w, r.WithContext(ctx))
	}
}
