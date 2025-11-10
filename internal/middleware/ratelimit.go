package middleware

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	DefaultMaxKeys = 10000

	DefaultCleanupInterval = 5 * time.Minute

	DefaultMaxStaleAge = 10 * time.Minute
)

var TrustProxyHeaders = false

func SetTrustProxyHeaders(value bool) {
	TrustProxyHeaders = value
}

type limiterKey struct {
	window time.Duration
	limit  int
}

var (
	limiterRegistry = make(map[limiterKey]*RateLimiter)
	registryMutex   sync.RWMutex
	registryCleanup []*RateLimiter
	cleanupMutex    sync.Mutex
)

func getOrCreateLimiter(window time.Duration, limit int) *RateLimiter {
	key := limiterKey{window: window, limit: limit}

	registryMutex.RLock()
	if limiter, exists := limiterRegistry[key]; exists {
		registryMutex.RUnlock()
		return limiter
	}
	registryMutex.RUnlock()

	registryMutex.Lock()
	defer registryMutex.Unlock()

	if limiter, exists := limiterRegistry[key]; exists {
		return limiter
	}

	limiter := NewRateLimiter(window, limit)
	limiterRegistry[key] = limiter

	cleanupMutex.Lock()
	registryCleanup = append(registryCleanup, limiter)
	cleanupMutex.Unlock()

	return limiter
}

func StopAllRateLimiters() {
	cleanupMutex.Lock()
	defer cleanupMutex.Unlock()

	for _, limiter := range registryCleanup {
		limiter.StopCleanup()
	}
	registryCleanup = nil

	registryMutex.Lock()
	limiterRegistry = make(map[limiterKey]*RateLimiter)
	registryMutex.Unlock()
}

type clock interface {
	Now() time.Time
}

type realClock struct{}

func (c *realClock) Now() time.Time {
	return time.Now()
}

type keyEntry struct {
	requests   []time.Time
	lastAccess time.Time
}

type RateLimiter struct {
	entries         map[string]*keyEntry
	mutex           sync.RWMutex
	window          time.Duration
	limit           int
	maxKeys         int
	cleanupInterval time.Duration
	maxStaleAge     time.Duration
	stopCleanup     chan struct{}
	cleanupOnce     sync.Once
	stopOnce        sync.Once
	clock           clock
}

func NewRateLimiter(window time.Duration, limit int) *RateLimiter {
	return NewRateLimiterWithConfig(window, limit, DefaultMaxKeys, DefaultCleanupInterval, DefaultMaxStaleAge)
}

func NewRateLimiterWithConfig(window time.Duration, limit int, maxKeys int, cleanupInterval time.Duration, maxStaleAge time.Duration) *RateLimiter {
	rl := &RateLimiter{
		entries:         make(map[string]*keyEntry),
		window:          window,
		limit:           limit,
		maxKeys:         maxKeys,
		cleanupInterval: cleanupInterval,
		maxStaleAge:     maxStaleAge,
		stopCleanup:     make(chan struct{}),
		clock:           &realClock{},
	}

	rl.StartCleanup()

	return rl
}

func newRateLimiterWithClock(window time.Duration, limit int, c clock) *RateLimiter {
	rl := &RateLimiter{
		entries:         make(map[string]*keyEntry),
		window:          window,
		limit:           limit,
		maxKeys:         DefaultMaxKeys,
		cleanupInterval: DefaultCleanupInterval,
		maxStaleAge:     DefaultMaxStaleAge,
		stopCleanup:     make(chan struct{}),
		clock:           c,
	}

	return rl
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := rl.clock.Now()
	cutoff := now.Add(-rl.window)

	var entry *keyEntry
	var exists bool

	if entry, exists = rl.entries[key]; exists {

		isStale := now.Sub(entry.lastAccess) > rl.maxStaleAge

		var validRequests []time.Time
		for _, reqTime := range entry.requests {
			if reqTime.After(cutoff) {
				validRequests = append(validRequests, reqTime)
			}
		}
		entry.requests = validRequests

		if len(entry.requests) == 0 && isStale {
			delete(rl.entries, key)
			exists = false
		} else {

			entry.lastAccess = now
		}
	}

	if !exists {

		if len(rl.entries) >= rl.maxKeys {

			rl.evictLRU()
		}

		entry = &keyEntry{
			requests:   []time.Time{now},
			lastAccess: now,
		}
		rl.entries[key] = entry
		return true
	}

	requestCount := len(entry.requests)
	if requestCount >= rl.limit {
		return false
	}

	entry.requests = append(entry.requests, now)
	entry.lastAccess = now
	return true
}

func (rl *RateLimiter) evictLRU() {
	if len(rl.entries) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, entry := range rl.entries {
		if first || entry.lastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.lastAccess
			first = false
		}
	}

	if oldestKey != "" {
		delete(rl.entries, oldestKey)
	}
}

func (rl *RateLimiter) GetRemainingTime(key string) time.Duration {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()

	if entry, exists := rl.entries[key]; exists && len(entry.requests) > 0 {
		oldestRequest := entry.requests[0]
		return rl.window - rl.clock.Now().Sub(oldestRequest)
	}
	return 0
}

func (rl *RateLimiter) Cleanup() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := rl.clock.Now()
	cutoff := now.Add(-rl.window)
	staleCutoff := now.Add(-rl.maxStaleAge)

	for key, entry := range rl.entries {

		var validRequests []time.Time
		for _, reqTime := range entry.requests {
			if reqTime.After(cutoff) {
				validRequests = append(validRequests, reqTime)
			}
		}
		entry.requests = validRequests

		if len(entry.requests) == 0 && entry.lastAccess.Before(staleCutoff) {
			delete(rl.entries, key)
		}
	}
}

func (rl *RateLimiter) StartCleanup() {
	rl.cleanupOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(rl.cleanupInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					rl.Cleanup()
				case <-rl.stopCleanup:
					return
				}
			}
		}()
	})
}

func (rl *RateLimiter) StopCleanup() {
	rl.stopOnce.Do(func() {
		close(rl.stopCleanup)
	})
}

func (rl *RateLimiter) GetSize() int {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()
	return len(rl.entries)
}

func GetSecureClientIP(r *http.Request) string {
	if TrustProxyHeaders {

		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {

			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				ip := strings.TrimSpace(ips[0])
				if net.ParseIP(ip) != nil {
					return ip
				}
			}
		}

		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			ip := strings.TrimSpace(xri)
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {

		if net.ParseIP(r.RemoteAddr) != nil {
			return r.RemoteAddr
		}

		return r.RemoteAddr
	}

	if net.ParseIP(ip) != nil {
		return ip
	}

	return ip
}

func GetKey(r *http.Request) string {
	ip := GetSecureClientIP(r)

	if userID := GetUserIDFromContext(r.Context()); userID != 0 {
		return fmt.Sprintf("user:%d:ip:%s", userID, ip)
	}

	return fmt.Sprintf("ip:%s", ip)
}

func RateLimitMiddleware(window time.Duration, limit int) func(http.Handler) http.Handler {
	limiter := getOrCreateLimiter(window, limit)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := GetKey(r)

			if !limiter.Allow(key) {
				remainingTime := limiter.GetRemainingTime(key)

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", remainingTime.Seconds()))
				w.WriteHeader(http.StatusTooManyRequests)

				response := map[string]any{
					"error":       "Rate limit exceeded",
					"message":     fmt.Sprintf("Too many requests. Please try again in %d seconds.", int(remainingTime.Seconds())),
					"retry_after": remainingTime.Seconds(),
				}

				jsonData, err := json.Marshal(response)
				if err != nil {
					jsonData = []byte(`{"error":"Rate limit exceeded"}`)
				}

				w.Write(jsonData)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func AuthRateLimitMiddleware() func(http.Handler) http.Handler {
	return RateLimitMiddleware(1*time.Minute, 5)
}

func AuthRateLimitMiddlewareWithLimit(limit int) func(http.Handler) http.Handler {
	return RateLimitMiddleware(1*time.Minute, limit)
}

func GeneralRateLimitMiddleware() func(http.Handler) http.Handler {
	return RateLimitMiddleware(1*time.Minute, 100)
}

func GeneralRateLimitMiddlewareWithLimit(limit int) func(http.Handler) http.Handler {
	return RateLimitMiddleware(1*time.Minute, limit)
}

func HealthRateLimitMiddleware(limit int) func(http.Handler) http.Handler {
	return RateLimitMiddleware(1*time.Minute, limit)
}

func MetricsRateLimitMiddleware(limit int) func(http.Handler) http.Handler {
	return RateLimitMiddleware(1*time.Minute, limit)
}
