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

func TestIntegration_EdgeCases(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("Expired_Token_Handling", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "expired_user", "expired@example.com")

		expiredToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MDAwMDAwMDB9.expired"

		req := httptest.NewRequest("GET", "/api/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+expiredToken)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusUnauthorized)
	})

	t.Run("Concurrent_Vote_Operations", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user1 := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "vote_user1", "vote1@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user1.User.ID, "Concurrent Vote Post", "https://example.com/concurrent")

		var wg sync.WaitGroup
		errors := make(chan error, 10)

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				voteBody := map[string]string{"type": "up"}
				body, _ := json.Marshal(voteBody)
				req := httptest.NewRequest("POST", fmt.Sprintf("/api/posts/%d/vote", post.ID), bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+user1.Token)
				req = testutils.WithUserContext(req, middleware.UserIDKey, user1.User.ID)
				req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
				rec := httptest.NewRecorder()
				ctx.Router.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					errors <- fmt.Errorf("unexpected status: %d", rec.Code)
				}
			}()
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Error(err)
		}
	})

	t.Run("Large_Payload_Handling", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "large_user", "large@example.com")

		largeContent := make([]byte, 10001)
		for i := range largeContent {
			largeContent[i] = 'a'
		}

		postBody := map[string]string{
			"title":   "Large Post",
			"url":     "https://example.com/large",
			"content": string(largeContent),
		}
		body, _ := json.Marshal(postBody)
		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusBadRequest)

		smallContent := make([]byte, 1000)
		for i := range smallContent {
			smallContent[i] = 'a'
		}

		postBody2 := map[string]string{
			"title":   "Small Post",
			"url":     "https://example.com/small",
			"content": string(smallContent),
		}
		body2, _ := json.Marshal(postBody2)
		req2 := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(body2))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Authorization", "Bearer "+user.Token)
		req2 = testutils.WithUserContext(req2, middleware.UserIDKey, user.User.ID)
		rec2 := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec2, req2)

		assertStatus(t, rec2, http.StatusCreated)
	})

	t.Run("Malformed_JSON_Payloads", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "malformed_user", "malformed@example.com")

		malformedPayloads := []string{
			`{"title": "test"`,
			`{"title": "test",}`,
			`{title: "test"}`,
			`{"title": 'test'}`,
			`{"title": "test" "url": ""}`,
		}

		for _, payload := range malformedPayloads {
			req := httptest.NewRequest("POST", "/api/posts", bytes.NewBufferString(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+user.Token)
			req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
			rec := httptest.NewRecorder()

			ctx.Router.ServeHTTP(rec, req)

			assertErrorResponse(t, rec, http.StatusBadRequest)
		}
	})

	t.Run("Race_Condition_Vote_Removal", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "race_user", "race@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Race Post", "https://example.com/race")

		voteBody := map[string]string{"type": "up"}
		body, _ := json.Marshal(voteBody)

		voteReq := httptest.NewRequest("POST", fmt.Sprintf("/api/posts/%d/vote", post.ID), bytes.NewBuffer(body))
		voteReq.Header.Set("Content-Type", "application/json")
		voteReq.Header.Set("Authorization", "Bearer "+user.Token)
		voteReq = testutils.WithUserContext(voteReq, middleware.UserIDKey, user.User.ID)
		voteReq = testutils.WithURLParams(voteReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		voteRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(voteRec, voteReq)
		assertStatus(t, voteRec, http.StatusOK)

		var wg sync.WaitGroup
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/posts/%d/vote", post.ID), nil)
				req.Header.Set("Authorization", "Bearer "+user.Token)
				req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
				req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
				rec := httptest.NewRecorder()
				ctx.Router.ServeHTTP(rec, req)
			}()
		}
		wg.Wait()

		getVotesReq := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%d/votes", post.ID), nil)
		getVotesReq.Header.Set("Authorization", "Bearer "+user.Token)
		getVotesReq = testutils.WithUserContext(getVotesReq, middleware.UserIDKey, user.User.ID)
		getVotesReq = testutils.WithURLParams(getVotesReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		getVotesRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(getVotesRec, getVotesReq)

		votesResponse := assertJSONResponse(t, getVotesRec, http.StatusOK)
		if votesResponse != nil {
			if data, ok := votesResponse["data"].(map[string]any); ok {
				if votes, ok := data["votes"].([]any); ok {
					userVoteCount := 0
					for _, vote := range votes {
						if voteMap, ok := vote.(map[string]any); ok {
							if userID, ok := voteMap["user_id"].(float64); ok && uint(userID) == user.User.ID {
								userVoteCount++
							}
						}
					}
					if userVoteCount > 1 {
						t.Errorf("Expected at most 1 vote from user, got %d", userVoteCount)
					}
				}
			}
		}
	})
}
