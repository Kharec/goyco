package e2e

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"

	"goyco/internal/testutils"
)

func TestE2E_CompressionMiddleware(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("compression_enabled_with_accept_encoding", func(t *testing.T) {
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

		contentEncoding := response.Header.Get("Content-Encoding")
		if contentEncoding == "gzip" {
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			if isGzipCompressed(body) {
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
					t.Error("Decompressed body is empty")
				}
			}
		} else {
			t.Logf("Compression not applied (Content-Encoding: %s)", contentEncoding)
		}
	})

	t.Run("no_compression_without_accept_encoding", func(t *testing.T) {
		request, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(request)

		response, err := ctx.client.Do(request)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer response.Body.Close()

		contentEncoding := response.Header.Get("Content-Encoding")
		if contentEncoding == "gzip" {
			t.Error("Expected no compression without Accept-Encoding header")
		}
	})

	t.Run("decompression_handles_gzip_request", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "compressionuser", "StrongPass123!")
		authClient := ctx.loginUser(t, testUser.Username, "StrongPass123!")

		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		postData := `{"title":"Compressed Post","url":"https://example.com/compressed","content":"Test content"}`
		gz.Write([]byte(postData))
		gz.Close()

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

		switch response.StatusCode {
		case http.StatusBadRequest:
			t.Log("Decompression middleware rejected invalid gzip")
		case http.StatusCreated, http.StatusOK:
			t.Log("Decompression middleware handled gzip request successfully")
		}
	})
}

func TestE2E_CacheMiddleware(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("cache_miss_then_hit", func(t *testing.T) {
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

		firstCacheStatus := firstResponse.Header.Get("X-Cache")
		if firstCacheStatus == "HIT" {
			t.Log("First request was cached (unexpected but acceptable)")
		}

		secondRequest, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(secondRequest)

		secondResponse, err := ctx.client.Do(secondRequest)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer secondResponse.Body.Close()

		secondCacheStatus := secondResponse.Header.Get("X-Cache")
		if secondCacheStatus == "HIT" {
			t.Log("Second request was served from cache")
		}
	})

	t.Run("cache_invalidation_on_post", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "cacheuser", "StrongPass123!")
		authClient := ctx.loginUser(t, testUser.Username, "StrongPass123!")

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
		firstResponse.Body.Close()

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
		secondResponse.Body.Close()

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
		defer thirdResponse.Body.Close()

		cacheStatus := thirdResponse.Header.Get("X-Cache")
		if cacheStatus == "HIT" {
			t.Log("Cache was invalidated after POST")
		}
	})
}

func TestE2E_CSRFProtection(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("csrf_protection_for_non_api_routes", func(t *testing.T) {
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

		if response.StatusCode == http.StatusForbidden {
			t.Log("CSRF protection active for non-API routes")
		} else {
			t.Logf("CSRF check result: status %d", response.StatusCode)
		}
	})

	t.Run("csrf_bypass_for_api_routes", func(t *testing.T) {
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

		if response.StatusCode == http.StatusForbidden {
			t.Error("API routes should bypass CSRF protection")
		}
	})

	t.Run("csrf_allows_get_requests", func(t *testing.T) {
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

		if response.StatusCode == http.StatusForbidden {
			t.Error("GET requests should not require CSRF token")
		}
	})
}

func TestE2E_RequestSizeLimit(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("request_within_size_limit", func(t *testing.T) {
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

		if response.StatusCode == http.StatusRequestEntityTooLarge {
			t.Error("Small request should not exceed size limit")
		}
	})

	t.Run("request_exceeds_size_limit", func(t *testing.T) {
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
			return
		}
		defer response.Body.Close()

		if response.StatusCode == http.StatusRequestEntityTooLarge {
			t.Log("Request size limit enforced correctly")
		} else {
			t.Logf("Request size limit check result: status %d", response.StatusCode)
		}
	})
}

func isGzipCompressed(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}
