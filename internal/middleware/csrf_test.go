package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFTokenGeneration(t *testing.T) {
	token1, err := CSRFToken()
	if err != nil {
		t.Fatalf("Failed to generate CSRF token: %v", err)
	}

	token2, err := CSRFToken()
	if err != nil {
		t.Fatalf("Failed to generate second CSRF token: %v", err)
	}

	if token1 == token2 {
		t.Error("Generated CSRF tokens should be unique")
	}

	if token1 == "" || token2 == "" {
		t.Error("Generated CSRF tokens should not be empty")
	}
}

func TestCSRFTokenValidation(t *testing.T) {
	token, err := CSRFToken()
	if err != nil {
		t.Fatalf("Failed to generate CSRF token: %v", err)
	}

	request := httptest.NewRequest("POST", "/test", nil)
	request.Form = make(map[string][]string)
	request.Form["csrf_token"] = []string{token}

	request.AddCookie(&http.Cookie{
		Name:  CSRFTokenCookieName,
		Value: token,
	})

	if !ValidateCSRFToken(request) {
		t.Error("Valid CSRF token should pass validation")
	}
}

func TestCSRFTokenValidationFailure(t *testing.T) {
	token1, err := CSRFToken()
	if err != nil {
		t.Fatalf("Failed to generate first CSRF token: %v", err)
	}

	token2, err := CSRFToken()
	if err != nil {
		t.Fatalf("Failed to generate second CSRF token: %v", err)
	}

	request := httptest.NewRequest("POST", "/test", nil)
	request.Form = make(map[string][]string)
	request.Form["csrf_token"] = []string{token1}

	request.AddCookie(&http.Cookie{
		Name:  CSRFTokenCookieName,
		Value: token2,
	})

	if ValidateCSRFToken(request) {
		t.Error("Mismatched CSRF tokens should fail validation")
	}
}

func TestCSRFTokenValidationMissingToken(t *testing.T) {
	request := httptest.NewRequest("POST", "/test", nil)

	if ValidateCSRFToken(request) {
		t.Error("Request without CSRF token should fail validation")
	}
}

func TestCSRFTokenValidationMissingCookie(t *testing.T) {
	token, err := CSRFToken()
	if err != nil {
		t.Fatalf("Failed to generate CSRF token: %v", err)
	}

	request := httptest.NewRequest("POST", "/test", nil)
	request.Form = make(map[string][]string)
	request.Form["csrf_token"] = []string{token}

	if ValidateCSRFToken(request) {
		t.Error("Request with token in form but no cookie should fail validation")
	}
}

func TestCSRFTokenValidationOnlyCookie(t *testing.T) {
	token, err := CSRFToken()
	if err != nil {
		t.Fatalf("Failed to generate CSRF token: %v", err)
	}

	request := httptest.NewRequest("POST", "/test", nil)
	request.AddCookie(&http.Cookie{
		Name:  CSRFTokenCookieName,
		Value: token,
	})

	if ValidateCSRFToken(request) {
		t.Error("Request with only cookie (no form/header token) should fail validation")
	}
}

func TestCSRFTokenValidationHeader(t *testing.T) {
	token, err := CSRFToken()
	if err != nil {
		t.Fatalf("Failed to generate CSRF token: %v", err)
	}

	request := httptest.NewRequest("POST", "/test", nil)
	request.Header.Set(CSRFTokenHeaderName, token)
	request.AddCookie(&http.Cookie{
		Name:  CSRFTokenCookieName,
		Value: token,
	})

	if !ValidateCSRFToken(request) {
		t.Error("Valid CSRF token in header should pass validation")
	}
}

func TestCSRFMiddleware(t *testing.T) {
	request := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	handler := CSRFMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("GET request should be allowed through CSRF middleware, got status %d", recorder.Code)
	}
}

func TestCSRFMiddlewareBlocksInvalidToken(t *testing.T) {
	request := httptest.NewRequest("POST", "/test", nil)
	recorder := httptest.NewRecorder()

	handler := CSRFMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("POST request without valid CSRF token should be blocked, got status %d", recorder.Code)
	}
}

func TestCSRFMiddlewareAllowsValidToken(t *testing.T) {
	token, err := CSRFToken()
	if err != nil {
		t.Fatalf("Failed to generate CSRF token: %v", err)
	}

	request := httptest.NewRequest("POST", "/test", nil)
	request.Form = make(map[string][]string)
	request.Form["csrf_token"] = []string{token}
	request.AddCookie(&http.Cookie{
		Name:  CSRFTokenCookieName,
		Value: token,
	})

	recorder := httptest.NewRecorder()

	handler := CSRFMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("POST request with valid CSRF token should be allowed, got status %d", recorder.Code)
	}
}

func TestCSRFMiddlewareSkipsAPIWithBearerToken(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/test", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	recorder := httptest.NewRecorder()

	handler := CSRFMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("API requests with Bearer token should skip CSRF validation, got status %d", recorder.Code)
	}
}

func TestCSRFMiddlewareBlocksAPIWithoutBearerToken(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/test", nil)
	recorder := httptest.NewRecorder()

	handler := CSRFMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("API requests without Bearer token should require CSRF validation, got status %d", recorder.Code)
	}
}

func TestSetCSRFToken(t *testing.T) {
	token, err := CSRFToken()
	if err != nil {
		t.Fatalf("Failed to generate CSRF token: %v", err)
	}

	request := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	SetCSRFToken(recorder, request, token)

	cookies := recorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("Expected CSRF token cookie to be set")
	}

	cookie := cookies[0]
	if cookie.Name != CSRFTokenCookieName {
		t.Errorf("Expected cookie name %s, got %s", CSRFTokenCookieName, cookie.Name)
	}

	if cookie.Value != token {
		t.Errorf("Expected cookie value %s, got %s", token, cookie.Value)
	}

	if cookie.HttpOnly {
		t.Error("CSRF token cookie must not be HttpOnly so JS can mirror it to X-CSRF-Token")
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("Expected SameSite %v, got %v", http.SameSiteLaxMode, cookie.SameSite)
	}
}
