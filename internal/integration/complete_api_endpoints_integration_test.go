package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"goyco/internal/middleware"
	"goyco/internal/testutils"
)

func TestIntegration_CompleteAPIEndpoints(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("Auth_Logout_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "logout_user", "logout@example.com")

		reqBody := map[string]string{}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/logout", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)
	})

	t.Run("Auth_Revoke_Token_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "revoke_user", "revoke@example.com")

		loginResult, err := ctx.AuthService.Login("revoke_user", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		reqBody := map[string]string{
			"refresh_token": loginResult.RefreshToken,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/revoke", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)
	})

	t.Run("Auth_Revoke_All_Tokens_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "revoke_all_user", "revoke_all@example.com")

		reqBody := map[string]string{}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/revoke-all", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)
	})

	t.Run("Auth_Resend_Verification_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		reqBody := map[string]string{
			"email": "resend@example.com",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/resend-verification", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertStatusRange(t, rec, http.StatusOK, http.StatusNotFound)
	})

	t.Run("Auth_Confirm_Email_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "confirm_api_user", "confirm_api@example.com")

		token := ctx.Suite.EmailSender.VerificationToken()
		if token == "" {
			token = "test-token"
		}

		req := httptest.NewRequest("GET", "/api/auth/confirm?token="+url.QueryEscape(token), nil)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertStatusRange(t, rec, http.StatusOK, http.StatusBadRequest)
	})

	t.Run("Auth_Update_Email_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "update_email_api_user", "update_email_api@example.com")

		reqBody := map[string]string{
			"email": "newemail@example.com",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PUT", "/api/auth/email", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
		if response != nil {
			if data, ok := response["data"].(map[string]any); ok {
				if email, ok := data["email"].(string); ok && email != "newemail@example.com" {
					t.Errorf("Expected email to be updated, got %s", email)
				}
			}
		}
	})

	t.Run("Auth_Update_Username_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "update_username_api_user", "update_username_api@example.com")

		reqBody := map[string]string{
			"username": "new_username",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PUT", "/api/auth/username", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
		if response != nil {
			if data, ok := response["data"].(map[string]any); ok {
				if username, ok := data["username"].(string); ok && username != "new_username" {
					t.Errorf("Expected username to be updated, got %s", username)
				}
			}
		}
	})

	t.Run("Users_List_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "users_list_user", "users_list@example.com")

		req := httptest.NewRequest("GET", "/api/users", nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
		if response != nil {
			if data, ok := response["data"].(map[string]any); ok {
				if _, exists := data["users"]; !exists {
					t.Error("Expected users in response")
				}
			}
		}
	})

	t.Run("Users_Get_By_ID_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "users_get_user", "users_get@example.com")

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/users/%d", user.User.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", user.User.ID)})
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
		if response != nil {
			if data, ok := response["data"].(map[string]any); ok {
				if userData, ok := data["user"].(map[string]any); ok {
					if id, ok := userData["id"].(float64); ok && uint(id) != user.User.ID {
						t.Errorf("Expected user ID %d, got %.0f", user.User.ID, id)
					}
				}
			}
		}
	})

	t.Run("Users_Get_Posts_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "users_posts_user", "users_posts@example.com")

		testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "User Posts Test", "https://example.com/user-posts")

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/users/%d/posts", user.User.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", user.User.ID)})
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
		if response != nil {
			if data, ok := response["data"].(map[string]any); ok {
				if posts, ok := data["posts"].([]any); ok {
					if len(posts) == 0 {
						t.Error("Expected at least one post in response")
					}
				} else {
					t.Error("Expected posts array in response")
				}
			}
		}
	})

	t.Run("Users_Create_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "users_create_admin", "users_create_admin@example.com")

		reqBody := map[string]string{
			"username": "created_user",
			"email":    "created@example.com",
			"password": "SecurePass123!",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/users", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusCreated)
		if response != nil {
			if data, ok := response["data"].(map[string]any); ok {
				if _, exists := data["user"]; !exists {
					t.Error("Expected user in response")
				}
			}
		}
	})

	t.Run("Posts_Update_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "posts_update_user", "posts_update@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Update Test Post", "https://example.com/update-test")

		reqBody := map[string]string{
			"title":   "Updated Title",
			"content": "Updated content",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/posts/%d", post.ID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
		if response != nil {
			if data, ok := response["data"].(map[string]any); ok {
				if postData, ok := data["post"].(map[string]any); ok {
					if title, ok := postData["title"].(string); ok && title != "Updated Title" {
						t.Errorf("Expected title 'Updated Title', got '%s'", title)
					}
				}
			}
		}
	})

	t.Run("Posts_Delete_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "posts_delete_user", "posts_delete@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Delete Test Post", "https://example.com/delete-test")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/posts/%d", post.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)

		getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%d", post.ID), nil)
		getRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(getRec, getReq)
		assertStatus(t, getRec, http.StatusNotFound)
	})

	t.Run("Votes_Get_All_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "votes_get_all_user", "votes_get_all@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Votes Test Post", "https://example.com/votes-test")

		voteBody := map[string]string{"type": "up"}
		voteBodyBytes, _ := json.Marshal(voteBody)
		voteReq := httptest.NewRequest("POST", fmt.Sprintf("/api/posts/%d/vote", post.ID), bytes.NewBuffer(voteBodyBytes))
		voteReq.Header.Set("Content-Type", "application/json")
		voteReq.Header.Set("Authorization", "Bearer "+user.Token)
		voteReq = testutils.WithUserContext(voteReq, middleware.UserIDKey, user.User.ID)
		voteReq = testutils.WithURLParams(voteReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		voteRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(voteRec, voteReq)

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%d/votes", post.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
		if response != nil {
			if data, ok := response["data"].(map[string]any); ok {
				if votes, ok := data["votes"].([]any); ok {
					if len(votes) == 0 {
						t.Error("Expected at least one vote in response")
					}
				} else {
					t.Error("Expected votes array in response")
				}
			}
		}
	})

	t.Run("Votes_Remove_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "votes_remove_user", "votes_remove@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Vote Remove Test", "https://example.com/vote-remove")

		voteBody := map[string]string{"type": "up"}
		voteBodyBytes, _ := json.Marshal(voteBody)
		voteReq := httptest.NewRequest("POST", fmt.Sprintf("/api/posts/%d/vote", post.ID), bytes.NewBuffer(voteBodyBytes))
		voteReq.Header.Set("Content-Type", "application/json")
		voteReq.Header.Set("Authorization", "Bearer "+user.Token)
		voteReq = testutils.WithUserContext(voteReq, middleware.UserIDKey, user.User.ID)
		voteReq = testutils.WithURLParams(voteReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		voteRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(voteRec, voteReq)

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/posts/%d/vote", post.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)
	})

	t.Run("API_Info_Endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api", nil)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
		if response != nil {
			if data, ok := response["data"].(map[string]any); ok {
				if _, exists := data["endpoints"]; !exists {
					t.Error("Expected endpoints in API info")
				}
			}
		}
	})

	t.Run("Swagger_Documentation_Endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/swagger/index.html", nil)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertStatusRange(t, rec, http.StatusOK, http.StatusNotFound)
	})
}
