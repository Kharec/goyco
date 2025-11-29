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

		request := httptest.NewRequest("GET", "/api/auth/me", nil)
		request.Header.Set("Authorization", "Bearer "+expiredToken)
		recorder := httptest.NewRecorder()

		ctx.Router.ServeHTTP(recorder, request)

		assertErrorResponse(t, recorder, http.StatusUnauthorized)
	})

	t.Run("Concurrent_Vote_Operations", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		firstUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "vote_user1", "vote1@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, firstUser.User.ID, "Concurrent Vote Post", "https://example.com/concurrent")

		var wg sync.WaitGroup
		errors := make(chan error, 10)

		for range 5 {
			wg.Go(func() {
				voteBody := map[string]string{"type": "up"}
				body, _ := json.Marshal(voteBody)
				request := httptest.NewRequest("POST", fmt.Sprintf("/api/posts/%d/vote", post.ID), bytes.NewBuffer(body))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Authorization", "Bearer "+firstUser.Token)
				request = testutils.WithUserContext(request, middleware.UserIDKey, firstUser.User.ID)
				request = testutils.WithURLParams(request, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
				recorder := httptest.NewRecorder()
				ctx.Router.ServeHTTP(recorder, request)
				if recorder.Code != http.StatusOK {
					errors <- fmt.Errorf("unexpected status: %d", recorder.Code)
				}
			})
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
		for idx := range largeContent {
			largeContent[idx] = 'a'
		}

		postBody := map[string]string{
			"title":   "Large Post",
			"url":     "https://example.com/large",
			"content": string(largeContent),
		}
		body, _ := json.Marshal(postBody)
		request := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+user.Token)
		request = testutils.WithUserContext(request, middleware.UserIDKey, user.User.ID)
		recorder := httptest.NewRecorder()

		ctx.Router.ServeHTTP(recorder, request)

		assertErrorResponse(t, recorder, http.StatusBadRequest)

		smallContent := make([]byte, 1000)
		for idx := range smallContent {
			smallContent[idx] = 'a'
		}

		secondPostBody := map[string]string{
			"title":   "Small Post",
			"url":     "https://example.com/small",
			"content": string(smallContent),
		}
		secondBody, _ := json.Marshal(secondPostBody)
		secondRequest := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(secondBody))
		secondRequest.Header.Set("Content-Type", "application/json")
		secondRequest.Header.Set("Authorization", "Bearer "+user.Token)
		secondRequest = testutils.WithUserContext(secondRequest, middleware.UserIDKey, user.User.ID)
		secondRecorder := httptest.NewRecorder()

		ctx.Router.ServeHTTP(secondRecorder, secondRequest)

		assertStatus(t, secondRecorder, http.StatusCreated)
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
			request := httptest.NewRequest("POST", "/api/posts", bytes.NewBufferString(payload))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+user.Token)
			request = testutils.WithUserContext(request, middleware.UserIDKey, user.User.ID)
			recorder := httptest.NewRecorder()

			ctx.Router.ServeHTTP(recorder, request)

			assertErrorResponse(t, recorder, http.StatusBadRequest)
		}
	})

	t.Run("Race_Condition_Vote_Removal", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "race_user", "race@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Race Post", "https://example.com/race")

		voteBody := map[string]string{"type": "up"}
		body, _ := json.Marshal(voteBody)

		voteRequest := httptest.NewRequest("POST", fmt.Sprintf("/api/posts/%d/vote", post.ID), bytes.NewBuffer(body))
		voteRequest.Header.Set("Content-Type", "application/json")
		voteRequest.Header.Set("Authorization", "Bearer "+user.Token)
		voteRequest = testutils.WithUserContext(voteRequest, middleware.UserIDKey, user.User.ID)
		voteRequest = testutils.WithURLParams(voteRequest, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		voteRecorder := httptest.NewRecorder()
		ctx.Router.ServeHTTP(voteRecorder, voteRequest)
		assertStatus(t, voteRecorder, http.StatusOK)

		var wg sync.WaitGroup
		for range 3 {
			wg.Go(func() {
				request := httptest.NewRequest("DELETE", fmt.Sprintf("/api/posts/%d/vote", post.ID), nil)
				request.Header.Set("Authorization", "Bearer "+user.Token)
				request = testutils.WithUserContext(request, middleware.UserIDKey, user.User.ID)
				request = testutils.WithURLParams(request, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
				recorder := httptest.NewRecorder()
				ctx.Router.ServeHTTP(recorder, request)
			})
		}
		wg.Wait()

		getVotesRequest := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%d/votes", post.ID), nil)
		getVotesRequest.Header.Set("Authorization", "Bearer "+user.Token)
		getVotesRequest = testutils.WithUserContext(getVotesRequest, middleware.UserIDKey, user.User.ID)
		getVotesRequest = testutils.WithURLParams(getVotesRequest, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		getVotesRecorder := httptest.NewRecorder()
		ctx.Router.ServeHTTP(getVotesRecorder, getVotesRequest)

		votesResponse := assertJSONResponse(t, getVotesRecorder, http.StatusOK)
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
