package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
	"goyco/internal/database"
	"goyco/internal/middleware"
	"goyco/internal/security"
	"goyco/internal/testutils"
	"goyco/internal/validation"
)

func TestPostHandler_XSSProtection_Comprehensive(t *testing.T) {
	maliciousInputs := testutils.GetMaliciousInputs()

	for _, payload := range maliciousInputs.XSSPayloads {
		t.Run("XSS_"+payload[:minLen(20, len(payload))], func(t *testing.T) {
			repo := &testutils.PostRepositoryStub{
				CreateFn: func(post *database.Post) error {
					sanitizedTitle := security.SanitizeInput(payload)
					if post.Title != sanitizedTitle {
						t.Errorf("Expected sanitized title, got %q", post.Title)
					}
					return nil
				},
			}

			handler := NewPostHandler(repo, nil, nil)

			postData := map[string]string{
				"title":   payload,
				"url":     "https://example.com",
				"content": "Test content",
			}

			body, _ := json.Marshal(postData)
			request := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(body))
			request.Header.Set("Content-Type", "application/json")
			request = request.WithContext(context.WithValue(request.Context(), middleware.UserIDKey, uint(1)))
			recorder := httptest.NewRecorder()

			handler.CreatePost(recorder, request)

			testutils.AssertHTTPStatus(t, recorder, http.StatusCreated)
		})
	}
}

func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestPostHandler_InputValidation(t *testing.T) {
	tests := []struct {
		name           string
		title          string
		content        string
		url            string
		expectedStatus int
		description    string
	}{
		{
			name:           "title too long",
			title:          string(make([]byte, 201)),
			content:        "Normal content",
			url:            "https://example.com",
			expectedStatus: http.StatusBadRequest,
			description:    "Title should be limited to 200 characters",
		},
		{
			name:           "content too long",
			title:          "Normal title",
			content:        string(make([]byte, 10001)),
			url:            "https://example.com",
			expectedStatus: http.StatusBadRequest,
			description:    "Content should be limited to 10,000 characters",
		},
		{
			name:           "invalid URL protocol",
			title:          "Normal title",
			content:        "Normal content",
			url:            "ftp://example.com",
			expectedStatus: http.StatusBadRequest,
			description:    "Only HTTP and HTTPS URLs should be allowed",
		},
		{
			name:           "localhost URL blocked",
			title:          "Normal title",
			content:        "Normal content",
			url:            "http://localhost:8080",
			expectedStatus: http.StatusBadRequest,
			description:    "Localhost URLs should be blocked",
		},
		{
			name:           "private IP URL blocked",
			title:          "Normal title",
			content:        "Normal content",
			url:            "http://192.168.1.1",
			expectedStatus: http.StatusBadRequest,
			description:    "Private IP URLs should be blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &testutils.PostRepositoryStub{}
			handler := NewPostHandler(repo, nil, nil)

			postData := map[string]string{
				"title":   tt.title,
				"url":     tt.url,
				"content": tt.content,
			}

			body, _ := json.Marshal(postData)
			request := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(body))
			request.Header.Set("Content-Type", "application/json")
			request = request.WithContext(context.WithValue(request.Context(), middleware.UserIDKey, uint(1)))
			recorder := httptest.NewRecorder()

			handler.CreatePost(recorder, request)

			testutils.AssertHTTPStatus(t, recorder, tt.expectedStatus)
		})
	}
}

func TestAuthHandler_PasswordValidation(t *testing.T) {
	tests := []struct {
		name           string
		password       string
		expectedStatus int
		description    string
	}{
		{
			name:           "weak password",
			password:       "123",
			expectedStatus: http.StatusBadRequest,
			description:    "Weak passwords should be rejected",
		},
		{
			name:           "password without letters",
			password:       "12345678",
			expectedStatus: http.StatusBadRequest,
			description:    "Passwords without letters should be rejected",
		},
		{
			name:           "password without numbers",
			password:       "password",
			expectedStatus: http.StatusBadRequest,
			description:    "Passwords without numbers should be rejected",
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
			name:           "password too long",
			password:       string(make([]byte, 129)),
			expectedStatus: http.StatusBadRequest,
			description:    "Passwords that are too long should be rejected",
		},
		{
			name:           "empty password",
			password:       "",
			expectedStatus: http.StatusBadRequest,
			description:    "Empty passwords should be rejected",
		},
		{
			name:           "valid password",
			password:       "Password123!",
			expectedStatus: http.StatusCreated,
			description:    "Valid passwords should be accepted",
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
		{
			name:           "valid password with unicode",
			password:       "Pássw0rd123!",
			expectedStatus: http.StatusCreated,
			description:    "Valid passwords with unicode should be accepted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &testutils.UserRepositoryStub{
				GetByUsernameFn: func(string) (*database.User, error) {
					return nil, gorm.ErrRecordNotFound
				},
				CreateFn: func(user *database.User) error {
					return nil
				},
			}

			handler := newAuthHandler(repo)

			registerData := map[string]string{
				"username": "testuser",
				"email":    "test@example.com",
				"password": tt.password,
			}

			body, _ := json.Marshal(registerData)
			request := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			handler.Register(recorder, request)

			testutils.AssertHTTPStatus(t, recorder, tt.expectedStatus)
		})
	}
}

func TestAuthHandler_UsernameSanitization(t *testing.T) {
	tests := []struct {
		name           string
		username       string
		expectedStatus int
		description    string
	}{
		{
			name:           "username with special chars",
			username:       "test@user#123",
			expectedStatus: http.StatusCreated,
			description:    "Special characters should be removed from username",
		},
		{
			name:           "username with script tags",
			username:       "test<script>alert('xss')</script>user",
			expectedStatus: http.StatusCreated,
			description:    "Script tags should be removed from username",
		},
		{
			name:           "username starting with special char",
			username:       "@testuser",
			expectedStatus: http.StatusCreated,
			description:    "Username starting with special char should be prefixed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedUsername string
			repo := &testutils.UserRepositoryStub{
				GetByUsernameFn: func(username string) (*database.User, error) {
					capturedUsername = username
					return nil, gorm.ErrRecordNotFound
				},
				CreateFn: func(user *database.User) error {
					return nil
				},
			}

			handler := newAuthHandler(repo)

			registerData := map[string]string{
				"username": tt.username,
				"email":    "test@example.com",
				"password": "Password123!",
			}

			body, _ := json.Marshal(registerData)
			request := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			handler.Register(recorder, request)

			testutils.AssertHTTPStatus(t, recorder, tt.expectedStatus)

			expectedUsername := security.SanitizeUsername(tt.username)
			if capturedUsername != expectedUsername {
				t.Errorf("Expected sanitized username %q, got %q", expectedUsername, capturedUsername)
			}
		})
	}
}

func TestPostHandler_AuthorizationBypass(t *testing.T) {
	repo := &testutils.PostRepositoryStub{
		GetByIDFn: func(id uint) (*database.Post, error) {
			authorID := uint(2)
			return &database.Post{ID: id, Title: "Test Post", AuthorID: &authorID}, nil
		},
	}

	handler := NewPostHandler(repo, nil, nil)

	updateData := map[string]string{
		"title":   "Updated Title",
		"content": "Updated content",
	}

	body, _ := json.Marshal(updateData)
	request := httptest.NewRequest("PUT", "/api/posts/1", bytes.NewBuffer(body))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(context.WithValue(request.Context(), middleware.UserIDKey, uint(1)))

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "1")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeCtx))

	recorder := httptest.NewRecorder()

	handler.UpdatePost(recorder, request)

	if recorder.Result().StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d. Users should not be able to edit other users' posts", recorder.Result().StatusCode)
	}
}

func TestPageHandler_PasswordResetValidation(t *testing.T) {

	tests := []struct {
		name          string
		password      string
		expectedError bool
		description   string
	}{
		{
			name:          "valid password",
			password:      "Password123!",
			expectedError: false,
			description:   "Valid passwords should pass validation",
		},
		{
			name:          "password without special chars",
			password:      "Password123",
			expectedError: true,
			description:   "Passwords without special characters should be rejected",
		},
		{
			name:          "password too short",
			password:      "Pass1!",
			expectedError: true,
			description:   "Passwords shorter than 8 characters should be rejected",
		},
		{
			name:          "password without letters",
			password:      "12345678!",
			expectedError: true,
			description:   "Passwords without letters should be rejected",
		},
		{
			name:          "password without numbers",
			password:      "Password!",
			expectedError: true,
			description:   "Passwords without numbers should be rejected",
		},
		{
			name:          "empty password",
			password:      "",
			expectedError: true,
			description:   "Empty passwords should be rejected",
		},
		{
			name:          "password too long",
			password:      string(make([]byte, 129)),
			expectedError: true,
			description:   "Passwords longer than 128 characters should be rejected",
		},
		{
			name:          "valid password with unicode",
			password:      "Pássw0rd123!",
			expectedError: false,
			description:   "Valid passwords with unicode should pass validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidatePassword(tt.password)

			if tt.expectedError && err == nil {
				t.Errorf("ValidatePassword(%q) expected error, got nil. %s", tt.password, tt.description)
			}
			if !tt.expectedError && err != nil {
				t.Errorf("ValidatePassword(%q) unexpected error: %v. %s", tt.password, err, tt.description)
			}
		})
	}
}
