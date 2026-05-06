package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := SecurityHeadersMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))

	request := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Server":                 "",
	}

	for header, expectedValue := range expectedHeaders {
		actualValue := recorder.Header().Get(header)
		if actualValue != expectedValue {
			t.Errorf("Expected %s: %s, got %s", header, expectedValue, actualValue)
		}
	}

	csp := recorder.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policy header should be present")
	}

	expectedCSPDirectives := []string{
		"default-src 'self'",
		"img-src 'self' data: https:",
		"font-src 'self' data:",
		"connect-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
	}

	for _, directive := range expectedCSPDirectives {
		if !strings.Contains(csp, directive) {
			t.Errorf("Content-Security-Policy should contain directive: %s", directive)
		}
	}

	if strings.Contains(csp, "'unsafe-inline'") {
		t.Error("Content-Security-Policy should NOT contain 'unsafe-inline'")
	}
	if strings.Contains(csp, "'unsafe-eval'") {
		t.Error("Content-Security-Policy should NOT contain 'unsafe-eval'")
	}

	if !strings.Contains(csp, "script-src") {
		t.Error("Content-Security-Policy should contain script-src directive")
	}
	if !strings.Contains(csp, "style-src") {
		t.Error("Content-Security-Policy should contain style-src directive")
	}

	if strings.Contains(csp, "script-src 'self'") && !strings.Contains(csp, "nonce-") {

		if !strings.Contains(csp, "script-src 'self'") {
			t.Error("Content-Security-Policy script-src should contain 'self'")
		}
	} else if !strings.Contains(csp, "nonce-") {
		t.Error("Content-Security-Policy should contain nonce-based script-src and style-src")
	}

	permissionsPolicy := recorder.Header().Get("Permissions-Policy")
	if permissionsPolicy == "" {
		t.Error("Permissions-Policy header should be present")
	}

	expectedPermissions := []string{
		"geolocation=()",
		"microphone=()",
		"camera=()",
		"payment=()",
		"usb=()",
		"magnetometer=()",
		"gyroscope=()",
		"speaker=()",
		"vibrate=()",
		"fullscreen=(self)",
		"sync-xhr=()",
	}

	for _, permission := range expectedPermissions {
		if !strings.Contains(permissionsPolicy, permission) {
			t.Errorf("Permissions-Policy should contain permission: %s", permission)
		}
	}
}

func TestHSTSMiddleware_HTTPS(t *testing.T) {
	handler := HSTSMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest("GET", "https://example.com/test", nil)
	request.TLS = &tls.ConnectionState{}
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	hsts := recorder.Header().Get("Strict-Transport-Security")
	expectedHSTS := "max-age=31536000; includeSubDomains; preload"

	if hsts != expectedHSTS {
		t.Errorf("Expected HSTS header: %s, got: %s", expectedHSTS, hsts)
	}
}

func TestHSTSMiddleware_HTTP(t *testing.T) {
	handler := HSTSMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest("GET", "http://example.com/test", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	hsts := recorder.Header().Get("Strict-Transport-Security")
	if hsts != "" {
		t.Errorf("Expected no HSTS header for HTTP request, got: %s", hsts)
	}
}

func TestSecurityHeadersMiddleware_ResponsePassthrough(t *testing.T) {
	expectedBody := "test response body"
	expectedStatus := http.StatusCreated

	handler := SecurityHeadersMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(expectedStatus)
		w.Write([]byte(expectedBody))
	}))

	request := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != expectedStatus {
		t.Errorf("Expected status %d, got %d", expectedStatus, recorder.Code)
	}

	if recorder.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, recorder.Body.String())
	}
}

func TestSecurityHeadersMiddleware_MultipleRequests(t *testing.T) {
	handler := SecurityHeadersMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := range 3 {
		request := httptest.NewRequest("GET", "/test", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		requiredHeaders := []string{
			"X-Content-Type-Options",
			"X-Frame-Options",
			"Referrer-Policy",
			"Content-Security-Policy",
			"Permissions-Policy",
		}

		for _, header := range requiredHeaders {
			if recorder.Header().Get(header) == "" {
				t.Errorf("Request %d: Expected header %s to be present", i+1, header)
			}
		}
	}
}

func TestSecurityHeadersMiddleware_ContentSecurityPolicyFormat(t *testing.T) {
	handler := SecurityHeadersMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	csp := recorder.Header().Get("Content-Security-Policy")

	if strings.Contains(csp, "  ") {
		t.Error("Content-Security-Policy should not contain double spaces")
	}

	directives := strings.Split(csp, "; ")
	if len(directives) < 8 {
		t.Errorf("Content-Security-Policy should have at least 8 directives, got %d", len(directives))
	}

	if strings.HasSuffix(csp, ";") {
		t.Error("Content-Security-Policy should not end with semicolon")
	}
}

func TestSecurityHeadersMiddleware_PermissionsPolicyFormat(t *testing.T) {
	handler := SecurityHeadersMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	permissionsPolicy := recorder.Header().Get("Permissions-Policy")

	if strings.Contains(permissionsPolicy, "  ") {
		t.Error("Permissions-Policy should not contain double spaces")
	}

	permissions := strings.Split(permissionsPolicy, ", ")
	if len(permissions) < 10 {
		t.Errorf("Permissions-Policy should have at least 10 permissions, got %d", len(permissions))
	}

	if strings.HasSuffix(permissionsPolicy, ",") {
		t.Error("Permissions-Policy should not end with comma")
	}
}

func TestCSPNonceGeneration(t *testing.T) {

	nonce1, err := GenerateCSPNonce()
	if err != nil {
		t.Fatalf("Failed to generate CSP nonce: %v", err)
	}

	if nonce1 == "" {
		t.Error("Generated nonce should not be empty")
	}

	if len(nonce1) < 16 {
		t.Errorf("Generated nonce should be at least 16 characters, got %d", len(nonce1))
	}

	nonce2, err := GenerateCSPNonce()
	if err != nil {
		t.Fatalf("Failed to generate second CSP nonce: %v", err)
	}

	if nonce1 == nonce2 {
		t.Error("Generated nonces should be unique")
	}
}

func TestCSPNonceInContext(t *testing.T) {
	var capturedNonce string

	handler := SecurityHeadersMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedNonce = GetCSPNonceFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if capturedNonce == "" {
		t.Error("CSP nonce should be available in request context")
	}

	csp := recorder.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "nonce-"+capturedNonce) {
		t.Errorf("CSP header should contain nonce from context. CSP: %s, Nonce: %s", csp, capturedNonce)
	}
}

func TestSecurityHeadersMiddleware_SwaggerStrictWhenRelaxedDisabled(t *testing.T) {
	handler := SecurityHeadersMiddlewareWithConfig(SecurityHeadersConfig{RelaxSwaggerCSP: false})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

	req := httptest.NewRequest("GET", "/swagger/index.html", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("expected CSP")
	}
	if strings.Contains(csp, "'unsafe-eval'") || strings.Contains(csp, "'unsafe-inline'") {
		t.Fatalf("unexpected relaxed CSP for swagger path: %s", csp)
	}
	if !strings.Contains(csp, "nonce-") {
		t.Fatalf("expected nonce CSP for swagger path, got %s", csp)
	}
}
