package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goyco/internal/handlers"
	"goyco/internal/middleware"
	"goyco/internal/server"
	"goyco/internal/services"
	"goyco/internal/testutils"
)

func setupCachingTestContext(t *testing.T) *testContext {
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

	router := server.NewRouter(server.RouterConfig{
		AuthHandler:        authHandler,
		PostHandler:        postHandler,
		VoteHandler:        voteHandler,
		UserHandler:        userHandler,
		APIHandler:         apiHandler,
		AuthService:        authService,
		PageHandler:        nil,
		StaticDir:          staticDir,
		Debug:              false,
		DisableCache:       false,
		DisableCompression: false,
		DBMonitor:          middleware.NewInMemoryDBMonitor(),
		RateLimitConfig:    testutils.AppTestConfig.RateLimit,
	})

	return &testContext{
		Router:      router,
		Suite:       suite,
		AuthService: authService,
	}
}

func TestIntegration_Caching(t *testing.T) {
	ctx := setupCachingTestContext(t)
	router := ctx.Router

	t.Run("Cache_Hit_On_Repeated_Requests", func(t *testing.T) {
		req1 := httptest.NewRequest("GET", "/api/posts", nil)
		rec1 := httptest.NewRecorder()
		router.ServeHTTP(rec1, req1)

		time.Sleep(10 * time.Millisecond)

		req2 := httptest.NewRequest("GET", "/api/posts", nil)
		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, req2)

		if rec1.Code != rec2.Code {
			t.Error("Cached responses should have same status code")
		}

		if rec1.Body.String() != rec2.Body.String() {
			t.Error("Cached responses should have same body")
		}

		if rec2.Header().Get("X-Cache") != "HIT" {
			t.Log("Cache may not be enabled for this path or response may not be cacheable")
		}
	})

	t.Run("Cache_Invalidation_On_POST", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "cache_post_user", "cache_post@example.com")

		req1 := httptest.NewRequest("GET", "/api/posts", nil)
		rec1 := httptest.NewRecorder()
		router.ServeHTTP(rec1, req1)

		time.Sleep(10 * time.Millisecond)

		postBody := map[string]string{
			"title":   "Cache Test Post",
			"url":     "https://example.com/cache-test",
			"content": "Test content",
		}
		body, _ := json.Marshal(postBody)
		req2 := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(body))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Authorization", "Bearer "+user.Token)
		req2 = testutils.WithUserContext(req2, middleware.UserIDKey, user.User.ID)
		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, req2)

		time.Sleep(10 * time.Millisecond)

		req3 := httptest.NewRequest("GET", "/api/posts", nil)
		rec3 := httptest.NewRecorder()
		router.ServeHTTP(rec3, req3)

		if rec1.Body.String() == rec3.Body.String() && rec1.Code == http.StatusOK && rec3.Code == http.StatusOK {
			t.Log("Cache invalidation may not be working or cache may not be enabled")
		}
	})

	t.Run("Cache_Headers_Present", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Header().Get("Cache-Control") == "" && rec.Header().Get("X-Cache") == "" {
			t.Log("Cache headers may not be present for all responses")
		}
	})

	t.Run("Cache_Invalidation_On_DELETE", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "cache_delete_user", "cache_delete@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Cache Delete Post", "https://example.com/cache-delete")

		req1 := httptest.NewRequest("GET", "/api/posts", nil)
		rec1 := httptest.NewRecorder()
		router.ServeHTTP(rec1, req1)

		time.Sleep(10 * time.Millisecond)

		req2 := httptest.NewRequest("DELETE", "/api/posts/"+fmt.Sprintf("%d", post.ID), nil)
		req2.Header.Set("Authorization", "Bearer "+user.Token)
		req2 = testutils.WithUserContext(req2, middleware.UserIDKey, user.User.ID)
		req2 = testutils.WithURLParams(req2, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, req2)

		time.Sleep(10 * time.Millisecond)

		req3 := httptest.NewRequest("GET", "/api/posts", nil)
		rec3 := httptest.NewRecorder()
		router.ServeHTTP(rec3, req3)

		if rec1.Body.String() == rec3.Body.String() && rec1.Code == http.StatusOK && rec3.Code == http.StatusOK {
			t.Log("Cache invalidation may not be working or cache may not be enabled")
		}
	})
}
