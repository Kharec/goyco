package integration

import (
	"fmt"
	"net/http"
	"testing"

	"goyco/internal/testutils"
)

func TestIntegration_CrossComponentAuthorization(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("Post_Owner_Authorization", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		owner := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "post_owner", "post_owner@example.com")
		otherUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "post_other", "post_other@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, owner.User.ID, "Owner Post", "https://example.com/owner")

		request := makePutRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d", post.ID), map[string]any{
			"title":   "Updated Title",
			"content": "Updated content",
		}, otherUser, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

		assertErrorResponse(t, request, http.StatusForbidden)

		request = makePutRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d", post.ID), map[string]any{
			"title":   "Updated Title",
			"content": "Updated content",
		}, owner, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

		assertStatus(t, request, http.StatusOK)
	})

	t.Run("Post_Delete_Authorization", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		owner := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "delete_owner", "delete_owner@example.com")
		otherUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "delete_other", "delete_other@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, owner.User.ID, "Delete Post", "https://example.com/delete")

		request := makeDeleteRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d", post.ID), otherUser, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

		assertErrorResponse(t, request, http.StatusForbidden)

		request = makeDeleteRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d", post.ID), owner, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

		assertStatus(t, request, http.StatusOK)
	})

	t.Run("User_Profile_Access_Authorization", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		firstUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "profile_user1", "profile_user1@example.com")
		secondUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "profile_user2", "profile_user2@example.com")

		request := makeAuthenticatedGetRequest(t, ctx.Router, fmt.Sprintf("/api/users/%d", firstUser.User.ID), secondUser, map[string]string{"id": fmt.Sprintf("%d", firstUser.User.ID)})

		response := assertJSONResponse(t, request, http.StatusOK)
		if data, ok := getDataFromResponse(response); ok {
			if userData, ok := data["user"].(map[string]any); ok {
				if id, ok := userData["id"].(float64); ok && uint(id) != firstUser.User.ID {
					t.Errorf("Expected user ID %d, got %.0f", firstUser.User.ID, id)
				}
			}
		}
	})

	t.Run("User_Settings_Authorization", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "settings_auth_user", "settings_auth@example.com")
		otherUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "settings_auth_other", "settings_auth_other@example.com")

		request := makePutRequest(t, ctx.Router, "/api/auth/email", map[string]any{"email": "newemail@example.com"}, otherUser, nil)

		response := assertJSONResponse(t, request, http.StatusOK)
		if response == nil {
			return
		}
		if data, ok := getDataFromResponse(response); ok {
			if userData, ok := data["user"].(map[string]any); ok {
				if email, ok := userData["email"].(string); ok && email == "newemail@example.com" {
					if id, ok := userData["id"].(float64); ok && uint(id) != otherUser.User.ID {
						t.Error("Expected email update to affect the authenticated user, not another user")
					}
				}
			}
		}

		request = makePutRequest(t, ctx.Router, "/api/auth/email", map[string]any{"email": "anothernewemail@example.com"}, user, nil)

		assertStatus(t, request, http.StatusOK)
	})

	t.Run("Vote_Authorization", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "vote_auth_user", "vote_auth@example.com")
		postOwner := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "vote_auth_owner", "vote_auth_owner@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, postOwner.User.ID, "Vote Auth Post", "https://example.com/vote-auth")

		request := makePostRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d/vote", post.ID), map[string]any{"type": "up"}, user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

		assertStatus(t, request, http.StatusOK)

		request = makePostRequestWithJSON(t, ctx.Router, fmt.Sprintf("/api/posts/%d/vote", post.ID), map[string]any{"type": "up"})

		assertErrorResponse(t, request, http.StatusUnauthorized)
	})

	t.Run("Protected_Endpoint_Without_Auth", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		request := makePostRequestWithJSON(t, ctx.Router, "/api/posts", map[string]any{})

		assertErrorResponse(t, request, http.StatusUnauthorized)
	})

	t.Run("Protected_Endpoint_With_Invalid_Token", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		request := makeRequest(t, ctx.Router, "POST", "/api/posts", []byte("{}"), map[string]string{"Content-Type": "application/json", "Authorization": "Bearer invalid-token"})

		assertErrorResponse(t, request, http.StatusUnauthorized)
	})

	t.Run("User_List_Authorization", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "list_auth_user", "list_auth@example.com")

		request := makeAuthenticatedGetRequest(t, ctx.Router, "/api/users", user, nil)

		assertStatus(t, request, http.StatusOK)

		request = makeGetRequest(t, ctx.Router, "/api/users")

		assertErrorResponse(t, request, http.StatusUnauthorized)
	})

	t.Run("Refresh_Token_Authorization", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "refresh_auth_user", "refresh_auth@example.com")

		loginResult, err := ctx.AuthService.Login("refresh_auth_user", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		request := makePostRequestWithJSON(t, ctx.Router, "/api/auth/refresh", map[string]any{"refresh_token": loginResult.RefreshToken})

		response := assertJSONResponse(t, request, http.StatusOK)
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

		request = makePostRequestWithJSON(t, ctx.Router, "/api/auth/refresh", map[string]any{"refresh_token": "invalid-refresh-token"})

		assertErrorResponse(t, request, http.StatusUnauthorized)
	})
}
