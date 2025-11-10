package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"goyco/internal/middleware"
	"goyco/internal/testutils"
)

func TestIntegration_EndToEndUserJourneys(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("Complete_Registration_To_Post_Creation_Journey", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		registerBody := map[string]string{
			"username": "journey_user",
			"email":    "journey@example.com",
			"password": "SecurePass123!",
		}
		body, _ := json.Marshal(registerBody)
		registerReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
		registerReq.Header.Set("Content-Type", "application/json")
		registerRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(registerRec, registerReq)

		assertStatus(t, registerRec, http.StatusCreated)

		verificationToken := ctx.Suite.EmailSender.VerificationToken()
		if verificationToken == "" {
			t.Fatal("Verification token not sent")
		}

		confirmReq := httptest.NewRequest("GET", "/api/auth/confirm?token="+url.QueryEscape(verificationToken), nil)
		confirmRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(confirmRec, confirmReq)

		assertStatus(t, confirmRec, http.StatusOK)

		loginBody := map[string]string{
			"username": "journey_user",
			"password": "SecurePass123!",
		}
		loginBodyBytes, _ := json.Marshal(loginBody)
		loginReq := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(loginBodyBytes))
		loginReq.Header.Set("Content-Type", "application/json")
		loginRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(loginRec, loginReq)

		loginResponse := assertJSONResponse(t, loginRec, http.StatusOK)
		if loginResponse == nil {
			return
		}

		data, ok := loginResponse["data"].(map[string]any)
		if !ok {
			t.Fatal("Login response missing data")
		}

		var token string
		if accessToken, ok := data["access_token"].(string); ok && accessToken != "" {
			token = accessToken
		} else if tokenVal, ok := data["token"].(string); ok && tokenVal != "" {
			token = tokenVal
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

		postBody := map[string]string{
			"title":   "Journey Test Post",
			"url":     "https://example.com/journey",
			"content": "Test content",
		}
		postBodyBytes, _ := json.Marshal(postBody)
		postReq := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(postBodyBytes))
		postReq.Header.Set("Content-Type", "application/json")
		postReq.Header.Set("Authorization", "Bearer "+token)
		postReq = testutils.WithUserContext(postReq, middleware.UserIDKey, uint(userID))
		postRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(postRec, postReq)

		postResponse := assertJSONResponse(t, postRec, http.StatusCreated)
		if postResponse == nil {
			return
		}

		postData, ok := postResponse["data"].(map[string]any)
		if !ok {
			t.Fatal("Post response missing data")
		}

		postID, ok := postData["id"].(float64)
		if !ok {
			t.Fatal("Post response missing id")
		}

		getPostReq := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%.0f", postID), nil)
		getPostRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(getPostRec, getPostReq)

		assertStatus(t, getPostRec, http.StatusOK)
	})

	t.Run("Complete_Password_Reset_Journey", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "reset_journey_user", "reset_journey@example.com")

		resetBody := map[string]string{
			"username_or_email": "reset_journey@example.com",
		}
		resetBodyBytes, _ := json.Marshal(resetBody)
		resetReq := httptest.NewRequest("POST", "/api/auth/forgot-password", bytes.NewBuffer(resetBodyBytes))
		resetReq.Header.Set("Content-Type", "application/json")
		resetRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(resetRec, resetReq)

		assertStatus(t, resetRec, http.StatusOK)

		resetToken := ctx.Suite.EmailSender.GetLastPasswordResetToken()
		if resetToken == "" {
			t.Fatal("Password reset token not sent")
		}

		newPasswordBody := map[string]string{
			"token":        resetToken,
			"new_password": "NewSecurePass123!",
		}
		newPasswordBodyBytes, _ := json.Marshal(newPasswordBody)
		newPasswordReq := httptest.NewRequest("POST", "/api/auth/reset-password", bytes.NewBuffer(newPasswordBodyBytes))
		newPasswordReq.Header.Set("Content-Type", "application/json")
		newPasswordRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(newPasswordRec, newPasswordReq)

		assertStatus(t, newPasswordRec, http.StatusOK)

		loginBody := map[string]string{
			"username": "reset_journey_user",
			"password": "NewSecurePass123!",
		}
		loginBodyBytes, _ := json.Marshal(loginBody)
		loginReq := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(loginBodyBytes))
		loginReq.Header.Set("Content-Type", "application/json")
		loginRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(loginRec, loginReq)

		assertStatus(t, loginRec, http.StatusOK)
	})

	t.Run("Complete_Vote_And_Unvote_Journey", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "vote_journey_user", "vote_journey@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Vote Journey Post", "https://example.com/vote-journey")

		voteBody := map[string]string{"type": "up"}
		voteBodyBytes, _ := json.Marshal(voteBody)
		voteReq := httptest.NewRequest("POST", fmt.Sprintf("/api/posts/%d/vote", post.ID), bytes.NewBuffer(voteBodyBytes))
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

		if data, ok := votesResponse["data"].(map[string]any); ok {
			if votes, ok := data["votes"].([]any); ok && len(votes) > 0 {
				unvoteReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/posts/%d/vote", post.ID), nil)
				unvoteReq.Header.Set("Authorization", "Bearer "+user.Token)
				unvoteReq = testutils.WithUserContext(unvoteReq, middleware.UserIDKey, user.User.ID)
				unvoteReq = testutils.WithURLParams(unvoteReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
				unvoteRec := httptest.NewRecorder()
				ctx.Router.ServeHTTP(unvoteRec, unvoteReq)

				assertStatus(t, unvoteRec, http.StatusOK)
			}
		}
	})

	t.Run("Complete_Page_Handler_Registration_Journey", func(t *testing.T) {
		pageCtx := setupPageHandlerTestContext(t)
		pageRouter := pageCtx.Router
		pageCtx.Suite.EmailSender.Reset()

		csrfToken := getCSRFToken(t, pageRouter, "/register")

		reqBody := url.Values{}
		reqBody.Set("username", "page_journey_user")
		reqBody.Set("email", "page_journey@example.com")
		reqBody.Set("password", "SecurePass123!")
		reqBody.Set("password_confirm", "SecurePass123!")
		reqBody.Set("csrf_token", csrfToken)

		req := httptest.NewRequest("POST", "/register", strings.NewReader(reqBody.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		rec := httptest.NewRecorder()
		pageRouter.ServeHTTP(rec, req)

		assertStatusRange(t, rec, http.StatusOK, http.StatusSeeOther)

		verificationToken := pageCtx.Suite.EmailSender.VerificationToken()
		if verificationToken == "" {
			t.Fatal("Verification token not sent")
		}

		confirmReq := httptest.NewRequest("GET", "/confirm?token="+url.QueryEscape(verificationToken), nil)
		confirmRec := httptest.NewRecorder()
		pageRouter.ServeHTTP(confirmRec, confirmReq)

		assertStatusRange(t, confirmRec, http.StatusOK, http.StatusSeeOther)

		loginCSRFToken := getCSRFToken(t, pageRouter, "/login")

		loginBody := url.Values{}
		loginBody.Set("username", "page_journey_user")
		loginBody.Set("password", "SecurePass123!")
		loginBody.Set("csrf_token", loginCSRFToken)

		loginReq := httptest.NewRequest("POST", "/login", strings.NewReader(loginBody.Encode()))
		loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		loginReq.AddCookie(&http.Cookie{Name: "csrf_token", Value: loginCSRFToken})
		loginRec := httptest.NewRecorder()
		pageRouter.ServeHTTP(loginRec, loginReq)

		assertStatus(t, loginRec, http.StatusSeeOther)

		loginCookies := loginRec.Result().Cookies()
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

		homeReq := httptest.NewRequest("GET", "/", nil)
		homeReq.AddCookie(&http.Cookie{Name: "auth_token", Value: authToken})
		homeRec := httptest.NewRecorder()
		pageRouter.ServeHTTP(homeRec, homeReq)

		assertStatus(t, homeRec, http.StatusOK)
	})

	t.Run("Complete_Post_Creation_And_Update_Journey", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "post_update_journey_user", "post_update_journey@example.com")

		postBody := map[string]string{
			"title":   "Original Title",
			"url":     "https://example.com/original",
			"content": "Original content",
		}
		postBodyBytes, _ := json.Marshal(postBody)
		postReq := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(postBodyBytes))
		postReq.Header.Set("Content-Type", "application/json")
		postReq.Header.Set("Authorization", "Bearer "+user.Token)
		postReq = testutils.WithUserContext(postReq, middleware.UserIDKey, user.User.ID)
		postRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(postRec, postReq)

		postResponse := assertJSONResponse(t, postRec, http.StatusCreated)
		if postResponse == nil {
			return
		}

		postData, ok := postResponse["data"].(map[string]any)
		if !ok {
			t.Fatal("Post response missing data")
		}

		postID, ok := postData["id"].(float64)
		if !ok {
			t.Fatal("Post response missing id")
		}

		updateBody := map[string]string{
			"title":   "Updated Title",
			"content": "Updated content",
		}
		updateBodyBytes, _ := json.Marshal(updateBody)
		updateReq := httptest.NewRequest("PUT", fmt.Sprintf("/api/posts/%.0f", postID), bytes.NewBuffer(updateBodyBytes))
		updateReq.Header.Set("Content-Type", "application/json")
		updateReq.Header.Set("Authorization", "Bearer "+user.Token)
		updateReq = testutils.WithUserContext(updateReq, middleware.UserIDKey, user.User.ID)
		updateReq = testutils.WithURLParams(updateReq, map[string]string{"id": fmt.Sprintf("%.0f", postID)})
		updateRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(updateRec, updateReq)

		updateResponse := assertJSONResponse(t, updateRec, http.StatusOK)
		if updateResponse == nil {
			return
		}

		getPostReq := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%.0f", postID), nil)
		getPostRec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(getPostRec, getPostReq)

		getPostResponse := assertJSONResponse(t, getPostRec, http.StatusOK)
		if getPostResponse == nil {
			return
		}

		if data, ok := getPostResponse["data"].(map[string]any); ok {
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
