package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestIntegration_CSRF_Protection(t *testing.T) {
	ctx := setupPageHandlerTestContext(t)
	router := ctx.Router

	getCSRFToken := func(t *testing.T, path string, cookies ...*http.Cookie) *http.Cookie {
		t.Helper()

		request := httptest.NewRequest("GET", path, nil)
		for _, c := range cookies {
			request.AddCookie(c)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)

		for _, cookie := range recorder.Result().Cookies() {
			if cookie.Name == "csrf_token" {
				return cookie
			}
		}

		t.Fatalf("Expected CSRF cookie to be set for %s", path)
		return nil
	}

	t.Run("CSRF_Blocks_Form_Without_Token", func(t *testing.T) {
		requestBody := url.Values{}
		requestBody.Set("username", "testuser")
		requestBody.Set("email", "test@example.com")
		requestBody.Set("password", "SecurePass123!")

		request := httptest.NewRequest("POST", "/register", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d. Body: %s", recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), "Invalid CSRF token") {
			t.Error("Expected CSRF error message")
		}
	})

	t.Run("CSRF_Allows_Form_With_Valid_Token", func(t *testing.T) {
		csrfCookie := getCSRFToken(t, "/register")

		requestBody := url.Values{}
		requestBody.Set("username", "csrf_user")
		requestBody.Set("email", "csrf@example.com")
		requestBody.Set("password", "SecurePass123!")
		requestBody.Set("csrf_token", csrfCookie.Value)

		request := httptest.NewRequest("POST", "/register", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(csrfCookie)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code == http.StatusForbidden {
			t.Error("Expected form submission with valid CSRF token to succeed")
		}
	})

	t.Run("CSRF_Allows_API_Requests", func(t *testing.T) {
		requestBody := map[string]string{
			"username": "api_user",
			"email":    "api@example.com",
			"password": "SecurePass123!",
		}
		body, _ := json.Marshal(requestBody)
		request := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code == http.StatusForbidden {
			t.Error("Expected API requests to bypass CSRF protection")
		}
	})

	t.Run("CSRF_Blocks_Mismatched_Token", func(t *testing.T) {
		csrfCookie := getCSRFToken(t, "/register")

		requestBody := url.Values{}
		requestBody.Set("username", "mismatch_user")
		requestBody.Set("email", "mismatch@example.com")
		requestBody.Set("password", "SecurePass123!")
		requestBody.Set("csrf_token", "wrong-token")

		request := httptest.NewRequest("POST", "/register", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(csrfCookie)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d. Body: %s", recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), "Invalid CSRF token") {
			t.Error("Expected CSRF error message")
		}
	})

	t.Run("CSRF_Allows_GET_Requests", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/register", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code == http.StatusForbidden {
			t.Error("Expected GET requests to bypass CSRF protection")
		}
	})

	t.Run("CSRF_Token_In_Header", func(t *testing.T) {
		csrfCookie := getCSRFToken(t, "/register")

		requestBody := url.Values{}
		requestBody.Set("username", "header_user")
		requestBody.Set("email", "header@example.com")
		requestBody.Set("password", "SecurePass123!")

		request := httptest.NewRequest("POST", "/register", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("X-CSRF-Token", csrfCookie.Value)
		request.AddCookie(csrfCookie)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code == http.StatusForbidden {
			t.Error("Expected CSRF token in header to be accepted")
		}
	})

	t.Run("CSRF_With_PageHandler_Forms", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createUserWithCleanup(t, ctx, "csrf_form_user", "csrf_form@example.com")

		authCookie := &http.Cookie{Name: "auth_token", Value: user.Token}
		csrfCookie := getCSRFToken(t, "/posts/new", authCookie)

		requestBody := url.Values{}
		requestBody.Set("title", "CSRF Test Post")
		requestBody.Set("url", "https://example.com/csrf-test")
		requestBody.Set("content", "Test content")
		requestBody.Set("csrf_token", csrfCookie.Value)

		request := httptest.NewRequest("POST", "/posts", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(authCookie)
		request.AddCookie(csrfCookie)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code == http.StatusForbidden {
			t.Error("Expected post creation with valid CSRF token to succeed")
		}
	})

	t.Run("CSRF_Blocks_Request_With_Only_Cookie", func(t *testing.T) {
		csrfCookie := getCSRFToken(t, "/register")

		requestBody := url.Values{}
		requestBody.Set("username", "cookie_only_user")
		requestBody.Set("email", "cookie_only@example.com")
		requestBody.Set("password", "SecurePass123!")

		request := httptest.NewRequest("POST", "/register", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(csrfCookie)

		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusForbidden {
			t.Errorf("Expected status 403 for request with only cookie (no form/header token), got %d. Body: %s", recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), "Invalid CSRF token") {
			t.Error("Expected CSRF error message")
		}
	})
}
