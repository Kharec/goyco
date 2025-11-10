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

func TestIntegration_CrossComponentAuthorization(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("Post_Owner_Authorization", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		owner := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "post_owner", "post_owner@example.com")
		otherUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "post_other", "post_other@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, owner.User.ID, "Owner Post", "https://example.com/owner")

		updateBody := map[string]string{
			"title":   "Updated Title",
			"content": "Updated content",
		}
		body, _ := json.Marshal(updateBody)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/posts/%d", post.ID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+otherUser.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, otherUser.User.ID)
		req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusForbidden)

		req = httptest.NewRequest("PUT", fmt.Sprintf("/api/posts/%d", post.ID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+owner.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, owner.User.ID)
		req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		rec = httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)
	})

	t.Run("Post_Delete_Authorization", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		owner := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "delete_owner", "delete_owner@example.com")
		otherUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "delete_other", "delete_other@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, owner.User.ID, "Delete Post", "https://example.com/delete")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/posts/%d", post.ID), nil)
		req.Header.Set("Authorization", "Bearer "+otherUser.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, otherUser.User.ID)
		req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusForbidden)

		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/posts/%d", post.ID), nil)
		req.Header.Set("Authorization", "Bearer "+owner.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, owner.User.ID)
		req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		rec = httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)
	})

	t.Run("User_Profile_Access_Authorization", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user1 := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "profile_user1", "profile_user1@example.com")
		user2 := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "profile_user2", "profile_user2@example.com")

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/users/%d", user1.User.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user2.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user2.User.ID)
		req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", user1.User.ID)})
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
		if response != nil {
			if data, ok := response["data"].(map[string]any); ok {
				if userData, ok := data["user"].(map[string]any); ok {
					if id, ok := userData["id"].(float64); ok && uint(id) != user1.User.ID {
						t.Errorf("Expected user ID %d, got %.0f", user1.User.ID, id)
					}
				}
			}
		}
	})

	t.Run("User_Settings_Authorization", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "settings_auth_user", "settings_auth@example.com")
		otherUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "settings_auth_other", "settings_auth_other@example.com")

		updateBody := map[string]string{
			"email": "newemail@example.com",
		}
		body, _ := json.Marshal(updateBody)

		req := httptest.NewRequest("PUT", "/api/auth/email", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+otherUser.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, otherUser.User.ID)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
		if response == nil {
			return
		}
		if data, ok := response["data"].(map[string]any); ok {
			if userData, ok := data["user"].(map[string]any); ok {
				if email, ok := userData["email"].(string); ok && email == "newemail@example.com" {
					if id, ok := userData["id"].(float64); ok && uint(id) != otherUser.User.ID {
						t.Error("Expected email update to affect the authenticated user, not another user")
					}
				}
			}
		}

		updateBody2 := map[string]string{
			"email": "anothernewemail@example.com",
		}
		body2, _ := json.Marshal(updateBody2)

		req = httptest.NewRequest("PUT", "/api/auth/email", bytes.NewBuffer(body2))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec = httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)
	})

	t.Run("Vote_Authorization", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "vote_auth_user", "vote_auth@example.com")
		postOwner := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "vote_auth_owner", "vote_auth_owner@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, postOwner.User.ID, "Vote Auth Post", "https://example.com/vote-auth")

		voteBody := map[string]string{"type": "up"}
		body, _ := json.Marshal(voteBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/posts/%d/vote", post.ID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)

		req = httptest.NewRequest("POST", fmt.Sprintf("/api/posts/%d/vote", post.ID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusUnauthorized)
	})

	t.Run("Protected_Endpoint_Without_Auth", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusUnauthorized)
	})

	t.Run("Protected_Endpoint_With_Invalid_Token", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusUnauthorized)
	})

	t.Run("User_List_Authorization", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "list_auth_user", "list_auth@example.com")

		req := httptest.NewRequest("GET", "/api/users", nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)

		req = httptest.NewRequest("GET", "/api/users", nil)
		rec = httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusUnauthorized)
	})

	t.Run("Refresh_Token_Authorization", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "refresh_auth_user", "refresh_auth@example.com")

		loginResult, err := ctx.AuthService.Login("refresh_auth_user", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		refreshBody := map[string]string{
			"refresh_token": loginResult.RefreshToken,
		}
		body, _ := json.Marshal(refreshBody)

		req := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
		if response == nil {
			return
		}
		if data, ok := response["data"].(map[string]any); ok {
			if _, exists := data["access_token"]; !exists {
				t.Error("Expected access_token in refresh response")
			}
		} else {
			t.Error("Expected data field in refresh response")
		}

		refreshBody = map[string]string{
			"refresh_token": "invalid-refresh-token",
		}
		body, _ = json.Marshal(refreshBody)

		req = httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusUnauthorized)
	})
}
