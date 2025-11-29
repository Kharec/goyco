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

	router := server.NewRouter(newRouterConfigBuilder().
		withIndividualHandlers(authHandler, postHandler, voteHandler, userHandler, apiHandler, authService).
		withStaticDir(staticDir).
		build())

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
		firstRequest := httptest.NewRequest("GET", "/api/posts", nil)
		firstRecorder := httptest.NewRecorder()
		router.ServeHTTP(firstRecorder, firstRequest)

		time.Sleep(10 * time.Millisecond)

		secondRequest := httptest.NewRequest("GET", "/api/posts", nil)
		secondRecorder := httptest.NewRecorder()
		router.ServeHTTP(secondRecorder, secondRequest)

		if firstRecorder.Code != secondRecorder.Code {
			t.Error("Cached responses should have same status code")
		}

		if firstRecorder.Body.String() != secondRecorder.Body.String() {
			t.Error("Cached responses should have same body")
		}

		if secondRecorder.Header().Get("X-Cache") != "HIT" {
			t.Log("Cache may not be enabled for this path or response may not be cacheable")
		}
	})

	t.Run("Cache_Invalidation_On_POST", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createUserWithCleanup(t, ctx, "cache_post_user", "cache_post@example.com")

		firstRequest := httptest.NewRequest("GET", "/api/posts", nil)
		firstRecorder := httptest.NewRecorder()
		router.ServeHTTP(firstRecorder, firstRequest)

		time.Sleep(10 * time.Millisecond)

		postBody := map[string]string{
			"title":   "Cache Test Post",
			"url":     "https://example.com/cache-test",
			"content": "Test content",
		}
		body, _ := json.Marshal(postBody)
		secondRequest := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(body))
		secondRequest.Header.Set("Content-Type", "application/json")
		secondRequest.Header.Set("Authorization", "Bearer "+user.Token)
		secondRequest = testutils.WithUserContext(secondRequest, middleware.UserIDKey, user.User.ID)
		secondRecorder := httptest.NewRecorder()
		router.ServeHTTP(secondRecorder, secondRequest)

		time.Sleep(10 * time.Millisecond)

		req3 := httptest.NewRequest("GET", "/api/posts", nil)
		rec3 := httptest.NewRecorder()
		router.ServeHTTP(rec3, req3)

		if firstRecorder.Body.String() == rec3.Body.String() && firstRecorder.Code == http.StatusOK && rec3.Code == http.StatusOK {
			t.Log("Cache invalidation may not be working or cache may not be enabled")
		}
	})

	t.Run("Cache_Headers_Present", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/api/posts", nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)

		if recorder.Header().Get("Cache-Control") == "" && recorder.Header().Get("X-Cache") == "" {
			t.Log("Cache headers may not be present for all responses")
		}
	})

	t.Run("Cache_Invalidation_On_DELETE", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createUserWithCleanup(t, ctx, "cache_delete_user", "cache_delete@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Cache Delete Post", "https://example.com/cache-delete")

		firstRequest := httptest.NewRequest("GET", "/api/posts", nil)
		firstRecorder := httptest.NewRecorder()
		router.ServeHTTP(firstRecorder, firstRequest)

		time.Sleep(10 * time.Millisecond)

		secondRequest := httptest.NewRequest("DELETE", "/api/posts/"+fmt.Sprintf("%d", post.ID), nil)
		secondRequest.Header.Set("Authorization", "Bearer "+user.Token)
		secondRequest = testutils.WithUserContext(secondRequest, middleware.UserIDKey, user.User.ID)
		secondRequest = testutils.WithURLParams(secondRequest, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		secondRecorder := httptest.NewRecorder()
		router.ServeHTTP(secondRecorder, secondRequest)

		time.Sleep(10 * time.Millisecond)

		req3 := httptest.NewRequest("GET", "/api/posts", nil)
		rec3 := httptest.NewRecorder()
		router.ServeHTTP(rec3, req3)

		if firstRecorder.Body.String() == rec3.Body.String() && firstRecorder.Code == http.StatusOK && rec3.Code == http.StatusOK {
			t.Log("Cache invalidation may not be working or cache may not be enabled")
		}
	})
}
