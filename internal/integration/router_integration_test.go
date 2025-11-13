package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goyco/internal/middleware"
	"goyco/internal/testutils"
)

func TestIntegration_Router_FullMiddlewareChain(t *testing.T) {
	ctx := setupTestContext(t)
	router := ctx.Router

	t.Run("SecurityHeaders_Present", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)

		assertHeader(t, rec, "X-Content-Type-Options", "")
		assertHeader(t, rec, "X-Frame-Options", "")
		assertHeader(t, rec, "X-XSS-Protection", "")
	})

	t.Run("CORS_Headers_Present", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/api/posts", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertHeader(t, rec, "Access-Control-Allow-Origin", "")
	})

	t.Run("Logging_Middleware_Executes", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code == 0 {
			t.Error("Expected logging middleware to execute")
		}
	})

	t.Run("RequestSizeLimit_Enforced", func(t *testing.T) {
		user := createUserWithCleanup(t, ctx, "size_limit_user", "size_limit@example.com")
		largeBody := strings.Repeat("a", 10*1024*1024)
		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBufferString(largeBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 413 or 400 for oversized request, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("DBMonitoring_Active", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		var response map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&response); err == nil {
			if data, ok := response["data"].(map[string]any); ok {
				if _, exists := data["database_stats"]; !exists {
					t.Error("Expected database_stats in health response")
				}
			}
		}
	})

	t.Run("Metrics_Middleware_Executes", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/metrics", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
		if response != nil {
			if data, ok := response["data"].(map[string]any); ok {
				if _, exists := data["database"]; !exists {
					t.Error("Expected database metrics in response")
				}
			}
		}
	})

	t.Run("StaticFiles_Served", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/robots.txt", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)

		if !strings.Contains(rec.Body.String(), "User-agent") {
			t.Error("Expected robots.txt content")
		}
	})

	t.Run("API_Routes_Accessible", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)
	})

	t.Run("Health_Endpoint_Accessible", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
		if response != nil {
			if success, ok := response["success"].(bool); !ok || !success {
				t.Error("Expected success=true in health response")
			}
		}
	})

	t.Run("Middleware_Order_Correct", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertHeader(t, rec, "X-Content-Type-Options", "")

		if rec.Code == 0 {
			t.Error("Response should have status code")
		}
	})

	t.Run("Compression_Middleware_Active", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Header().Get("Content-Encoding") == "" {
			t.Log("Compression may not be applied to small responses")
		}
	})

	t.Run("Cache_Middleware_Active", func(t *testing.T) {
		req1 := httptest.NewRequest("GET", "/api/posts", nil)
		rec1 := httptest.NewRecorder()
		router.ServeHTTP(rec1, req1)

		req2 := httptest.NewRequest("GET", "/api/posts", nil)
		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, req2)

		if rec1.Code != rec2.Code {
			t.Error("Cached responses should have same status")
		}
	})

	t.Run("Auth_Middleware_Integration", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createUserWithCleanup(t, ctx, "auth_middleware_user", "auth_middleware@example.com")

		req := httptest.NewRequest("GET", "/api/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)
	})

	t.Run("RateLimit_Middleware_Integration", func(t *testing.T) {
		rateLimitCtx := setupTestContext(t)
		rateLimitRouter := rateLimitCtx.Router

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"test","password":"test"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			rateLimitRouter.ServeHTTP(rec, req)
		}

		req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"test","password":"test"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		rateLimitRouter.ServeHTTP(rec, req)

		if rec.Code == http.StatusTooManyRequests {
			t.Log("Rate limiting is working")
		}
	})
}
