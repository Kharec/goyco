package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/gorm"
	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/repositories"
	"goyco/internal/services"
	"goyco/internal/testutils"
)

func newUserHandler(repo repositories.UserRepository) *UserHandler {
	return newUserHandlerWithSender(repo, &testutils.EmailSenderStub{})
}

func newUserHandlerWithSender(repo repositories.UserRepository, sender services.EmailSender) *UserHandler {
	cfg := &config.Config{
		JWT: config.JWTConfig{Secret: "secret", Expiration: 1},
		App: config.AppConfig{BaseURL: "https://test.example.com"},
	}
	mockRefreshRepo := &mockRefreshTokenRepository{}
	authService, err := services.NewAuthFacadeForTest(cfg, repo, nil, nil, mockRefreshRepo, sender)
	if err != nil {
		panic(fmt.Sprintf("Failed to create auth service: %v", err))
	}
	return NewUserHandler(repo, authService)
}

func TestUserHandlerGetUsers(t *testing.T) {
	var limit, offset int
	repo := testutils.NewUserRepositoryStub()
	repo.GetAllFn = func(l, o int) ([]database.User, error) {
		limit, offset = l, o
		return []database.User{{ID: 1}}, nil
	}

	handler := newUserHandler(repo)

	request := httptest.NewRequest(http.MethodGet, "/api/users?limit=5&offset=2", nil)
	recorder := httptest.NewRecorder()

	handler.GetUsers(recorder, request)

	if limit != 5 || offset != 2 {
		t.Fatalf("expected limit=5 offset=2, got %d %d", limit, offset)
	}

	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)
}

func TestUserHandlerGetUser(t *testing.T) {
	repo := testutils.NewUserRepositoryStub()
	handler := newUserHandler(repo)

	request := httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
	recorder := httptest.NewRecorder()
	handler.GetUser(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

	request = httptest.NewRequest(http.MethodGet, "/api/users/abc", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "abc"})
	recorder = httptest.NewRecorder()
	handler.GetUser(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

	repo.GetByIDFn = func(uint) (*database.User, error) { return nil, gorm.ErrRecordNotFound }
	request = httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	recorder = httptest.NewRecorder()
	handler.GetUser(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusNotFound)

	repo.GetByIDFn = func(id uint) (*database.User, error) {
		return &database.User{ID: id, Username: "user"}, nil
	}
	request = httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	recorder = httptest.NewRecorder()
	handler.GetUser(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)
}

func TestUserHandlerCreateUser(t *testing.T) {
	repo := testutils.NewUserRepositoryStub()
	repo.CreateFn = func(u *database.User) error {
		u.ID = 10
		return nil
	}
	sent := false
	handler := newUserHandlerWithSender(repo, &testutils.EmailSenderStub{SendFn: func(to, subject, body string) error {
		sent = true
		if to != "user@example.com" {
			t.Fatalf("expected email to user@example.com, got %q", to)
		}
		return nil
	}})

	request := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(`{"username":"user","email":"user@example.com","password":"Password123!"}`))
	recorder := httptest.NewRecorder()
	handler.CreateUser(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusCreated)

	var resp UserResponse
	_ = json.NewDecoder(recorder.Body).Decode(&resp)
	data := resp.Data.(map[string]any)
	if !resp.Success {
		t.Fatalf("expected success response")
	}
	if v, ok := data["verification_sent"].(bool); !ok || !v {
		t.Fatalf("expected verification_sent true, got %+v", data["verification_sent"])
	}
	userData := data["user"].(map[string]any)
	if _, ok := userData["password"]; ok {
		t.Fatalf("expected password field to be omitted, got %+v", userData)
	}
	if !sent {
		t.Fatalf("expected verification email to be sent")
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString("invalid"))
	handler.CreateUser(recorder, request)
	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid json, got %d", recorder.Result().StatusCode)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(`{"username":"","email":"","password":""}`))
	handler.CreateUser(recorder, request)
	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing fields, got %d", recorder.Result().StatusCode)
	}

	repo.GetByUsernameFn = func(string) (*database.User, error) {
		return &database.User{ID: 1}, nil
	}
	handler = newUserHandler(repo)
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(`{"username":"user","email":"user@example.com","password":"Password123!"}`))
	handler.CreateUser(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusConflict)
}

func TestUserHandlerGetUserPosts(t *testing.T) {
	repo := testutils.NewUserRepositoryStub()
	repo.GetPostsFn = func(userID uint, limit, offset int) ([]database.Post, error) {
		return []database.Post{{ID: 1, AuthorID: &userID}}, nil
	}
	handler := newUserHandler(repo)

	request := httptest.NewRequest(http.MethodGet, "/api/users/1/posts?limit=2&offset=1", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	recorder := httptest.NewRecorder()

	handler.GetUserPosts(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	repo.GetPostsFn = func(uint, int, int) ([]database.Post, error) {
		return nil, gorm.ErrInvalidValue
	}
	recorder = httptest.NewRecorder()
	handler.GetUserPosts(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusInternalServerError)
}

func TestUserHandlerDataSanitization(t *testing.T) {
	repo := testutils.NewUserRepositoryStub()
	repo.GetAllFn = func(l, o int) ([]database.User, error) {
		users := []database.User{
			{
				ID:                     1,
				Username:               "user1",
				Email:                  "user1@example.com",
				Password:               "hashedpassword",
				EmailVerified:          true,
				EmailVerifiedAt:        &[]time.Time{time.Now()}[0],
				EmailVerificationToken: "secret-token",
				PasswordResetToken:     "reset-token",
				Locked:                 false,
				CreatedAt:              time.Now(),
				UpdatedAt:              time.Now(),
			},
			{
				ID:                     2,
				Username:               "user2",
				Email:                  "user2@example.com",
				Password:               "another-hashed-password",
				EmailVerified:          false,
				EmailVerificationToken: "another-secret-token",
				Locked:                 true,
				CreatedAt:              time.Now(),
				UpdatedAt:              time.Now(),
			},
		}
		return users, nil
	}

	handler := newUserHandler(repo)

	request := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	recorder := httptest.NewRecorder()

	handler.GetUsers(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	var response map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := response["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data field in response")
	}

	users, ok := data["users"].([]any)
	if !ok {
		t.Fatalf("expected users field in data")
	}

	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	for i, userInterface := range users {
		user, ok := userInterface.(map[string]any)
		if !ok {
			t.Fatalf("expected user %d to be a map", i)
		}

		expectedFields := []string{"id", "username", "created_at", "updated_at"}
		for _, field := range expectedFields {
			if _, exists := user[field]; !exists {
				t.Errorf("expected field %s to be present in user %d", field, i)
			}
		}

		sensitiveFields := []string{"email", "password", "email_verified", "email_verified_at",
			"email_verification_token", "password_reset_token", "locked", "deleted_at"}
		for _, field := range sensitiveFields {
			if _, exists := user[field]; exists {
				t.Errorf("sensitive field %s should not be present in user %d", field, i)
			}
		}
	}
}

func TestUserHandler_PasswordValidation(t *testing.T) {

	tests := []struct {
		name           string
		password       string
		expectedStatus int
		description    string
	}{
		{
			name:           "valid password",
			password:       "Password123!",
			expectedStatus: http.StatusCreated,
			description:    "Valid passwords should be accepted",
		},
		{
			name:           "password without special chars",
			password:       "Password123",
			expectedStatus: http.StatusBadRequest,
			description:    "Passwords without special characters should be rejected",
		},
		{
			name:           "password too short",
			password:       "Pass1!",
			expectedStatus: http.StatusBadRequest,
			description:    "Passwords shorter than 8 characters should be rejected",
		},
		{
			name:           "password without letters",
			password:       "12345678!",
			expectedStatus: http.StatusBadRequest,
			description:    "Passwords without letters should be rejected",
		},
		{
			name:           "password without numbers",
			password:       "Password!",
			expectedStatus: http.StatusBadRequest,
			description:    "Passwords without numbers should be rejected",
		},
		{
			name:           "empty password",
			password:       "",
			expectedStatus: http.StatusBadRequest,
			description:    "Empty passwords should be rejected",
		},
		{
			name:           "password too long",
			password:       string(make([]byte, 129)),
			expectedStatus: http.StatusBadRequest,
			description:    "Passwords longer than 128 characters should be rejected",
		},
		{
			name:           "valid password with unicode",
			password:       "Pássw0rd123!",
			expectedStatus: http.StatusCreated,
			description:    "Valid passwords with unicode should be accepted",
		},
		{
			name:           "valid password with underscore",
			password:       "Password123_",
			expectedStatus: http.StatusCreated,
			description:    "Valid passwords with underscore should be accepted",
		},
		{
			name:           "valid password with hyphen",
			password:       "Password123-",
			expectedStatus: http.StatusCreated,
			description:    "Valid passwords with hyphen should be accepted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testutils.NewUserRepositoryStub()
			repo.CreateFn = func(user *database.User) error {
				return nil
			}
			repo.GetByUsernameFn = func(username string) (*database.User, error) {
				return nil, gorm.ErrRecordNotFound
			}
			repo.GetByEmailFn = func(email string) (*database.User, error) {
				return nil, gorm.ErrRecordNotFound
			}

			cfg := &config.Config{
				JWT: config.JWTConfig{Secret: "secret", Expiration: 1},
				App: config.AppConfig{BaseURL: "https://test.example.com"},
			}
			emailSender := &testutils.MockEmailSender{}
			mockRefreshRepo := &mockRefreshTokenRepository{}
			authService, err := services.NewAuthFacadeForTest(cfg, repo, nil, nil, mockRefreshRepo, emailSender)
			if err != nil {
				t.Fatalf("Failed to create auth service: %v", err)
			}
			handler := NewUserHandler(repo, authService)

			requestBody := fmt.Sprintf(`{"username":"testuser","email":"test@example.com","password":"%s"}`, tt.password)
			request := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(requestBody))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			handler.CreateUser(recorder, request)

			testutils.AssertHTTPStatus(t, recorder, tt.expectedStatus)
		})
	}
}
