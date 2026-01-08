package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"goyco/internal/services"
	"goyco/internal/testutils"
)

func TestIntegration_CompleteAPIEndpoints(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("Auth_Logout_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "logout_user", "logout@example.com")

		request := makePostRequest(t, ctx.Router, "/api/auth/logout", map[string]any{}, user, nil)
		assertStatus(t, request, http.StatusOK)
	})

	t.Run("Auth_Revoke_Token_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "revoke_user", "revoke@example.com")

		loginResult, err := ctx.AuthService.Login("revoke_user", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		request := makePostRequest(t, ctx.Router, "/api/auth/revoke", map[string]any{"refresh_token": loginResult.RefreshToken}, user, nil)
		assertStatus(t, request, http.StatusOK)
	})

	t.Run("Auth_Revoke_All_Tokens_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "revoke_all_user", "revoke_all@example.com")

		request := makePostRequest(t, ctx.Router, "/api/auth/revoke-all", map[string]any{}, user, nil)
		assertStatus(t, request, http.StatusOK)
	})

	t.Run("Auth_Resend_Verification_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		request := makePostRequestWithJSON(t, ctx.Router, "/api/auth/resend-verification", map[string]any{"email": "resend@example.com"})
		assertStatusRange(t, request, http.StatusOK, http.StatusNotFound)
	})

	t.Run("Auth_Confirm_Email_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "confirm_api_user", "confirm_api@example.com")

		token := ctx.Suite.EmailSender.VerificationToken()
		if token == "" {
			token = "test-token"
		}

		request := makeGetRequest(t, ctx.Router, "/api/auth/confirm?token="+url.QueryEscape(token))
		assertStatusRange(t, request, http.StatusOK, http.StatusBadRequest)
	})

	t.Run("Auth_Update_Email_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "update_email_api_user", "update_email_api@example.com")

		request := makePutRequest(t, ctx.Router, "/api/auth/email", map[string]any{"email": "newemail@example.com"}, user, nil)

		response := assertJSONResponse(t, request, http.StatusOK)
		if data, ok := getDataFromResponse(response); ok {
			if email, ok := data["email"].(string); ok && email != "newemail@example.com" {
				t.Errorf("Expected email to be updated, got %s", email)
			}
		}
	})

	t.Run("Auth_Update_Username_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "update_username_api_user", "update_username_api@example.com")

		request := makePutRequest(t, ctx.Router, "/api/auth/username", map[string]any{"username": "new_username"}, user, nil)

		response := assertJSONResponse(t, request, http.StatusOK)
		if data, ok := getDataFromResponse(response); ok {
			if username, ok := data["username"].(string); ok && username != "new_username" {
				t.Errorf("Expected username to be updated, got %s", username)
			}
		}
	})

	t.Run("Users_List_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "users_list_user", "users_list@example.com")

		request := makeAuthenticatedGetRequest(t, ctx.Router, "/api/users", user, nil)

		response := assertJSONResponse(t, request, http.StatusOK)
		if data, ok := getDataFromResponse(response); ok {
			if _, exists := data["users"]; !exists {
				t.Error("Expected users in response")
			}
		}
	})

	t.Run("Users_Get_By_ID_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "users_get_user", "users_get@example.com")

		request := makeAuthenticatedGetRequest(t, ctx.Router, fmt.Sprintf("/api/users/%d", user.User.ID), user, map[string]string{"id": fmt.Sprintf("%d", user.User.ID)})

		response := assertJSONResponse(t, request, http.StatusOK)
		if data, ok := getDataFromResponse(response); ok {
			if userData, ok := data["user"].(map[string]any); ok {
				if id, ok := userData["id"].(float64); ok && uint(id) != user.User.ID {
					t.Errorf("Expected user ID %d, got %.0f", user.User.ID, id)
				}
			}
		}
	})

	t.Run("Users_Get_Posts_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "users_posts_user", "users_posts@example.com")

		testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "User Posts Test", "https://example.com/user-posts")

		request := makeAuthenticatedGetRequest(t, ctx.Router, fmt.Sprintf("/api/users/%d/posts", user.User.ID), user, map[string]string{"id": fmt.Sprintf("%d", user.User.ID)})

		response := assertJSONResponse(t, request, http.StatusOK)
		if data, ok := getDataFromResponse(response); ok {
			if posts, ok := data["posts"].([]any); ok {
				if len(posts) == 0 {
					t.Error("Expected at least one post in response")
				}
			} else {
				t.Error("Expected posts array in response")
			}
		}
	})

	t.Run("Users_Create_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "users_create_admin", "users_create_admin@example.com")

		request := makePostRequest(t, ctx.Router, "/api/users", map[string]any{
			"username": "created_user",
			"email":    "created@example.com",
			"password": "SecurePass123!",
		}, user, nil)

		response := assertJSONResponse(t, request, http.StatusCreated)
		if data, ok := getDataFromResponse(response); ok {
			if _, exists := data["user"]; !exists {
				t.Error("Expected user in response")
			}
		}
	})

	t.Run("Posts_Update_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "posts_update_user", "posts_update@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Update Test Post", "https://example.com/update-test")

		request := makePutRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d", post.ID), map[string]any{
			"title":   "Updated Title",
			"content": "Updated content",
		}, user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

		response := assertJSONResponse(t, request, http.StatusOK)
		if data, ok := getDataFromResponse(response); ok {
			if postData, ok := data["post"].(map[string]any); ok {
				if title, ok := postData["title"].(string); ok && title != "Updated Title" {
					t.Errorf("Expected title 'Updated Title', got '%s'", title)
				}
			}
		}
	})

	t.Run("Posts_Delete_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "posts_delete_user", "posts_delete@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Delete Test Post", "https://example.com/delete-test")

		request := makeDeleteRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d", post.ID), user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		assertStatus(t, request, http.StatusOK)

		getRequest := makeGetRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d", post.ID))
		assertStatus(t, getRequest, http.StatusNotFound)
	})

	t.Run("Votes_Get_All_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "votes_get_all_user", "votes_get_all@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Votes Test Post", "https://example.com/votes-test")
		makePostRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d/vote", post.ID), map[string]any{"type": "up"}, user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		request := makeAuthenticatedGetRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d/votes", post.ID), user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

		response := assertJSONResponse(t, request, http.StatusOK)
		if data, ok := getDataFromResponse(response); ok {
			if votes, ok := data["votes"].([]any); ok {
				if len(votes) == 0 {
					t.Error("Expected at least one vote in response")
				}
			} else {
				t.Error("Expected votes array in response")
			}
		}
	})

	t.Run("Votes_Remove_Endpoint", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "votes_remove_user", "votes_remove@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Vote Remove Test", "https://example.com/vote-remove")

		makePostRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d/vote", post.ID), map[string]any{"type": "up"}, user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

		request := makeDeleteRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d/vote", post.ID), user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		assertStatus(t, request, http.StatusOK)
	})

	t.Run("API_Info_Endpoint", func(t *testing.T) {
		request := makeGetRequest(t, ctx.Router, "/api")

		response := assertJSONResponse(t, request, http.StatusOK)
		if data, ok := getDataFromResponse(response); ok {
			if _, exists := data["endpoints"]; !exists {
				t.Error("Expected endpoints in API info")
			}
		}
	})

	t.Run("Swagger_Documentation_Endpoint", func(t *testing.T) {
		request := makeGetRequest(t, ctx.Router, "/swagger/index.html")
		assertStatusRange(t, request, http.StatusOK, http.StatusNotFound)
	})

	t.Run("Search_Endpoint_Edge_Cases", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, uniqueTestUsername(t, "search_edge"), uniqueTestEmail(t, "search_edge"))

		testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Searchable Post One", "https://example.com/one")
		testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Searchable Post Two", "https://example.com/two")
		testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Different Content", "https://example.com/three")

		t.Run("Empty_Search_Results", func(t *testing.T) {
			request := makeGetRequest(t, ctx.Router, "/api/posts/search?q=nonexistentterm12345")

			response := assertJSONResponse(t, request, http.StatusOK)
			if data, ok := getDataFromResponse(response); ok {
				if posts, ok := data["posts"].([]any); ok {
					if len(posts) != 0 {
						t.Errorf("Expected empty search results, got %d posts", len(posts))
					}
				}
				if count, ok := data["count"].(float64); ok && count != 0 {
					t.Errorf("Expected count 0, got %.0f", count)
				}
			}
		})

		t.Run("Search_With_Pagination", func(t *testing.T) {
			request := makeGetRequest(t, ctx.Router, "/api/posts/search?q=Searchable&limit=1&offset=0")

			response := assertJSONResponse(t, request, http.StatusOK)
			var firstPostID any
			if data, ok := getDataFromResponse(response); ok {
				if posts, ok := data["posts"].([]any); ok {
					if len(posts) > 1 {
						t.Errorf("Expected at most 1 post with limit=1, got %d", len(posts))
					}
					if len(posts) > 0 {
						if post, ok := posts[0].(map[string]any); ok {
							firstPostID = post["id"]
						}
					}
				}
				if limit, ok := data["limit"].(float64); ok && limit != 1 {
					t.Errorf("Expected limit 1 in response, got %.0f", limit)
				}
				if offset, ok := data["offset"].(float64); ok && offset != 0 {
					t.Errorf("Expected offset 0 in response, got %.0f", offset)
				}
			}

			secondRequest := makeGetRequest(t, ctx.Router, "/api/posts/search?q=Searchable&limit=1&offset=1")

			secondResponse := assertJSONResponse(t, secondRequest, http.StatusOK)
			if data, ok := getDataFromResponse(secondResponse); ok {
				if posts, ok := data["posts"].([]any); ok {
					if len(posts) > 1 {
						t.Errorf("Expected at most 1 post with limit=1 and offset=1, got %d", len(posts))
					}
					if len(posts) > 0 && firstPostID != nil {
						if post, ok := posts[0].(map[string]any); ok {
							if post["id"] == firstPostID {
								t.Error("Expected different post with offset=1, got same post as offset=0")
							}
						}
					}
				}
			}
		})

		t.Run("Search_With_Special_Characters", func(t *testing.T) {
			specialQueries := []string{
				"Searchable%20Post",
				"Searchable'Post",
				"Searchable\"Post",
				"Searchable;Post",
				"Searchable--Post",
			}

			for _, query := range specialQueries {
				request := makeGetRequest(t, ctx.Router, "/api/posts/search?q="+url.QueryEscape(query))
				assertStatusRange(t, request, http.StatusOK, http.StatusBadRequest)
			}
		})

		t.Run("Search_Empty_Query", func(t *testing.T) {
			request := makeGetRequest(t, ctx.Router, "/api/posts/search?q=")

			response := assertJSONResponse(t, request, http.StatusOK)
			if data, ok := getDataFromResponse(response); ok {
				if posts, ok := data["posts"].([]any); ok {
					if len(posts) != 0 {
						t.Errorf("Expected empty results for empty query, got %d posts", len(posts))
					}
				}
				if count, ok := data["count"].(float64); ok && count != 0 {
					t.Errorf("Expected count 0 for empty query, got %.0f", count)
				}
			}
		})

		t.Run("Search_With_Very_Long_Query", func(t *testing.T) {
			longQuery := strings.Repeat("a", 1000)
			request := makeGetRequest(t, ctx.Router, "/api/posts/search?q="+url.QueryEscape(longQuery))
			assertStatusRange(t, request, http.StatusOK, http.StatusBadRequest)
		})

		t.Run("Search_Case_Insensitive", func(t *testing.T) {
			request := makeGetRequest(t, ctx.Router, "/api/posts/search?q=SEARCHABLE")

			response := assertJSONResponse(t, request, http.StatusOK)
			if data, ok := getDataFromResponse(response); ok {
				if posts, ok := data["posts"].([]any); ok {
					if len(posts) == 0 {
						t.Error("Expected case-insensitive search to find posts")
					}
				}
			}
		})
	})

	t.Run("Title_Fetch_Endpoint_Edge_Cases", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		t.Run("Missing_URL_Parameter", func(t *testing.T) {
			request := makeGetRequest(t, ctx.Router, "/api/posts/title")
			assertErrorResponse(t, request, http.StatusBadRequest)
		})

		t.Run("Empty_URL_Parameter", func(t *testing.T) {
			request := makeGetRequest(t, ctx.Router, "/api/posts/title?url=")
			assertErrorResponse(t, request, http.StatusBadRequest)
		})

		t.Run("Invalid_URL_Format", func(t *testing.T) {
			invalidURLs := []string{
				"not-a-url",
				"://invalid",
				"http://",
				"https://",
			}

			for _, invalidURL := range invalidURLs {
				ctx.Suite.TitleFetcher.SetError(services.ErrUnsupportedScheme)
				request := makeGetRequest(t, ctx.Router, "/api/posts/title?url="+url.QueryEscape(invalidURL))
				assertErrorResponse(t, request, http.StatusBadRequest)
			}
		})

		t.Run("Unsupported_URL_Schemes", func(t *testing.T) {
			unsupportedSchemes := []string{
				"ftp://example.com",
				"file:///etc/passwd",
				"javascript:alert(1)",
				"data:text/html,<script>alert(1)</script>",
			}

			for _, schemeURL := range unsupportedSchemes {
				request := makeGetRequest(t, ctx.Router, "/api/posts/title?url="+url.QueryEscape(schemeURL))
				assertErrorResponse(t, request, http.StatusBadRequest)
			}
		})

		t.Run("SSRF_Protection_Localhost", func(t *testing.T) {
			ssrfURLs := []string{
				"http://localhost",
				"http://127.0.0.1",
				"http://127.0.0.1:8080",
				"http://[::1]",
				"http://0.0.0.0",
			}

			for _, ssrfURL := range ssrfURLs {
				ctx.Suite.TitleFetcher.SetError(services.ErrSSRFBlocked)
				request := makeGetRequest(t, ctx.Router, "/api/posts/title?url="+url.QueryEscape(ssrfURL))
				assertStatusRange(t, request, http.StatusBadRequest, http.StatusBadGateway)
			}
		})

		t.Run("SSRF_Protection_Private_IPs", func(t *testing.T) {
			privateIPs := []string{
				"http://192.168.1.1",
				"http://10.0.0.1",
				"http://172.16.0.1",
			}

			for _, privateIP := range privateIPs {
				ctx.Suite.TitleFetcher.SetError(services.ErrSSRFBlocked)
				request := makeGetRequest(t, ctx.Router, "/api/posts/title?url="+url.QueryEscape(privateIP))
				assertStatusRange(t, request, http.StatusBadRequest, http.StatusBadGateway)
			}
		})

		t.Run("Title_Fetch_Error_Handling", func(t *testing.T) {
			ctx.Suite.TitleFetcher.SetError(services.ErrTitleNotFound)

			request := makeGetRequest(t, ctx.Router, "/api/posts/title?url=https://example.com/notitle")
			assertErrorResponse(t, request, http.StatusBadRequest)
		})

		t.Run("Valid_URL_Success", func(t *testing.T) {
			ctx.Suite.TitleFetcher.SetTitle("Valid Title")

			request := makeGetRequest(t, ctx.Router, "/api/posts/title?url=https://example.com/valid")

			response := assertJSONResponse(t, request, http.StatusOK)
			if data, ok := getDataFromResponse(response); ok {
				if title, ok := data["title"].(string); ok {
					if title != "Valid Title" {
						t.Errorf("Expected title 'Valid Title', got '%s'", title)
					}
				} else {
					t.Error("Expected title in response data")
				}
			}
		})
	})

	t.Run("Get_User_Vote_Edge_Cases", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, uniqueTestUsername(t, "vote_edge"), uniqueTestEmail(t, "vote_edge"))
		secondUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, uniqueTestUsername(t, "vote_edge2"), uniqueTestEmail(t, "vote_edge2"))

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Vote Edge Test Post", "https://example.com/vote-edge")

		t.Run("Get_Vote_When_User_Has_Voted", func(t *testing.T) {
			voteRequest := makePostRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d/vote", post.ID), map[string]any{"type": "up"}, user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
			assertStatus(t, voteRequest, http.StatusOK)

			request := makeAuthenticatedGetRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d/vote", post.ID), user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

			response := assertJSONResponse(t, request, http.StatusOK)
			if data, ok := getDataFromResponse(response); ok {
				if hasVote, ok := data["has_vote"].(bool); !ok || !hasVote {
					t.Error("Expected has_vote to be true when user has voted")
				}
				if vote, ok := data["vote"]; !ok || vote == nil {
					t.Error("Expected vote object when user has voted")
				}
			}
		})

		t.Run("Get_Vote_When_User_Has_Not_Voted", func(t *testing.T) {
			request := makeAuthenticatedGetRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d/vote", post.ID), secondUser, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

			response := assertJSONResponse(t, request, http.StatusOK)
			if data, ok := getDataFromResponse(response); ok {
				if hasVote, ok := data["has_vote"].(bool); ok {
					if hasVote {
						t.Error("Expected has_vote to be false when user has not voted")
					}
				} else {
					t.Error("Expected has_vote field in response")
				}
			}
		})

		t.Run("Get_Vote_Invalid_Post_ID", func(t *testing.T) {
			request := makeAuthenticatedGetRequest(t, ctx.Router, "/api/posts/999999/vote", user, map[string]string{"id": "999999"})

			if request.Code != http.StatusOK && request.Code != http.StatusNotFound {
				t.Errorf("Expected status 200 or 404 for invalid post ID, got %d", request.Code)
			}
		})

		t.Run("Get_Vote_Unauthenticated", func(t *testing.T) {
			request := makeGetRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d/vote", post.ID))
			assertErrorResponse(t, request, http.StatusUnauthorized)
		})

		t.Run("Get_Vote_Response_Structure", func(t *testing.T) {
			request := makeAuthenticatedGetRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d/vote", post.ID), user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

			response := assertJSONResponse(t, request, http.StatusOK)
			if success, ok := response["success"].(bool); !ok || !success {
				t.Error("Expected success field to be true")
			}
			if data, ok := getDataFromResponse(response); ok {
				if _, exists := data["has_vote"]; !exists {
					t.Error("Expected has_vote field in response data")
				}
				if _, exists := data["is_anonymous"]; !exists {
					t.Error("Expected is_anonymous field in response data")
				}
			} else {
				t.Error("Expected data field in response")
			}
		})
	})

	t.Run("Refresh_Token_Edge_Cases", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		refreshUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, uniqueTestUsername(t, "refresh_edge"), uniqueTestEmail(t, "refresh_edge"))

		t.Run("Refresh_With_Expired_Token", func(t *testing.T) {
			loginResult, err := ctx.AuthService.Login(refreshUser.User.Username, "SecurePass123!")
			if err != nil {
				t.Fatalf("Failed to login: %v", err)
			}

			refreshToken, err := ctx.Suite.RefreshTokenRepo.GetByTokenHash(testutils.HashVerificationToken(loginResult.RefreshToken))
			if err != nil {
				t.Fatalf("Failed to get refresh token: %v", err)
			}

			refreshToken.ExpiresAt = time.Now().Add(-1 * time.Hour)
			if err := ctx.Suite.DB.Model(refreshToken).Update("expires_at", refreshToken.ExpiresAt).Error; err != nil {
				t.Fatalf("Failed to expire refresh token: %v", err)
			}

			request := makePostRequestWithJSON(t, ctx.Router, "/api/auth/refresh", map[string]any{"refresh_token": loginResult.RefreshToken})
			assertErrorResponse(t, request, http.StatusUnauthorized)
		})

		t.Run("Refresh_With_Revoked_Token", func(t *testing.T) {
			loginResult, err := ctx.AuthService.Login(refreshUser.User.Username, "SecurePass123!")
			if err != nil {
				t.Fatalf("Failed to login: %v", err)
			}

			if err := ctx.AuthService.RevokeRefreshToken(loginResult.RefreshToken); err != nil {
				t.Fatalf("Failed to revoke refresh token: %v", err)
			}

			request := makePostRequestWithJSON(t, ctx.Router, "/api/auth/refresh", map[string]any{"refresh_token": loginResult.RefreshToken})
			assertErrorResponse(t, request, http.StatusUnauthorized)
		})

		t.Run("Refresh_With_Empty_Token", func(t *testing.T) {
			request := makePostRequestWithJSON(t, ctx.Router, "/api/auth/refresh", map[string]any{"refresh_token": ""})
			assertErrorResponse(t, request, http.StatusBadRequest)
		})

		t.Run("Refresh_With_Missing_Token_Field", func(t *testing.T) {
			request := makePostRequestWithJSON(t, ctx.Router, "/api/auth/refresh", map[string]any{})
			assertErrorResponse(t, request, http.StatusBadRequest)
		})

		t.Run("Refresh_Token_Rotation", func(t *testing.T) {
			loginResult, err := ctx.AuthService.Login(refreshUser.User.Username, "SecurePass123!")
			if err != nil {
				t.Fatalf("Failed to login: %v", err)
			}

			originalRefreshToken := loginResult.RefreshToken

			request := makePostRequestWithJSON(t, ctx.Router, "/api/auth/refresh", map[string]any{"refresh_token": originalRefreshToken})

			response := assertJSONResponse(t, request, http.StatusOK)
			if data, ok := getDataFromResponse(response); ok {
				newAccessToken, _ := data["access_token"].(string)
				if newAccessToken == "" {
					t.Error("Expected new access token in refresh response")
				}

				newRefreshToken, _ := data["refresh_token"].(string)
				if newRefreshToken == "" {
					t.Error("Expected new refresh token in refresh response")
				}
				if newRefreshToken == originalRefreshToken {
					t.Error("Expected refresh token to rotate")
				}
			}

			request = makePostRequestWithJSON(t, ctx.Router, "/api/auth/refresh", map[string]any{"refresh_token": originalRefreshToken})
			assertErrorResponse(t, request, http.StatusUnauthorized)
		})

		t.Run("Refresh_After_Account_Lock", func(t *testing.T) {
			lockedUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, uniqueTestUsername(t, "refresh_lock"), uniqueTestEmail(t, "refresh_lock"))

			loginResult, err := ctx.AuthService.Login(lockedUser.User.Username, "SecurePass123!")
			if err != nil {
				t.Fatalf("Failed to login: %v", err)
			}

			lockedUser.User.Locked = true
			if err := ctx.Suite.UserRepo.Update(lockedUser.User); err != nil {
				t.Fatalf("Failed to lock user: %v", err)
			}

			request := makePostRequestWithJSON(t, ctx.Router, "/api/auth/refresh", map[string]any{"refresh_token": loginResult.RefreshToken})

			assertStatusRange(t, request, http.StatusUnauthorized, http.StatusForbidden)
		})

		t.Run("Refresh_With_Invalid_Token_Format", func(t *testing.T) {
			request := makePostRequestWithJSON(t, ctx.Router, "/api/auth/refresh", map[string]any{"refresh_token": "invalid-token-format-12345"})
			assertErrorResponse(t, request, http.StatusUnauthorized)
		})
	})

	t.Run("Pagination_Edge_Cases", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		paginationUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, uniqueTestUsername(t, "pagination_edge"), uniqueTestEmail(t, "pagination_edge"))

		for i := 0; i < 5; i++ {
			testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, paginationUser.User.ID, fmt.Sprintf("Pagination Post %d", i), fmt.Sprintf("https://example.com/pag%d", i))
		}

		t.Run("Negative_Limit", func(t *testing.T) {
			request := makeGetRequest(t, ctx.Router, "/api/posts?limit=-1")
			assertStatusRange(t, request, http.StatusOK, http.StatusBadRequest)
		})

		t.Run("Negative_Offset", func(t *testing.T) {
			request := makeGetRequest(t, ctx.Router, "/api/posts?offset=-1")
			assertStatusRange(t, request, http.StatusOK, http.StatusBadRequest)
		})

		t.Run("Very_Large_Limit", func(t *testing.T) {
			request := makeGetRequest(t, ctx.Router, "/api/posts?limit=10000")
			assertStatusRange(t, request, http.StatusOK, http.StatusBadRequest)
		})

		t.Run("Very_Large_Offset", func(t *testing.T) {
			request := makeGetRequest(t, ctx.Router, "/api/posts?offset=10000")

			response := assertJSONResponse(t, request, http.StatusOK)
			if data, ok := getDataFromResponse(response); ok {
				if posts, ok := data["posts"].([]any); ok {
					if len(posts) > 0 {
						t.Logf("Large offset returned %d posts (may be expected)", len(posts))
					}
				}
			}
		})

		t.Run("Invalid_Pagination_Parameters", func(t *testing.T) {
			invalidParams := []string{
				"limit=abc",
				"offset=xyz",
				"limit=",
				"offset=",
			}

			for _, param := range invalidParams {
				request := makeGetRequest(t, ctx.Router, "/api/posts?"+param)
				assertStatus(t, request, http.StatusOK)
			}
		})
	})
}
