package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSWithAuthHeader(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins:   []string{"http://example.com"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		MaxAge:           3600,
		AllowCredentials: true,
	}

	handler := CORSWithConfig(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	testCases := []struct {
		name           string
		origin         string
		path           string
		hasAuth        bool
		expectedOrigin string
		expectedStatus int
	}{
		{
			name:           "Allowed origin with auth on API path",
			origin:         "http://example.com",
			path:           "/api/test",
			hasAuth:        true,
			expectedOrigin: "http://example.com",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Disallowed origin with auth on API path",
			origin:         "http://malicious.com",
			path:           "/api/test",
			hasAuth:        true,
			expectedOrigin: "",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Allowed origin without auth on API path",
			origin:         "http://example.com",
			path:           "/api/test",
			hasAuth:        false,
			expectedOrigin: "http://example.com",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Disallowed origin without auth on API path",
			origin:         "http://malicious.com",
			path:           "/api/test",
			hasAuth:        false,
			expectedOrigin: "",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Allowed origin with auth on non-API path",
			origin:         "http://example.com",
			path:           "/public/page",
			hasAuth:        true,
			expectedOrigin: "http://example.com",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Disallowed origin with auth on non-API path",
			origin:         "http://malicious.com",
			path:           "/public/page",
			hasAuth:        true,
			expectedOrigin: "",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			req.Header.Set("Origin", tc.origin)
			if tc.hasAuth {
				req.Header.Set("Authorization", "Bearer fake-token")
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}
			if w.Header().Get("Access-Control-Allow-Origin") != tc.expectedOrigin {
				t.Errorf("Expected Access-Control-Allow-Origin to be '%s', got '%s'",
					tc.expectedOrigin, w.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestCORSWithConfig_AllowedOrigin(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins:   []string{"http://example.com"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type"},
		MaxAge:           3600,
		AllowCredentials: true,
	}

	handler := CORSWithConfig(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin to be 'http://example.com', got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("Expected Access-Control-Allow-Credentials to be 'true', got '%s'", w.Header().Get("Access-Control-Allow-Credentials"))
	}
}

func TestCORSWithConfig_DisallowedOrigin(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins:   []string{"http://example.com"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type"},
		MaxAge:           3600,
		AllowCredentials: false,
	}

	handler := CORSWithConfig(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://malicious.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for disallowed origin, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("Expected Access-Control-Allow-Origin to be empty for disallowed origin, got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSWithConfig_WildcardOrigin(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type"},
		MaxAge:           3600,
		AllowCredentials: false,
	}

	handler := CORSWithConfig(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin to be '*', got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
	}

	if w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Errorf("Expected Access-Control-Allow-Credentials to be empty with wildcard, got '%s'", w.Header().Get("Access-Control-Allow-Credentials"))
	}
}

func TestCORSWithConfig_WildcardWithCredentials(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type"},
		MaxAge:           3600,
		AllowCredentials: true,
	}

	handler := CORSWithConfig(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin to be 'http://example.com', got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
	}

	if w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Errorf("Expected Access-Control-Allow-Credentials to be empty with wildcard, got '%s'", w.Header().Get("Access-Control-Allow-Credentials"))
	}
}

func TestCORSWithConfig_NoOriginHeader(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins:   []string{"http://example.com"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type"},
		MaxAge:           3600,
		AllowCredentials: false,
	}

	handler := CORSWithConfig(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)

	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("Expected Access-Control-Allow-Origin to be empty, got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSWithConfig_NoOriginWithWildcard(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type"},
		MaxAge:           3600,
		AllowCredentials: false,
	}

	handler := CORSWithConfig(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)

	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("Expected Access-Control-Allow-Origin to be empty (no origin in request), got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSWithConfig_PreflightRequest(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins:   []string{"http://example.com"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		MaxAge:           86400,
		AllowCredentials: true,
	}

	handler := CORSWithConfig(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called for OPTIONS request")
	}))

	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin to be 'http://example.com', got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Methods") != "GET, POST, PUT, DELETE" {
		t.Errorf("Expected Access-Control-Allow-Methods to be 'GET, POST, PUT, DELETE', got '%s'", w.Header().Get("Access-Control-Allow-Methods"))
	}
	if w.Header().Get("Access-Control-Allow-Headers") != "Content-Type, Authorization" {
		t.Errorf("Expected Access-Control-Allow-Headers to be 'Content-Type, Authorization', got '%s'", w.Header().Get("Access-Control-Allow-Headers"))
	}
	if w.Header().Get("Access-Control-Max-Age") != "86400" {
		t.Errorf("Expected Access-Control-Max-Age to be '86400', got '%s'", w.Header().Get("Access-Control-Max-Age"))
	}
}

func TestCORSWithConfig_MultipleAllowedOrigins(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins:   []string{"http://example1.com", "http://example2.com", "http://example3.com"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type"},
		MaxAge:           3600,
		AllowCredentials: true,
	}

	testCases := []struct {
		origin   string
		expected string
		status   int
	}{
		{"http://example1.com", "http://example1.com", http.StatusOK},
		{"http://example2.com", "http://example2.com", http.StatusOK},
		{"http://example3.com", "http://example3.com", http.StatusOK},
		{"http://notallowed.com", "", http.StatusForbidden},
	}

	for _, tc := range testCases {
		t.Run(tc.origin, func(t *testing.T) {
			handler := CORSWithConfig(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tc.status {
				t.Errorf("For origin '%s', expected status %d, got %d", tc.origin, tc.status, w.Code)
			}
			if w.Header().Get("Access-Control-Allow-Origin") != tc.expected {
				t.Errorf("For origin '%s', expected Access-Control-Allow-Origin to be '%s', got '%s'",
					tc.origin, tc.expected, w.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestCORSWithConfig_CORSHeaders(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins:   []string{"http://example.com"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Custom-Header"},
		MaxAge:           7200,
		AllowCredentials: true,
	}

	handler := CORSWithConfig(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin to be 'http://example.com', got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("Expected Access-Control-Allow-Credentials to be 'true', got '%s'", w.Header().Get("Access-Control-Allow-Credentials"))
	}

}

func TestCORSOPTIONSRequest(t *testing.T) {
	t.Setenv("GOYCO_ENV", "development")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,https://yourdomain.com")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("should not reach handler"))
	})

	middleware := CORS(handler)
	request := httptest.NewRequest("OPTIONS", "/api/posts", nil)
	request.Header.Set("Origin", "http://localhost:3000")

	recorder := httptest.NewRecorder()
	middleware.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	if recorder.Body.String() != "" {
		t.Error("OPTIONS request should not reach the handler")
	}
}

func TestCORSAllowedOrigins(t *testing.T) {
	t.Setenv("GOYCO_ENV", "development")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,https://yourdomain.com")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := CORS(handler)

	allowedOrigins := []string{
		"http://localhost:3000",
		"https://yourdomain.com",
	}

	unauthorizedOrigins := []string{
		"https://malicious.com",
		"http://evil.com",
		"https://attacker.net",
	}

	for _, origin := range allowedOrigins {
		request := httptest.NewRequest("GET", "/api/auth/me", nil)
		request.Header.Set("Origin", origin)
		request.Header.Set("Authorization", "Bearer token123")

		recorder := httptest.NewRecorder()
		middleware.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Origin %s should be allowed, got status %d", origin, recorder.Code)
		}
		actualOrigin := recorder.Header().Get("Access-Control-Allow-Origin")
		if actualOrigin != origin {
			t.Errorf("Origin %s should be allowed, got Access-Control-Allow-Origin %s", origin, actualOrigin)
		}
	}

	for _, origin := range unauthorizedOrigins {
		request := httptest.NewRequest("GET", "/api/auth/me", nil)
		request.Header.Set("Origin", origin)
		request.Header.Set("Authorization", "Bearer token123")

		recorder := httptest.NewRecorder()
		middleware.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusForbidden {
			t.Errorf("Origin %s should be blocked (403), got status %d", origin, recorder.Code)
		}
		actualOrigin := recorder.Header().Get("Access-Control-Allow-Origin")
		if actualOrigin != "" {
			t.Errorf("Origin %s should be blocked, got Access-Control-Allow-Origin %s", origin, actualOrigin)
		}
	}
}

func TestCORSWithoutOrigin(t *testing.T) {
	testCases := []struct {
		name                string
		allowedOrigins      []string
		expectedAllowOrigin string
		shouldSetHeader     bool
	}{
		{
			name:                "No origin header with wildcard config",
			allowedOrigins:      []string{"*"},
			expectedAllowOrigin: "",
			shouldSetHeader:     false,
		},
		{
			name:                "No origin header without wildcard config",
			allowedOrigins:      []string{"http://example.com"},
			expectedAllowOrigin: "",
			shouldSetHeader:     false,
		},
		{
			name:                "No origin header with multiple specific origins",
			allowedOrigins:      []string{"http://example1.com", "http://example2.com"},
			expectedAllowOrigin: "",
			shouldSetHeader:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &CORSConfig{
				AllowedOrigins:   tc.allowedOrigins,
				AllowedMethods:   []string{"GET", "POST"},
				AllowedHeaders:   []string{"Content-Type"},
				MaxAge:           3600,
				AllowCredentials: false,
			}

			handler := CORSWithConfig(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/", nil)

			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			actualOrigin := w.Header().Get("Access-Control-Allow-Origin")

			if tc.shouldSetHeader {
				if actualOrigin != tc.expectedAllowOrigin {
					t.Errorf("Expected Access-Control-Allow-Origin to be '%s', got '%s'",
						tc.expectedAllowOrigin, actualOrigin)
				}
			} else {
				if actualOrigin != "" {
					t.Errorf("Expected Access-Control-Allow-Origin to be empty (not set), got '%s'",
						actualOrigin)
				}
			}
		})
	}
}
