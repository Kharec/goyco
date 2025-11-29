package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"goyco/internal/database"
	"goyco/internal/testutils"
)

func TestIntegration_EndToEndUserJourneys(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("Complete_Registration_To_Post_Creation_Journey", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		registerRequest := makePostRequestWithJSON(t, ctx.Router, "/api/auth/register", map[string]any{
			"username": "journey_user",
			"email":    "journey@example.com",
			"password": "SecurePass123!",
		})

		assertStatus(t, registerRequest, http.StatusCreated)

		verificationToken := ctx.Suite.EmailSender.VerificationToken()
		if verificationToken == "" {
			t.Fatal("Verification token not sent")
		}

		confirmRequest := makeGetRequest(t, ctx.Router, "/api/auth/confirm?token="+url.QueryEscape(verificationToken))

		assertStatus(t, confirmRequest, http.StatusOK)

		loginRequest := makePostRequestWithJSON(t, ctx.Router, "/api/auth/login", map[string]any{
			"username": "journey_user",
			"password": "SecurePass123!",
		})

		loginResponse := assertJSONResponse(t, loginRequest, http.StatusOK)
		if loginResponse == nil {
			return
		}

		data, ok := getDataFromResponse(loginResponse)
		if !ok {
			t.Fatal("Login response missing data")
		}

		var token string
		if accessToken, ok := data["access_token"].(string); ok && accessToken != "" {
			token = accessToken
		} else if tokenValue, ok := data["token"].(string); ok && tokenValue != "" {
			token = tokenValue
		} else {
			t.Fatal("Login response missing access_token or token")
		}

		var userID uint
		if userData, ok := data["user"].(map[string]any); ok {
			if id, ok := userData["id"].(float64); ok {
				userID = uint(id)
			} else if id, ok := userData["ID"].(float64); ok {
				userID = uint(id)
			}
		}
		if userID == 0 {
			if id, ok := data["user_id"].(float64); ok {
				userID = uint(id)
			}
		}
		if userID == 0 {
			t.Fatalf("Login response missing user.id. Data: %+v", data)
		}

		postBodyBytes, _ := json.Marshal(map[string]any{
			"title":   "Journey Test Post",
			"url":     "https://example.com/journey",
			"content": "Test content",
		})
		postRequest := makeAuthenticatedRequest(t, ctx.Router, "POST", "/api/posts", postBodyBytes, &authenticatedUser{User: &database.User{ID: uint(userID)}, Token: token}, nil)

		postResponse := assertJSONResponse(t, postRequest, http.StatusCreated)
		if postResponse == nil {
			return
		}

		postData, ok := getDataFromResponse(postResponse)
		if !ok {
			t.Fatal("Post response missing data")
		}

		postID, ok := postData["id"].(float64)
		if !ok {
			t.Fatal("Post response missing id")
		}

		getPostRequest := makeGetRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%.0f", postID))

		assertStatus(t, getPostRequest, http.StatusOK)
	})

	t.Run("Complete_Password_Reset_Journey", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "reset_journey_user", "reset_journey@example.com")

		resetRequest := makePostRequestWithJSON(t, ctx.Router, "/api/auth/forgot-password", map[string]any{
			"username_or_email": "reset_journey@example.com",
		})

		assertStatus(t, resetRequest, http.StatusOK)

		resetToken := ctx.Suite.EmailSender.GetLastPasswordResetToken()
		if resetToken == "" {
			t.Fatal("Password reset token not sent")
		}

		newPasswordRequest := makePostRequestWithJSON(t, ctx.Router, "/api/auth/reset-password", map[string]any{
			"token":        resetToken,
			"new_password": "NewSecurePass123!",
		})

		assertStatus(t, newPasswordRequest, http.StatusOK)

		loginRequest := makePostRequestWithJSON(t, ctx.Router, "/api/auth/login", map[string]any{
			"username": "reset_journey_user",
			"password": "NewSecurePass123!",
		})

		assertStatus(t, loginRequest, http.StatusOK)
	})

	t.Run("Complete_Vote_And_Unvote_Journey", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "vote_journey_user", "vote_journey@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Vote Journey Post", "https://example.com/vote-journey")

		voteRequest := makePostRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d/vote", post.ID), map[string]any{"type": "up"}, user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		assertStatus(t, voteRequest, http.StatusOK)

		getVotesRequest := makeAuthenticatedGetRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d/votes", post.ID), user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

		votesResponse := assertJSONResponse(t, getVotesRequest, http.StatusOK)
		if votesResponse == nil {
			return
		}

		if data, ok := getDataFromResponse(votesResponse); ok {
			if votes, ok := data["votes"].([]any); ok && len(votes) > 0 {
				unvoteRequest := makeDeleteRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d/vote", post.ID), user, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

				assertStatus(t, unvoteRequest, http.StatusOK)
			}
		}
	})

	t.Run("Complete_Page_Handler_Registration_Journey", func(t *testing.T) {
		pageCtx := setupPageHandlerTestContext(t)
		pageRouter := pageCtx.Router
		pageCtx.Suite.EmailSender.Reset()

		csrfToken := getCSRFToken(t, pageRouter, "/register")

		requestBody := url.Values{}
		requestBody.Set("username", "page_journey_user")
		requestBody.Set("email", "page_journey@example.com")
		requestBody.Set("password", "SecurePass123!")
		requestBody.Set("password_confirm", "SecurePass123!")
		requestBody.Set("csrf_token", csrfToken)

		request := httptest.NewRequest("POST", "/register", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		recorder := httptest.NewRecorder()
		pageRouter.ServeHTTP(recorder, request)

		assertStatusRange(t, recorder, http.StatusOK, http.StatusSeeOther)

		verificationToken := pageCtx.Suite.EmailSender.VerificationToken()
		if verificationToken == "" {
			t.Fatal("Verification token not sent")
		}

		confirmRequest := httptest.NewRequest("GET", "/confirm?token="+url.QueryEscape(verificationToken), nil)
		confirmRecorder := httptest.NewRecorder()
		pageRouter.ServeHTTP(confirmRecorder, confirmRequest)

		assertStatusRange(t, confirmRecorder, http.StatusOK, http.StatusSeeOther)

		loginCSRFToken := getCSRFToken(t, pageRouter, "/login")

		loginBody := url.Values{}
		loginBody.Set("username", "page_journey_user")
		loginBody.Set("password", "SecurePass123!")
		loginBody.Set("csrf_token", loginCSRFToken)

		loginRequest := httptest.NewRequest("POST", "/login", strings.NewReader(loginBody.Encode()))
		loginRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		loginRequest.AddCookie(&http.Cookie{Name: "csrf_token", Value: loginCSRFToken})
		loginRecorder := httptest.NewRecorder()
		pageRouter.ServeHTTP(loginRecorder, loginRequest)

		assertStatus(t, loginRecorder, http.StatusSeeOther)

		loginCookies := loginRecorder.Result().Cookies()
		var authToken string
		for _, cookie := range loginCookies {
			if cookie.Name == "auth_token" {
				authToken = cookie.Value
				break
			}
		}

		if authToken == "" {
			t.Fatal("Auth token not set after login")
		}

		homeRequest := httptest.NewRequest("GET", "/", nil)
		homeRequest.AddCookie(&http.Cookie{Name: "auth_token", Value: authToken})
		homeRecorder := httptest.NewRecorder()
		pageRouter.ServeHTTP(homeRecorder, homeRequest)

		assertStatus(t, homeRecorder, http.StatusOK)
	})

	t.Run("Complete_Post_Creation_And_Update_Journey", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "post_update_journey_user", "post_update_journey@example.com")

		postBodyBytes, _ := json.Marshal(map[string]any{
			"title":   "Original Title",
			"url":     "https://example.com/original",
			"content": "Original content",
		})
		postRequest := makeAuthenticatedRequest(t, ctx.Router, "POST", "/api/posts", postBodyBytes, user, nil)

		postResponse := assertJSONResponse(t, postRequest, http.StatusCreated)
		if postResponse == nil {
			return
		}

		postData, ok := getDataFromResponse(postResponse)
		if !ok {
			t.Fatal("Post response missing data")
		}

		postID, ok := postData["id"].(float64)
		if !ok {
			t.Fatal("Post response missing id")
		}

		updateBodyBytes, _ := json.Marshal(map[string]any{
			"title":   "Updated Title",
			"content": "Updated content",
		})
		updateRequest := makeAuthenticatedRequest(t, ctx.Router, "PUT", fmt.Sprintf("/api/posts/%.0f", postID), updateBodyBytes, user, map[string]string{"id": fmt.Sprintf("%.0f", postID)})

		updateResponse := assertJSONResponse(t, updateRequest, http.StatusOK)
		if updateResponse == nil {
			return
		}

		getPostRequest := makeGetRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%.0f", postID))

		getPostResponse := assertJSONResponse(t, getPostRequest, http.StatusOK)
		if getPostResponse == nil {
			return
		}

		if data, ok := getDataFromResponse(getPostResponse); ok {
			if post, ok := data["post"].(map[string]any); ok {
				if title, ok := post["title"].(string); ok && title != "Updated Title" {
					t.Errorf("Post title not updated: expected 'Updated Title', got '%s'", title)
				}
				if content, ok := post["content"].(string); ok && content != "Updated content" {
					t.Errorf("Post content not updated: expected 'Updated content', got '%s'", content)
				}
			}
		}
	})
}
