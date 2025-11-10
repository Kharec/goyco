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

	t.Run("CSRF_Blocks_Form_Without_Token", func(t *testing.T) {
		reqBody := url.Values{}
		reqBody.Set("username", "testuser")
		reqBody.Set("email", "test@example.com")
		reqBody.Set("password", "SecurePass123!")

		req := httptest.NewRequest("POST", "/register", strings.NewReader(reqBody.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "Invalid CSRF token") {
			t.Error("Expected CSRF error message")
		}
	})

	t.Run("CSRF_Allows_Form_With_Valid_Token", func(t *testing.T) {
		getReq := httptest.NewRequest("GET", "/register", nil)
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)

		cookies := getRec.Result().Cookies()
		var csrfCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "csrf_token" {
				csrfCookie = cookie
				break
			}
		}

		if csrfCookie == nil {
			t.Fatal("Expected CSRF cookie to be set")
		}

		csrfToken := csrfCookie.Value

		reqBody := url.Values{}
		reqBody.Set("username", "csrf_user")
		reqBody.Set("email", "csrf@example.com")
		reqBody.Set("password", "SecurePass123!")
		reqBody.Set("csrf_token", csrfToken)

		req := httptest.NewRequest("POST", "/register", strings.NewReader(reqBody.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(csrfCookie)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code == http.StatusForbidden {
			t.Error("Expected form submission with valid CSRF token to succeed")
		}
	})

	t.Run("CSRF_Allows_API_Requests", func(t *testing.T) {
		reqBody := map[string]string{
			"username": "api_user",
			"email":    "api@example.com",
			"password": "SecurePass123!",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code == http.StatusForbidden {
			t.Error("Expected API requests to bypass CSRF protection")
		}
	})

	t.Run("CSRF_Blocks_Mismatched_Token", func(t *testing.T) {
		getReq := httptest.NewRequest("GET", "/register", nil)
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)

		cookies := getRec.Result().Cookies()
		var csrfCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "csrf_token" {
				csrfCookie = cookie
				break
			}
		}

		if csrfCookie == nil {
			t.Fatal("Expected CSRF cookie to be set")
		}

		reqBody := url.Values{}
		reqBody.Set("username", "mismatch_user")
		reqBody.Set("email", "mismatch@example.com")
		reqBody.Set("password", "SecurePass123!")
		reqBody.Set("csrf_token", "wrong-token")

		req := httptest.NewRequest("POST", "/register", strings.NewReader(reqBody.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(csrfCookie)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "Invalid CSRF token") {
			t.Error("Expected CSRF error message")
		}
	})

	t.Run("CSRF_Allows_GET_Requests", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/register", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code == http.StatusForbidden {
			t.Error("Expected GET requests to bypass CSRF protection")
		}
	})

	t.Run("CSRF_Token_In_Header", func(t *testing.T) {
		getReq := httptest.NewRequest("GET", "/register", nil)
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)

		cookies := getRec.Result().Cookies()
		var csrfCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "csrf_token" {
				csrfCookie = cookie
				break
			}
		}

		if csrfCookie == nil {
			t.Fatal("Expected CSRF cookie to be set")
		}

		csrfToken := csrfCookie.Value

		reqBody := url.Values{}
		reqBody.Set("username", "header_user")
		reqBody.Set("email", "header@example.com")
		reqBody.Set("password", "SecurePass123!")

		req := httptest.NewRequest("POST", "/register", strings.NewReader(reqBody.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-CSRF-Token", csrfToken)
		req.AddCookie(csrfCookie)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code == http.StatusForbidden {
			t.Error("Expected CSRF token in header to be accepted")
		}
	})

	t.Run("CSRF_With_PageHandler_Forms", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "csrf_form_user", "csrf_form@example.com")

		getReq := httptest.NewRequest("GET", "/posts/new", nil)
		getReq.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)

		cookies := getRec.Result().Cookies()
		var csrfCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "csrf_token" {
				csrfCookie = cookie
				break
			}
		}

		if csrfCookie == nil {
			t.Fatal("Expected CSRF cookie to be set")
		}

		csrfToken := csrfCookie.Value

		reqBody := url.Values{}
		reqBody.Set("title", "CSRF Test Post")
		reqBody.Set("url", "https://example.com/csrf-test")
		reqBody.Set("content", "Test content")
		reqBody.Set("csrf_token", csrfToken)

		req := httptest.NewRequest("POST", "/posts", strings.NewReader(reqBody.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		req.AddCookie(csrfCookie)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code == http.StatusForbidden {
			t.Error("Expected post creation with valid CSRF token to succeed")
		}
	})
}
