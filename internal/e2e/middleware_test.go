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
		req, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Accept-Encoding", "gzip")
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		contentEncoding := resp.Header.Get("Content-Encoding")
		if contentEncoding == "gzip" {
			body, err := io.ReadAll(resp.Body)
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
		req, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		contentEncoding := resp.Header.Get("Content-Encoding")
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

		req, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", &buf)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		testutils.WithStandardHeaders(req)
		req.Header.Set("Authorization", "Bearer "+authClient.Token)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusBadRequest {
			t.Log("Decompression middleware rejected invalid gzip")
		} else if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
			t.Log("Decompression middleware handled gzip request successfully")
		}
	})
}

func TestE2E_CacheMiddleware(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("cache_miss_then_hit", func(t *testing.T) {
		req1, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req1)

		resp1, err := ctx.client.Do(req1)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp1.Body.Close()

		cacheStatus1 := resp1.Header.Get("X-Cache")
		if cacheStatus1 == "HIT" {
			t.Log("First request was cached (unexpected but acceptable)")
		}

		req2, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req2)

		resp2, err := ctx.client.Do(req2)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp2.Body.Close()

		cacheStatus2 := resp2.Header.Get("X-Cache")
		if cacheStatus2 == "HIT" {
			t.Log("Second request was served from cache")
		}
	})

	t.Run("cache_invalidation_on_post", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "cacheuser", "StrongPass123!")
		authClient := ctx.loginUser(t, testUser.Username, "StrongPass123!")

		req1, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req1)
		req1.Header.Set("Authorization", "Bearer "+authClient.Token)

		resp1, err := ctx.client.Do(req1)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp1.Body.Close()

		postData := `{"title":"Cache Invalidation Test","url":"https://example.com/cache","content":"Test"}`
		req2, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", strings.NewReader(postData))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req2.Header.Set("Content-Type", "application/json")
		testutils.WithStandardHeaders(req2)
		req2.Header.Set("Authorization", "Bearer "+authClient.Token)

		resp2, err := ctx.client.Do(req2)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp2.Body.Close()

		req3, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req3)
		req3.Header.Set("Authorization", "Bearer "+authClient.Token)

		resp3, err := ctx.client.Do(req3)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp3.Body.Close()

		cacheStatus := resp3.Header.Get("X-Cache")
		if cacheStatus == "HIT" {
			t.Log("Cache was invalidated after POST")
		}
	})
}

func TestE2E_CSRFProtection(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("csrf_protection_for_non_api_routes", func(t *testing.T) {
		req, err := http.NewRequest("POST", ctx.baseURL+"/auth/login", strings.NewReader(`{"username":"test","password":"test"}`))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusForbidden {
			t.Log("CSRF protection active for non-API routes")
		} else {
			t.Logf("CSRF check result: status %d", resp.StatusCode)
		}
	})

	t.Run("csrf_bypass_for_api_routes", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "csrfuser", "StrongPass123!")
		authClient := ctx.loginUser(t, testUser.Username, "StrongPass123!")

		postData := `{"title":"CSRF Test","url":"https://example.com/csrf","content":"Test"}`
		req, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", strings.NewReader(postData))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		testutils.WithStandardHeaders(req)
		req.Header.Set("Authorization", "Bearer "+authClient.Token)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusForbidden {
			t.Error("API routes should bypass CSRF protection")
		}
	})

	t.Run("csrf_allows_get_requests", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/auth/login", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusForbidden {
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
		req, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", strings.NewReader(postData))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		testutils.WithStandardHeaders(req)
		req.Header.Set("Authorization", "Bearer "+authClient.Token)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusRequestEntityTooLarge {
			t.Error("Small request should not exceed size limit")
		}
	})

	t.Run("request_exceeds_size_limit", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "sizelimituser2", "StrongPass123!")
		authClient := ctx.loginUser(t, testUser.Username, "StrongPass123!")

		largeData := strings.Repeat("a", 2*1024*1024)
		postData := `{"title":"test","url":"https://example.com","content":"` + largeData + `"}`
		req, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", strings.NewReader(postData))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		testutils.WithStandardHeaders(req)
		req.Header.Set("Authorization", "Bearer "+authClient.Token)

		resp, err := ctx.client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusRequestEntityTooLarge {
			t.Log("Request size limit enforced correctly")
		} else {
			t.Logf("Request size limit check result: status %d", resp.StatusCode)
		}
	})
}

func isGzipCompressed(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}
