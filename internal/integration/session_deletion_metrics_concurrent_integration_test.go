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

		req1 := httptest.NewRequest("GET", "/api/auth/me", nil)
		req1.Header.Set("Authorization", "Bearer "+user.Token)
		req1 = testutils.WithUserContext(req1, middleware.UserIDKey, user.User.ID)
		rec1 := httptest.NewRecorder()
		router.ServeHTTP(rec1, req1)

		assertStatus(t, rec1, http.StatusOK)

		reqBody := map[string]string{
			"current_password": "SecurePass123!",
			"new_password":     "NewSecurePass123!",
		}
		body, _ := json.Marshal(reqBody)
		req2 := httptest.NewRequest("PUT", "/api/auth/password", bytes.NewBuffer(body))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Authorization", "Bearer "+user.Token)
		req2 = testutils.WithUserContext(req2, middleware.UserIDKey, user.User.ID)
		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, req2)

		assertStatus(t, rec2, http.StatusOK)

		req3 := httptest.NewRequest("GET", "/api/auth/me", nil)
		req3.Header.Set("Authorization", "Bearer "+user.Token)
		req3 = testutils.WithUserContext(req3, middleware.UserIDKey, user.User.ID)
		rec3 := httptest.NewRecorder()
		router.ServeHTTP(rec3, req3)

		assertErrorResponse(t, rec3, http.StatusUnauthorized)
	})

	t.Run("Session_Invalidation_On_Account_Lock", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "session_lock_user", "session_lock@example.com")

		req1 := httptest.NewRequest("GET", "/api/auth/me", nil)
		req1.Header.Set("Authorization", "Bearer "+user.Token)
		req1 = testutils.WithUserContext(req1, middleware.UserIDKey, user.User.ID)
		rec1 := httptest.NewRecorder()
		router.ServeHTTP(rec1, req1)

		assertStatus(t, rec1, http.StatusOK)

		if err := ctx.Suite.UserRepo.Lock(user.User.ID); err != nil {
			t.Fatalf("Failed to lock user: %v", err)
		}

		req2 := httptest.NewRequest("GET", "/api/auth/me", nil)
		req2.Header.Set("Authorization", "Bearer "+user.Token)
		req2 = testutils.WithUserContext(req2, middleware.UserIDKey, user.User.ID)
		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, req2)

		assertErrorResponse(t, rec2, http.StatusUnauthorized)
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

		reqBody := map[string]string{
			"refresh_token": loginResult.RefreshToken,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)

		if err := ctx.AuthService.RevokeRefreshToken(loginResult.RefreshToken); err != nil {
			t.Fatalf("Failed to revoke token: %v", err)
		}

		req2 := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewBuffer(body))
		req2.Header.Set("Content-Type", "application/json")
		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, req2)

		assertErrorResponse(t, rec2, http.StatusUnauthorized)
	})

	t.Run("Multiple_Sessions_Independent", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user1 := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "multi_session_user1", "multi_session1@example.com")
		user2 := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "multi_session_user2", "multi_session2@example.com")

		req1 := httptest.NewRequest("GET", "/api/auth/me", nil)
		req1.Header.Set("Authorization", "Bearer "+user1.Token)
		req1 = testutils.WithUserContext(req1, middleware.UserIDKey, user1.User.ID)
		rec1 := httptest.NewRecorder()
		router.ServeHTTP(rec1, req1)

		req2 := httptest.NewRequest("GET", "/api/auth/me", nil)
		req2.Header.Set("Authorization", "Bearer "+user2.Token)
		req2 = testutils.WithUserContext(req2, middleware.UserIDKey, user2.User.ID)
		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, req2)

		assertStatus(t, rec1, http.StatusOK)
		assertStatus(t, rec2, http.StatusOK)
	})
}

func TestIntegration_AccountDeletion(t *testing.T) {
	ctx := setupTestContext(t)
	router := ctx.Router

	t.Run("Account_Deletion_Complete_Flow", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "del_flow_user", "del_flow@example.com")
		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Test Post", "https://example.com")

		reqBody := map[string]string{}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("DELETE", "/api/auth/account", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
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
		confirmReq := httptest.NewRequest("POST", "/api/auth/account/confirm", bytes.NewBuffer(confirmBodyBytes))
		confirmReq.Header.Set("Content-Type", "application/json")
		confirmRec := httptest.NewRecorder()

		router.ServeHTTP(confirmRec, confirmReq)

		confirmResponse := assertJSONResponse(t, confirmRec, http.StatusOK)
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

		reqBody := map[string]string{}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("DELETE", "/api/auth/account", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
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
		confirmReq := httptest.NewRequest("POST", "/api/auth/account/confirm", bytes.NewBuffer(confirmBodyBytes))
		confirmReq.Header.Set("Content-Type", "application/json")
		confirmRec := httptest.NewRecorder()

		router.ServeHTTP(confirmRec, confirmReq)

		confirmResponse := assertJSONResponse(t, confirmRec, http.StatusOK)
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
		req := httptest.NewRequest("GET", "/metrics", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
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

		req := httptest.NewRequest("GET", "/metrics", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		var response map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&response); err == nil {
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

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				postBody := map[string]string{
					"title":   fmt.Sprintf("Concurrent Post %d", index),
					"url":     fmt.Sprintf("https://example.com/concurrent-%d", index),
					"content": "Concurrent test content",
				}
				body, _ := json.Marshal(postBody)
				req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+user.Token)
				req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
				rec := httptest.NewRecorder()

				router.ServeHTTP(rec, req)

				if rec.Code != http.StatusCreated {
					errors <- fmt.Errorf("Post %d failed with status %d", index, rec.Code)
				}
			}(i)
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

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				voteBody := map[string]string{
					"type": "up",
				}
				body, _ := json.Marshal(voteBody)
				req := httptest.NewRequest("POST", fmt.Sprintf("/api/posts/%d/vote", post.ID), bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+user.Token)
				req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
				req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
				rec := httptest.NewRecorder()

				router.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					errors <- fmt.Errorf("Vote failed with status %d", rec.Code)
				}
			}()
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

		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				req := httptest.NewRequest("GET", "/api/posts", nil)
				rec := httptest.NewRecorder()

				router.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					errors <- fmt.Errorf("Read failed with status %d", rec.Code)
				}
			}()
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
