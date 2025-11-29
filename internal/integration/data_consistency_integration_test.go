package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"goyco/internal/middleware"
	"goyco/internal/testutils"
)

func TestIntegration_DataConsistency(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("Post_Creation_Consistency", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "consistency_user", "consistency@example.com")

		request := makePostRequest(t, ctx.Router, "/api/posts", map[string]any{
			"title":   "Consistency Test Post",
			"url":     "https://example.com/consistency",
			"content": "Test content",
		}, user, nil)

		createResponse := assertJSONResponse(t, request, http.StatusCreated)
		if createResponse == nil {
			return
		}

		postData, ok := getDataFromResponse(createResponse)
		if !ok {
			t.Fatal("Response missing data")
		}

		postID, ok := postData["id"].(float64)
		if !ok {
			t.Fatal("Response missing post id")
		}

		createdTitle := postData["title"]
		createdURL := postData["url"]
		createdContent := postData["content"]

		getRequest := makeGetRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%.0f", postID))

		getResponse := assertJSONResponse(t, getRequest, http.StatusOK)
		if getResponse == nil {
			return
		}

		getPostData, ok := getDataFromResponse(getResponse)
		if !ok {
			t.Fatal("Get response missing data")
		}

		if getPostData["title"] != createdTitle {
			t.Errorf("Title mismatch: created=%v, retrieved=%v", createdTitle, getPostData["title"])
		}

		if getPostData["url"] != createdURL {
			t.Errorf("URL mismatch: created=%v, retrieved=%v", createdURL, getPostData["url"])
		}

		if getPostData["content"] != createdContent {
			t.Errorf("Content mismatch: created=%v, retrieved=%v", createdContent, getPostData["content"])
		}

		if getPostData["author_id"] == nil {
			t.Error("Expected author_id to be set")
		} else if authorID, ok := getPostData["author_id"].(float64); ok {
			if uint(authorID) != user.User.ID {
				t.Errorf("Author ID mismatch: expected=%d, got=%.0f", user.User.ID, authorID)
			}
		} else {
			t.Errorf("Author ID type mismatch: expected float64, got %T", getPostData["author_id"])
		}
	})

	t.Run("Vote_Consistency", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "vote_consistency_user", "vote_consistency@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Vote Consistency Post", "https://example.com/vote-consistency")

		voteRequest := makePostRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d/vote", post.ID), map[string]any{"type": "up"}, user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		assertStatus(t, voteRequest, http.StatusOK)

		getVotesRequest := makeAuthenticatedGetRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d/votes", post.ID), user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

		votesResponse := assertJSONResponse(t, getVotesRequest, http.StatusOK)
		if votesResponse == nil {
			return
		}

		votesData, ok := getDataFromResponse(votesResponse)
		if !ok {
			t.Fatal("Votes response missing data")
		}

		votes, ok := votesData["votes"].([]any)
		if !ok {
			t.Fatal("Votes response missing votes array")
		}

		if len(votes) == 0 {
			t.Error("Expected at least one vote")
		}

		foundUserVote := false
		for _, vote := range votes {
			if voteMap, ok := vote.(map[string]any); ok {
				var userIDVal any
				var exists bool
				if userIDVal, exists = voteMap["user_id"]; !exists {
					userIDVal, exists = voteMap["UserID"]
				}
				if exists && userIDVal != nil {
					if userID, ok := userIDVal.(float64); ok && uint(userID) == user.User.ID {
						var voteType string
						if vt, ok := voteMap["type"].(string); ok {
							voteType = vt
						} else if vt, ok := voteMap["Type"].(string); ok {
							voteType = vt
						}
						if voteType != "" && voteType != "up" {
							t.Errorf("Expected vote type 'up', got '%s'", voteType)
						}
						foundUserVote = true
						break
					}
				}
			}
		}

		if !foundUserVote {
			t.Error("User vote not found in votes list")
		}
	})

	t.Run("Post_Update_Consistency", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "update_consistency_user", "update_consistency@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Original Title", "https://example.com/original")

		updateRequest := makePutRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d", post.ID), map[string]any{
			"title":   "Updated Title",
			"content": "Updated content",
		}, user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

		assertStatus(t, updateRequest, http.StatusOK)

		getRequest := makeGetRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d", post.ID))

		getResponse := assertJSONResponse(t, getRequest, http.StatusOK)
		if getResponse == nil {
			return
		}

		getPostData, ok := getDataFromResponse(getResponse)
		if !ok {
			t.Fatal("Get response missing data")
		}

		if getPostData["title"] != "Updated Title" {
			t.Errorf("Title not updated: expected 'Updated Title', got %v", getPostData["title"])
		}

		if getPostData["content"] != "Updated content" {
			t.Errorf("Content not updated: expected 'Updated content', got %v", getPostData["content"])
		}
	})

	t.Run("User_Posts_Consistency", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "user_posts_consistency", "user_posts_consistency@example.com")

		firstPost := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Post 1", "https://example.com/post1")
		secondPost := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Post 2", "https://example.com/post2")

		request := httptest.NewRequest("GET", fmt.Sprintf("/api/users/%d/posts", user.User.ID), nil)
		request.Header.Set("Authorization", "Bearer "+user.Token)
		request = testutils.WithUserContext(request, middleware.UserIDKey, user.User.ID)
		request = testutils.WithURLParams(request, map[string]string{"id": fmt.Sprintf("%d", user.User.ID)})
		recorder := httptest.NewRecorder()

		ctx.Router.ServeHTTP(recorder, request)

		response := assertJSONResponse(t, recorder, http.StatusOK)
		if response == nil {
			return
		}

		data, ok := response["data"].(map[string]any)
		if !ok {
			t.Fatal("Response missing data")
		}

		posts, ok := data["posts"].([]any)
		if !ok {
			t.Fatal("Response missing posts array")
		}

		if len(posts) < 2 {
			t.Errorf("Expected at least 2 posts, got %d", len(posts))
		}

		foundFirstPost := false
		foundSecondPost := false
		for _, post := range posts {
			if postMap, ok := post.(map[string]any); ok {
				if postID, ok := postMap["id"].(float64); ok {
					if uint(postID) == firstPost.ID {
						foundFirstPost = true
					}
					if uint(postID) == secondPost.ID {
						foundSecondPost = true
					}
				}
			}
		}

		if !foundFirstPost {
			t.Error("Post 1 not found in user posts")
		}

		if !foundSecondPost {
			t.Error("Post 2 not found in user posts")
		}
	})

	t.Run("Post_Deletion_Consistency", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "delete_consistency_user", "delete_consistency@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Delete Consistency Post", "https://example.com/delete-consistency")

		deleteRequest := httptest.NewRequest("DELETE", fmt.Sprintf("/api/posts/%d", post.ID), nil)
		deleteRequest.Header.Set("Authorization", "Bearer "+user.Token)
		deleteRequest = testutils.WithUserContext(deleteRequest, middleware.UserIDKey, user.User.ID)
		deleteRequest = testutils.WithURLParams(deleteRequest, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		deleteRecorder := httptest.NewRecorder()
		ctx.Router.ServeHTTP(deleteRecorder, deleteRequest)

		assertStatus(t, deleteRecorder, http.StatusOK)

		getRequest := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%d", post.ID), nil)
		getRecorder := httptest.NewRecorder()
		ctx.Router.ServeHTTP(getRecorder, getRequest)

		assertStatus(t, getRecorder, http.StatusNotFound)
	})

	t.Run("Vote_Removal_Consistency", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "vote_remove_consistency", "vote_remove_consistency@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Vote Remove Consistency", "https://example.com/vote-remove-consistency")

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

		removeVoteRequest := httptest.NewRequest("DELETE", fmt.Sprintf("/api/posts/%d/vote", post.ID), nil)
		removeVoteRequest.Header.Set("Authorization", "Bearer "+user.Token)
		removeVoteRequest = testutils.WithUserContext(removeVoteRequest, middleware.UserIDKey, user.User.ID)
		removeVoteRequest = testutils.WithURLParams(removeVoteRequest, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		removeVoteRecorder := httptest.NewRecorder()
		ctx.Router.ServeHTTP(removeVoteRecorder, removeVoteRequest)

		assertStatus(t, removeVoteRecorder, http.StatusOK)

		getVotesRequest := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%d/votes", post.ID), nil)
		getVotesRequest.Header.Set("Authorization", "Bearer "+user.Token)
		getVotesRequest = testutils.WithUserContext(getVotesRequest, middleware.UserIDKey, user.User.ID)
		getVotesRequest = testutils.WithURLParams(getVotesRequest, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		getVotesRecorder := httptest.NewRecorder()
		ctx.Router.ServeHTTP(getVotesRecorder, getVotesRequest)

		votesResponse := assertJSONResponse(t, getVotesRecorder, http.StatusOK)
		if votesResponse == nil {
			return
		}

		if data, ok := votesResponse["data"].(map[string]any); ok {
			if votes, ok := data["votes"].([]any); ok {
				for _, vote := range votes {
					if voteMap, ok := vote.(map[string]any); ok {
						if userID, ok := voteMap["user_id"].(float64); ok && uint(userID) == user.User.ID {
							t.Error("User vote still exists after removal")
						}
					}
				}
			}
		}
	})
}
