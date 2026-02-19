package middleware

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestInMemoryCache(t *testing.T) {
	cache := NewInMemoryCache()

	t.Run("Set and Get", func(t *testing.T) {
		entry := &CacheEntry{
			Data:      []byte("test data"),
			Headers:   make(http.Header),
			Timestamp: time.Now(),
			TTL:       5 * time.Minute,
		}

		err := cache.Set("test-key", entry)
		if err != nil {
			t.Fatalf("Failed to set cache entry: %v", err)
		}

		retrieved, err := cache.Get("test-key")
		if err != nil {
			t.Fatalf("Failed to get cache entry: %v", err)
		}

		if string(retrieved.Data) != "test data" {
			t.Errorf("Expected 'test data', got '%s'", string(retrieved.Data))
		}
	})

	t.Run("Get non-existent key", func(t *testing.T) {
		_, err := cache.Get("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent key")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		entry := &CacheEntry{
			Data:      []byte("delete test"),
			Headers:   make(http.Header),
			Timestamp: time.Now(),
			TTL:       5 * time.Minute,
		}

		cache.Set("delete-key", entry)
		err := cache.Delete("delete-key")
		if err != nil {
			t.Fatalf("Failed to delete cache entry: %v", err)
		}

		_, err = cache.Get("delete-key")
		if err == nil {
			t.Error("Expected error after deletion")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		entry := &CacheEntry{
			Data:      []byte("clear test"),
			Headers:   make(http.Header),
			Timestamp: time.Now(),
			TTL:       5 * time.Minute,
		}

		cache.Set("clear-key", entry)
		err := cache.Clear()
		if err != nil {
			t.Fatalf("Failed to clear cache: %v", err)
		}

		_, err = cache.Get("clear-key")
		if err == nil {
			t.Error("Expected error after clear")
		}
	})

	t.Run("Expired entry", func(t *testing.T) {
		entry := &CacheEntry{
			Data:      []byte("expired data"),
			Headers:   make(http.Header),
			Timestamp: time.Now().Add(-10 * time.Minute),
			TTL:       5 * time.Minute,
		}

		cache.Set("expired-key", entry)
		_, err := cache.Get("expired-key")
		if err == nil {
			t.Error("Expected error for expired entry")
		}
	})
}

func TestCacheMiddleware(t *testing.T) {
	cache := NewInMemoryCache()
	config := &CacheConfig{
		TTL:     5 * time.Minute,
		MaxSize: 1000,
	}
	middleware := CacheMiddleware(cache, config)

	t.Run("Cache miss", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/api/posts", nil)
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Body.String() != "test response" {
			t.Errorf("Expected 'test response', got '%s'", recorder.Body.String())
		}
	})

	t.Run("Cache hit", func(t *testing.T) {
		testCache := NewInMemoryCache()
		testConfig := &CacheConfig{
			TTL:            5 * time.Minute,
			MaxSize:        1000,
			CacheablePaths: []string{"/api/posts"},
		}
		testMiddleware := CacheMiddleware(testCache, testConfig)

		request := httptest.NewRequest("GET", "/api/posts", nil)
		recorder := httptest.NewRecorder()

		callCount := 0
		handler := testMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("cached response"))
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}
		if callCount != 1 {
			t.Errorf("Expected handler to be called once, got %d", callCount)
		}

		cacheKey := generateCacheKey(request)
		entry := &CacheEntry{
			Data:      []byte("cached response"),
			Headers:   make(http.Header),
			Timestamp: time.Now(),
			TTL:       5 * time.Minute,
		}
		testCache.Set(cacheKey, entry)

		request2 := httptest.NewRequest("GET", "/api/posts", nil)
		recorder2 := httptest.NewRecorder()

		handler.ServeHTTP(recorder2, request2)

		if recorder2.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder2.Code)
		}
		if callCount != 1 {
			t.Errorf("Expected handler to be called once total, got %d", callCount)
		}
		if recorder2.Body.String() != "cached response" {
			t.Errorf("Expected 'cached response', got '%s'", recorder2.Body.String())
		}
		if recorder2.Header().Get("X-Cache") != "HIT" {
			t.Error("Expected X-Cache header to be HIT")
		}
	})

	t.Run("POST request not cached", func(t *testing.T) {
		request := httptest.NewRequest("POST", "/test", nil)
		recorder := httptest.NewRecorder()

		callCount := 0
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("post response"))
		}))

		handler.ServeHTTP(recorder, request)
		if callCount != 1 {
			t.Errorf("Expected handler to be called once, got %d", callCount)
		}

		recorder2 := httptest.NewRecorder()
		handler.ServeHTTP(recorder2, request)
		if callCount != 2 {
			t.Errorf("Expected handler to be called twice, got %d", callCount)
		}
	})

	t.Run("Personalized endpoints not cached by default", func(t *testing.T) {

		testCache := NewInMemoryCache()
		testConfig := DefaultCacheConfig()
		testMiddleware := CacheMiddleware(testCache, testConfig)

		personalizedPaths := []string{
			"/api/posts",
			"/api/posts/search",
		}

		for _, path := range personalizedPaths {
			request := httptest.NewRequest("GET", path, nil)
			recorder := httptest.NewRecorder()

			callCount := 0
			handler := testMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("response"))
			}))

			handler.ServeHTTP(recorder, request)
			if callCount != 1 {
				t.Errorf("Expected handler to be called once for %s, got %d", path, callCount)
			}
			if recorder.Header().Get("X-Cache") == "HIT" {
				t.Errorf("Expected %s not to be cached, but got cache HIT", path)
			}

			recorder2 := httptest.NewRecorder()
			handler.ServeHTTP(recorder2, request)
			if callCount != 2 {
				t.Errorf("Expected handler to be called twice for %s (not cached), got %d", path, callCount)
			}
			if recorder2.Header().Get("X-Cache") == "HIT" {
				t.Errorf("Expected %s not to be cached on second request, but got cache HIT", path)
			}
		}
	})
}

func TestCacheKeyGeneration(t *testing.T) {
	tests := []struct {
		method   string
		path     string
		query    string
		expected string
	}{
		{"GET", "/test", "", "cache:e2b43a77e8b6707afcc1571382ca7c73"},
		{"GET", "/test", "param=value", "cache:067b4b550d6cee93dfb106d6912ef91b"},
		{"POST", "/test", "", "cache:fb3126bb69b4d21769b5fa4d78318b0e"},
		{"PUT", "/users/123", "", "cache:40b0b7a2306bfd4998d6219c1ef29783"},
	}

	for _, tt := range tests {
		t.Run(tt.method+tt.path+tt.query, func(t *testing.T) {
			url := tt.path
			if tt.query != "" {
				url += "?" + tt.query
			}
			request := httptest.NewRequest(tt.method, url, nil)
			key := generateCacheKey(request)
			if key != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, key)
			}
		})
	}
}

func TestInMemoryCacheConcurrent(t *testing.T) {
	cache := NewInMemoryCache()
	numGoroutines := 100
	numOps := 100

	t.Run("Concurrent writes", func(t *testing.T) {
		done := make(chan bool, numGoroutines)
		for i := range numGoroutines {
			go func(id int) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Goroutine %d panicked: %v", id, r)
					}
				}()
				for j := range numOps {
					entry := &CacheEntry{
						Data:      fmt.Appendf(nil, "data-%d-%d", id, j),
						Headers:   make(http.Header),
						Timestamp: time.Now(),
						TTL:       5 * time.Minute,
					}
					key := fmt.Sprintf("key-%d-%d", id, j)
					if err := cache.Set(key, entry); err != nil {
						t.Errorf("Failed to set cache entry: %v", err)
					}
				}
				done <- true
			}(i)
		}

		for range numGoroutines {
			<-done
		}
	})

	t.Run("Concurrent reads and writes", func(t *testing.T) {

		for i := range 10 {
			entry := &CacheEntry{
				Data:      fmt.Appendf(nil, "data-%d", i),
				Headers:   make(http.Header),
				Timestamp: time.Now(),
				TTL:       5 * time.Minute,
			}
			cache.Set(fmt.Sprintf("key-%d", i), entry)
		}

		done := make(chan bool, numGoroutines*2)

		for i := range numGoroutines {
			go func(id int) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Writer goroutine %d panicked: %v", id, r)
					}
				}()
				for j := range numOps {
					entry := &CacheEntry{
						Data:      fmt.Appendf(nil, "write-%d-%d", id, j),
						Headers:   make(http.Header),
						Timestamp: time.Now(),
						TTL:       5 * time.Minute,
					}
					key := fmt.Sprintf("write-key-%d-%d", id, j)
					cache.Set(key, entry)
				}
				done <- true
			}(i)
		}

		for i := range numGoroutines {
			go func(id int) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Reader goroutine %d panicked: %v", id, r)
					}
				}()
				for j := range numOps {
					key := fmt.Sprintf("key-%d", j%10)
					cache.Get(key)
				}
				done <- true
			}(i)
		}

		for i := 0; i < numGoroutines*2; i++ {
			<-done
		}
	})

	t.Run("Concurrent deletes", func(t *testing.T) {

		for i := range numGoroutines {
			entry := &CacheEntry{
				Data:      fmt.Appendf(nil, "data-%d", i),
				Headers:   make(http.Header),
				Timestamp: time.Now(),
				TTL:       5 * time.Minute,
			}
			cache.Set(fmt.Sprintf("del-key-%d", i), entry)
		}

		done := make(chan bool, numGoroutines)
		for i := range numGoroutines {
			go func(id int) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Delete goroutine %d panicked: %v", id, r)
					}
				}()
				cache.Delete(fmt.Sprintf("del-key-%d", id))
				done <- true
			}(i)
		}

		for range numGoroutines {
			<-done
		}
	})
}

func TestCacheMiddlewareTTLExpiration(t *testing.T) {

	testCache := NewInMemoryCache()
	testConfig := &CacheConfig{
		TTL:            100 * time.Millisecond,
		MaxSize:        1000,
		CacheablePaths: []string{"/test"},
	}
	testMiddleware := CacheMiddleware(testCache, testConfig)

	callCount := 0
	handler := testMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	}))

	request := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
	if callCount != 1 {
		t.Errorf("Expected handler to be called once, got %d", callCount)
	}
	if recorder.Header().Get("X-Cache") != "" {
		t.Error("First request should not have X-Cache header")
	}

	time.Sleep(50 * time.Millisecond)

	recorder2 := httptest.NewRecorder()
	handler.ServeHTTP(recorder2, request)

	if recorder2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder2.Code)
	}
	if callCount != 1 {
		t.Errorf("Expected handler to still be called once (cached), got %d", callCount)
	}
	if recorder2.Header().Get("X-Cache") != "HIT" {
		t.Error("Second request should have X-Cache: HIT header")
	}
	if recorder2.Body.String() != "response" {
		t.Errorf("Expected 'response', got '%s'", recorder2.Body.String())
	}

	time.Sleep(150 * time.Millisecond)

	recorder3 := httptest.NewRecorder()
	handler.ServeHTTP(recorder3, request)

	if recorder3.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder3.Code)
	}
	if callCount != 2 {
		t.Errorf("Expected handler to be called twice (after expiry), got %d", callCount)
	}
	if recorder3.Header().Get("X-Cache") != "" {
		t.Error("Request after expiry should not have X-Cache header")
	}

	time.Sleep(50 * time.Millisecond)

	recorder4 := httptest.NewRecorder()
	handler.ServeHTTP(recorder4, request)

	if recorder4.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder4.Code)
	}
	if callCount != 2 {
		t.Errorf("Expected handler to still be called twice (cached again), got %d", callCount)
	}
	if recorder4.Header().Get("X-Cache") != "HIT" {
		t.Error("Fourth request should have X-Cache: HIT header")
	}
}

func TestCacheMiddlewareRequestResponseSerialization(t *testing.T) {

	testCache := NewInMemoryCache()
	testConfig := &CacheConfig{
		TTL:            5 * time.Minute,
		MaxSize:        1000,
		CacheablePaths: []string{"/api/data"},
	}
	testMiddleware := CacheMiddleware(testCache, testConfig)

	callCount := 0
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	handler := testMiddleware(testHandler)

	request := httptest.NewRequest("GET", "/api/data?param=value", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
	if callCount != 1 {
		t.Errorf("Expected handler to be called once, got %d", callCount)
	}
	if recorder.Body.String() != `{"status":"ok"}` {
		t.Errorf("Expected JSON response, got %s", recorder.Body.String())
	}

	time.Sleep(50 * time.Millisecond)

	request2 := httptest.NewRequest("GET", "/api/data?param=value", nil)
	recorder2 := httptest.NewRecorder()
	handler.ServeHTTP(recorder2, request2)

	if recorder2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder2.Code)
	}
	if callCount != 1 {
		t.Errorf("Expected handler to still be called once (cached), got %d", callCount)
	}
	if recorder2.Header().Get("X-Cache") != "HIT" {
		t.Error("Expected X-Cache: HIT header")
	}

	if recorder2.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type header from cache, got %q", recorder2.Header().Get("Content-Type"))
	}
	if recorder2.Header().Get("X-Custom-Header") != "test-value" {
		t.Errorf("Expected X-Custom-Header from cache, got %q", recorder2.Header().Get("X-Custom-Header"))
	}
	if recorder2.Body.String() != `{"status":"ok"}` {
		t.Errorf("Expected cached JSON response, got %s", recorder2.Body.String())
	}

	request3 := httptest.NewRequest("GET", "/api/data?param=different", nil)
	recorder3 := httptest.NewRecorder()
	handler.ServeHTTP(recorder3, request3)

	if recorder3.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder3.Code)
	}
	if callCount != 2 {
		t.Errorf("Expected handler to be called twice (different query params), got %d", callCount)
	}
	if recorder3.Header().Get("X-Cache") != "" {
		t.Error("Request with different params should not have X-Cache header")
	}
}

func TestCacheMiddlewarePreservesSecurityHeaders(t *testing.T) {
	testCache := NewInMemoryCache()
	testConfig := &CacheConfig{
		TTL:            5 * time.Minute,
		MaxSize:        1000,
		CacheablePaths: []string{"/test"},
	}

	securityMiddleware := SecurityHeadersMiddleware()
	cacheMiddleware := CacheMiddleware(testCache, testConfig)

	callCount := 0
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	handler := securityMiddleware(cacheMiddleware(testHandler))

	request := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
	if callCount != 1 {
		t.Errorf("Expected handler to be called once, got %d", callCount)
	}

	securityHeaders := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Referrer-Policy",
		"Content-Security-Policy",
		"Permissions-Policy",
	}

	for _, headerName := range securityHeaders {
		if recorder.Header().Get(headerName) == "" {
			t.Errorf("Expected security header %s to be present on first request", headerName)
		}
	}

	time.Sleep(50 * time.Millisecond)

	request2 := httptest.NewRequest("GET", "/test", nil)
	recorder2 := httptest.NewRecorder()
	handler.ServeHTTP(recorder2, request2)

	if recorder2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder2.Code)
	}
	if callCount != 1 {
		t.Errorf("Expected handler to still be called once (cached), got %d", callCount)
	}
	if recorder2.Header().Get("X-Cache") != "HIT" {
		t.Error("Expected X-Cache: HIT header")
	}

	for _, headerName := range securityHeaders {
		if recorder2.Header().Get(headerName) == "" {
			t.Errorf("Expected security header %s to be present on cache hit", headerName)
		}
		if headerName == "Content-Security-Policy" {
			csp1 := recorder.Header().Get(headerName)
			csp2 := recorder2.Header().Get(headerName)
			if !strings.Contains(csp1, "script-src") || !strings.Contains(csp2, "script-src") {
				t.Errorf("Expected CSP to contain script-src directive on both requests")
			}
		} else {
			if recorder.Header().Get(headerName) != recorder2.Header().Get(headerName) {
				t.Errorf("Expected security header %s to match between first request and cache hit", headerName)
			}
		}
	}
}

func TestCacheMiddlewarePreservesHSTSHeader(t *testing.T) {
	testCache := NewInMemoryCache()
	testConfig := &CacheConfig{
		TTL:            5 * time.Minute,
		MaxSize:        1000,
		CacheablePaths: []string{"/test"},
	}

	hstsMiddleware := HSTSMiddleware()
	cacheMiddleware := CacheMiddleware(testCache, testConfig)

	callCount := 0
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	handler := hstsMiddleware(cacheMiddleware(testHandler))

	request := httptest.NewRequest("GET", "/test", nil)
	request.TLS = &tls.ConnectionState{}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
	if callCount != 1 {
		t.Errorf("Expected handler to be called once, got %d", callCount)
	}

	hstsValue := recorder.Header().Get("Strict-Transport-Security")
	if hstsValue == "" {
		t.Error("Expected Strict-Transport-Security header to be present on first request")
	}

	time.Sleep(50 * time.Millisecond)

	request2 := httptest.NewRequest("GET", "/test", nil)
	request2.TLS = &tls.ConnectionState{}
	recorder2 := httptest.NewRecorder()
	handler.ServeHTTP(recorder2, request2)

	if recorder2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder2.Code)
	}
	if callCount != 1 {
		t.Errorf("Expected handler to still be called once (cached), got %d", callCount)
	}
	if recorder2.Header().Get("X-Cache") != "HIT" {
		t.Error("Expected X-Cache: HIT header")
	}

	hstsValue2 := recorder2.Header().Get("Strict-Transport-Security")
	if hstsValue2 == "" {
		t.Error("Expected Strict-Transport-Security header to be present on cache hit")
	}
	if hstsValue != hstsValue2 {
		t.Error("Expected Strict-Transport-Security header to match between first request and cache hit")
	}
}

func TestCacheInvalidationMiddleware(t *testing.T) {
	cache := NewInMemoryCache()

	entries := []struct {
		key   string
		entry *CacheEntry
	}{
		{"cache:abc123", &CacheEntry{Data: []byte("data1"), Headers: make(http.Header), Timestamp: time.Now(), TTL: 5 * time.Minute}},
		{"cache:def456", &CacheEntry{Data: []byte("data2"), Headers: make(http.Header), Timestamp: time.Now(), TTL: 5 * time.Minute}},
		{"cache:ghi789", &CacheEntry{Data: []byte("data3"), Headers: make(http.Header), Timestamp: time.Now(), TTL: 5 * time.Minute}},
	}

	for _, e := range entries {
		if err := cache.Set(e.key, e.entry); err != nil {
			t.Fatalf("Failed to set cache entry: %v", err)
		}
	}

	for _, e := range entries {
		if _, err := cache.Get(e.key); err != nil {
			t.Fatalf("Expected entry %s to exist, got error: %v", e.key, err)
		}
	}

	middleware := CacheInvalidationMiddleware(cache)

	t.Run("POST clears cache", func(t *testing.T) {
		request := httptest.NewRequest("POST", "/api/posts", nil)
		recorder := httptest.NewRecorder()

		middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(recorder, request)

		time.Sleep(100 * time.Millisecond)

		for _, e := range entries {
			if _, err := cache.Get(e.key); err == nil {
				t.Errorf("Expected entry %s to be cleared, but it still exists", e.key)
			}
		}
	})

	for _, e := range entries {
		if err := cache.Set(e.key, e.entry); err != nil {
			t.Fatalf("Failed to repopulate cache: %v", err)
		}
	}

	t.Run("PUT clears cache", func(t *testing.T) {
		request := httptest.NewRequest("PUT", "/api/posts/1", nil)
		recorder := httptest.NewRecorder()

		middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(recorder, request)

		time.Sleep(100 * time.Millisecond)

		for _, e := range entries {
			if _, err := cache.Get(e.key); err == nil {
				t.Errorf("Expected entry %s to be cleared, but it still exists", e.key)
			}
		}
	})

	for _, e := range entries {
		if err := cache.Set(e.key, e.entry); err != nil {
			t.Fatalf("Failed to repopulate cache: %v", err)
		}
	}

	t.Run("DELETE clears cache", func(t *testing.T) {
		request := httptest.NewRequest("DELETE", "/api/posts/1", nil)
		recorder := httptest.NewRecorder()

		middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(recorder, request)

		time.Sleep(100 * time.Millisecond)

		for _, e := range entries {
			if _, err := cache.Get(e.key); err == nil {
				t.Errorf("Expected entry %s to be cleared, but it still exists", e.key)
			}
		}
	})

	t.Run("GET does not clear cache", func(t *testing.T) {

		for _, e := range entries {
			if err := cache.Set(e.key, e.entry); err != nil {
				t.Fatalf("Failed to repopulate cache: %v", err)
			}
		}

		request := httptest.NewRequest("GET", "/api/posts", nil)
		recorder := httptest.NewRecorder()

		middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(recorder, request)

		time.Sleep(100 * time.Millisecond)

		for _, e := range entries {
			if _, err := cache.Get(e.key); err != nil {
				t.Errorf("Expected entry %s to still exist, got error: %v", e.key, err)
			}
		}
	})
}
