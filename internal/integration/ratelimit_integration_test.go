package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"goyco/internal/config"
	"goyco/internal/handlers"
	"goyco/internal/middleware"
	"goyco/internal/server"
	"goyco/internal/services"
	"goyco/internal/testutils"
)

func setupRateLimitRouter(t *testing.T, rateLimitConfig config.RateLimitConfig) (http.Handler, *testutils.ServiceSuite) {
	t.Helper()
	suite := testutils.NewServiceSuite(t)

	authService, err := services.NewAuthFacadeForTest(testutils.AppTestConfig, suite.UserRepo, suite.PostRepo, suite.DeletionRepo, suite.RefreshTokenRepo, suite.EmailSender)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	voteService := services.NewVoteService(suite.VoteRepo, suite.PostRepo, suite.DB)
	metadataService := services.NewURLMetadataService()

	authHandler := handlers.NewAuthHandler(authService, suite.UserRepo)
	postHandler := handlers.NewPostHandler(suite.PostRepo, metadataService, voteService)
	voteHandler := handlers.NewVoteHandler(voteService)
	userHandler := handlers.NewUserHandler(suite.UserRepo, authService)
	apiHandler := handlers.NewAPIHandlerWithMonitoring(testutils.AppTestConfig, suite.PostRepo, suite.UserRepo, voteService, suite.DB, middleware.NewInMemoryDBMonitor())

	staticDir := t.TempDir()

	router := server.NewRouter(newRouterConfigBuilder().
		withIndividualHandlers(authHandler, postHandler, voteHandler, userHandler, apiHandler, authService).
		withStaticDir(staticDir).
		withRateLimitConfig(rateLimitConfig).
		build())

	return router, suite
}

func TestIntegration_RateLimiting(t *testing.T) {
	t.Run("Auth_RateLimit_Enforced", func(t *testing.T) {
		rateLimitConfig := testutils.AppTestConfig.RateLimit
		rateLimitConfig.AuthLimit = 2
		router, _ := setupRateLimitRouter(t, rateLimitConfig)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"test","password":"test"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
		}

		req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"test","password":"test"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusTooManyRequests)

		if rec.Header().Get("Retry-After") == "" {
			t.Error("Expected Retry-After header")
		}

		var response map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&response); err == nil {
			if _, exists := response["retry_after"]; !exists {
				t.Error("Expected retry_after in response")
			}
		}
	})

	t.Run("General_RateLimit_Enforced", func(t *testing.T) {
		rateLimitConfig := testutils.AppTestConfig.RateLimit
		rateLimitConfig.GeneralLimit = 5
		router, _ := setupRateLimitRouter(t, rateLimitConfig)

		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("GET", "/api/posts", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
		}

		req := httptest.NewRequest("GET", "/api/posts", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusTooManyRequests)
	})

	t.Run("Health_RateLimit_Enforced", func(t *testing.T) {
		rateLimitConfig := testutils.AppTestConfig.RateLimit
		rateLimitConfig.HealthLimit = 3
		router, _ := setupRateLimitRouter(t, rateLimitConfig)

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/health", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
		}

		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusTooManyRequests)
	})

	t.Run("Metrics_RateLimit_Enforced", func(t *testing.T) {
		rateLimitConfig := testutils.AppTestConfig.RateLimit
		rateLimitConfig.MetricsLimit = 2
		router, _ := setupRateLimitRouter(t, rateLimitConfig)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/metrics", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
		}

		req := httptest.NewRequest("GET", "/metrics", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusTooManyRequests)
	})

	t.Run("RateLimit_Different_Endpoints_Independent", func(t *testing.T) {
		rateLimitConfig := testutils.AppTestConfig.RateLimit
		rateLimitConfig.AuthLimit = 2
		rateLimitConfig.GeneralLimit = 10
		router, _ := setupRateLimitRouter(t, rateLimitConfig)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"test","password":"test"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
		}

		req := httptest.NewRequest("GET", "/api/posts", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)
	})

	t.Run("RateLimit_With_Authentication", func(t *testing.T) {
		rateLimitConfig := testutils.AppTestConfig.RateLimit
		rateLimitConfig.GeneralLimit = 3
		router, suite := setupRateLimitRouter(t, rateLimitConfig)

		authService, err := services.NewAuthFacadeForTest(testutils.AppTestConfig, suite.UserRepo, suite.PostRepo, suite.DeletionRepo, suite.RefreshTokenRepo, suite.EmailSender)
		if err != nil {
			t.Fatalf("Failed to create auth service: %v", err)
		}

		suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, authService, suite.UserRepo, uniqueTestUsername(t, "ratelimit_auth"), uniqueTestEmail(t, "ratelimit_auth"))

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/api/auth/me", nil)
			req.Header.Set("Authorization", "Bearer "+user.Token)
			req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
		}

		req := httptest.NewRequest("GET", "/api/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusTooManyRequests)
	})
}
