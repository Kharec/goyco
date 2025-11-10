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

		postBody := map[string]string{
			"title":   "Consistency Test Post",
			"url":     "https://example.com/consistency",
			"content": "Test content",
		}
		body, _ := json.Marshal(postBody)

		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		createResponse := assertJSONResponse(t, rec, http.StatusCreated)
		if createResponse == nil {
			return
		}

		postData, ok := createResponse["data"].(map[string]any)
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

		getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%.0f", postID), nil)
		getRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(getRec, getReq)

		getResponse := assertJSONResponse(t, getRec, http.StatusOK)
		if getResponse == nil {
			return
		}

		getPostData, ok := getResponse["data"].(map[string]any)
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

		getVotesReq := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%d/votes", post.ID), nil)
		getVotesReq.Header.Set("Authorization", "Bearer "+user.Token)
		getVotesReq = testutils.WithUserContext(getVotesReq, middleware.UserIDKey, user.User.ID)
		getVotesReq = testutils.WithURLParams(getVotesReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		getVotesRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(getVotesRec, getVotesReq)

		votesResponse := assertJSONResponse(t, getVotesRec, http.StatusOK)
		if votesResponse == nil {
			return
		}

		votesData, ok := votesResponse["data"].(map[string]any)
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

		updateBody := map[string]string{
			"title":   "Updated Title",
			"content": "Updated content",
		}
		body, _ := json.Marshal(updateBody)

		updateReq := httptest.NewRequest("PUT", fmt.Sprintf("/api/posts/%d", post.ID), bytes.NewBuffer(body))
		updateReq.Header.Set("Content-Type", "application/json")
		updateReq.Header.Set("Authorization", "Bearer "+user.Token)
		updateReq = testutils.WithUserContext(updateReq, middleware.UserIDKey, user.User.ID)
		updateReq = testutils.WithURLParams(updateReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		updateRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(updateRec, updateReq)

		assertStatus(t, updateRec, http.StatusOK)

		getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%d", post.ID), nil)
		getRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(getRec, getReq)

		getResponse := assertJSONResponse(t, getRec, http.StatusOK)
		if getResponse == nil {
			return
		}

		getPostData, ok := getResponse["data"].(map[string]any)
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

		post1 := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Post 1", "https://example.com/post1")
		post2 := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Post 2", "https://example.com/post2")

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/users/%d/posts", user.User.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", user.User.ID)})
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
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

		foundPost1 := false
		foundPost2 := false
		for _, post := range posts {
			if postMap, ok := post.(map[string]any); ok {
				if postID, ok := postMap["id"].(float64); ok {
					if uint(postID) == post1.ID {
						foundPost1 = true
					}
					if uint(postID) == post2.ID {
						foundPost2 = true
					}
				}
			}
		}

		if !foundPost1 {
			t.Error("Post 1 not found in user posts")
		}

		if !foundPost2 {
			t.Error("Post 2 not found in user posts")
		}
	})

	t.Run("Post_Deletion_Consistency", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "delete_consistency_user", "delete_consistency@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Delete Consistency Post", "https://example.com/delete-consistency")

		deleteReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/posts/%d", post.ID), nil)
		deleteReq.Header.Set("Authorization", "Bearer "+user.Token)
		deleteReq = testutils.WithUserContext(deleteReq, middleware.UserIDKey, user.User.ID)
		deleteReq = testutils.WithURLParams(deleteReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		deleteRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(deleteRec, deleteReq)

		assertStatus(t, deleteRec, http.StatusOK)

		getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%d", post.ID), nil)
		getRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(getRec, getReq)

		assertStatus(t, getRec, http.StatusNotFound)
	})

	t.Run("Vote_Removal_Consistency", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "vote_remove_consistency", "vote_remove_consistency@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Vote Remove Consistency", "https://example.com/vote-remove-consistency")

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

		removeVoteReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/posts/%d/vote", post.ID), nil)
		removeVoteReq.Header.Set("Authorization", "Bearer "+user.Token)
		removeVoteReq = testutils.WithUserContext(removeVoteReq, middleware.UserIDKey, user.User.ID)
		removeVoteReq = testutils.WithURLParams(removeVoteReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		removeVoteRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(removeVoteRec, removeVoteReq)

		assertStatus(t, removeVoteRec, http.StatusOK)

		getVotesReq := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%d/votes", post.ID), nil)
		getVotesReq.Header.Set("Authorization", "Bearer "+user.Token)
		getVotesReq = testutils.WithUserContext(getVotesReq, middleware.UserIDKey, user.User.ID)
		getVotesReq = testutils.WithURLParams(getVotesReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		getVotesRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(getVotesRec, getVotesReq)

		votesResponse := assertJSONResponse(t, getVotesRec, http.StatusOK)
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
