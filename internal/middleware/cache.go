package middleware

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
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

func applyCacheMaxSize(cache Cache, max int) {
	if im, ok := cache.(*InMemoryCache); ok {
		im.SetMaxEntries(max)
	}
}

func registerIndexedCacheKey(cache Cache, cacheKey string, path string, cacheablePrefixes []string) {
	if im, ok := cache.(*InMemoryCache); ok {
		im.RegisterKeyForPath(cacheKey, path, cacheablePrefixes)
	}
}

func invalidateCacheForMutation(cache Cache, mutationPath string, cacheablePrefixes []string) {
	if im, ok := cache.(*InMemoryCache); ok {
		im.InvalidateForMutationPath(mutationPath, cacheablePrefixes)
		return
	}
	_ = cache.Clear()
}

type InMemoryCache struct {
	mu sync.Mutex

	data    map[string]*CacheEntry
	maxSize int

	ll    *list.List
	lruEl map[string]*list.Element

	prefixKeys  map[string]map[string]struct{}
	keyPrefixes map[string]map[string]struct{}
}

func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		data:        make(map[string]*CacheEntry),
		maxSize:     1000,
		ll:          list.New(),
		lruEl:       make(map[string]*list.Element),
		prefixKeys:  make(map[string]map[string]struct{}),
		keyPrefixes: make(map[string]map[string]struct{}),
	}
}

func (cache *InMemoryCache) SetMaxEntries(n int) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.maxSize = n
	for n > 0 && len(cache.data) > n {
		cache.evictOldestLocked()
	}
}

func (cache *InMemoryCache) RegisterKeyForPath(cacheKey string, path string, cacheablePrefixes []string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	for _, p := range matchingCachePrefixes(path, cacheablePrefixes) {
		if cache.prefixKeys[p] == nil {
			cache.prefixKeys[p] = make(map[string]struct{})
		}
		cache.prefixKeys[p][cacheKey] = struct{}{}

		if cache.keyPrefixes[cacheKey] == nil {
			cache.keyPrefixes[cacheKey] = make(map[string]struct{})
		}
		cache.keyPrefixes[cacheKey][p] = struct{}{}
	}
}

func (cache *InMemoryCache) InvalidateForMutationPath(mutationPath string, cacheablePrefixes []string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	seen := make(map[string]struct{})
	var stale []string
	for _, prefix := range cacheablePrefixes {
		if !strings.HasPrefix(mutationPath, prefix) {
			continue
		}
		for key := range cache.prefixKeys[prefix] {
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			stale = append(stale, key)
		}
	}

	for _, key := range stale {
		cache.removeKeyLocked(key)
	}
}

func matchingCachePrefixes(path string, cacheablePrefixes []string) []string {
	var out []string
	for _, p := range cacheablePrefixes {
		if strings.HasPrefix(path, p) {
			out = append(out, p)
		}
	}
	return out
}

func (cache *InMemoryCache) Get(key string) (*CacheEntry, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	entry, exists := cache.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found")
	}

	if time.Since(entry.Timestamp) > entry.TTL {
		cache.removeKeyLocked(key)
		return nil, fmt.Errorf("entry expired")
	}

	if el, ok := cache.lruEl[key]; ok && cache.ll != nil {
		cache.ll.MoveToFront(el)
	}

	return entry, nil
}

func (cache *InMemoryCache) Set(key string, entry *CacheEntry) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if _, exists := cache.data[key]; exists {
		cache.data[key] = entry
		if el, ok := cache.lruEl[key]; ok && cache.ll != nil {
			cache.ll.MoveToFront(el)
		}
		return nil
	}

	cache.data[key] = entry
	if cache.ll != nil {
		el := cache.ll.PushFront(key)
		cache.lruEl[key] = el
	}

	for cache.maxSize > 0 && len(cache.data) > cache.maxSize {
		cache.evictOldestLocked()
	}
	return nil
}

func (cache *InMemoryCache) evictOldestLocked() {
	if cache.ll == nil || cache.ll.Len() == 0 {
		return
	}
	el := cache.ll.Back()
	if el == nil {
		return
	}
	key, _ := el.Value.(string)
	cache.removeKeyLocked(key)
}

func (cache *InMemoryCache) removeKeyLocked(key string) {
	delete(cache.data, key)

	if el, ok := cache.lruEl[key]; ok && cache.ll != nil {
		cache.ll.Remove(el)
	}
	delete(cache.lruEl, key)

	for p := range cache.keyPrefixes[key] {
		if m, ok := cache.prefixKeys[p]; ok {
			delete(m, key)
			if len(m) == 0 {
				delete(cache.prefixKeys, p)
			}
		}
	}
	delete(cache.keyPrefixes, key)
}

func (cache *InMemoryCache) Delete(key string) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if _, ok := cache.data[key]; !ok {
		return fmt.Errorf("key not found")
	}
	cache.removeKeyLocked(key)
	return nil
}

func (cache *InMemoryCache) Clear() error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.data = make(map[string]*CacheEntry)
	cache.prefixKeys = make(map[string]map[string]struct{})
	cache.keyPrefixes = make(map[string]map[string]struct{})
	cache.lruEl = make(map[string]*list.Element)
	cache.ll = list.New()
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
	applyCacheMaxSize(cache, config.MaxSize)

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
					if len(values) > 0 {
						w.Header().Set(key, values[0])
						for i := 1; i < len(values); i++ {
							w.Header().Add(key, values[i])
						}
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

				path := r.URL.Path
				prefixes := config.CacheablePaths

				go func() {
					if err := cache.Set(cacheKey, entry); err != nil {
						log.Printf("middleware cache Set: %v", err)
						return
					}
					registerIndexedCacheKey(cache, cacheKey, path, prefixes)
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
	for key, values := range rc.headers {
		for _, value := range values {
			rc.ResponseWriter.Header().Add(key, value)
		}
	}
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

	if userID := GetUserIDFromContext(r.Context()); userID != nil {
		key += fmt.Sprintf(":user:%d", *userID)
	}

	sum := sha256.Sum256([]byte(key))
	return "cache:" + hex.EncodeToString(sum[:])
}

func CacheInvalidationMiddleware(cache Cache, cacheablePrefixes []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
				mPath := r.URL.Path
				go func() {
					invalidateCacheForMutation(cache, mPath, cacheablePrefixes)
				}()
			}

			next.ServeHTTP(w, r)
		})
	}
}
