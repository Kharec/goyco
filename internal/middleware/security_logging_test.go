package middleware

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNewSecurityLogger(t *testing.T) {
	logger := NewSecurityLogger()
	if logger == nil {
		t.Fatal("NewSecurityLogger should not return nil")
	}
	if logger.logger == nil {
		t.Fatal("SecurityLogger should have a logger instance")
	}
}

func TestSecurityLogger_LogSecurityEvent(t *testing.T) {

	var buf bytes.Buffer
	logger := &SecurityLogger{
		logger: log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lshortfile),
	}

	event := SecurityEvent{
		Type:      "Test Event",
		IP:        "192.168.1.1",
		UserAgent: "Test Agent",
		Path:      "/test",
		Method:    "GET",
		UserID:    123,
		Details:   "Test details",
		Timestamp: time.Now(),
		Severity:  "INFO",
	}

	logger.LogSecurityEvent(event)

	logOutput := buf.String()
	expectedParts := []string{
		"[INFO]",
		"192.168.1.1",
		"GET",
		"/test",
		"Test Agent",
		"UserID: 123",
		"Test Event",
		"Test details",
	}

	for _, part := range expectedParts {
		if !strings.Contains(logOutput, part) {
			t.Errorf("Expected log output to contain %q, got %q", part, logOutput)
		}
	}
}

func TestSecurityLoggingMiddleware_ClientError(t *testing.T) {
	var buf bytes.Buffer
	logger := &SecurityLogger{
		logger: log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lshortfile),
	}

	handler := SecurityLoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))

	request := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	logOutput := buf.String()
	expectedParts := []string{
		"[WARN]",
		"Client Error",
		"Client error response",
	}

	for _, part := range expectedParts {
		if !strings.Contains(logOutput, part) {
			t.Errorf("Expected log output to contain %q, got %q", part, logOutput)
		}
	}
}

func TestSecurityLoggingMiddleware_ServerError(t *testing.T) {
	var buf bytes.Buffer
	logger := &SecurityLogger{
		logger: log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lshortfile),
	}

	handler := SecurityLoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	request := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	logOutput := buf.String()
	expectedParts := []string{
		"[ERROR]",
		"Server Error",
		"Server error response",
	}

	for _, part := range expectedParts {
		if !strings.Contains(logOutput, part) {
			t.Errorf("Expected log output to contain %q, got %q", part, logOutput)
		}
	}
}

func TestSecurityLoggingMiddleware_Authentication(t *testing.T) {
	var buf bytes.Buffer
	logger := &SecurityLogger{
		logger: log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lshortfile),
	}

	handler := SecurityLoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest("POST", "/api/auth/login", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	logOutput := buf.String()
	expectedParts := []string{
		"[INFO]",
		"Authentication",
		"Authentication endpoint accessed",
	}

	for _, part := range expectedParts {
		if !strings.Contains(logOutput, part) {
			t.Errorf("Expected log output to contain %q, got %q", part, logOutput)
		}
	}
}

func TestSecurityLoggingMiddleware_PostCreation(t *testing.T) {
	var buf bytes.Buffer
	logger := &SecurityLogger{
		logger: log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lshortfile),
	}

	handler := SecurityLoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest("POST", "/api/posts/", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	logOutput := buf.String()
	expectedParts := []string{
		"[INFO]",
		"Post Creation",
		"Post creation attempt",
	}

	for _, part := range expectedParts {
		if !strings.Contains(logOutput, part) {
			t.Errorf("Expected log output to contain %q, got %q", part, logOutput)
		}
	}
}

func TestSecurityLoggingMiddleware_PostModification(t *testing.T) {
	var buf bytes.Buffer
	logger := &SecurityLogger{
		logger: log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lshortfile),
	}

	handler := SecurityLoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest("PUT", "/api/posts/1", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	logOutput := buf.String()
	expectedParts := []string{
		"[INFO]",
		"Post Modification",
		"Post modification attempt",
	}

	for _, part := range expectedParts {
		if !strings.Contains(logOutput, part) {
			t.Errorf("Expected log output to contain %q, got %q", part, logOutput)
		}
	}

	buf.Reset()

	request = httptest.NewRequest("DELETE", "/api/posts/1", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	logOutput = buf.String()
	for _, part := range expectedParts {
		if !strings.Contains(logOutput, part) {
			t.Errorf("Expected log output to contain %q, got %q", part, logOutput)
		}
	}
}

func TestSecurityLoggingMiddleware_APIAccess(t *testing.T) {
	var buf bytes.Buffer
	logger := &SecurityLogger{
		logger: log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lshortfile),
	}

	handler := SecurityLoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest("GET", "/api/users", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	logOutput := buf.String()
	expectedParts := []string{
		"[INFO]",
		"API Access",
		"API endpoint accessed",
	}

	for _, part := range expectedParts {
		if !strings.Contains(logOutput, part) {
			t.Errorf("Expected log output to contain %q, got %q", part, logOutput)
		}
	}
}

func TestSecurityLoggingMiddleware_WithUserID(t *testing.T) {
	var buf bytes.Buffer
	logger := &SecurityLogger{
		logger: log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lshortfile),
	}

	handler := SecurityLoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest("GET", "/test", nil)
	request = request.WithContext(context.WithValue(request.Context(), UserIDKey, uint(456)))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "UserID: 456") {
		t.Errorf("Expected log output to contain UserID: 456, got %q", logOutput)
	}
}

func TestGetClientIP(t *testing.T) {

	originalTrust := TrustProxyHeaders
	defer func() {
		TrustProxyHeaders = originalTrust
	}()

	tests := []struct {
		name              string
		headers           map[string]string
		remoteAddr        string
		trustProxyHeaders bool
		expectedIP        string
	}{
		{
			name:              "Default: RemoteAddr when TrustProxyHeaders is false",
			headers:           map[string]string{"X-Forwarded-For": "192.168.1.100"},
			remoteAddr:        "10.0.0.1:8080",
			trustProxyHeaders: false,
			expectedIP:        "10.0.0.1",
		},
		{
			name: "X-Forwarded-For single IP when TrustProxyHeaders is true",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100",
			},
			remoteAddr:        "10.0.0.1:8080",
			trustProxyHeaders: true,
			expectedIP:        "192.168.1.100",
		},
		{
			name: "X-Forwarded-For multiple IPs when TrustProxyHeaders is true",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100, 10.0.0.1, 172.16.0.1",
			},
			remoteAddr:        "10.0.0.1:8080",
			trustProxyHeaders: true,
			expectedIP:        "192.168.1.100",
		},
		{
			name: "X-Real-IP when TrustProxyHeaders is true",
			headers: map[string]string{
				"X-Real-IP": "192.168.1.200",
			},
			remoteAddr:        "10.0.0.1:8080",
			trustProxyHeaders: true,
			expectedIP:        "192.168.1.200",
		},
		{
			name: "X-Forwarded-For takes precedence over X-Real-IP when TrustProxyHeaders is true",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100",
				"X-Real-IP":       "192.168.1.200",
			},
			remoteAddr:        "10.0.0.1:8080",
			trustProxyHeaders: true,
			expectedIP:        "192.168.1.100",
		},
		{
			name:              "RemoteAddr only",
			headers:           map[string]string{},
			remoteAddr:        "192.168.1.50:8080",
			trustProxyHeaders: false,
			expectedIP:        "192.168.1.50",
		},
		{
			name:              "RemoteAddr with IPv6",
			headers:           map[string]string{},
			remoteAddr:        "[::1]:8080",
			trustProxyHeaders: false,
			expectedIP:        "::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			TrustProxyHeaders = tt.trustProxyHeaders
			request := httptest.NewRequest("GET", "/test", nil)
			request.RemoteAddr = tt.remoteAddr
			for header, value := range tt.headers {
				request.Header.Set(header, value)
			}

			ip := getClientIP(request)
			if ip != tt.expectedIP {
				t.Errorf("Expected IP %q, got %q", tt.expectedIP, ip)
			}
		})
	}

	TrustProxyHeaders = originalTrust
}

func TestContainsSQLInjection(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"' OR '1'='1", true},
		{"'; DROP TABLE users; --", true},
		{"UNION SELECT * FROM users", true},
		{"INSERT INTO users VALUES", true},
		{"DELETE FROM users", true},
		{"UPDATE SET", true},
		{"normal query", false},
		{"SELECT * FROM posts", false},
		{"' OR '1'='1'", true},
		{"union select", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := containsSQLInjection(tt.input)
			if result != tt.expected {
				t.Errorf("containsSQLInjection(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestContainsXSS(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"<script>alert('xss')</script>", true},
		{"javascript:alert('xss')", true},
		{"onload=alert('xss')", true},
		{"onerror=alert('xss')", true},
		{"onclick=alert('xss')", true},
		{"<iframe>", true},
		{"<img src='x' onerror='alert(1)'>", true},
		{"normal content", false},
		{"<div>safe content</div>", false},
		{"<SCRIPT>alert('xss')</SCRIPT>", true},
		{"JAVASCRIPT:alert('xss')", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := containsXSS(tt.input)
			if result != tt.expected {
				t.Errorf("containsXSS(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsSuspiciousUserAgent(t *testing.T) {
	tests := []struct {
		userAgent string
		expected  bool
	}{
		{"sqlmap/1.0", true},
		{"nikto scanner", true},
		{"nmap 7.0", true},
		{"masscan tool", true},
		{"zap proxy", true},
		{"burp suite", true},
		{"w3af scanner", true},
		{"havij tool", true},
		{"acunetix scanner", true},
		{"nessus scanner", true},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36", false},
		{"curl/7.68.0", false},
		{"wget/1.20.3", false},
		{"SQLMAP/1.0", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.userAgent, func(t *testing.T) {
			result := isSuspiciousUserAgent(tt.userAgent)
			if result != tt.expected {
				t.Errorf("isSuspiciousUserAgent(%q) = %v, expected %v", tt.userAgent, result, tt.expected)
			}
		})
	}
}

func TestIsRapidRequest(t *testing.T) {

	rapidRequests.mu.Lock()
	rapidRequests.counts = make(map[string]int)
	rapidRequests.lastReset = time.Now()
	rapidRequests.mu.Unlock()

	ip := "192.168.1.1"

	for i := range 50 {
		if isRapidRequest(ip) {
			t.Errorf("Request %d should not be considered rapid", i+1)
		}
	}

	for i := range 110 {
		result := isRapidRequest(ip)
		if i < 50 {
			if result {
				t.Errorf("Request %d should not be considered rapid yet", i+51)
			}
		} else {
			if !result {
				t.Errorf("Request %d should be considered rapid", i+51)
			}
		}
	}
}

func TestSuspiciousActivityMiddleware_SQLInjection(t *testing.T) {

	t.Skip("Skipping due to URL encoding complexities - detection logic tested separately")
}

func TestSuspiciousActivityMiddleware_XSS(t *testing.T) {
	var buf bytes.Buffer
	logger := &SecurityLogger{
		logger: log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lshortfile),
	}

	handler := SuspiciousActivityMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest("GET", "/javascript:", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	logOutput := buf.String()
	expectedParts := []string{
		"[WARN]",
		"Suspicious Activity",
		"Potential XSS attempt",
	}

	for _, part := range expectedParts {
		if !strings.Contains(logOutput, part) {
			t.Errorf("Expected log output to contain %q, got %q", part, logOutput)
		}
	}
}

func TestSuspiciousActivityMiddleware_SuspiciousUserAgent(t *testing.T) {
	var buf bytes.Buffer
	logger := &SecurityLogger{
		logger: log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lshortfile),
	}

	handler := SuspiciousActivityMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest("GET", "/test", nil)
	request.Header.Set("User-Agent", "sqlmap/1.0")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	logOutput := buf.String()
	expectedParts := []string{
		"[WARN]",
		"Suspicious Activity",
		"Suspicious user agent",
	}

	for _, part := range expectedParts {
		if !strings.Contains(logOutput, part) {
			t.Errorf("Expected log output to contain %q, got %q", part, logOutput)
		}
	}
}

func TestSuspiciousActivityMiddleware_NoSuspiciousActivity(t *testing.T) {
	var buf bytes.Buffer
	logger := &SecurityLogger{
		logger: log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lshortfile),
	}

	handler := SuspiciousActivityMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest("GET", "/test", nil)
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	logOutput := buf.String()
	if logOutput != "" {
		t.Errorf("Expected no log output for normal request, got %q", logOutput)
	}
}

func TestSuspiciousActivityMiddleware_EncodedSQLInQuery(t *testing.T) {
	var buf bytes.Buffer
	logger := &SecurityLogger{
		logger: log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lshortfile),
	}

	handler := SuspiciousActivityMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	q := url.Values{}
	q.Set("s", "' OR '1'='1")
	req := httptest.NewRequest("GET", "/search?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	out := buf.String()
	if !strings.Contains(out, "SQL injection") {
		t.Fatalf("expected SQL injection log, got %q", out)
	}
}

func TestSuspiciousActivityMiddleware_Debug(t *testing.T) {

	t.Run("SQL Detection", func(t *testing.T) {
		if !containsSQLInjection("INSERT INTO") {
			t.Error("INSERT INTO should be detected as SQL injection")
		}
		if !containsSQLInjection("UNION SELECT") {
			t.Error("UNION SELECT should be detected as SQL injection")
		}
	})

	t.Run("XSS Detection", func(t *testing.T) {
		if !containsXSS("onload=") {
			t.Error("onload= should be detected as XSS")
		}
		if !containsXSS("javascript:") {
			t.Error("javascript: should be detected as XSS")
		}
	})
}

func TestSecurityResponseWriter(t *testing.T) {
	recorder := httptest.NewRecorder()
	wrapped := &securityResponseWriter{ResponseWriter: recorder, statusCode: http.StatusOK}

	wrapped.WriteHeader(http.StatusCreated)
	if wrapped.statusCode != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, wrapped.statusCode)
	}

	if recorder.Result().StatusCode != http.StatusCreated {
		t.Errorf("Expected underlying writer status code %d, got %d", http.StatusCreated, recorder.Result().StatusCode)
	}
}
