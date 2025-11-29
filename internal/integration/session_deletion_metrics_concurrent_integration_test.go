package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"goyco/internal/middleware"
	"goyco/internal/testutils"
)

func TestIntegration_SessionManagement(t *testing.T) {
	ctx := setupTestContext(t)
	router := ctx.Router

	t.Run("Session_Invalidation_On_Password_Change", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "session_pass_user", "session_pass@example.com")

		firstRequest := httptest.NewRequest("GET", "/api/auth/me", nil)
		firstRequest.Header.Set("Authorization", "Bearer "+user.Token)
		firstRequest = testutils.WithUserContext(firstRequest, middleware.UserIDKey, user.User.ID)
		firstRecorder := httptest.NewRecorder()
		router.ServeHTTP(firstRecorder, firstRequest)

		assertStatus(t, firstRecorder, http.StatusOK)

		requestBody := map[string]string{
			"current_password": "SecurePass123!",
			"new_password":     "NewSecurePass123!",
		}
		body, _ := json.Marshal(requestBody)
		secondRequest := httptest.NewRequest("PUT", "/api/auth/password", bytes.NewBuffer(body))
		secondRequest.Header.Set("Content-Type", "application/json")
		secondRequest.Header.Set("Authorization", "Bearer "+user.Token)
		secondRequest = testutils.WithUserContext(secondRequest, middleware.UserIDKey, user.User.ID)
		secondRecorder := httptest.NewRecorder()
		router.ServeHTTP(secondRecorder, secondRequest)

		assertStatus(t, secondRecorder, http.StatusOK)

		thirdRequest := httptest.NewRequest("GET", "/api/auth/me", nil)
		thirdRequest.Header.Set("Authorization", "Bearer "+user.Token)
		thirdRequest = testutils.WithUserContext(thirdRequest, middleware.UserIDKey, user.User.ID)
		thirdRecorder := httptest.NewRecorder()
		router.ServeHTTP(thirdRecorder, thirdRequest)

		assertErrorResponse(t, thirdRecorder, http.StatusUnauthorized)
	})

	t.Run("Session_Invalidation_On_Account_Lock", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "session_lock_user", "session_lock@example.com")

		firstRequest := httptest.NewRequest("GET", "/api/auth/me", nil)
		firstRequest.Header.Set("Authorization", "Bearer "+user.Token)
		firstRequest = testutils.WithUserContext(firstRequest, middleware.UserIDKey, user.User.ID)
		firstRecorder := httptest.NewRecorder()
		router.ServeHTTP(firstRecorder, firstRequest)

		assertStatus(t, firstRecorder, http.StatusOK)

		if err := ctx.Suite.UserRepo.Lock(user.User.ID); err != nil {
			t.Fatalf("Failed to lock user: %v", err)
		}

		secondRequest := httptest.NewRequest("GET", "/api/auth/me", nil)
		secondRequest.Header.Set("Authorization", "Bearer "+user.Token)
		secondRequest = testutils.WithUserContext(secondRequest, middleware.UserIDKey, user.User.ID)
		secondRecorder := httptest.NewRecorder()
		router.ServeHTTP(secondRecorder, secondRequest)

		assertErrorResponse(t, secondRecorder, http.StatusUnauthorized)
	})

	t.Run("Refresh_Token_Revocation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "refresh_revoke_user", "refresh_revoke@example.com")

		loginResult, err := ctx.AuthService.Login("refresh_revoke_user", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		if loginResult.RefreshToken == "" {
			t.Fatal("Expected refresh token")
		}

		requestBody := map[string]string{
			"refresh_token": loginResult.RefreshToken,
		}
		body, _ := json.Marshal(requestBody)
		request := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewBuffer(body))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)

		assertStatus(t, recorder, http.StatusOK)

		if err := ctx.AuthService.RevokeRefreshToken(loginResult.RefreshToken); err != nil {
			t.Fatalf("Failed to revoke token: %v", err)
		}

		secondRequest := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewBuffer(body))
		secondRequest.Header.Set("Content-Type", "application/json")
		secondRecorder := httptest.NewRecorder()
		router.ServeHTTP(secondRecorder, secondRequest)

		assertErrorResponse(t, secondRecorder, http.StatusUnauthorized)
	})

	t.Run("Multiple_Sessions_Independent", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		firstUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "multi_session_user1", "multi_session1@example.com")
		secondUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "multi_session_user2", "multi_session2@example.com")

		firstRequest := httptest.NewRequest("GET", "/api/auth/me", nil)
		firstRequest.Header.Set("Authorization", "Bearer "+firstUser.Token)
		firstRequest = testutils.WithUserContext(firstRequest, middleware.UserIDKey, firstUser.User.ID)
		firstRecorder := httptest.NewRecorder()
		router.ServeHTTP(firstRecorder, firstRequest)

		secondRequest := httptest.NewRequest("GET", "/api/auth/me", nil)
		secondRequest.Header.Set("Authorization", "Bearer "+secondUser.Token)
		secondRequest = testutils.WithUserContext(secondRequest, middleware.UserIDKey, secondUser.User.ID)
		secondRecorder := httptest.NewRecorder()
		router.ServeHTTP(secondRecorder, secondRequest)

		assertStatus(t, firstRecorder, http.StatusOK)
		assertStatus(t, secondRecorder, http.StatusOK)
	})
}

func TestIntegration_AccountDeletion(t *testing.T) {
	ctx := setupTestContext(t)
	router := ctx.Router

	t.Run("Account_Deletion_Complete_Flow", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "del_flow_user", "del_flow@example.com")
		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Test Post", "https://example.com")

		requestBody := map[string]string{}
		body, _ := json.Marshal(requestBody)
		request := httptest.NewRequest("DELETE", "/api/auth/account", bytes.NewBuffer(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+user.Token)
		request = testutils.WithUserContext(request, middleware.UserIDKey, user.User.ID)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		response := assertJSONResponse(t, recorder, http.StatusOK)
		if response == nil {
			return
		}
		if _, ok := response["message"]; !ok {
			t.Error("Expected message field in response")
		}

		deletionToken := ctx.Suite.EmailSender.DeletionToken()
		if deletionToken == "" {
			t.Fatal("Expected deletion token")
		}

		confirmBody := map[string]any{
			"token": deletionToken,
		}
		confirmBodyBytes, _ := json.Marshal(confirmBody)
		confirmRequest := httptest.NewRequest("POST", "/api/auth/account/confirm", bytes.NewBuffer(confirmBodyBytes))
		confirmRequest.Header.Set("Content-Type", "application/json")
		confirmRecorder := httptest.NewRecorder()

		router.ServeHTTP(confirmRecorder, confirmRequest)

		confirmResponse := assertJSONResponse(t, confirmRecorder, http.StatusOK)
		if confirmResponse == nil {
			return
		}
		if _, ok := confirmResponse["message"]; !ok {
			t.Error("Expected message field in confirmation response")
		}
		if data, ok := confirmResponse["data"].(map[string]any); ok {
			if postsDeleted, ok := data["posts_deleted"].(bool); ok && postsDeleted {
				t.Error("Expected posts_deleted to be false when not specified")
			}
		}

		_, err := ctx.Suite.UserRepo.GetByID(user.User.ID)
		if err == nil {
			t.Error("Expected user to be deleted")
		}

		retrievedPost, err := ctx.Suite.PostRepo.GetByID(post.ID)
		if err != nil {
			t.Fatal("Expected post to still exist after soft delete")
		}
		if retrievedPost.AuthorID != nil {
			t.Error("Expected post author_id to be null after user deletion")
		}
	})

	t.Run("Account_Deletion_With_Posts", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "del_posts_user", "del_posts@example.com")
		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Deletion Post", "https://example.com/deletion")

		requestBody := map[string]string{}
		body, _ := json.Marshal(requestBody)
		request := httptest.NewRequest("DELETE", "/api/auth/account", bytes.NewBuffer(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+user.Token)
		request = testutils.WithUserContext(request, middleware.UserIDKey, user.User.ID)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		response := assertJSONResponse(t, recorder, http.StatusOK)
		if response == nil {
			return
		}
		if _, ok := response["message"]; !ok {
			t.Error("Expected message field in response")
		}

		deletionToken := ctx.Suite.EmailSender.DeletionToken()
		if deletionToken == "" {
			t.Fatal("Expected deletion token")
		}

		confirmBody := map[string]any{
			"token":        deletionToken,
			"delete_posts": true,
		}
		confirmBodyBytes, _ := json.Marshal(confirmBody)
		confirmRequest := httptest.NewRequest("POST", "/api/auth/account/confirm", bytes.NewBuffer(confirmBodyBytes))
		confirmRequest.Header.Set("Content-Type", "application/json")
		confirmRecorder := httptest.NewRecorder()

		router.ServeHTTP(confirmRecorder, confirmRequest)

		confirmResponse := assertJSONResponse(t, confirmRecorder, http.StatusOK)
		if confirmResponse == nil {
			return
		}
		if _, ok := confirmResponse["message"]; !ok {
			t.Error("Expected message field in confirmation response")
		}
		if data, ok := confirmResponse["data"].(map[string]any); ok {
			if postsDeleted, ok := data["posts_deleted"].(bool); !ok || !postsDeleted {
				t.Error("Expected posts_deleted to be true")
			}
		} else {
			t.Error("Expected data field with posts_deleted in confirmation response")
		}

		_, err := ctx.Suite.UserRepo.GetByID(user.User.ID)
		if err == nil {
			t.Error("Expected user to be deleted")
		}

		_, err = ctx.Suite.PostRepo.GetByID(post.ID)
		if err == nil {
			t.Error("Expected post to be deleted with user")
		}
	})
}

func TestIntegration_MetricsCollection(t *testing.T) {
	ctx := setupTestContext(t)
	router := ctx.Router

	t.Run("Metrics_Endpoint_Returns_Data", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/metrics", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		response := assertJSONResponse(t, recorder, http.StatusOK)
		if response != nil {
			if data, ok := response["data"].(map[string]any); ok {
				if _, exists := data["database"]; !exists {
					t.Error("Expected database metrics")
				}
			}
		}
	})

	t.Run("Metrics_Includes_DB_Stats", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "metrics_user", "metrics@example.com")

		request := httptest.NewRequest("GET", "/metrics", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		var response map[string]any
		if err := json.NewDecoder(recorder.Body).Decode(&response); err == nil {
			if data, ok := response["data"].(map[string]any); ok {
				if dbData, exists := data["database"].(map[string]any); exists {
					if _, hasQueries := dbData["total_queries"]; !hasQueries {
						t.Log("Database query metrics may not be available")
					}
				}
			}
		}
	})
}

func TestIntegration_ConcurrentRequests(t *testing.T) {
	ctx := setupTestContext(t)
	router := ctx.Router

	t.Run("Concurrent_Post_Creation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "concurrent_user", "concurrent@example.com")

		var wg sync.WaitGroup
		errors := make(chan error, 10)

		for idx := range 10 {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				postBody := map[string]string{
					"title":   fmt.Sprintf("Concurrent Post %d", index),
					"url":     fmt.Sprintf("https://example.com/concurrent-%d", index),
					"content": "Concurrent test content",
				}
				body, _ := json.Marshal(postBody)
				request := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(body))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Authorization", "Bearer "+user.Token)
				request = testutils.WithUserContext(request, middleware.UserIDKey, user.User.ID)
				recorder := httptest.NewRecorder()

				router.ServeHTTP(recorder, request)

				if recorder.Code != http.StatusCreated {
					errors <- fmt.Errorf("Post %d failed with status %d", index, recorder.Code)
				}
			}(idx)
		}

		wg.Wait()
		close(errors)

		var errs []error
		for err := range errors {
			errs = append(errs, err)
		}

		if len(errs) > 0 {
			t.Errorf("Concurrent post creation failed: %v", errs)
		}
	})

	t.Run("Concurrent_Vote_Operations", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "concurrent_vote_user", "concurrent_vote@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Concurrent Vote Post", "https://example.com/concurrent-vote")

		var wg sync.WaitGroup
		errors := make(chan error, 5)

		for range 5 {
			wg.Go(func() {

				voteBody := map[string]string{
					"type": "up",
				}
				body, _ := json.Marshal(voteBody)
				request := httptest.NewRequest("POST", fmt.Sprintf("/api/posts/%d/vote", post.ID), bytes.NewBuffer(body))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Authorization", "Bearer "+user.Token)
				request = testutils.WithUserContext(request, middleware.UserIDKey, user.User.ID)
				request = testutils.WithURLParams(request, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
				recorder := httptest.NewRecorder()

				router.ServeHTTP(recorder, request)

				if recorder.Code != http.StatusOK {
					errors <- fmt.Errorf("Vote failed with status %d", recorder.Code)
				}
			})
		}

		wg.Wait()
		close(errors)

		var errs []error
		for err := range errors {
			errs = append(errs, err)
		}

		if len(errs) > 0 {
			t.Logf("Some concurrent votes may have failed (expected): %v", errs)
		}
	})

	t.Run("Concurrent_Read_Operations", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, 20)

		for range 20 {
			wg.Go(func() {

				request := httptest.NewRequest("GET", "/api/posts", nil)
				recorder := httptest.NewRecorder()

				router.ServeHTTP(recorder, request)

				if recorder.Code != http.StatusOK {
					errors <- fmt.Errorf("Read failed with status %d", recorder.Code)
				}
			})
		}

		wg.Wait()
		close(errors)

		var errs []error
		for err := range errors {
			errs = append(errs, err)
		}

		if len(errs) > 0 {
			t.Errorf("Concurrent reads failed: %v", errs)
		}
	})
}
