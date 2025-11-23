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

	"goyco/internal/database"
	"goyco/internal/dto"
	"goyco/internal/middleware"
	"goyco/internal/services"
	"goyco/internal/testutils"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

func TestSendSuccessResponse(t *testing.T) {
	helper := testutils.NewHandlerTestHelper(t)

	w := httptest.NewRecorder()
	SendSuccessResponse(w, "Test message", map[string]string{"key": "value"})

	helper.AssertStatusCode(t, w, http.StatusOK)
	response := helper.DecodeResponse(t, w)
	helper.AssertResponseSuccess(t, response)

	if response["message"] != "Test message" {
		t.Errorf("Expected message 'Test message', got %v", response["message"])
	}
}

func TestSendCreatedResponse(t *testing.T) {
	helper := testutils.NewHandlerTestHelper(t)

	w := httptest.NewRecorder()
	SendCreatedResponse(w, "Created message", map[string]string{"id": "123"})

	helper.AssertStatusCode(t, w, http.StatusCreated)
	response := helper.DecodeResponse(t, w)
	helper.AssertResponseSuccess(t, response)

	if response["message"] != "Created message" {
		t.Errorf("Expected message 'Created message', got %v", response["message"])
	}
}

func TestSendErrorResponse(t *testing.T) {
	helper := testutils.NewHandlerTestHelper(t)

	w := httptest.NewRecorder()
	SendErrorResponse(w, "Error message", http.StatusBadRequest)

	helper.AssertStatusCode(t, w, http.StatusBadRequest)
	response := helper.DecodeResponse(t, w)
	helper.AssertResponseError(t, response)

	if response["error"] != "Error message" {
		t.Errorf("Expected error 'Error message', got %v", response["error"])
	}
}

func TestGetClientIP(t *testing.T) {

	originalTrust := middleware.TrustProxyHeaders
	defer func() {
		middleware.TrustProxyHeaders = originalTrust
	}()

	tests := []struct {
		name              string
		headers           map[string]string
		remoteAddr        string
		trustProxyHeaders bool
		expected          string
	}{
		{
			name:              "Default: RemoteAddr when TrustProxyHeaders is false",
			headers:           map[string]string{"X-Forwarded-For": "192.168.1.1"},
			remoteAddr:        "127.0.0.1:8080",
			trustProxyHeaders: false,
			expected:          "127.0.0.1",
		},
		{
			name:              "X-Forwarded-For header when TrustProxyHeaders is true",
			headers:           map[string]string{"X-Forwarded-For": "192.168.1.1"},
			remoteAddr:        "127.0.0.1:8080",
			trustProxyHeaders: true,
			expected:          "192.168.1.1",
		},
		{
			name:              "X-Real-IP header when TrustProxyHeaders is true",
			headers:           map[string]string{"X-Real-IP": "10.0.0.1"},
			remoteAddr:        "127.0.0.1:8080",
			trustProxyHeaders: true,
			expected:          "10.0.0.1",
		},
		{
			name:              "RemoteAddr fallback",
			headers:           map[string]string{},
			remoteAddr:        "127.0.0.1:8080",
			trustProxyHeaders: false,
			expected:          "127.0.0.1",
		},
		{
			name:              "X-Forwarded-For with multiple IPs uses leftmost",
			headers:           map[string]string{"X-Forwarded-For": "203.0.113.1, 198.51.100.1"},
			remoteAddr:        "127.0.0.1:8080",
			trustProxyHeaders: true,
			expected:          "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware.TrustProxyHeaders = tt.trustProxyHeaders
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := GetClientIP(req)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}

	middleware.TrustProxyHeaders = originalTrust
}

func TestIsHTTPS(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		tls      bool
		expected bool
	}{
		{
			name:     "TLS connection",
			headers:  map[string]string{},
			tls:      true,
			expected: true,
		},
		{
			name:     "X-Forwarded-Proto https",
			headers:  map[string]string{"X-Forwarded-Proto": "https"},
			tls:      false,
			expected: true,
		},
		{
			name:     "X-Forwarded-Ssl on",
			headers:  map[string]string{"X-Forwarded-Ssl": "on"},
			tls:      false,
			expected: true,
		},
		{
			name:     "X-Forwarded-Scheme https",
			headers:  map[string]string{"X-Forwarded-Scheme": "https"},
			tls:      false,
			expected: true,
		},
		{
			name:     "HTTP connection",
			headers:  map[string]string{},
			tls:      false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			if tt.tls {
				t.Skip("Cannot test TLS with httptest.NewRequest")
			}

			result := IsHTTPS(req)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSanitizeUser(t *testing.T) {
	user := &database.User{
		ID:       1,
		Username: "testuser",
		Email:    "test@example.com",
		Password: "hashedpassword",
	}

	sanitized := SanitizeUser(user)

	if sanitized.ID != user.ID {
		t.Errorf("Expected ID %d, got %d", user.ID, sanitized.ID)
	}

	if sanitized.Username != user.Username {
		t.Errorf("Expected username %s, got %s", user.Username, sanitized.Username)
	}

	if sanitized.CreatedAt != user.CreatedAt {
		t.Errorf("Expected CreatedAt to match")
	}

	if sanitized.UpdatedAt != user.UpdatedAt {
		t.Errorf("Expected UpdatedAt to match")
	}
}

func TestSanitizeUserNil(t *testing.T) {
	sanitized := SanitizeUser(nil)

	if sanitized.ID != 0 {
		t.Errorf("Expected zero value for nil user, got %v", sanitized)
	}
}

func TestSanitizeUsers(t *testing.T) {
	users := []database.User{
		{ID: 1, Username: "user1", Email: "user1@example.com", Password: "hash1"},
		{ID: 2, Username: "user2", Email: "user2@example.com", Password: "hash2"},
	}

	sanitized := SanitizeUsers(users)

	if len(sanitized) != len(users) {
		t.Errorf("Expected %d users, got %d", len(users), len(sanitized))
	}

	for i, user := range sanitized {
		if user.ID != users[i].ID {
			t.Errorf("User %d: Expected ID %d, got %d", i, users[i].ID, user.ID)
		}
		if user.Username != users[i].Username {
			t.Errorf("User %d: Expected username %s, got %s", i, users[i].Username, user.Username)
		}
	}
}

func TestSetVoteCookie(t *testing.T) {
	tests := []struct {
		name         string
		request      *http.Request
		expectSecure bool
	}{
		{
			name:         "HTTP request - Secure flag false",
			request:      httptest.NewRequest("POST", "/vote", nil),
			expectSecure: false,
		},
		{
			name: "HTTPS via X-Forwarded-Proto - Secure flag true",
			request: func() *http.Request {
				req := httptest.NewRequest("POST", "/vote", nil)
				req.Header.Set("X-Forwarded-Proto", "https")
				return req
			}(),
			expectSecure: true,
		},
		{
			name: "HTTPS via X-Forwarded-Ssl - Secure flag true",
			request: func() *http.Request {
				req := httptest.NewRequest("POST", "/vote", nil)
				req.Header.Set("X-Forwarded-Ssl", "on")
				return req
			}(),
			expectSecure: true,
		},
		{
			name: "HTTPS via X-Forwarded-Scheme - Secure flag true",
			request: func() *http.Request {
				req := httptest.NewRequest("POST", "/vote", nil)
				req.Header.Set("X-Forwarded-Scheme", "https")
				return req
			}(),
			expectSecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			SetVoteCookie(w, tt.request, 123, database.VoteUp)

			cookies := w.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("Expected 1 cookie, got %d", len(cookies))
			}

			cookie := cookies[0]
			if cookie.Name != "vote_123" {
				t.Errorf("Expected cookie name 'vote_123', got %s", cookie.Name)
			}

			if cookie.MaxAge != 86400*30 {
				t.Errorf("Expected MaxAge %d, got %d", 86400*30, cookie.MaxAge)
			}

			if cookie.Secure != tt.expectSecure {
				t.Errorf("Expected Secure flag %v, got %v", tt.expectSecure, cookie.Secure)
			}

			if !cookie.HttpOnly {
				t.Error("Expected HttpOnly flag to be true")
			}

			if cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("Expected SameSite to be LaxMode, got %v", cookie.SameSite)
			}
		})
	}
}

func TestGetVoteCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "vote_123",
		Value: "up:1234567890",
	})

	value := GetVoteCookie(req, 123)
	if value != "up:1234567890" {
		t.Errorf("Expected 'up:1234567890', got %s", value)
	}

	value = GetVoteCookie(req, 456)
	if value != "" {
		t.Errorf("Expected empty string, got %s", value)
	}
}

func TestClearVoteCookie(t *testing.T) {
	w := httptest.NewRecorder()
	ClearVoteCookie(w, 123)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Expected 1 cookie, got %d", len(cookies))
	}

	cookie := cookies[0]
	if cookie.Name != "vote_123" {
		t.Errorf("Expected cookie name 'vote_123', got %s", cookie.Name)
	}

	if cookie.Value != "" {
		t.Errorf("Expected empty cookie value, got %s", cookie.Value)
	}

	if cookie.MaxAge != -1 {
		t.Errorf("Expected MaxAge -1, got %d", cookie.MaxAge)
	}
}

func TestValidateRedirectURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{

		{
			name:     "Valid simple path",
			input:    "/posts/123",
			expected: "/posts/123",
		},
		{
			name:     "Valid path with query",
			input:    "/posts/123?sort=date",
			expected: "/posts/123?sort=date",
		},
		{
			name:     "Valid path with fragment",
			input:    "/posts/123#comments",
			expected: "/posts/123#comments",
		},
		{
			name:     "Valid path with query and fragment",
			input:    "/posts/123?sort=date#comments",
			expected: "/posts/123?sort=date#comments",
		},
		{
			name:     "Root path",
			input:    "/",
			expected: "/",
		},
		{
			name:     "Path with multiple segments",
			input:    "/api/posts/123/comments",
			expected: "/api/posts/123/comments",
		},

		{
			name:     "Absolute URL with scheme",
			input:    "https://evil.com",
			expected: "",
		},
		{
			name:     "Absolute URL with http",
			input:    "http://evil.com",
			expected: "",
		},
		{
			name:     "Protocol-relative URL",
			input:    "//evil.com",
			expected: "",
		},
		{
			name:     "URL without leading slash",
			input:    "posts/123",
			expected: "",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Whitespace only",
			input:    "   ",
			expected: "",
		},
		{
			name:     "URL with scheme in path",
			input:    "/https://evil.com",
			expected: "/https://evil.com",
		},
		{
			name:     "Too long URL",
			input:    "/" + strings.Repeat("a", 512),
			expected: "",
		},
		{
			name:     "Path with encoded characters",
			input:    "/posts/123%20test",
			expected: "/posts/123%20test",
		},
		{
			name:     "Malformed URL",
			input:    "/posts/\x00",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateRedirectURL(tt.input)
			if result != tt.expected {
				t.Errorf("ValidateRedirectURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseUintParam(t *testing.T) {
	tests := []struct {
		name           string
		paramValue     string
		paramName      string
		entityName     string
		expectedID     uint
		expectedOK     bool
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "valid ID",
			paramValue:     "123",
			paramName:      "id",
			entityName:     "Post",
			expectedID:     123,
			expectedOK:     true,
			expectedStatus: 0,
		},
		{
			name:           "missing parameter",
			paramValue:     "",
			paramName:      "id",
			entityName:     "Post",
			expectedID:     0,
			expectedOK:     false,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Post ID is required",
		},
		{
			name:           "invalid ID - not a number",
			paramValue:     "abc",
			paramName:      "id",
			entityName:     "Post",
			expectedID:     0,
			expectedOK:     false,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid Post ID",
		},
		{
			name:           "invalid ID - negative number",
			paramValue:     "-1",
			paramName:      "id",
			entityName:     "User",
			expectedID:     0,
			expectedOK:     false,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid User ID",
		},
		{
			name:           "large valid ID",
			paramValue:     "4294967295",
			paramName:      "id",
			entityName:     "Post",
			expectedID:     4294967295,
			expectedOK:     true,
			expectedStatus: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)

			ctx := chi.NewRouteContext()
			if tt.paramValue != "" {
				ctx.URLParams.Add(tt.paramName, tt.paramValue)
			}
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, ctx))

			id, ok := ParseUintParam(w, r, tt.paramName, tt.entityName)

			if ok != tt.expectedOK {
				t.Errorf("ParseUintParam() ok = %v, want %v", ok, tt.expectedOK)
			}

			if id != tt.expectedID {
				t.Errorf("ParseUintParam() id = %v, want %v", id, tt.expectedID)
			}

			if !tt.expectedOK {
				result := w.Result()
				if result.StatusCode != tt.expectedStatus {
					t.Errorf("ParseUintParam() status = %v, want %v", result.StatusCode, tt.expectedStatus)
				}

				var response map[string]any
				json.NewDecoder(w.Body).Decode(&response)
				if tt.expectedError != "" && !strings.Contains(response["error"].(string), tt.expectedError) {
					t.Errorf("ParseUintParam() error = %v, want to contain %v", response["error"], tt.expectedError)
				}
			}
		})
	}
}

func TestRequireAuth(t *testing.T) {
	tests := []struct {
		name           string
		userID         uint
		expectedID     uint
		expectedOK     bool
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "authenticated user",
			userID:         123,
			expectedID:     123,
			expectedOK:     true,
			expectedStatus: 0,
		},
		{
			name:           "unauthenticated user (no userID)",
			userID:         0,
			expectedID:     0,
			expectedOK:     false,
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authentication required",
		},
		{
			name:           "authenticated user with large ID",
			userID:         4294967295,
			expectedID:     4294967295,
			expectedOK:     true,
			expectedStatus: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)

			ctx := context.WithValue(r.Context(), middleware.UserIDKey, tt.userID)
			r = r.WithContext(ctx)

			userID, ok := RequireAuth(w, r)

			if ok != tt.expectedOK {
				t.Errorf("RequireAuth() ok = %v, want %v", ok, tt.expectedOK)
			}

			if userID != tt.expectedID {
				t.Errorf("RequireAuth() userID = %v, want %v", userID, tt.expectedID)
			}

			if !tt.expectedOK {
				result := w.Result()
				if result.StatusCode != tt.expectedStatus {
					t.Errorf("RequireAuth() status = %v, want %v", result.StatusCode, tt.expectedStatus)
				}

				var response map[string]any
				json.NewDecoder(w.Body).Decode(&response)
				if tt.expectedError != "" && response["error"] != tt.expectedError {
					t.Errorf("RequireAuth() error = %v, want %v", response["error"], tt.expectedError)
				}
			}
		})
	}
}

func TestDecodeJSONRequest(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		target         any
		expectedOK     bool
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid JSON",
			body: `{"username": "test", "password": "pass123"}`,
			target: &struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}{},
			expectedOK:     true,
			expectedStatus: 0,
		},
		{
			name: "invalid JSON",
			body: `{"username": "test", "password":}`,
			target: &struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}{},
			expectedOK:     false,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
		{
			name: "empty body",
			body: "",
			target: &struct {
				Username string `json:"username"`
			}{},
			expectedOK:     false,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
		{
			name: "malformed JSON",
			body: `{username: test}`,
			target: &struct {
				Username string `json:"username"`
			}{},
			expectedOK:     false,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", "application/json")

			ok := DecodeJSONRequest(w, r, tt.target)

			if ok != tt.expectedOK {
				t.Errorf("DecodeJSONRequest() ok = %v, want %v", ok, tt.expectedOK)
			}

			if tt.expectedOK {
				if tt.name == "valid JSON" {
					decoded := tt.target.(*struct {
						Username string `json:"username"`
						Password string `json:"password"`
					})
					if decoded.Username != "test" || decoded.Password != "pass123" {
						t.Errorf("DecodeJSONRequest() failed to decode data correctly")
					}
				}
			} else {
				result := w.Result()
				if result.StatusCode != tt.expectedStatus {
					t.Errorf("DecodeJSONRequest() status = %v, want %v", result.StatusCode, tt.expectedStatus)
				}

				var response map[string]any
				json.NewDecoder(w.Body).Decode(&response)
				if tt.expectedError != "" && response["error"] != tt.expectedError {
					t.Errorf("DecodeJSONRequest() error = %v, want %v", response["error"], tt.expectedError)
				}
			}
		})
	}
}

func createRequestWithDTO[T any](method, url string, body []byte) *http.Request {
	r := httptest.NewRequest(method, url, bytes.NewReader(body))
	var dto T
	if len(body) > 0 {
		if err := json.NewDecoder(bytes.NewReader(body)).Decode(&dto); err != nil {
			return r
		}
	}
	ctx := middleware.SetValidatedDTOInContext(r.Context(), &dto)
	return r.WithContext(ctx)
}

func createLoginRequest(body string) *http.Request {
	return createRequestWithDTO[dto.LoginRequest](http.MethodPost, "/api/auth/login", []byte(body))
}

func createRegisterRequest(body string) *http.Request {
	return createRequestWithDTO[dto.RegisterRequest](http.MethodPost, "/api/auth/register", []byte(body))
}

func createResendVerificationRequest(body string) *http.Request {
	return createRequestWithDTO[dto.ResendVerificationRequest](http.MethodPost, "/api/auth/resend-verification", []byte(body))
}

func createForgotPasswordRequest(body string) *http.Request {
	return createRequestWithDTO[dto.ForgotPasswordRequest](http.MethodPost, "/api/auth/forgot-password", []byte(body))
}

func createResetPasswordRequest(body string) *http.Request {
	return createRequestWithDTO[dto.ResetPasswordRequest](http.MethodPost, "/api/auth/reset-password", []byte(body))
}

func createUpdateEmailRequest(body string) *http.Request {
	return createRequestWithDTO[dto.UpdateEmailRequest](http.MethodPut, "/api/auth/email", []byte(body))
}

func createUpdateUsernameRequest(body string) *http.Request {
	return createRequestWithDTO[dto.UpdateUsernameRequest](http.MethodPut, "/api/auth/username", []byte(body))
}

func createUpdatePasswordRequest(body string) *http.Request {
	return createRequestWithDTO[dto.UpdatePasswordRequest](http.MethodPut, "/api/auth/password", []byte(body))
}

func createConfirmAccountDeletionRequest(body string) *http.Request {
	return createRequestWithDTO[dto.ConfirmAccountDeletionRequest](http.MethodPost, "/api/auth/account/confirm", []byte(body))
}

func createRefreshTokenRequest(body string) *http.Request {
	return createRequestWithDTO[dto.RefreshTokenRequest](http.MethodPost, "/api/auth/refresh", []byte(body))
}

func createRevokeTokenRequest(body string) *http.Request {
	return createRequestWithDTO[dto.RevokeTokenRequest](http.MethodPost, "/api/auth/revoke", []byte(body))
}

func createCreatePostRequest(body string) *http.Request {
	return createRequestWithDTO[dto.CreatePostRequest](http.MethodPost, "/api/posts", []byte(body))
}

func createUpdatePostRequest(body string) *http.Request {
	return createRequestWithDTO[dto.UpdatePostRequest](http.MethodPut, "/api/posts/1", []byte(body))
}

func createVoteRequest(body string) *http.Request {
	return createRequestWithDTO[dto.CastVoteRequest](http.MethodPost, "/api/posts/1/vote", []byte(body))
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    map[string]string
		expectedLimit  int
		expectedOffset int
	}{
		{
			name:           "default values - no params",
			queryParams:    map[string]string{},
			expectedLimit:  20,
			expectedOffset: 0,
		},
		{
			name:           "valid limit and offset",
			queryParams:    map[string]string{"limit": "10", "offset": "5"},
			expectedLimit:  10,
			expectedOffset: 5,
		},
		{
			name:           "only limit",
			queryParams:    map[string]string{"limit": "50"},
			expectedLimit:  50,
			expectedOffset: 0,
		},
		{
			name:           "only offset",
			queryParams:    map[string]string{"offset": "100"},
			expectedLimit:  20,
			expectedOffset: 100,
		},
		{
			name:           "invalid limit - not a number",
			queryParams:    map[string]string{"limit": "abc", "offset": "5"},
			expectedLimit:  20,
			expectedOffset: 5,
		},
		{
			name:           "invalid limit - zero",
			queryParams:    map[string]string{"limit": "0", "offset": "5"},
			expectedLimit:  20,
			expectedOffset: 5,
		},
		{
			name:           "invalid limit - negative",
			queryParams:    map[string]string{"limit": "-5", "offset": "5"},
			expectedLimit:  20,
			expectedOffset: 5,
		},
		{
			name:           "invalid offset - not a number",
			queryParams:    map[string]string{"limit": "10", "offset": "abc"},
			expectedLimit:  10,
			expectedOffset: 0,
		},
		{
			name:           "invalid offset - negative",
			queryParams:    map[string]string{"limit": "10", "offset": "-5"},
			expectedLimit:  10,
			expectedOffset: 0,
		},
		{
			name:           "offset zero is valid",
			queryParams:    map[string]string{"limit": "10", "offset": "0"},
			expectedLimit:  10,
			expectedOffset: 0,
		},
		{
			name:           "large valid values",
			queryParams:    map[string]string{"limit": "1000", "offset": "500"},
			expectedLimit:  1000,
			expectedOffset: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			q := req.URL.Query()
			for key, value := range tt.queryParams {
				q.Set(key, value)
			}
			req.URL.RawQuery = q.Encode()

			limit, offset := parsePagination(req)

			if limit != tt.expectedLimit {
				t.Errorf("parsePagination() limit = %v, want %v", limit, tt.expectedLimit)
			}
			if offset != tt.expectedOffset {
				t.Errorf("parsePagination() offset = %v, want %v", offset, tt.expectedOffset)
			}
		})
	}
}

func TestNewVoteContext(t *testing.T) {
	tests := []struct {
		name           string
		userID         uint
		headers        map[string]string
		remoteAddr     string
		userAgent      string
		expectedUserID uint
		expectedIP     string
		expectedAgent  string
	}{
		{
			name:           "authenticated user with all fields",
			userID:         123,
			headers:        map[string]string{"X-Forwarded-For": "192.168.1.1"},
			remoteAddr:     "127.0.0.1:8080",
			userAgent:      "Mozilla/5.0",
			expectedUserID: 123,
			expectedIP:     "192.168.1.1",
			expectedAgent:  "Mozilla/5.0",
		},
		{
			name:           "unauthenticated user",
			userID:         0,
			headers:        map[string]string{},
			remoteAddr:     "127.0.0.1:8080",
			userAgent:      "Go-http-client/1.1",
			expectedUserID: 0,
			expectedIP:     "127.0.0.1",
			expectedAgent:  "Go-http-client/1.1",
		},
		{
			name:           "missing user agent",
			userID:         456,
			headers:        map[string]string{},
			remoteAddr:     "10.0.0.1:8080",
			userAgent:      "",
			expectedUserID: 456,
			expectedIP:     "10.0.0.1",
			expectedAgent:  "",
		},
	}

	originalTrust := middleware.TrustProxyHeaders
	defer func() {
		middleware.TrustProxyHeaders = originalTrust
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware.TrustProxyHeaders = len(tt.headers) > 0

			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.userAgent != "" {
				req.Header.Set("User-Agent", tt.userAgent)
			}
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			ctx := context.WithValue(req.Context(), middleware.UserIDKey, tt.userID)
			req = req.WithContext(ctx)

			voteCtx := NewVoteContext(req)

			if voteCtx.UserID != tt.expectedUserID {
				t.Errorf("NewVoteContext() UserID = %v, want %v", voteCtx.UserID, tt.expectedUserID)
			}

			if voteCtx.IPAddress != tt.expectedIP {
				t.Errorf("NewVoteContext() IPAddress = %v, want %v", voteCtx.IPAddress, tt.expectedIP)
			}

			if voteCtx.UserAgent != tt.expectedAgent {
				t.Errorf("NewVoteContext() UserAgent = %v, want %v", voteCtx.UserAgent, tt.expectedAgent)
			}
		})
	}

	middleware.TrustProxyHeaders = originalTrust
}

func TestHandleRepoError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		entityName     string
		expectedOK     bool
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "nil error",
			err:            nil,
			entityName:     "Post",
			expectedOK:     true,
			expectedStatus: 0,
		},
		{
			name:           "gorm.ErrRecordNotFound",
			err:            gorm.ErrRecordNotFound,
			entityName:     "Post",
			expectedOK:     false,
			expectedStatus: http.StatusNotFound,
			expectedError:  "Post not found",
		},
		{
			name:           "wrapped gorm.ErrRecordNotFound",
			err:            fmt.Errorf("database error: %w", gorm.ErrRecordNotFound),
			entityName:     "User",
			expectedOK:     false,
			expectedStatus: http.StatusNotFound,
			expectedError:  "User not found",
		},
		{
			name:           "other error",
			err:            errors.New("database connection failed"),
			entityName:     "Post",
			expectedOK:     false,
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Failed to retrieve Post",
		},
		{
			name:           "generic error with custom entity",
			err:            errors.New("timeout"),
			entityName:     "Comment",
			expectedOK:     false,
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Failed to retrieve Comment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			ok := HandleRepoError(w, tt.err, tt.entityName)

			if ok != tt.expectedOK {
				t.Errorf("HandleRepoError() ok = %v, want %v", ok, tt.expectedOK)
			}

			if !tt.expectedOK {
				result := w.Result()
				if result.StatusCode != tt.expectedStatus {
					t.Errorf("HandleRepoError() status = %v, want %v", result.StatusCode, tt.expectedStatus)
				}

				var response map[string]any
				json.NewDecoder(w.Body).Decode(&response)
				if tt.expectedError != "" && response["error"] != tt.expectedError {
					t.Errorf("HandleRepoError() error = %v, want %v", response["error"], tt.expectedError)
				}
			}
		})
	}
}

func TestHandleServiceError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		defaultMsg     string
		defaultCode    int
		expectedOK     bool
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "nil error",
			err:            nil,
			defaultMsg:     "Default error",
			defaultCode:    http.StatusInternalServerError,
			expectedOK:     true,
			expectedStatus: 0,
		},
		{
			name:           "ErrInvalidCredentials",
			err:            services.ErrInvalidCredentials,
			defaultMsg:     "Default error",
			defaultCode:    http.StatusInternalServerError,
			expectedOK:     false,
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid username or password",
		},
		{
			name:           "ErrEmailNotVerified",
			err:            services.ErrEmailNotVerified,
			defaultMsg:     "Default error",
			defaultCode:    http.StatusInternalServerError,
			expectedOK:     false,
			expectedStatus: http.StatusForbidden,
			expectedError:  "Please confirm your email before logging in",
		},
		{
			name:           "ErrAccountLocked",
			err:            services.ErrAccountLocked,
			defaultMsg:     "Default error",
			defaultCode:    http.StatusInternalServerError,
			expectedOK:     false,
			expectedStatus: http.StatusForbidden,
			expectedError:  "Your account has been locked. Please contact us for assistance.",
		},
		{
			name:           "ErrUsernameTaken",
			err:            services.ErrUsernameTaken,
			defaultMsg:     "Default error",
			defaultCode:    http.StatusInternalServerError,
			expectedOK:     false,
			expectedStatus: http.StatusConflict,
			expectedError:  "Username is already taken",
		},
		{
			name:           "ErrEmailTaken",
			err:            services.ErrEmailTaken,
			defaultMsg:     "Default error",
			defaultCode:    http.StatusInternalServerError,
			expectedOK:     false,
			expectedStatus: http.StatusConflict,
			expectedError:  "Email is already registered",
		},
		{
			name:           "ErrInvalidEmail",
			err:            services.ErrInvalidEmail,
			defaultMsg:     "Default error",
			defaultCode:    http.StatusInternalServerError,
			expectedOK:     false,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid email address",
		},
		{
			name:           "ErrPasswordTooShort",
			err:            services.ErrPasswordTooShort,
			defaultMsg:     "Default error",
			defaultCode:    http.StatusInternalServerError,
			expectedOK:     false,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Password must be at least 8 characters",
		},
		{
			name:           "ErrInvalidVerificationToken",
			err:            services.ErrInvalidVerificationToken,
			defaultMsg:     "Default error",
			defaultCode:    http.StatusInternalServerError,
			expectedOK:     false,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid or expired verification token",
		},
		{
			name:           "ErrRefreshTokenExpired",
			err:            services.ErrRefreshTokenExpired,
			defaultMsg:     "Default error",
			defaultCode:    http.StatusInternalServerError,
			expectedOK:     false,
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Refresh token has expired",
		},
		{
			name:           "ErrRefreshTokenInvalid",
			err:            services.ErrRefreshTokenInvalid,
			defaultMsg:     "Default error",
			defaultCode:    http.StatusInternalServerError,
			expectedOK:     false,
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid refresh token",
		},
		{
			name:           "ErrInvalidDeletionToken",
			err:            services.ErrInvalidDeletionToken,
			defaultMsg:     "Default error",
			defaultCode:    http.StatusInternalServerError,
			expectedOK:     false,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "This deletion link is invalid or has expired.",
		},
		{
			name:           "ErrEmailSenderUnavailable",
			err:            services.ErrEmailSenderUnavailable,
			defaultMsg:     "Default error",
			defaultCode:    http.StatusInternalServerError,
			expectedOK:     false,
			expectedStatus: http.StatusServiceUnavailable,
			expectedError:  "Email service is unavailable. Please try again later.",
		},
		{
			name:           "wrapped ErrInvalidCredentials",
			err:            fmt.Errorf("auth failed: %w", services.ErrInvalidCredentials),
			defaultMsg:     "Default error",
			defaultCode:    http.StatusInternalServerError,
			expectedOK:     false,
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid username or password",
		},
		{
			name:           "unmapped error - uses default",
			err:            errors.New("unknown error"),
			defaultMsg:     "Something went wrong",
			defaultCode:    http.StatusInternalServerError,
			expectedOK:     false,
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Something went wrong",
		},
		{
			name:           "unmapped error with custom default",
			err:            errors.New("timeout"),
			defaultMsg:     "Request timeout",
			defaultCode:    http.StatusGatewayTimeout,
			expectedOK:     false,
			expectedStatus: http.StatusGatewayTimeout,
			expectedError:  "Request timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			ok := HandleServiceError(w, tt.err, tt.defaultMsg, tt.defaultCode)

			if ok != tt.expectedOK {
				t.Errorf("HandleServiceError() ok = %v, want %v", ok, tt.expectedOK)
			}

			if !tt.expectedOK {
				result := w.Result()
				if result.StatusCode != tt.expectedStatus {
					t.Errorf("HandleServiceError() status = %v, want %v", result.StatusCode, tt.expectedStatus)
				}

				var response map[string]any
				json.NewDecoder(w.Body).Decode(&response)
				if tt.expectedError != "" && response["error"] != tt.expectedError {
					t.Errorf("HandleServiceError() error = %v, want %v", response["error"], tt.expectedError)
				}
			}
		})
	}
}
