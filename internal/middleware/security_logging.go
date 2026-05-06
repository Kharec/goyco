package middleware

import (
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type SecurityLogger struct {
	logger *log.Logger
}

func NewSecurityLogger() *SecurityLogger {
	return &SecurityLogger{
		logger: log.New(os.Stdout, "[SECURITY] ", log.LstdFlags|log.Lshortfile),
	}
}

type SecurityEvent struct {
	Type      string
	IP        string
	UserAgent string
	Path      string
	Method    string
	UserID    uint
	Details   string
	Timestamp time.Time
	Severity  string
}

func (sl *SecurityLogger) LogSecurityEvent(event SecurityEvent) {
	sl.logger.Printf("[%s] %s - %s %s %s - UserID: %d - %s - %s",
		event.Severity,
		event.IP,
		event.Method,
		event.Path,
		event.UserAgent,
		event.UserID,
		event.Type,
		event.Details,
	)
}

func SecurityLoggingMiddleware(logger *SecurityLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rw := &securityResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, r)

			userID := GetUserIDFromContext(r.Context())
			ip := getClientIP(r)

			event := SecurityEvent{
				IP:        ip,
				UserAgent: r.UserAgent(),
				Path:      r.URL.Path,
				Method:    r.Method,
				UserID:    userID,
				Timestamp: start,
			}

			switch {
			case rw.statusCode >= 400 && rw.statusCode < 500:
				event.Type = "Client Error"
				event.Severity = "WARN"
				event.Details = "Client error response"
			case rw.statusCode >= 500:
				event.Type = "Server Error"
				event.Severity = "ERROR"
				event.Details = "Server error response"
			case strings.HasPrefix(r.URL.Path, "/api/auth/"):
				event.Type = "Authentication"
				event.Severity = "INFO"
				event.Details = "Authentication endpoint accessed"
			case strings.HasPrefix(r.URL.Path, "/api/posts/") && r.Method == "POST":
				event.Type = "Post Creation"
				event.Severity = "INFO"
				event.Details = "Post creation attempt"
			case strings.HasPrefix(r.URL.Path, "/api/posts/") && (r.Method == "PUT" || r.Method == "DELETE"):
				event.Type = "Post Modification"
				event.Severity = "INFO"
				event.Details = "Post modification attempt"
			default:
				event.Type = "API Access"
				event.Severity = "INFO"
				event.Details = "API endpoint accessed"
			}

			logger.LogSecurityEvent(event)
		})
	}
}

func SuspiciousActivityMiddleware(logger *SecurityLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)
			userAgent := r.UserAgent()

			suspicious := false
			details := ""

			if containsSQLInjection(r.URL.RawQuery) || containsSQLInjection(r.URL.Path) {
				suspicious = true
				details = "Potential SQL injection attempt"
			}

			if containsXSS(r.URL.RawQuery) || containsXSS(r.URL.Path) {
				suspicious = true
				details = "Potential XSS attempt"
			}

			if isSuspiciousUserAgent(userAgent) {
				suspicious = true
				details = "Suspicious user agent"
			}

			if isRapidRequest(ip) {
				suspicious = true
				details = "Rapid request pattern"
			}

			if suspicious {
				event := SecurityEvent{
					Type:      "Suspicious Activity",
					IP:        ip,
					UserAgent: userAgent,
					Path:      r.URL.Path,
					Method:    r.Method,
					Details:   details,
					Timestamp: time.Now(),
					Severity:  "WARN",
				}
				logger.LogSecurityEvent(event)
			}

			next.ServeHTTP(w, r)
		})
	}
}

type securityResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *securityResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func getClientIP(r *http.Request) string {
	return GetSecureClientIP(r)
}

func containsSQLInjection(input string) bool {
	sqlPatterns := []string{
		"' OR '1'='1",
		"'; DROP TABLE",
		"UNION SELECT",
		"INSERT INTO",
		"DELETE FROM",
		"UPDATE SET",
	}

	input = strings.ToUpper(input)
	for _, pattern := range sqlPatterns {
		if strings.Contains(input, strings.ToUpper(pattern)) {
			return true
		}
	}
	return false
}

func containsXSS(input string) bool {
	xssPatterns := []string{
		"<script>",
		"javascript:",
		"onload=",
		"onerror=",
		"onclick=",
		"<iframe>",
		"<img src=",
	}

	input = strings.ToLower(input)
	for _, pattern := range xssPatterns {
		if strings.Contains(input, pattern) {
			return true
		}
	}
	return false
}

func isSuspiciousUserAgent(userAgent string) bool {
	suspiciousPatterns := []string{
		"sqlmap",
		"nikto",
		"nmap",
		"masscan",
		"zap",
		"burp",
		"w3af",
		"havij",
		"acunetix",
		"nessus",
	}

	userAgent = strings.ToLower(userAgent)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(userAgent, pattern) {
			return true
		}
	}
	return false
}

type rapidRequestTracker struct {
	mu        sync.Mutex
	counts    map[string]int
	lastReset time.Time
}

var rapidRequests = rapidRequestTracker{
	counts:    make(map[string]int),
	lastReset: time.Now(),
}

func isRapidRequest(ip string) bool {
	rapidRequests.mu.Lock()
	defer rapidRequests.mu.Unlock()

	now := time.Now()

	if now.Sub(rapidRequests.lastReset) > time.Minute {
		rapidRequests.counts = make(map[string]int)
		rapidRequests.lastReset = now
	}

	rapidRequests.counts[ip]++

	return rapidRequests.counts[ip] > 100
}
