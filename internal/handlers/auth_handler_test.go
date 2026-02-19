package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/middleware"
	"goyco/internal/repositories"
	"goyco/internal/services"
	"goyco/internal/testutils"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func newAuthHandler(repo repositories.UserRepository) *AuthHandler {
	return newAuthHandlerWithSender(repo, &testutils.EmailSenderStub{})
}

func newAuthHandlerWithSender(repo repositories.UserRepository, sender services.EmailSender) *AuthHandler {
	cfg := &config.Config{
		JWT: config.JWTConfig{Secret: "secret", Expiration: 1},
		App: config.AppConfig{BaseURL: "https://test.example.com"},
	}

	mockRefreshRepo := &mockRefreshTokenRepository{}
	authService, err := services.NewAuthFacadeForTest(cfg, repo, nil, nil, mockRefreshRepo, sender)
	if err != nil {
		panic(fmt.Sprintf("Failed to create auth service: %v", err))
	}
	return NewAuthHandler(authService, repo)
}

type mockAuthService struct {
	loginFunc                           func(string, string) (*services.AuthResult, error)
	registerFunc                        func(string, string, string) (*services.RegistrationResult, error)
	confirmEmailFunc                    func(string) (*database.User, error)
	resendVerificationFunc              func(string) error
	requestPasswordResetFunc            func(string) error
	resetPasswordFunc                   func(string, string) error
	updateEmailFunc                     func(uint, string) (*database.User, error)
	updateUsernameFunc                  func(uint, string) (*database.User, error)
	updatePasswordFunc                  func(uint, string, string) (*database.User, error)
	deleteAccountFunc                   func(uint) error
	confirmAccountDeletionWithPostsFunc func(string, bool) error
	refreshAccessTokenFunc              func(string) (*services.AuthResult, error)
	revokeRefreshTokenFunc              func(string) error
	revokeAllUserTokensFunc             func(uint) error
	invalidateAllSessionsFunc           func(uint) error
	getAdminEmailFunc                   func() string
	verifyTokenFunc                     func(string) (uint, error)
	getUserIDFromDeletionTokenFunc      func(string) (uint, error)
	userHasPostsFunc                    func(uint) (bool, int64, error)
}

func (m *mockAuthService) Login(username, password string) (*services.AuthResult, error) {
	if m.loginFunc != nil {
		return m.loginFunc(username, password)
	}
	return &services.AuthResult{
		User:         &database.User{ID: 1, Username: username},
		AccessToken:  "access_token",
		RefreshToken: "refresh_token",
	}, nil
}

func (m *mockAuthService) Register(username, email, password string) (*services.RegistrationResult, error) {
	if m.registerFunc != nil {
		return m.registerFunc(username, email, password)
	}
	return &services.RegistrationResult{
		User:             &database.User{ID: 1, Username: username, Email: email},
		VerificationSent: true,
	}, nil
}

func (m *mockAuthService) ConfirmEmail(token string) (*database.User, error) {
	if m.confirmEmailFunc != nil {
		return m.confirmEmailFunc(token)
	}
	return &database.User{ID: 1}, nil
}

func (m *mockAuthService) ResendVerificationEmail(email string) error {
	if m.resendVerificationFunc != nil {
		return m.resendVerificationFunc(email)
	}
	return nil
}

func (m *mockAuthService) RequestPasswordReset(usernameOrEmail string) error {
	if m.requestPasswordResetFunc != nil {
		return m.requestPasswordResetFunc(usernameOrEmail)
	}
	return nil
}

func (m *mockAuthService) ResetPassword(token, newPassword string) error {
	if m.resetPasswordFunc != nil {
		return m.resetPasswordFunc(token, newPassword)
	}
	return nil
}

func (m *mockAuthService) UpdateEmail(userID uint, email string) (*database.User, error) {
	if m.updateEmailFunc != nil {
		return m.updateEmailFunc(userID, email)
	}
	return &database.User{ID: userID, Email: email}, nil
}

func (m *mockAuthService) UpdateUsername(userID uint, username string) (*database.User, error) {
	if m.updateUsernameFunc != nil {
		return m.updateUsernameFunc(userID, username)
	}
	return &database.User{ID: userID, Username: username}, nil
}

func (m *mockAuthService) UpdatePassword(userID uint, currentPassword, newPassword string) (*database.User, error) {
	if m.updatePasswordFunc != nil {
		return m.updatePasswordFunc(userID, currentPassword, newPassword)
	}
	return &database.User{ID: userID}, nil
}

func (m *mockAuthService) RequestAccountDeletion(userID uint) error {
	if m.deleteAccountFunc != nil {
		return m.deleteAccountFunc(userID)
	}
	return nil
}

func (m *mockAuthService) ConfirmAccountDeletionWithPosts(token string, deletePosts bool) error {
	if m.confirmAccountDeletionWithPostsFunc != nil {
		return m.confirmAccountDeletionWithPostsFunc(token, deletePosts)
	}
	return nil
}

func (m *mockAuthService) RefreshAccessToken(refreshToken string) (*services.AuthResult, error) {
	if m.refreshAccessTokenFunc != nil {
		return m.refreshAccessTokenFunc(refreshToken)
	}
	return &services.AuthResult{
		User:         &database.User{ID: 1, Username: "testuser"},
		AccessToken:  "new_access_token",
		RefreshToken: refreshToken,
	}, nil
}

func (m *mockAuthService) RevokeRefreshToken(refreshToken string) error {
	if m.revokeRefreshTokenFunc != nil {
		return m.revokeRefreshTokenFunc(refreshToken)
	}
	return nil
}

func (m *mockAuthService) RevokeAllUserTokens(userID uint) error {
	if m.revokeAllUserTokensFunc != nil {
		return m.revokeAllUserTokensFunc(userID)
	}
	return nil
}

func (m *mockAuthService) InvalidateAllSessions(userID uint) error {
	if m.invalidateAllSessionsFunc != nil {
		return m.invalidateAllSessionsFunc(userID)
	}
	return nil
}

func (m *mockAuthService) GetAdminEmail() string {
	if m.getAdminEmailFunc != nil {
		return m.getAdminEmailFunc()
	}
	return "admin@example.com"
}

func (m *mockAuthService) VerifyToken(tokenString string) (uint, error) {
	if m.verifyTokenFunc != nil {
		return m.verifyTokenFunc(tokenString)
	}
	return 1, nil
}

func (m *mockAuthService) GetUserIDFromDeletionToken(token string) (uint, error) {
	if m.getUserIDFromDeletionTokenFunc != nil {
		return m.getUserIDFromDeletionTokenFunc(token)
	}
	return 1, nil
}

func (m *mockAuthService) UserHasPosts(userID uint) (bool, int64, error) {
	if m.userHasPostsFunc != nil {
		return m.userHasPostsFunc(userID)
	}
	return false, 0, nil
}

type mockRefreshTokenRepository struct{}

func (m *mockRefreshTokenRepository) Create(token *database.RefreshToken) error {
	return nil
}

func (m *mockRefreshTokenRepository) GetByTokenHash(tokenHash string) (*database.RefreshToken, error) {
	return nil, gorm.ErrRecordNotFound
}

func (m *mockRefreshTokenRepository) DeleteByUserID(userID uint) error {
	return nil
}

func (m *mockRefreshTokenRepository) DeleteExpired() error {
	return nil
}

func (m *mockRefreshTokenRepository) DeleteByID(id uint) error {
	return nil
}

func (m *mockRefreshTokenRepository) GetByUserID(userID uint) ([]database.RefreshToken, error) {
	return []database.RefreshToken{}, nil
}

func (m *mockRefreshTokenRepository) CountByUserID(userID uint) (int64, error) {
	return 0, nil
}

func newMockAuthHandler(repo repositories.UserRepository, mockService *mockAuthService) *AuthHandler {
	return &AuthHandler{
		authService: mockService,
		userRepo:    repo,
	}
}

func TestAuthHandlerLoginSuccess(t *testing.T) {
	hashed, _ := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.DefaultCost)
	repo := &testutils.UserRepositoryStub{
		GetByUsernameFn: func(username string) (*database.User, error) {
			return &database.User{ID: 1, Username: username, Password: string(hashed), EmailVerified: true}, nil
		},
	}
	handler := newAuthHandler(repo)

	bodyStr := `{"username":"user","password":"Password123!"}`
	request := createLoginRequest(bodyStr)
	recorder := httptest.NewRecorder()

	handler.Login(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	var resp AuthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if !resp.Success || resp.Data == nil {
		t.Fatalf("expected success response, got %+v", resp)
	}
}

func TestAuthHandlerLoginWithLegacyPassword(t *testing.T) {
	legacyPassword := "password"
	hashed, _ := bcrypt.GenerateFromPassword([]byte(legacyPassword), bcrypt.DefaultCost)
	repo := &testutils.UserRepositoryStub{
		GetByUsernameFn: func(username string) (*database.User, error) {
			return &database.User{ID: 1, Username: username, Password: string(hashed), EmailVerified: true}, nil
		},
	}
	handler := newAuthHandler(repo)

	bodyStr := fmt.Sprintf(`{"username":"user","password":"%s"}`, legacyPassword)
	request := createLoginRequest(bodyStr)
	recorder := httptest.NewRecorder()

	handler.Login(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	var resp AuthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if !resp.Success || resp.Data == nil {
		t.Fatalf("expected success response for legacy password, got %+v", resp)
	}
}

func TestAuthHandlerLoginErrors(t *testing.T) {
	handler := newAuthHandler(&testutils.UserRepositoryStub{})

	recorder := httptest.NewRecorder()
	request := createLoginRequest("invalid")
	handler.Login(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

	recorder = httptest.NewRecorder()
	request = createLoginRequest(`{"username":" ","password":""}`)
	handler.Login(recorder, request)
	if recorder.Code != http.StatusBadRequest && recorder.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 400 or 401 for empty password, got %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = createLoginRequest(`{"username":"user","password":"WrongPass123!"}`)
	handler.Login(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusUnauthorized)

	hashed, _ := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.DefaultCost)
	repo := &testutils.UserRepositoryStub{GetByUsernameFn: func(string) (*database.User, error) {
		return &database.User{ID: 1, Username: "user", Password: string(hashed), EmailVerified: false}, nil
	}}
	handler = newAuthHandler(repo)
	recorder = httptest.NewRecorder()
	request = createLoginRequest(`{"username":"user","password":"Password123!"}`)
	handler.Login(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusForbidden)

	repo = &testutils.UserRepositoryStub{GetByUsernameFn: func(string) (*database.User, error) {
		return nil, errors.New("database offline")
	}}
	handler = newAuthHandler(repo)

	recorder = httptest.NewRecorder()
	request = createLoginRequest(`{"username":"user","password":"Password123!"}`)
	handler.Login(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusInternalServerError)

	if !strings.Contains(recorder.Body.String(), "Authentication failed") {
		t.Fatalf("expected response to include generic error message, got %q", recorder.Body.String())
	}
}

func TestAuthHandlerRegisterSuccess(t *testing.T) {
	repo := &testutils.UserRepositoryStub{
		GetByUsernameFn: func(string) (*database.User, error) {
			return nil, gorm.ErrRecordNotFound
		},
		CreateFn: func(user *database.User) error {
			user.ID = 1
			return nil
		},
	}

	sent := false
	handler := newAuthHandlerWithSender(repo, &testutils.EmailSenderStub{SendFn: func(to, subject, body string) error {
		sent = true
		return nil
	}})

	request := createRegisterRequest(`{"username":"newuser","email":"new@example.com","password":"Password123!"}`)
	recorder := httptest.NewRecorder()

	handler.Register(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusCreated)

	var resp AuthResponse
	_ = json.NewDecoder(recorder.Body).Decode(&resp)
	if !resp.Success {
		t.Fatalf("expected success response, got %v", resp)
	}

	if !sent {
		t.Fatalf("expected verification email to be sent")
	}
}

func TestAuthHandlerRegisterErrors(t *testing.T) {
	repo := &testutils.UserRepositoryStub{}
	handler := newAuthHandler(repo)

	recorder := httptest.NewRecorder()
	request := createRegisterRequest("invalid")
	handler.Register(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

	recorder = httptest.NewRecorder()
	request = createRegisterRequest(`{"username":"","email":"","password":""}`)
	handler.Register(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

	repo = &testutils.UserRepositoryStub{GetByUsernameFn: func(string) (*database.User, error) {
		return &database.User{ID: 1}, nil
	}}
	handler = newAuthHandler(repo)
	recorder = httptest.NewRecorder()
	request = createRegisterRequest(`{"username":"new","email":"taken@example.com","password":"Password123!"}`)
	handler.Register(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusConflict)

	repo = &testutils.UserRepositoryStub{
		GetByUsernameFn: func(string) (*database.User, error) {
			return nil, gorm.ErrRecordNotFound
		},
		GetByEmailFn: func(string) (*database.User, error) {
			return &database.User{ID: 2}, nil
		},
	}
	handler = newAuthHandler(repo)
	recorder = httptest.NewRecorder()
	request = createRegisterRequest(`{"username":"another","email":"taken@example.com","password":"Password123!"}`)
	handler.Register(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusConflict)
}

func TestAuthHandlerMe(t *testing.T) {
	repo := &testutils.UserRepositoryStub{
		GetByIDFn: func(id uint) (*database.User, error) {
			return &database.User{
				ID:            id,
				Username:      "user",
				Email:         "user@example.com",
				Password:      "secret",
				EmailVerified: true,
			}, nil
		},
	}
	handler := newAuthHandler(repo)

	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	recorder := httptest.NewRecorder()
	handler.Me(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusUnauthorized)

	request = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	ctx := context.WithValue(request.Context(), middleware.UserIDKey, uint(7))
	request = request.WithContext(ctx)
	recorder = httptest.NewRecorder()

	handler.Me(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	var resp AuthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	user, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("Expected user to be map[string]any, got %T", resp.Data)
	}

	if _, ok := user["password"]; ok {
		t.Fatalf("expected password field to be omitted, got %+v", user)
	}
}

func TestAuthHandlerConfirmEmail(t *testing.T) {
	repo := &testutils.UserRepositoryStub{}
	handler := newAuthHandler(repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/confirm", nil)
	handler.ConfirmEmail(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

	verified := false
	repo = &testutils.UserRepositoryStub{
		GetByVerificationFn: func(token string) (*database.User, error) {
			if token == "" {
				t.Fatalf("expected hashed token to be provided")
			}
			return &database.User{ID: 3, Username: "user", EmailVerified: false}, nil
		},
		UpdateFn: func(u *database.User) error {
			verified = u.EmailVerified
			return nil
		},
	}
	handler = newAuthHandler(repo)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/auth/confirm?token=abc123", nil)
	handler.ConfirmEmail(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	if !verified {
		t.Fatalf("expected update to mark user as verified")
	}
}

func TestAuthHandlerRequestPasswordReset(t *testing.T) {
	repo := &testutils.UserRepositoryStub{
		GetByEmailFn: func(email string) (*database.User, error) {
			if email == "user@example.com" {
				return &database.User{ID: 1, Username: "user", Email: "user@example.com"}, nil
			}
			return nil, gorm.ErrRecordNotFound
		},
	}
	handler := newAuthHandlerWithSender(repo, &testutils.EmailSenderStub{SendFn: func(to, subject, body string) error {
		return nil
	}})

	recorder := httptest.NewRecorder()
	request := createForgotPasswordRequest(`{"username_or_email":"user@example.com"}`)
	handler.RequestPasswordReset(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	repo = &testutils.UserRepositoryStub{
		GetByUsernameFn: func(username string) (*database.User, error) {
			if username == "user" {
				return &database.User{ID: 1, Username: "user", Email: "user@example.com"}, nil
			}
			return nil, gorm.ErrRecordNotFound
		},
	}
	handler = newAuthHandlerWithSender(repo, &testutils.EmailSenderStub{SendFn: func(to, subject, body string) error {
		return nil
	}})

	recorder = httptest.NewRecorder()
	request = createForgotPasswordRequest(`{"username_or_email":"user"}`)
	handler.RequestPasswordReset(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	recorder = httptest.NewRecorder()
	request = createForgotPasswordRequest(`{"username_or_email":""}`)
	handler.RequestPasswordReset(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

	recorder = httptest.NewRecorder()
	request = createForgotPasswordRequest(`invalid json`)
	handler.RequestPasswordReset(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)
}

func TestAuthHandlerResetPassword(t *testing.T) {
	repo := &testutils.UserRepositoryStub{}
	handler := newAuthHandler(repo)

	recorder := httptest.NewRecorder()
	request := createResetPasswordRequest(`{"new_password":"NewPassword123!"}`)
	handler.ResetPassword(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

	recorder = httptest.NewRecorder()
	request = createResetPasswordRequest(`{"token":"valid_token"}`)
	handler.ResetPassword(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

	recorder = httptest.NewRecorder()
	request = createResetPasswordRequest(`{"token":"valid_token","new_password":"short"}`)
	handler.ResetPassword(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

	recorder = httptest.NewRecorder()
	request = createResetPasswordRequest(`invalid json`)
	handler.ResetPassword(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)
}

func TestAuthHandlerResetPasswordServiceOutcomes(t *testing.T) {
	repo := &testutils.UserRepositoryStub{}
	tests := []struct {
		name           string
		setup          func(*mockAuthService)
		expectedStatus int
		expectedError  string
		expectedMsg    string
	}{
		{
			name: "expired token",
			setup: func(ms *mockAuthService) {
				ms.resetPasswordFunc = func(token, newPassword string) error {
					return fmt.Errorf("token expired")
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "The reset link has expired",
		},
		{
			name: "invalid token",
			setup: func(ms *mockAuthService) {
				ms.resetPasswordFunc = func(token, newPassword string) error {
					return fmt.Errorf("token invalid")
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "The reset link is invalid",
		},
		{
			name: "unexpected error",
			setup: func(ms *mockAuthService) {
				ms.resetPasswordFunc = func(token, newPassword string) error {
					return fmt.Errorf("smtp outage")
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Unable to reset password",
		},
		{
			name: "success",
			setup: func(ms *mockAuthService) {
				ms.resetPasswordFunc = func(token, newPassword string) error {
					return nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedMsg:    "Password reset successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockAuthService{}
			if tt.setup != nil {
				tt.setup(mockService)
			}

			handler := newMockAuthHandler(repo, mockService)

			request := createResetPasswordRequest(`{"token":"abc","new_password":"Password123!"}`)
			recorder := httptest.NewRecorder()

			handler.ResetPassword(recorder, request)

			testutils.AssertHTTPStatus(t, recorder, tt.expectedStatus)

			var resp AuthResponse
			_ = json.NewDecoder(recorder.Body).Decode(&resp)

			if tt.expectedError != "" {
				if !strings.Contains(resp.Error, tt.expectedError) {
					t.Fatalf("expected error to contain %q, got %q", tt.expectedError, resp.Error)
				}
				return
			}

			if resp.Message == "" || !strings.Contains(resp.Message, tt.expectedMsg) {
				t.Fatalf("expected success message containing %q, got %q", tt.expectedMsg, resp.Message)
			}
			if !resp.Success {
				t.Fatalf("expected success response")
			}
		})
	}
}

func TestAuthHandlerUpdateEmail(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		userID         uint
		mockSetup      func(*testutils.UserRepositoryStub)
		expectedStatus int
		expectedError  string
	}{
		{
			name:        "valid email update",
			requestBody: `{"email": "new@example.com"}`,
			userID:      1,
			mockSetup: func(repo *testutils.UserRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.User, error) {
					return &database.User{ID: id, Email: "old@example.com"}, nil
				}
				repo.UpdateFn = func(user *database.User) error { return nil }
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing user context",
			requestBody:    `{"email": "new@example.com"}`,
			userID:         0,
			mockSetup:      func(repo *testutils.UserRepositoryStub) {},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authentication required",
		},
		{
			name:           "invalid JSON",
			requestBody:    `invalid json`,
			userID:         1,
			mockSetup:      func(repo *testutils.UserRepositoryStub) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid JSON",
		},
		{
			name:           "empty email",
			requestBody:    `{"email": ""}`,
			userID:         1,
			mockSetup:      func(repo *testutils.UserRepositoryStub) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "email is required",
		},
		{
			name:        "email already taken",
			requestBody: `{"email": "taken@example.com"}`,
			userID:      1,
			mockSetup: func(repo *testutils.UserRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.User, error) {
					return &database.User{ID: id, Email: "old@example.com"}, nil
				}
			},
			expectedStatus: http.StatusConflict,
			expectedError:  "That email is already in use",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &testutils.UserRepositoryStub{}
			tt.mockSetup(repo)

			mockService := &mockAuthService{}
			if tt.name == "email already taken" {
				mockService.updateEmailFunc = func(userID uint, email string) (*database.User, error) {
					return nil, services.ErrEmailTaken
				}
			}

			handler := newMockAuthHandler(repo, mockService)

			request := createUpdateEmailRequest(tt.requestBody)
			if tt.userID > 0 {
				ctx := context.WithValue(request.Context(), middleware.UserIDKey, tt.userID)
				request = request.WithContext(ctx)
			}
			recorder := httptest.NewRecorder()

			handler.UpdateEmail(recorder, request)

			testutils.AssertHTTPStatus(t, recorder, tt.expectedStatus)

			if tt.expectedError != "" {
				var resp AuthResponse
				json.NewDecoder(recorder.Body).Decode(&resp)
				if !strings.Contains(resp.Error, tt.expectedError) {
					t.Fatalf("expected error to contain %q, got %q", tt.expectedError, resp.Error)
				}
			}
		})
	}
}

func TestAuthHandlerUpdateUsername(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		userID         uint
		mockSetup      func(*testutils.UserRepositoryStub)
		expectedStatus int
		expectedError  string
	}{
		{
			name:        "valid username update",
			requestBody: `{"username": "newusername"}`,
			userID:      1,
			mockSetup: func(repo *testutils.UserRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.User, error) {
					return &database.User{ID: id, Username: "oldusername"}, nil
				}
				repo.UpdateFn = func(user *database.User) error { return nil }
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing user context",
			requestBody:    `{"username": "newusername"}`,
			userID:         0,
			mockSetup:      func(repo *testutils.UserRepositoryStub) {},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authentication required",
		},
		{
			name:           "empty username",
			requestBody:    `{"username": ""}`,
			userID:         1,
			mockSetup:      func(repo *testutils.UserRepositoryStub) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "username is required",
		},
		{
			name:        "username already taken",
			requestBody: `{"username": "takenuser"}`,
			userID:      1,
			mockSetup: func(repo *testutils.UserRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.User, error) {
					return &database.User{ID: id, Username: "oldusername"}, nil
				}
			},
			expectedStatus: http.StatusConflict,
			expectedError:  "That username is already taken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &testutils.UserRepositoryStub{}
			tt.mockSetup(repo)

			mockService := &mockAuthService{}
			if tt.name == "username already taken" {
				mockService.updateUsernameFunc = func(userID uint, username string) (*database.User, error) {
					return nil, services.ErrUsernameTaken
				}
			}

			handler := newMockAuthHandler(repo, mockService)

			request := createUpdateUsernameRequest(tt.requestBody)
			if tt.userID > 0 {
				ctx := context.WithValue(request.Context(), middleware.UserIDKey, tt.userID)
				request = request.WithContext(ctx)
			}
			recorder := httptest.NewRecorder()

			handler.UpdateUsername(recorder, request)

			testutils.AssertHTTPStatus(t, recorder, tt.expectedStatus)

			if tt.expectedError != "" {
				var resp AuthResponse
				json.NewDecoder(recorder.Body).Decode(&resp)
				if !strings.Contains(resp.Error, tt.expectedError) {
					t.Fatalf("expected error to contain %q, got %q", tt.expectedError, resp.Error)
				}
			}
		})
	}
}

func TestAuthHandlerUpdatePassword(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		userID         uint
		mockSetup      func(*testutils.UserRepositoryStub)
		expectedStatus int
		expectedError  string
	}{
		{
			name:        "valid password update",
			requestBody: `{"current_password": "OldPass123!", "new_password": "NewPassword123!"}`,
			userID:      1,
			mockSetup: func(repo *testutils.UserRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.User, error) {
					hashed, _ := bcrypt.GenerateFromPassword([]byte("OldPass123!"), bcrypt.DefaultCost)
					return &database.User{ID: id, Password: string(hashed)}, nil
				}
				repo.UpdateFn = func(user *database.User) error { return nil }
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing user context",
			requestBody:    `{"current_password": "OldPass123!", "new_password": "NewPassword123!"}`,
			userID:         0,
			mockSetup:      func(repo *testutils.UserRepositoryStub) {},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authentication required",
		},
		{
			name:           "empty current password",
			requestBody:    `{"current_password": "", "new_password": "NewPassword123!"}`,
			userID:         1,
			mockSetup:      func(repo *testutils.UserRepositoryStub) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "current_password is required",
		},
		{
			name:           "empty new password",
			requestBody:    `{"current_password": "OldPass123!", "new_password": ""}`,
			userID:         1,
			mockSetup:      func(repo *testutils.UserRepositoryStub) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "new_password is required",
		},
		{
			name:           "short new password",
			requestBody:    `{"current_password": "OldPass123!", "new_password": "short"}`,
			userID:         1,
			mockSetup:      func(repo *testutils.UserRepositoryStub) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "new_password must be at least 8 characters",
		},
		{
			name:        "incorrect current password",
			requestBody: `{"current_password": "WrongPass123!", "new_password": "NewPassword123!"}`,
			userID:      1,
			mockSetup: func(repo *testutils.UserRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.User, error) {
					hashed, _ := bcrypt.GenerateFromPassword([]byte("CorrectPass123!"), bcrypt.DefaultCost)
					return &database.User{ID: id, Password: string(hashed)}, nil
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Current password is incorrect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &testutils.UserRepositoryStub{}
			tt.mockSetup(repo)
			handler := newAuthHandler(repo)

			request := createUpdatePasswordRequest(tt.requestBody)
			if tt.userID > 0 {
				ctx := context.WithValue(request.Context(), middleware.UserIDKey, tt.userID)
				request = request.WithContext(ctx)
			}
			recorder := httptest.NewRecorder()

			handler.UpdatePassword(recorder, request)

			testutils.AssertHTTPStatus(t, recorder, tt.expectedStatus)

			if tt.expectedError != "" {
				var resp AuthResponse
				json.NewDecoder(recorder.Body).Decode(&resp)
				if !strings.Contains(resp.Error, tt.expectedError) {
					t.Fatalf("expected error to contain %q, got %q", tt.expectedError, resp.Error)
				}
			}
		})
	}
}

func TestAuthHandlerDeleteAccount(t *testing.T) {
	tests := []struct {
		name           string
		userID         uint
		mockSetup      func(*testutils.UserRepositoryStub)
		expectedStatus int
		expectedError  string
	}{
		{
			name:   "valid account deletion request",
			userID: 1,
			mockSetup: func(repo *testutils.UserRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.User, error) {
					return &database.User{ID: id, Username: "user", Email: "user@example.com"}, nil
				}
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing user context",
			userID:         0,
			mockSetup:      func(repo *testutils.UserRepositoryStub) {},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authentication required",
		},
		{
			name:   "email service unavailable",
			userID: 1,
			mockSetup: func(repo *testutils.UserRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.User, error) {
					return &database.User{ID: id, Username: "user", Email: "user@example.com"}, nil
				}
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedError:  "Account deletion isn't available right now",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &testutils.UserRepositoryStub{}
			tt.mockSetup(repo)

			mockService := &mockAuthService{}
			if tt.name == "email service unavailable" {
				mockService.deleteAccountFunc = func(userID uint) error {
					return services.ErrEmailSenderUnavailable
				}
			}

			handler := newMockAuthHandler(repo, mockService)

			request := httptest.NewRequest(http.MethodDelete, "/api/auth/account", nil)
			if tt.userID > 0 {
				ctx := context.WithValue(request.Context(), middleware.UserIDKey, tt.userID)
				request = request.WithContext(ctx)
			}
			recorder := httptest.NewRecorder()

			handler.DeleteAccount(recorder, request)

			testutils.AssertHTTPStatus(t, recorder, tt.expectedStatus)

			if tt.expectedError != "" {
				var resp AuthResponse
				json.NewDecoder(recorder.Body).Decode(&resp)
				if !strings.Contains(resp.Error, tt.expectedError) {
					t.Fatalf("expected error to contain %q, got %q", tt.expectedError, resp.Error)
				}
			}
		})
	}
}

func TestAuthHandlerResendVerificationEmail(t *testing.T) {
	makeRequest := func(body string, setup func(*mockAuthService)) (*httptest.ResponseRecorder, AuthResponse) {
		request := createResendVerificationRequest(body)

		repo := &testutils.UserRepositoryStub{}
		mockService := &mockAuthService{}
		if setup != nil {
			setup(mockService)
		}

		handler := newMockAuthHandler(repo, mockService)
		recorder := httptest.NewRecorder()
		handler.ResendVerificationEmail(recorder, request)

		var resp AuthResponse
		_ = json.NewDecoder(recorder.Body).Decode(&resp)
		return recorder, resp
	}

	tests := []struct {
		name           string
		body           string
		setup          func(*mockAuthService)
		expectedStatus int
		expectedError  string
		expectedMsg    string
	}{
		{
			name:           "invalid json",
			body:           "not-json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid JSON",
		},
		{
			name:           "missing email",
			body:           `{}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "email is required",
		},
		{
			name: "account not found",
			body: `{"email":"user@example.com"}`,
			setup: func(ms *mockAuthService) {
				ms.resendVerificationFunc = func(email string) error {
					return services.ErrInvalidCredentials
				}
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "No account found with this email address",
		},
		{
			name: "invalid email format",
			body: `{"email":"user@example.com"}`,
			setup: func(ms *mockAuthService) {
				ms.resendVerificationFunc = func(email string) error {
					return services.ErrInvalidEmail
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid email address format",
		},
		{
			name: "email already verified",
			body: `{"email":"user@example.com"}`,
			setup: func(ms *mockAuthService) {
				ms.resendVerificationFunc = func(email string) error {
					return fmt.Errorf("email already verified")
				}
			},
			expectedStatus: http.StatusConflict,
			expectedError:  "This email address is already verified",
		},
		{
			name: "rate limited",
			body: `{"email":"user@example.com"}`,
			setup: func(ms *mockAuthService) {
				ms.resendVerificationFunc = func(email string) error {
					return fmt.Errorf("verification email sent recently, please wait before requesting another")
				}
			},
			expectedStatus: http.StatusTooManyRequests,
			expectedError:  "Please wait 5 minutes before requesting another verification email",
		},
		{
			name: "email service unavailable",
			body: `{"email":"user@example.com"}`,
			setup: func(ms *mockAuthService) {
				ms.resendVerificationFunc = func(email string) error {
					return services.ErrEmailSenderUnavailable
				}
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedError:  "We couldn't send the verification email. Try again later.",
		},
		{
			name: "unexpected error",
			body: `{"email":"user@example.com"}`,
			setup: func(ms *mockAuthService) {
				ms.resendVerificationFunc = func(email string) error {
					return fmt.Errorf("smtp failed")
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Unable to resend verification email",
		},
		{
			name:           "success",
			body:           `{"email":"user@example.com"}`,
			expectedStatus: http.StatusOK,
			expectedMsg:    "Verification email sent successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr, resp := makeRequest(tt.body, tt.setup)

			if rr.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectedError != "" {
				if !strings.Contains(resp.Error, tt.expectedError) {
					t.Fatalf("expected error to contain %q, got %q", tt.expectedError, resp.Error)
				}
				return
			}

			if resp.Message != tt.expectedMsg {
				t.Fatalf("expected message %q, got %q", tt.expectedMsg, resp.Message)
			}

			if !resp.Success {
				t.Fatalf("expected success response")
			}
		})
	}
}

func TestAuthHandlerConfirmAccountDeletion(t *testing.T) {
	repo := &testutils.UserRepositoryStub{}

	tests := []struct {
		name              string
		body              string
		setup             func(*mockAuthService)
		expectedStatus    int
		expectedError     string
		expectedMessage   string
		expectedSuccess   bool
		expectedPostsFlag *bool
	}{
		{
			name:           "invalid json",
			body:           "not-json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid JSON",
		},
		{
			name:           "missing token",
			body:           `{}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "token is required",
		},
		{
			name: "invalid token from service",
			body: `{"token":"abc"}`,
			setup: func(ms *mockAuthService) {
				ms.confirmAccountDeletionWithPostsFunc = func(token string, deletePosts bool) error {
					return services.ErrInvalidDeletionToken
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "This deletion link is invalid or has expired.",
		},
		{
			name: "email sender unavailable",
			body: `{"token":"abc"}`,
			setup: func(ms *mockAuthService) {
				ms.confirmAccountDeletionWithPostsFunc = func(token string, deletePosts bool) error {
					return services.ErrEmailSenderUnavailable
				}
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedError:  "Account deletion isn't available right now because email delivery is disabled.",
		},
		{
			name: "deletion succeeds but notification fails",
			body: `{"token":"abc","delete_posts":true}`,
			setup: func(ms *mockAuthService) {
				ms.confirmAccountDeletionWithPostsFunc = func(token string, deletePosts bool) error {
					return services.ErrDeletionEmailFailed
				}
			},
			expectedStatus:    http.StatusOK,
			expectedMessage:   "Your account has been deleted, but we couldn't send the confirmation email.",
			expectedSuccess:   true,
			expectedPostsFlag: func() *bool { b := true; return &b }(),
		},
		{
			name: "successful confirmation",
			body: `{"token":"abc","delete_posts":false}`,
			setup: func(ms *mockAuthService) {
				ms.confirmAccountDeletionWithPostsFunc = func(token string, deletePosts bool) error {
					if token != "abc" || deletePosts {
						return fmt.Errorf("unexpected arguments")
					}
					return nil
				}
			},
			expectedStatus:    http.StatusOK,
			expectedMessage:   "Your account has been deleted.",
			expectedSuccess:   true,
			expectedPostsFlag: func() *bool { b := false; return &b }(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockAuthService{}
			if tt.setup != nil {
				tt.setup(mockService)
			}

			handler := newMockAuthHandler(repo, mockService)

			request := createConfirmAccountDeletionRequest(tt.body)
			recorder := httptest.NewRecorder()

			handler.ConfirmAccountDeletion(recorder, request)

			testutils.AssertHTTPStatus(t, recorder, tt.expectedStatus)

			var resp AuthResponse
			_ = json.NewDecoder(recorder.Body).Decode(&resp)

			if tt.expectedError != "" {
				if !strings.Contains(resp.Error, tt.expectedError) {
					t.Fatalf("expected error to contain %q, got %q", tt.expectedError, resp.Error)
				}
				return
			}

			if tt.expectedSuccess != resp.Success {
				t.Fatalf("expected success=%t, got %t", tt.expectedSuccess, resp.Success)
			}

			if tt.expectedMessage != "" && tt.expectedMessage != resp.Message {
				t.Fatalf("expected message %q, got %q", tt.expectedMessage, resp.Message)
			}

			if tt.expectedPostsFlag != nil {
				data, ok := resp.Data.(map[string]any)
				if !ok {
					t.Fatalf("expected data map, got %#v", resp.Data)
				}
				postsDeleted, ok := data["posts_deleted"].(bool)
				if !ok || postsDeleted != *tt.expectedPostsFlag {
					t.Fatalf("expected posts_deleted=%t, got %#v", *tt.expectedPostsFlag, data["posts_deleted"])
				}
			}
		})
	}
}

func TestAuthHandlerLogout(t *testing.T) {
	handler := newAuthHandler(&testutils.UserRepositoryStub{})

	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	recorder := httptest.NewRecorder()

	handler.Logout(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	var resp AuthResponse
	json.NewDecoder(recorder.Body).Decode(&resp)
	if !resp.Success || resp.Message != "Logged out successfully" {
		t.Fatalf("expected success logout response, got %+v", resp)
	}
}

func TestAuthHandler_EdgeCases(t *testing.T) {
	authService := &mockAuthService{}
	userRepo := testutils.NewUserRepositoryStub()
	handler := NewAuthHandler(authService, userRepo)

	t.Run("Login with empty username", func(t *testing.T) {
		body := bytes.NewBufferString(`{"username":"","password":"Password123!"}`)
		req := httptest.NewRequest("POST", "/api/auth/login", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Login(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for empty username, got %d", w.Code)
		}
	})

	t.Run("Login with empty password", func(t *testing.T) {
		body := bytes.NewBufferString(`{"username":"testuser","password":""}`)
		req := httptest.NewRequest("POST", "/api/auth/login", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Login(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for empty password, got %d", w.Code)
		}
	})

	t.Run("Register with very long username", func(t *testing.T) {
		longUsername := strings.Repeat("a", 100)
		body := bytes.NewBufferString(fmt.Sprintf(`{"username":"%s","email":"test@example.com","password":"Password123!"}`, longUsername))
		req := httptest.NewRequest("POST", "/api/auth/register", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Register(w, req)

		testutils.AssertHTTPStatus(t, w, http.StatusBadRequest)
	})

	t.Run("Register with invalid email format", func(t *testing.T) {
		body := bytes.NewBufferString(`{"username":"testuser","email":"invalid-email","password":"Password123!"}`)
		req := httptest.NewRequest("POST", "/api/auth/register", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Register(w, req)

		testutils.AssertHTTPStatus(t, w, http.StatusBadRequest)
	})
}

func TestAuthHandler_ConcurrentAccess(t *testing.T) {
	authService := &mockAuthService{
		loginFunc: func(username, password string) (*services.AuthResult, error) {
			return &services.AuthResult{
				User:        &database.User{ID: 1, Username: username},
				AccessToken: "access_token",
			}, nil
		},
	}
	userRepo := testutils.NewUserRepositoryStub()
	handler := NewAuthHandler(authService, userRepo)

	t.Run("Concurrent login attempts", func(t *testing.T) {
		concurrency := 10
		done := make(chan bool, concurrency)

		for range concurrency {
			go func() {
				req := createLoginRequest(`{"username":"testuser","password":"Password123!"}`)
				w := httptest.NewRecorder()

				handler.Login(w, req)

				testutils.AssertHTTPStatus(t, w, http.StatusOK)
				done <- true
			}()
		}

		for range concurrency {
			<-done
		}
	})
}

func TestAuthHandler_RefreshToken(t *testing.T) {
	authService := &mockAuthService{}
	userRepo := testutils.NewUserRepositoryStub()
	handler := NewAuthHandler(authService, userRepo)

	t.Run("Successful_Refresh", func(t *testing.T) {
		authService.refreshAccessTokenFunc = func(refreshToken string) (*services.AuthResult, error) {
			return &services.AuthResult{
				User:         &database.User{ID: 1, Username: "testuser"},
				AccessToken:  "new_access_token",
				RefreshToken: refreshToken,
			}, nil
		}

		req := createRefreshTokenRequest(`{"refresh_token":"valid_refresh_token"}`)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RefreshToken(w, req)

		testutils.AssertSuccessResponse(t, w)
	})

	t.Run("Invalid_Request_Body", func(t *testing.T) {
		req := createRefreshTokenRequest(`invalid json`)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RefreshToken(w, req)

		testutils.AssertHTTPStatus(t, w, http.StatusBadRequest)
	})

	t.Run("Missing_Refresh_Token", func(t *testing.T) {
		req := createRefreshTokenRequest(`{"refresh_token":""}`)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RefreshToken(w, req)

		testutils.AssertHTTPStatus(t, w, http.StatusBadRequest)
	})

	t.Run("Expired_Refresh_Token", func(t *testing.T) {
		authService.refreshAccessTokenFunc = func(refreshToken string) (*services.AuthResult, error) {
			return nil, services.ErrRefreshTokenExpired
		}

		req := createRefreshTokenRequest(`{"refresh_token":"expired_token"}`)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RefreshToken(w, req)

		testutils.AssertHTTPStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("Invalid_Refresh_Token", func(t *testing.T) {
		authService.refreshAccessTokenFunc = func(refreshToken string) (*services.AuthResult, error) {
			return nil, services.ErrRefreshTokenInvalid
		}

		req := createRefreshTokenRequest(`{"refresh_token":"invalid_token"}`)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RefreshToken(w, req)

		testutils.AssertHTTPStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("Account_Locked", func(t *testing.T) {
		authService.refreshAccessTokenFunc = func(refreshToken string) (*services.AuthResult, error) {
			return nil, services.ErrAccountLocked
		}

		req := createRefreshTokenRequest(`{"refresh_token":"locked_token"}`)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RefreshToken(w, req)

		testutils.AssertHTTPStatus(t, w, http.StatusForbidden)
	})

	t.Run("Internal_Error", func(t *testing.T) {
		authService.refreshAccessTokenFunc = func(refreshToken string) (*services.AuthResult, error) {
			return nil, fmt.Errorf("internal error")
		}

		req := createRefreshTokenRequest(`{"refresh_token":"error_token"}`)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RefreshToken(w, req)

		testutils.AssertHTTPStatus(t, w, http.StatusInternalServerError)
	})
}

func TestAuthHandler_RevokeToken(t *testing.T) {
	authService := &mockAuthService{}
	userRepo := testutils.NewUserRepositoryStub()
	handler := NewAuthHandler(authService, userRepo)

	t.Run("Successful_Revoke", func(t *testing.T) {
		authService.revokeRefreshTokenFunc = func(refreshToken string) error {
			return nil
		}

		req := createRevokeTokenRequest(`{"refresh_token":"token_to_revoke"}`)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RevokeToken(w, req)

		testutils.AssertSuccessResponse(t, w)
	})

	t.Run("Invalid_Request_Body", func(t *testing.T) {
		req := createRevokeTokenRequest(`invalid json`)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RevokeToken(w, req)

		testutils.AssertHTTPStatus(t, w, http.StatusBadRequest)
	})

	t.Run("Missing_Refresh_Token", func(t *testing.T) {
		req := createRevokeTokenRequest(`{"refresh_token":""}`)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RevokeToken(w, req)

		testutils.AssertHTTPStatus(t, w, http.StatusBadRequest)
	})

	t.Run("Revoke_Error", func(t *testing.T) {
		authService.revokeRefreshTokenFunc = func(refreshToken string) error {
			return fmt.Errorf("revoke failed")
		}

		req := createRevokeTokenRequest(`{"refresh_token":"token"}`)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RevokeToken(w, req)

		testutils.AssertHTTPStatus(t, w, http.StatusInternalServerError)
	})
}

func TestAuthHandler_RevokeAllTokens(t *testing.T) {
	authService := &mockAuthService{}
	userRepo := testutils.NewUserRepositoryStub()
	handler := NewAuthHandler(authService, userRepo)

	t.Run("Successful_Revoke_All", func(t *testing.T) {
		authService.revokeAllUserTokensFunc = func(userID uint) error {
			return nil
		}

		req := httptest.NewRequest("POST", "/api/auth/revoke-all", nil)
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uint(1)))
		w := httptest.NewRecorder()

		handler.RevokeAllTokens(w, req)

		testutils.AssertSuccessResponse(t, w)
	})

	t.Run("Unauthenticated_Request", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/auth/revoke-all", nil)
		w := httptest.NewRecorder()

		handler.RevokeAllTokens(w, req)

		testutils.AssertHTTPStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("Revoke_Error", func(t *testing.T) {
		authService.revokeAllUserTokensFunc = func(userID uint) error {
			return fmt.Errorf("revoke failed")
		}

		req := httptest.NewRequest("POST", "/api/auth/revoke-all", nil)
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uint(1)))
		w := httptest.NewRecorder()

		handler.RevokeAllTokens(w, req)

		testutils.AssertHTTPStatus(t, w, http.StatusInternalServerError)
	})
}
