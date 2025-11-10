package middleware

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type CacheEntry struct {
	Data      []byte        `json:"data"`
	Headers   http.Header   `json:"headers"`
	Timestamp time.Time     `json:"timestamp"`
	TTL       time.Duration `json:"ttl"`
}

type Cache interface {
	Get(key string) (*CacheEntry, error)
	Set(key string, entry *CacheEntry) error
	Delete(key string) error
	Clear() error
}

type InMemoryCache struct {
	mu   sync.RWMutex
	data map[string]*CacheEntry
}

func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		data: make(map[string]*CacheEntry),
	}
}

func (cache *InMemoryCache) Get(key string) (*CacheEntry, error) {
	cache.mu.RLock()
	entry, exists := cache.data[key]
	cache.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("key not found")
	}

	if time.Since(entry.Timestamp) > entry.TTL {
		cache.mu.Lock()
		delete(cache.data, key)
		cache.mu.Unlock()
		return nil, fmt.Errorf("entry expired")
	}

	return entry, nil
}

func (cache *InMemoryCache) Set(key string, entry *CacheEntry) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.data[key] = entry
	return nil
}

func (cache *InMemoryCache) Delete(key string) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	delete(cache.data, key)
	return nil
}

func (cache *InMemoryCache) Clear() error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.data = make(map[string]*CacheEntry)
	return nil
}

type CacheConfig struct {
	TTL              time.Duration
	MaxSize          int
	CacheablePaths   []string
	CacheableMethods []string
}

func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		TTL:              5 * time.Minute,
		MaxSize:          1000,
		CacheablePaths:   []string{},
		CacheableMethods: []string{"GET"},
	}
}

func CacheMiddleware(cache Cache, config *CacheConfig) func(http.Handler) http.Handler {
	if config == nil {
		config = DefaultCacheConfig()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				next.ServeHTTP(w, r)
				return
			}

			if !isCacheablePath(r.URL.Path, config.CacheablePaths) {
				next.ServeHTTP(w, r)
				return
			}

			cacheKey := generateCacheKey(r)

			if entry, err := cache.Get(cacheKey); err == nil {
				for key, values := range entry.Headers {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}
				w.Header().Set("X-Cache", "HIT")
				w.WriteHeader(http.StatusOK)
				w.Write(entry.Data)
				return
			}

			capturer := &responseCapturer{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
				headers:        make(http.Header),
			}

			next.ServeHTTP(capturer, r)

			if capturer.statusCode == http.StatusOK {
				entry := &CacheEntry{
					Data:      capturer.body.Bytes(),
					Headers:   capturer.headers,
					Timestamp: time.Now(),
					TTL:       config.TTL,
				}

				go func() {
					cache.Set(cacheKey, entry)
				}()
			}
		})
	}
}

type responseCapturer struct {
	http.ResponseWriter
	body       *bytes.Buffer
	headers    http.Header
	statusCode int
}

func (rc *responseCapturer) WriteHeader(code int) {
	rc.statusCode = code
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *responseCapturer) Write(b []byte) (int, error) {
	rc.body.Write(b)
	return rc.ResponseWriter.Write(b)
}

func (rc *responseCapturer) Header() http.Header {
	return rc.headers
}

func isCacheablePath(path string, cacheablePaths []string) bool {
	for _, cacheablePath := range cacheablePaths {
		if strings.HasPrefix(path, cacheablePath) {
			return true
		}
	}
	return false
}

func generateCacheKey(r *http.Request) string {
	key := fmt.Sprintf("%s:%s", r.Method, r.URL.Path)
	if r.URL.RawQuery != "" {
		key += "?" + r.URL.RawQuery
	}

	if userID := GetUserIDFromContext(r.Context()); userID != 0 {
		key += fmt.Sprintf(":user:%d", userID)
	}

	hash := md5.Sum([]byte(key))
	return fmt.Sprintf("cache:%x", hash)
}

func CacheInvalidationMiddleware(cache Cache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
				go func() {
					cache.Clear()
				}()
			}

			next.ServeHTTP(w, r)
		})
	}
}
