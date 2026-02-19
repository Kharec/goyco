package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func init() {
	StopAllRateLimiters()
}

type mockClock struct {
	mu  sync.RWMutex
	now time.Time
}

func newMockClock() *mockClock {
	return &mockClock{
		now: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func (c *mockClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

func (c *mockClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func (c *mockClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func TestRateLimiterAllow(t *testing.T) {
	limiter := NewRateLimiter(1*time.Minute, 3)
	defer limiter.StopCleanup()

	for i := range 3 {
		if !limiter.Allow("test-key") {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	if limiter.Allow("test-key") {
		t.Error("4th request should be rejected")
	}
}

func TestRateLimiterWindow(t *testing.T) {
	clock := newMockClock()
	limiter := newRateLimiterWithClock(50*time.Millisecond, 2, clock)

	limiter.Allow("test-key")
	limiter.Allow("test-key")

	if limiter.Allow("test-key") {
		t.Error("Request should be rejected at limit")
	}

	clock.Advance(75 * time.Millisecond)

	if !limiter.Allow("test-key") {
		t.Error("Request should be allowed after window reset")
	}
}

func TestRateLimiterDifferentKeys(t *testing.T) {
	limiter := NewRateLimiter(1*time.Minute, 2)
	defer limiter.StopCleanup()

	limiter.Allow("key1")
	limiter.Allow("key1")
	limiter.Allow("key2")
	limiter.Allow("key2")

	if limiter.Allow("key1") {
		t.Error("key1 should be at limit")
	}
	if limiter.Allow("key2") {
		t.Error("key2 should be at limit")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	defer StopAllRateLimiters()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RateLimitMiddleware(1*time.Minute, 2)
	server := middleware(handler)

	for i := range 2 {
		request := httptest.NewRequest("GET", "/test", nil)
		request.RemoteAddr = "127.0.0.1:12345"
		recorder := httptest.NewRecorder()

		server.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Request %d should be allowed, got status %d", i+1, recorder.Code)
		}
	}

	request := httptest.NewRequest("GET", "/test", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", recorder.Code)
	}

	retryAfter := recorder.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Expected Retry-After header")
	}

	retryAfterVal, err := time.ParseDuration(retryAfter + "s")
	if err != nil {
		t.Errorf("Retry-After header value is not a valid duration: %q", retryAfter)
	}
	if retryAfterVal.Seconds() < 50 || retryAfterVal.Seconds() > 60 {
		t.Errorf("Retry-After should be approximately 60 seconds, got %.0f", retryAfterVal.Seconds())
	}

	var jsonResponse struct {
		Error      string  `json:"error"`
		Message    string  `json:"message"`
		RetryAfter float64 `json:"retry_after"`
	}

	body := recorder.Body.String()
	if err := json.Unmarshal([]byte(body), &jsonResponse); err != nil {
		t.Fatalf("Failed to decode JSON response: %v, body: %s", err, body)
	}

	if jsonResponse.Error != "Rate limit exceeded" {
		t.Errorf("Expected error 'Rate limit exceeded', got %q", jsonResponse.Error)
	}

	if !strings.Contains(jsonResponse.Message, "Too many requests") {
		t.Errorf("Expected message to contain 'Too many requests', got %q", jsonResponse.Message)
	}

	expectedRetryAfter := int(retryAfterVal.Seconds())
	actualRetryAfter := int(jsonResponse.RetryAfter)
	diff := actualRetryAfter - expectedRetryAfter
	if diff < -1 || diff > 0 {
		t.Errorf("Expected retry_after %d in JSON (within 1s), got %.0f", expectedRetryAfter, jsonResponse.RetryAfter)
	}

	if jsonResponse.RetryAfter <= 0 {
		t.Errorf("Expected retry_after to be positive, got %.0f", jsonResponse.RetryAfter)
	}

	if !strings.Contains(jsonResponse.Message, "Too many requests. Please try again in") {
		t.Errorf("Expected message to contain 'Too many requests. Please try again in', got %q", jsonResponse.Message)
	}
	if !strings.Contains(jsonResponse.Message, "seconds.") {
		t.Errorf("Expected message to end with 'seconds.', got %q", jsonResponse.Message)
	}
}

func TestAuthRateLimitMiddleware(t *testing.T) {
	defer StopAllRateLimiters()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := AuthRateLimitMiddleware()
	server := middleware(handler)

	for i := range 5 {
		request := httptest.NewRequest("POST", "/api/auth/login", nil)
		request.RemoteAddr = "127.0.0.1:12345"
		recorder := httptest.NewRecorder()

		server.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Request %d should be allowed, got status %d", i+1, recorder.Code)
		}
	}

	request := httptest.NewRequest("POST", "/api/auth/login", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", recorder.Code)
	}
}

func TestGeneralRateLimitMiddleware(t *testing.T) {
	defer StopAllRateLimiters()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := GeneralRateLimitMiddleware()
	server := middleware(handler)

	for i := range 10 {
		request := httptest.NewRequest("GET", "/api/posts", nil)
		request.RemoteAddr = "127.0.0.1:12345"
		recorder := httptest.NewRecorder()

		server.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Request %d should be allowed, got status %d", i+1, recorder.Code)
		}
	}
}

func TestGetKey(t *testing.T) {

	originalTrust := TrustProxyHeaders
	defer func() {
		TrustProxyHeaders = originalTrust
	}()

	TrustProxyHeaders = false
	request := httptest.NewRequest("GET", "/test", nil)
	request.RemoteAddr = "192.168.1.1:12345"
	key := GetKey(request)
	expected := "ip:192.168.1.1"
	if key != expected {
		t.Errorf("Expected key %s, got %s", expected, key)
	}

	request = httptest.NewRequest("GET", "/test", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("X-Forwarded-For", "203.0.113.1")
	key = GetKey(request)
	expected = "ip:127.0.0.1"
	if key != expected {
		t.Errorf("Expected key %s (proxy header ignored), got %s", expected, key)
	}

	TrustProxyHeaders = true
	request = httptest.NewRequest("GET", "/test", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("X-Forwarded-For", "203.0.113.1")
	key = GetKey(request)
	expected = "ip:203.0.113.1"
	if key != expected {
		t.Errorf("Expected key %s, got %s", expected, key)
	}

	request = httptest.NewRequest("GET", "/test", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1, 192.0.2.1")
	key = GetKey(request)
	expected = "ip:203.0.113.1"
	if key != expected {
		t.Errorf("Expected key %s (leftmost IP), got %s", expected, key)
	}

	request = httptest.NewRequest("GET", "/test", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("X-Real-IP", "198.51.100.1")
	key = GetKey(request)
	expected = "ip:198.51.100.1"
	if key != expected {
		t.Errorf("Expected key %s, got %s", expected, key)
	}

	TrustProxyHeaders = originalTrust
}

func TestGetSecureClientIP(t *testing.T) {

	originalTrust := TrustProxyHeaders
	defer func() {
		TrustProxyHeaders = originalTrust
	}()

	TrustProxyHeaders = false
	request := httptest.NewRequest("GET", "/test", nil)
	request.RemoteAddr = "192.168.1.1:12345"
	ip := GetSecureClientIP(request)
	if ip != "192.168.1.1" {
		t.Errorf("Expected IP 192.168.1.1, got %s", ip)
	}

	request = httptest.NewRequest("GET", "/test", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("X-Forwarded-For", "203.0.113.1")
	ip = GetSecureClientIP(request)
	if ip != "127.0.0.1" {
		t.Errorf("Expected IP 127.0.0.1 (proxy header ignored), got %s", ip)
	}

	TrustProxyHeaders = true
	request = httptest.NewRequest("GET", "/test", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("X-Forwarded-For", "203.0.113.1")
	ip = GetSecureClientIP(request)
	if ip != "203.0.113.1" {
		t.Errorf("Expected IP 203.0.113.1, got %s", ip)
	}

	request = httptest.NewRequest("GET", "/test", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1")
	ip = GetSecureClientIP(request)
	if ip != "203.0.113.1" {
		t.Errorf("Expected IP 203.0.113.1 (leftmost), got %s", ip)
	}

	TrustProxyHeaders = originalTrust
}

func TestRateLimiterCleanup(t *testing.T) {
	clock := newMockClock()
	limiter := newRateLimiterWithClock(25*time.Millisecond, 2, clock)

	limiter.Allow("test-key")
	limiter.Allow("test-key")

	clock.Advance(50 * time.Millisecond)

	limiter.Cleanup()

	if !limiter.Allow("test-key") {
		t.Error("Request should be allowed after cleanup")
	}
}

func TestRateLimiterConcurrent(t *testing.T) {
	limiter := NewRateLimiter(1*time.Minute, 10)
	defer limiter.StopCleanup()
	key := "concurrent-test"

	results := make(chan bool, 20)
	for range 20 {
		go func() {
			allowed := limiter.Allow(key)
			results <- allowed
		}()
	}

	allowedCount := 0
	rejectedCount := 0
	for range 20 {
		if <-results {
			allowedCount++
		} else {
			rejectedCount++
		}
	}

	if allowedCount != 10 {
		t.Errorf("Expected 10 allowed requests, got %d", allowedCount)
	}
	if rejectedCount != 10 {
		t.Errorf("Expected 10 rejected requests, got %d", rejectedCount)
	}

	if limiter.Allow(key) {
		t.Error("Should be at limit after concurrent requests")
	}
}

func TestRateLimiterMaxKeys(t *testing.T) {

	limiter := NewRateLimiterWithConfig(1*time.Minute, 10, 5, 1*time.Minute, 2*time.Minute)
	defer limiter.StopCleanup()

	for i := range 5 {
		key := fmt.Sprintf("key-%d", i)
		if !limiter.Allow(key) {
			t.Errorf("Key %s should be allowed", key)
		}
	}

	if limiter.GetSize() != 5 {
		t.Errorf("Expected size 5, got %d", limiter.GetSize())
	}

	limiter.Allow("key-1")
	limiter.Allow("key-2")
	limiter.Allow("key-3")
	limiter.Allow("key-4")

	if !limiter.Allow("key-5") {
		t.Error("Key-5 should be allowed (after LRU eviction)")
	}

	if limiter.GetSize() != 5 {
		t.Errorf("Expected size 5 after eviction, got %d", limiter.GetSize())
	}

	if !limiter.Allow("key-0") {
		t.Error("Key-0 should be allowed (new entry after eviction)")
	}
}

func TestRateLimiterRegistry(t *testing.T) {
	defer StopAllRateLimiters()

	middleware1 := RateLimitMiddleware(1*time.Minute, 100)
	middleware2 := RateLimitMiddleware(1*time.Minute, 100)
	middleware3 := RateLimitMiddleware(1*time.Minute, 50)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server1 := middleware1(handler)
	server2 := middleware2(handler)
	server3 := middleware3(handler)

	request := httptest.NewRequest("GET", "/test", nil)
	request.RemoteAddr = "127.0.0.1:12345"

	for i := range 50 {
		recorder := httptest.NewRecorder()
		server1.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Errorf("Request %d to server1 should be allowed", i+1)
		}
	}

	for i := range 50 {
		recorder2 := httptest.NewRecorder()
		server2.ServeHTTP(recorder2, request)
		if recorder2.Code != http.StatusOK {
			t.Errorf("Request %d to server2 should be allowed (shared limiter)", i+1)
		}
	}

	recorder := httptest.NewRecorder()
	server1.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusTooManyRequests {
		t.Error("101st request to server1 should be rejected (shared limiter reached limit)")
	}

	recorder2 := httptest.NewRecorder()
	server2.ServeHTTP(recorder2, request)
	if recorder2.Code != http.StatusTooManyRequests {
		t.Error("101st request to server2 should be rejected (shared limiter reached limit)")
	}

	for i := range 50 {
		recorder3 := httptest.NewRecorder()
		server3.ServeHTTP(recorder3, request)
		if recorder3.Code != http.StatusOK {
			t.Errorf("Request %d to server3 should be allowed", i+1)
		}
	}

	recorder3 := httptest.NewRecorder()
	server3.ServeHTTP(recorder3, request)
	if recorder3.Code != http.StatusTooManyRequests {
		t.Error("51st request to server3 should be rejected (different limit)")
	}
}

func TestStopAllRateLimiters(t *testing.T) {
	middleware1 := RateLimitMiddleware(1*time.Minute, 100)
	middleware2 := RateLimitMiddleware(1*time.Minute, 50)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	_ = middleware1(handler)
	_ = middleware2(handler)

	StopAllRateLimiters()

	middleware3 := RateLimitMiddleware(1*time.Minute, 100)
	server3 := middleware3(handler)

	request := httptest.NewRequest("GET", "/test", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	server3.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Error("New limiter after StopAllRateLimiters should work")
	}

	StopAllRateLimiters()
}

func TestRateLimiterCleanupStaleEntries(t *testing.T) {
	clock := newMockClock()

	limiter := &RateLimiter{
		entries:         make(map[string]*keyEntry),
		window:          50 * time.Millisecond,
		limit:           10,
		maxKeys:         100,
		cleanupInterval: 100 * time.Millisecond,
		maxStaleAge:     150 * time.Millisecond,
		stopCleanup:     make(chan struct{}),
		clock:           clock,
	}

	limiter.Allow("key1")
	if limiter.GetSize() != 1 {
		t.Errorf("Expected size 1, got %d", limiter.GetSize())
	}

	clock.Advance(100 * time.Millisecond)

	limiter.Cleanup()

	clock.Advance(100 * time.Millisecond)
	limiter.Cleanup()

	size := limiter.GetSize()
	if size != 0 {
		t.Errorf("Expected size 0 after cleanup, got %d", size)
	}
}

func TestRateLimiterGetSize(t *testing.T) {
	limiter := NewRateLimiter(1*time.Minute, 10)
	defer limiter.StopCleanup()

	if limiter.GetSize() != 0 {
		t.Errorf("Expected initial size 0, got %d", limiter.GetSize())
	}

	limiter.Allow("key1")
	if limiter.GetSize() != 1 {
		t.Errorf("Expected size 1, got %d", limiter.GetSize())
	}

	limiter.Allow("key2")
	if limiter.GetSize() != 2 {
		t.Errorf("Expected size 2, got %d", limiter.GetSize())
	}

	limiter.Allow("key1")
	if limiter.GetSize() != 2 {
		t.Errorf("Expected size 2, got %d", limiter.GetSize())
	}
}

func TestRateLimiterLRUEviction(t *testing.T) {
	clock := newMockClock()

	limiter := &RateLimiter{
		entries:         make(map[string]*keyEntry),
		window:          1 * time.Minute,
		limit:           10,
		maxKeys:         3,
		cleanupInterval: 1 * time.Minute,
		maxStaleAge:     2 * time.Minute,
		stopCleanup:     make(chan struct{}),
		clock:           clock,
	}

	limiter.Allow("key1")
	limiter.Allow("key2")
	limiter.Allow("key3")

	if limiter.GetSize() != 3 {
		t.Errorf("Expected size 3, got %d", limiter.GetSize())
	}

	clock.Advance(10 * time.Millisecond)
	limiter.Allow("key1")
	clock.Advance(10 * time.Millisecond)
	limiter.Allow("key2")

	limiter.Allow("key4")

	if limiter.GetSize() != 3 {
		t.Errorf("Expected size 3 after eviction, got %d", limiter.GetSize())
	}

	if !limiter.Allow("key4") {
		t.Error("Key4 should exist and be allowed")
	}
}
