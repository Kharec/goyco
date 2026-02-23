package e2e

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"goyco/internal/testutils"
)

func TestE2E_CompressionMiddleware(t *testing.T) {
	ctx := setupTestContextWithMiddleware(t)

	t.Run("compresses_response_when_accept_encoding_is_gzip", func(t *testing.T) {
		request, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		request.Header.Set("Accept-Encoding", "gzip")
		testutils.WithStandardHeaders(request)

		response, err := ctx.client.Do(request)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer response.Body.Close()
		failIfRateLimited(t, response.StatusCode, "compression enabled GET /api/posts")

		contentEncoding := response.Header.Get("Content-Encoding")
		if contentEncoding != "gzip" {
			t.Fatalf("Expected gzip compression, got Content-Encoding=%q", contentEncoding)
		}

		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		if !isGzipCompressed(body) {
			t.Fatalf("Expected gzip-compressed body bytes")
		}

		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			t.Fatalf("Failed to create gzip reader: %v", err)
		}
		defer reader.Close()

		decompressed, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("Failed to decompress: %v", err)
		}

		if len(decompressed) == 0 {
			t.Fatal("Decompressed body is empty")
		}
	})

	t.Run("does_not_compress_without_accept_encoding", func(t *testing.T) {
		request, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(request)
		request.Header.Del("Accept-Encoding")

		response, err := ctx.client.Do(request)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer response.Body.Close()
		failIfRateLimited(t, response.StatusCode, "compression disabled GET /api/posts")

		contentEncoding := response.Header.Get("Content-Encoding")
		if contentEncoding == "gzip" {
			t.Fatal("Expected no compression without Accept-Encoding header")
		}
	})

	t.Run("accepts_valid_gzip_request_body", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "compressionuser", "StrongPass123!")
		authClient := ctx.loginUser(t, testUser.Username, "StrongPass123!")

		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		postData := `{"title":"Compressed Post","url":"https://example.com/compressed","content":"Test content"}`
		if _, err := gz.Write([]byte(postData)); err != nil {
			t.Fatalf("Failed to gzip request body: %v", err)
		}
		if err := gz.Close(); err != nil {
			t.Fatalf("Failed to finalize gzip body: %v", err)
		}

		request, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", &buf)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Content-Encoding", "gzip")
		testutils.WithStandardHeaders(request)
		request.Header.Set("Authorization", "Bearer "+authClient.Token)

		response, err := ctx.client.Do(request)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer response.Body.Close()
		failIfRateLimited(t, response.StatusCode, "decompression POST /api/posts")

		switch response.StatusCode {
		case http.StatusCreated, http.StatusOK:
			return
		default:
			body, _ := io.ReadAll(response.Body)
			t.Fatalf("Expected status %d or %d for valid gzip request, got %d. Body: %s", http.StatusCreated, http.StatusOK, response.StatusCode, string(body))
		}
	})
}

func TestE2E_CacheMiddleware(t *testing.T) {
	ctx := setupTestContextWithMiddleware(t)

	t.Run("returns_hit_after_repeated_get", func(t *testing.T) {
		firstRequest, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(firstRequest)

		firstResponse, err := ctx.client.Do(firstRequest)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		firstResponse.Body.Close()
		failIfRateLimited(t, firstResponse.StatusCode, "first cache GET /api/posts")

		firstCacheStatus := firstResponse.Header.Get("X-Cache")
		if firstCacheStatus == "HIT" {
			t.Fatalf("Expected first request to be a cache miss, got X-Cache=%q", firstCacheStatus)
		}

		for range 8 {
			secondRequest, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			testutils.WithStandardHeaders(secondRequest)

			secondResponse, err := ctx.client.Do(secondRequest)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			failIfRateLimited(t, secondResponse.StatusCode, "cache warmup GET /api/posts")
			secondCacheStatus := secondResponse.Header.Get("X-Cache")
			secondResponse.Body.Close()

			if secondCacheStatus == "HIT" {
				return
			}

			time.Sleep(25 * time.Millisecond)
		}

		t.Fatal("Expected a cache HIT on repeated requests, but none observed")
	})

	t.Run("invalidates_cached_get_after_post", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "cacheuser", "StrongPass123!")
		authClient := ctx.loginUser(t, testUser.Username, "StrongPass123!")

		for attempt := range 8 {
			firstRequest, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			testutils.WithStandardHeaders(firstRequest)
			firstRequest.Header.Set("Authorization", "Bearer "+authClient.Token)

			firstResponse, err := ctx.client.Do(firstRequest)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			failIfRateLimited(t, firstResponse.StatusCode, "cache priming GET /api/posts")
			cacheStatus := firstResponse.Header.Get("X-Cache")
			firstResponse.Body.Close()

			if cacheStatus == "HIT" {
				break
			}

			if attempt == 7 {
				t.Fatal("Failed to prime cache: repeated GET requests never produced X-Cache=HIT")
			}

			time.Sleep(25 * time.Millisecond)
		}

		postData := `{"title":"Cache Invalidation Test","url":"https://example.com/cache","content":"Test"}`
		secondRequest, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", strings.NewReader(postData))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		secondRequest.Header.Set("Content-Type", "application/json")
		testutils.WithStandardHeaders(secondRequest)
		secondRequest.Header.Set("Authorization", "Bearer "+authClient.Token)

		secondResponse, err := ctx.client.Do(secondRequest)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		failIfRateLimited(t, secondResponse.StatusCode, "cache invalidation POST /api/posts")
		if secondResponse.StatusCode != http.StatusCreated && secondResponse.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(secondResponse.Body)
			secondResponse.Body.Close()
			t.Fatalf("Expected post creation status %d or %d, got %d. Body: %s", http.StatusCreated, http.StatusOK, secondResponse.StatusCode, string(body))
		}
		secondResponse.Body.Close()

		for range 8 {
			thirdRequest, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			testutils.WithStandardHeaders(thirdRequest)
			thirdRequest.Header.Set("Authorization", "Bearer "+authClient.Token)

			thirdResponse, err := ctx.client.Do(thirdRequest)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			failIfRateLimited(t, thirdResponse.StatusCode, "post-invalidation GET /api/posts")
			cacheStatus := thirdResponse.Header.Get("X-Cache")
			thirdResponse.Body.Close()

			if cacheStatus != "HIT" {
				return
			}

			time.Sleep(25 * time.Millisecond)
		}

		t.Fatal("Expected cache to be invalidated after POST, but X-Cache stayed HIT")
	})
}

func TestE2E_CSRFProtection(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("non_api_post_without_csrf_is_forbidden_or_unmounted", func(t *testing.T) {
		request, err := http.NewRequest("POST", ctx.baseURL+"/auth/login", strings.NewReader(`{"username":"test","password":"test"}`))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		request.Header.Set("Content-Type", "application/json")
		testutils.WithStandardHeaders(request)

		response, err := ctx.client.Do(request)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer response.Body.Close()
		failIfRateLimited(t, response.StatusCode, "CSRF non-API POST /auth/login")

		if response.StatusCode != http.StatusForbidden && response.StatusCode != http.StatusNotFound {
			body, _ := io.ReadAll(response.Body)
			t.Fatalf("Expected status %d (CSRF protected) or %d (route unavailable in test setup), got %d. Body: %s", http.StatusForbidden, http.StatusNotFound, response.StatusCode, string(body))
		}
	})

	t.Run("api_post_without_csrf_is_not_forbidden", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "csrfuser", "StrongPass123!")
		authClient := ctx.loginUser(t, testUser.Username, "StrongPass123!")

		postData := `{"title":"CSRF Test","url":"https://example.com/csrf","content":"Test"}`
		request, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", strings.NewReader(postData))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		request.Header.Set("Content-Type", "application/json")
		testutils.WithStandardHeaders(request)
		request.Header.Set("Authorization", "Bearer "+authClient.Token)

		response, err := ctx.client.Do(request)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer response.Body.Close()
		failIfRateLimited(t, response.StatusCode, "CSRF bypass POST /api/posts")

		if response.StatusCode == http.StatusForbidden {
			body, _ := io.ReadAll(response.Body)
			t.Fatalf("API routes should bypass CSRF protection, got 403. Body: %s", string(body))
		}
	})

	t.Run("get_request_without_csrf_is_not_forbidden", func(t *testing.T) {
		request, err := http.NewRequest("GET", ctx.baseURL+"/auth/login", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(request)

		response, err := ctx.client.Do(request)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer response.Body.Close()
		failIfRateLimited(t, response.StatusCode, "CSRF GET /auth/login")

		if response.StatusCode == http.StatusForbidden {
			t.Fatal("GET requests should not require CSRF token")
		}
	})
}

func TestE2E_RequestSizeLimit(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("accepts_request_within_size_limit", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "sizelimituser", "StrongPass123!")
		authClient := ctx.loginUser(t, testUser.Username, "StrongPass123!")

		smallData := strings.Repeat("a", 100)
		postData := `{"title":"` + smallData + `","url":"https://example.com","content":"test"}`
		request, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", strings.NewReader(postData))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		request.Header.Set("Content-Type", "application/json")
		testutils.WithStandardHeaders(request)
		request.Header.Set("Authorization", "Bearer "+authClient.Token)

		response, err := ctx.client.Do(request)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer response.Body.Close()
		failIfRateLimited(t, response.StatusCode, "request within size limit POST /api/posts")

		if response.StatusCode == http.StatusRequestEntityTooLarge {
			body, _ := io.ReadAll(response.Body)
			t.Fatalf("Small request should not exceed size limit. Body: %s", string(body))
		}
	})

	t.Run("rejects_or_fails_oversized_request", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "sizelimituser2", "StrongPass123!")
		authClient := ctx.loginUser(t, testUser.Username, "StrongPass123!")

		largeData := strings.Repeat("a", 2*1024*1024)
		postData := `{"title":"test","url":"https://example.com","content":"` + largeData + `"}`
		request, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", strings.NewReader(postData))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		request.Header.Set("Content-Type", "application/json")
		testutils.WithStandardHeaders(request)
		request.Header.Set("Authorization", "Bearer "+authClient.Token)

		response, err := ctx.client.Do(request)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer response.Body.Close()
		failIfRateLimited(t, response.StatusCode, "request exceeds size limit POST /api/posts")

		if response.StatusCode != http.StatusRequestEntityTooLarge && response.StatusCode != http.StatusBadRequest {
			body, _ := io.ReadAll(response.Body)
			t.Fatalf("Expected status %d or %d for oversized request, got %d. Body: %s", http.StatusRequestEntityTooLarge, http.StatusBadRequest, response.StatusCode, string(body))
		}
	})
}

func isGzipCompressed(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}
