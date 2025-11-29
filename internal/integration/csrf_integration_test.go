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
		getRequest := httptest.NewRequest("GET", "/register", nil)
		getRecorder := httptest.NewRecorder()
		router.ServeHTTP(getRecorder, getRequest)

		cookies := getRecorder.Result().Cookies()
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

		requestBody := url.Values{}
		requestBody.Set("username", "csrf_user")
		requestBody.Set("email", "csrf@example.com")
		requestBody.Set("password", "SecurePass123!")
		requestBody.Set("csrf_token", csrfToken)

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
		getRequest := httptest.NewRequest("GET", "/register", nil)
		getRecorder := httptest.NewRecorder()
		router.ServeHTTP(getRecorder, getRequest)

		cookies := getRecorder.Result().Cookies()
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
		getRequest := httptest.NewRequest("GET", "/register", nil)
		getRecorder := httptest.NewRecorder()
		router.ServeHTTP(getRecorder, getRequest)

		cookies := getRecorder.Result().Cookies()
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

		requestBody := url.Values{}
		requestBody.Set("username", "header_user")
		requestBody.Set("email", "header@example.com")
		requestBody.Set("password", "SecurePass123!")

		request := httptest.NewRequest("POST", "/register", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("X-CSRF-Token", csrfToken)
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

		getRequest := httptest.NewRequest("GET", "/posts/new", nil)
		getRequest.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		getRecorder := httptest.NewRecorder()
		router.ServeHTTP(getRecorder, getRequest)

		cookies := getRecorder.Result().Cookies()
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

		requestBody := url.Values{}
		requestBody.Set("title", "CSRF Test Post")
		requestBody.Set("url", "https://example.com/csrf-test")
		requestBody.Set("content", "Test content")
		requestBody.Set("csrf_token", csrfToken)

		request := httptest.NewRequest("POST", "/posts", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		request.AddCookie(csrfCookie)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code == http.StatusForbidden {
			t.Error("Expected post creation with valid CSRF token to succeed")
		}
	})
}
