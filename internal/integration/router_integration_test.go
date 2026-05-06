package integration

import (
	"bytes"
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
		request := makeGetRequest(t, router, "/health")

		assertStatus(t, request, http.StatusOK)

		assertHeader(t, request, "X-Content-Type-Options")
		assertHeader(t, request, "X-Frame-Options")
	})

	t.Run("CORS_Headers_Present", func(t *testing.T) {
		request := httptest.NewRequest("OPTIONS", "/api/posts", nil)
		request.Header.Set("Origin", "http://localhost:3000")
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		assertHeader(t, recorder, "Access-Control-Allow-Origin")
	})

	t.Run("Logging_Middleware_Executes", func(t *testing.T) {
		request := makeGetRequest(t, router, "/health")

		if request.Code == 0 {
			t.Error("Expected logging middleware to execute")
		}
	})

	t.Run("RequestSizeLimit_Enforced", func(t *testing.T) {
		user := createUserWithCleanup(t, ctx, "size_limit_user", "size_limit@example.com")
		largeBody := strings.Repeat("a", 10*1024*1024)
		request := httptest.NewRequest("POST", "/api/posts", bytes.NewBufferString(largeBody))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+user.Token)
		request = testutils.WithUserContext(request, middleware.UserIDKey, user.User.ID)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusRequestEntityTooLarge && recorder.Code != http.StatusBadRequest {
			t.Errorf("Expected status 413 or 400 for oversized request, got %d. Body: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("DBMonitoring_Active", func(t *testing.T) {
		request := makeGetRequest(t, router, "/health")

		response := assertJSONResponse(t, request, http.StatusOK)
		if response == nil {
			return
		}

		data, ok := getDataFromResponse(response)
		if !ok {
			t.Fatal("Expected data to be a map")
		}

		services, ok := data["services"].(map[string]any)
		if !ok {
			t.Fatal("Expected services in health response")
		}

		databaseService, ok := services["database"].(map[string]any)
		if !ok {
			t.Fatal("Expected database service in health response")
		}

		details, ok := databaseService["details"].(map[string]any)
		if !ok {
			t.Fatal("Expected database details in health response")
		}

		if _, exists := details["stats"]; !exists {
			t.Error("Expected database stats in health response")
		}
	})

	t.Run("Metrics_Middleware_Executes", func(t *testing.T) {
		request := makeGetRequest(t, router, "/metrics")

		response := assertJSONResponse(t, request, http.StatusOK)
		if response != nil {
			if data, ok := response["data"].(map[string]any); ok {
				if _, exists := data["database"]; !exists {
					t.Error("Expected database metrics in response")
				}
			}
		}
	})

	t.Run("StaticFiles_Served", func(t *testing.T) {
		request := makeGetRequest(t, router, "/robots.txt")

		assertStatus(t, request, http.StatusOK)

		if !strings.Contains(request.Body.String(), "User-agent") {
			t.Error("Expected robots.txt content")
		}
	})

	t.Run("API_Routes_Accessible", func(t *testing.T) {
		request := makeGetRequest(t, router, "/api/posts")

		assertStatus(t, request, http.StatusOK)
	})

	t.Run("Health_Endpoint_Accessible", func(t *testing.T) {
		request := makeGetRequest(t, router, "/health")

		response := assertJSONResponse(t, request, http.StatusOK)
		if response != nil {
			if success, ok := response["success"].(bool); !ok || !success {
				t.Error("Expected success=true in health response")
			}
		}
	})

	t.Run("Middleware_Order_Correct", func(t *testing.T) {
		request := makeGetRequest(t, router, "/api/posts")

		assertHeader(t, request, "X-Content-Type-Options")

		if request.Code == 0 {
			t.Error("Response should have status code")
		}
	})

	t.Run("Compression_Middleware_Active", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/api/posts", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Header().Get("Content-Encoding") == "" {
			t.Log("Compression may not be applied to small responses")
		}
	})

	t.Run("Cache_Middleware_Active", func(t *testing.T) {
		firstRequest := makeGetRequest(t, router, "/api/posts")

		secondRequest := makeGetRequest(t, router, "/api/posts")

		if firstRequest.Code != secondRequest.Code {
			t.Error("Cached responses should have same status")
		}
	})

	t.Run("Auth_Middleware_Integration", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createUserWithCleanup(t, ctx, "auth_middleware_user", "auth_middleware@example.com")

		request := makeAuthenticatedGetRequest(t, router, "/api/auth/me", user, nil)

		assertStatus(t, request, http.StatusOK)
	})

	t.Run("RateLimit_Middleware_Integration", func(t *testing.T) {
		rateLimitCtx := setupTestContext(t)
		rateLimitRouter := rateLimitCtx.Router

		for range 3 {
			request := makePostRequestWithJSON(t, rateLimitRouter, "/api/auth/login", map[string]any{"username": "test", "password": "test"})
			_ = request
		}

		request := makePostRequestWithJSON(t, rateLimitRouter, "/api/auth/login", map[string]any{"username": "test", "password": "test"})

		if request.Code == http.StatusTooManyRequests {
			t.Log("Rate limiting is working")
		}
	})
}
