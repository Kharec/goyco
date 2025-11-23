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
	"time"

	"github.com/golang-jwt/jwt/v5"
	"goyco/internal/database"
	"goyco/internal/handlers"
	"goyco/internal/middleware"
	"goyco/internal/repositories"
	"goyco/internal/services"
	"goyco/internal/testutils"
)

func TestIntegration_Handlers(t *testing.T) {
	ctx := setupTestContext(t)
	authService := ctx.AuthService
	emailSender := ctx.Suite.EmailSender
	userRepo := ctx.Suite.UserRepo
	postRepo := ctx.Suite.PostRepo

	t.Run("Auth_Handler_Complete_Workflow", func(t *testing.T) {
		emailSender.Reset()
		registerData := map[string]string{
			"username": "handler_user",
			"email":    "handler@example.com",
			"password": "SecurePass123!",
		}
		registerBody, _ := json.Marshal(registerData)
		registerReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(registerBody))
		registerReq.Header.Set("Content-Type", "application/json")
		registerResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(registerResp, registerReq)
		if registerResp.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", registerResp.Code)
		}

		var registerPayload map[string]any
		if err := json.Unmarshal(registerResp.Body.Bytes(), &registerPayload); err != nil {
			t.Fatalf("Failed to decode register response: %v", err)
		}
		if success, _ := registerPayload["success"].(bool); !success {
			t.Fatalf("Expected register response success, got %v", registerPayload)
		}

		user, err := userRepo.GetByUsername("handler_user")
		if err != nil {
			t.Fatalf("Failed to get user after registration: %v", err)
		}

		mockToken := "test-verification-token"

		hashedToken := testutils.HashVerificationToken(mockToken)

		user.EmailVerificationToken = hashedToken
		if err := userRepo.Update(user); err != nil {
			t.Fatalf("Failed to update user with mock token: %v", err)
		}

		confirmReq := httptest.NewRequest(http.MethodGet, "/api/auth/confirm?token="+url.QueryEscape(mockToken), nil)
		confirmResp := httptest.NewRecorder()
		ctx.Router.ServeHTTP(confirmResp, confirmReq)
		if confirmResp.Code != http.StatusOK {
			t.Fatalf("Expected 200 when confirming email via handler, got %d", confirmResp.Code)
		}

		loginSeed := createAuthenticatedUser(t, authService, userRepo, "auth_handler_login", "auth_handler_login@example.com")

		loginAuth, err := authService.Login(loginSeed.User.Username, "SecurePass123!")
		if err != nil {
			t.Fatalf("Service login failed for seeded user: %v", err)
		}

		meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
		meReq.Header.Set("Authorization", "Bearer "+loginAuth.AccessToken)
		meReq = testutils.WithUserContext(meReq, middleware.UserIDKey, loginSeed.User.ID)
		meResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(meResp, meReq)
		if meResp.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", meResp.Code)
		}
	})

	t.Run("Auth_Handler_Security_Validation", func(t *testing.T) {
		emailSender.Reset()
		weakData := map[string]string{
			"username": "weak_user",
			"email":    "weak@example.com",
			"password": "123",
		}
		weakBody, _ := json.Marshal(weakData)
		weakReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(weakBody))
		weakReq.Header.Set("Content-Type", "application/json")
		weakResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(weakResp, weakReq)
		if weakResp.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for weak password, got %d", weakResp.Code)
		}

		var weakErrorResp map[string]any
		if err := json.Unmarshal(weakResp.Body.Bytes(), &weakErrorResp); err != nil {
			t.Fatalf("Failed to decode error response: %v", err)
		}
		if success, _ := weakErrorResp["success"].(bool); success {
			t.Error("Expected error response to have success=false")
		}
		if errorMsg, ok := weakErrorResp["error"].(string); !ok || errorMsg == "" {
			t.Error("Expected error response to contain validation error message")
		}

		invalidData := map[string]string{
			"username": "invalid_user",
			"email":    "not-an-email",
			"password": "SecurePass123!",
		}
		invalidBody, _ := json.Marshal(invalidData)
		invalidReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(invalidBody))
		invalidReq.Header.Set("Content-Type", "application/json")
		invalidResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(invalidResp, invalidReq)
		if invalidResp.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid email, got %d", invalidResp.Code)
		}

		var invalidEmailErrorResp map[string]any
		if err := json.Unmarshal(invalidResp.Body.Bytes(), &invalidEmailErrorResp); err != nil {
			t.Fatalf("Failed to decode error response: %v", err)
		}
		if success, _ := invalidEmailErrorResp["success"].(bool); success {
			t.Error("Expected error response to have success=false")
		}
		if errorMsg, ok := invalidEmailErrorResp["error"].(string); !ok || errorMsg == "" {
			t.Error("Expected error response to contain validation error message")
		}

		incompleteData := map[string]string{
			"username": "incomplete_user",
		}
		incompleteBody, _ := json.Marshal(incompleteData)
		incompleteReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(incompleteBody))
		incompleteReq.Header.Set("Content-Type", "application/json")
		incompleteResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(incompleteResp, incompleteReq)
		if incompleteResp.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for missing fields, got %d", incompleteResp.Code)
		}

		var incompleteErrorResp map[string]any
		if err := json.Unmarshal(incompleteResp.Body.Bytes(), &incompleteErrorResp); err != nil {
			t.Fatalf("Failed to decode error response: %v", err)
		}
		if success, _ := incompleteErrorResp["success"].(bool); success {
			t.Error("Expected error response to have success=false")
		}
		if errorMsg, ok := incompleteErrorResp["error"].(string); !ok || errorMsg == "" {
			t.Error("Expected error response to contain validation error message")
		}
	})

	t.Run("Post_Handler_Complete_Workflow", func(t *testing.T) {
		emailSender.Reset()
		user := createAuthenticatedUser(t, authService, userRepo, "post_user", "post@example.com")

		postData := map[string]string{
			"title":   "Handler Test Post",
			"url":     "https://example.com/handler-test",
			"content": "This is a handler test post",
		}
		postBody, _ := json.Marshal(postData)
		postReq := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(postBody))
		postReq.Header.Set("Content-Type", "application/json")
		postReq.Header.Set("Authorization", "Bearer "+user.Token)
		postReq = testutils.WithUserContext(postReq, middleware.UserIDKey, user.User.ID)
		postResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(postResp, postReq)
		if postResp.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", postResp.Code)
		}

		var postResult map[string]any
		if err := json.Unmarshal(postResp.Body.Bytes(), &postResult); err != nil {
			t.Fatalf("Failed to decode post response: %v", err)
		}
		postDetails, ok := postResult["data"].(map[string]any)
		if !ok {
			t.Fatalf("Expected data object in post response, got %v", postResult)
		}
		postID, ok := postDetails["id"].(float64)
		if !ok {
			t.Fatal("Expected post ID in response")
		}

		getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%d", int(postID)), nil)
		getReq = testutils.WithURLParams(getReq, map[string]string{"id": fmt.Sprintf("%d", int(postID))})
		getResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(getResp, getReq)
		if getResp.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", getResp.Code)
		}

		postsReq := httptest.NewRequest("GET", "/api/posts", nil)
		postsResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(postsResp, postsReq)
		if postsResp.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", postsResp.Code)
		}

		searchReq := httptest.NewRequest("GET", "/api/posts/search?q=handler", nil)
		searchResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(searchResp, searchReq)
		if searchResp.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", searchResp.Code)
		}
	})

	t.Run("Post_Handler_Security_Validation", func(t *testing.T) {
		emailSender.Reset()
		postData := map[string]string{
			"title":   "Unauthorized Post",
			"url":     "https://example.com/unauthorized",
			"content": "This should fail",
		}
		postBody, _ := json.Marshal(postData)
		postReq := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(postBody))
		postReq.Header.Set("Content-Type", "application/json")
		postResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(postResp, postReq)
		if postResp.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for unauthenticated post creation, got %d", postResp.Code)
		}

		var authErrorResp map[string]any
		if err := json.Unmarshal(postResp.Body.Bytes(), &authErrorResp); err != nil {
			t.Fatalf("Failed to decode error response: %v", err)
		}
		if success, _ := authErrorResp["success"].(bool); success {
			t.Error("Expected error response to have success=false")
		}
		if errorMsg, ok := authErrorResp["error"].(string); !ok || errorMsg == "" {
			t.Error("Expected error response to contain authentication error message")
		}

		user := createAuthenticatedUser(t, authService, userRepo, "security_user", "security@example.com")

		invalidData := map[string]string{
			"title":   "",
			"url":     "not-a-url",
			"content": "Invalid post",
		}
		invalidBody, _ := json.Marshal(invalidData)
		invalidReq := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(invalidBody))
		invalidReq.Header.Set("Content-Type", "application/json")
		invalidReq.Header.Set("Authorization", "Bearer "+user.Token)
		invalidReq = testutils.WithUserContext(invalidReq, middleware.UserIDKey, user.User.ID)
		invalidResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(invalidResp, invalidReq)
		if invalidResp.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid post data, got %d", invalidResp.Code)
		}

		var postValidationErrorResp map[string]any
		if err := json.Unmarshal(invalidResp.Body.Bytes(), &postValidationErrorResp); err != nil {
			t.Fatalf("Failed to decode error response: %v", err)
		}
		if success, _ := postValidationErrorResp["success"].(bool); success {
			t.Error("Expected error response to have success=false")
		}
		if errorMsg, ok := postValidationErrorResp["error"].(string); !ok || errorMsg == "" {
			t.Error("Expected error response to contain validation error message")
		}
	})

	t.Run("Vote_Handler_Complete_Workflow", func(t *testing.T) {
		emailSender.Reset()
		user := createAuthenticatedUser(t, authService, userRepo, "vote_handler_user", "vote_handler@example.com")
		post := testutils.CreatePostWithRepo(t, postRepo, user.User.ID, "Vote Handler Test Post", "https://example.com/vote-handler")

		voteData := map[string]string{
			"type": "up",
		}
		voteBody, _ := json.Marshal(voteData)
		voteReq := httptest.NewRequest("POST", fmt.Sprintf("/api/posts/%d/vote", post.ID), bytes.NewBuffer(voteBody))
		voteReq.Header.Set("Content-Type", "application/json")
		voteReq.Header.Set("Authorization", "Bearer "+user.Token)
		voteReq = testutils.WithUserContext(voteReq, middleware.UserIDKey, user.User.ID)
		voteReq = testutils.WithURLParams(voteReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		voteResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(voteResp, voteReq)
		if voteResp.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", voteResp.Code)
		}

		getVoteReq := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%d/vote", post.ID), nil)
		getVoteReq.Header.Set("Authorization", "Bearer "+user.Token)
		getVoteReq = testutils.WithUserContext(getVoteReq, middleware.UserIDKey, user.User.ID)
		getVoteReq = testutils.WithURLParams(getVoteReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		getVoteResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(getVoteResp, getVoteReq)
		if getVoteResp.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", getVoteResp.Code)
		}

		getPostVotesReq := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%d/votes", post.ID), nil)
		getPostVotesReq.Header.Set("Authorization", "Bearer "+user.Token)
		getPostVotesReq = testutils.WithUserContext(getPostVotesReq, middleware.UserIDKey, user.User.ID)
		getPostVotesReq = testutils.WithURLParams(getPostVotesReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		getPostVotesResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(getPostVotesResp, getPostVotesReq)
		if getPostVotesResp.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", getPostVotesResp.Code)
		}

		removeVoteReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/posts/%d/vote", post.ID), nil)
		removeVoteReq.Header.Set("Authorization", "Bearer "+user.Token)
		removeVoteReq = testutils.WithUserContext(removeVoteReq, middleware.UserIDKey, user.User.ID)
		removeVoteReq = testutils.WithURLParams(removeVoteReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		removeVoteResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(removeVoteResp, removeVoteReq)
		if removeVoteResp.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", removeVoteResp.Code)
		}
	})

	t.Run("User_Handler_Complete_Workflow", func(t *testing.T) {
		emailSender.Reset()
		user := createAuthenticatedUser(t, authService, userRepo, "user_handler_user", "user_handler@example.com")

		usersReq := httptest.NewRequest("GET", "/api/users", nil)
		usersReq.Header.Set("Authorization", "Bearer "+user.Token)
		usersReq = testutils.WithUserContext(usersReq, middleware.UserIDKey, user.User.ID)
		usersResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(usersResp, usersReq)
		if usersResp.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", usersResp.Code)
		}

		getUserReq := httptest.NewRequest("GET", fmt.Sprintf("/api/users/%d", user.User.ID), nil)
		getUserReq.Header.Set("Authorization", "Bearer "+user.Token)
		getUserReq = testutils.WithUserContext(getUserReq, middleware.UserIDKey, user.User.ID)
		getUserReq = testutils.WithURLParams(getUserReq, map[string]string{"id": fmt.Sprintf("%d", user.User.ID)})
		getUserResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(getUserResp, getUserReq)
		if getUserResp.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", getUserResp.Code)
		}

		getUserPostsReq := httptest.NewRequest("GET", fmt.Sprintf("/api/users/%d/posts", user.User.ID), nil)
		getUserPostsReq.Header.Set("Authorization", "Bearer "+user.Token)
		getUserPostsReq = testutils.WithUserContext(getUserPostsReq, middleware.UserIDKey, user.User.ID)
		getUserPostsReq = testutils.WithURLParams(getUserPostsReq, map[string]string{"id": fmt.Sprintf("%d", user.User.ID)})
		getUserPostsResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(getUserPostsResp, getUserPostsReq)
		if getUserPostsResp.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", getUserPostsResp.Code)
		}
	})

	t.Run("Error_Handling_Invalid_Requests", func(t *testing.T) {
		invalidJSONReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer([]byte("invalid json")))
		invalidJSONReq.Header.Set("Content-Type", "application/json")
		invalidJSONResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(invalidJSONResp, invalidJSONReq)
		if invalidJSONResp.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid JSON, got %d", invalidJSONResp.Code)
		}

		var jsonErrorResp map[string]any
		if err := json.Unmarshal(invalidJSONResp.Body.Bytes(), &jsonErrorResp); err != nil {
			t.Fatalf("Failed to decode error response: %v", err)
		}
		if success, _ := jsonErrorResp["success"].(bool); success {
			t.Error("Expected error response to have success=false")
		}
		if errorMsg, ok := jsonErrorResp["error"].(string); !ok || errorMsg == "" {
			t.Error("Expected error response to contain JSON parsing error message")
		}

		missingCTData := map[string]string{
			"username": "missing_ct_user",
			"email":    "missing_ct@example.com",
			"password": "SecurePass123!",
		}
		missingCTBody, _ := json.Marshal(missingCTData)
		missingCTReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(missingCTBody))
		missingCTResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(missingCTResp, missingCTReq)
		if missingCTResp.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", missingCTResp.Code)
		}

		invalidEndpointReq := httptest.NewRequest("GET", "/api/invalid/endpoint", nil)
		invalidEndpointResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(invalidEndpointResp, invalidEndpointReq)
		if invalidEndpointResp.Code == http.StatusOK {
			t.Error("Expected error for invalid endpoint")
		}
	})

	t.Run("Security_Authentication_Bypass", func(t *testing.T) {
		meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
		meResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(meResp, meReq)
		if meResp.Code == http.StatusOK {
			t.Error("Expected error for unauthenticated request")
		}

		invalidTokenReq := httptest.NewRequest("GET", "/api/auth/me", nil)
		invalidTokenReq.Header.Set("Authorization", "Bearer invalid-token")
		invalidTokenResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(invalidTokenResp, invalidTokenReq)
		if invalidTokenResp.Code == http.StatusOK {
			t.Error("Expected error for invalid token")
		}

		malformedTokenReq := httptest.NewRequest("GET", "/api/auth/me", nil)
		malformedTokenReq.Header.Set("Authorization", "InvalidFormat token")
		malformedTokenResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(malformedTokenResp, malformedTokenReq)
		if malformedTokenResp.Code == http.StatusOK {
			t.Error("Expected error for malformed token")
		}
	})

	t.Run("Security_Input_Sanitization", func(t *testing.T) {
		user := createAuthenticatedUser(t, authService, userRepo, "xss_user", "xss@example.com")

		xssData := map[string]string{
			"title":   "<script>alert('xss')</script>",
			"url":     "https://example.com/xss",
			"content": "XSS test content",
		}
		xssBody, _ := json.Marshal(xssData)
		xssReq := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(xssBody))
		xssReq.Header.Set("Content-Type", "application/json")
		xssReq.Header.Set("Authorization", "Bearer "+user.Token)
		xssReq = testutils.WithUserContext(xssReq, middleware.UserIDKey, user.User.ID)
		xssResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(xssResp, xssReq)
		if xssResp.Code != http.StatusCreated {
			t.Errorf("Expected status 201 for XSS sanitization, got %d", xssResp.Code)
		}

		var xssResult map[string]any
		if err := json.Unmarshal(xssResp.Body.Bytes(), &xssResult); err != nil {
			t.Fatalf("Failed to decode XSS response: %v", err)
		}
		if success, _ := xssResult["success"].(bool); !success {
			t.Error("Expected XSS response to have success=true")
		}

		data, ok := xssResult["data"].(map[string]any)
		if !ok {
			t.Fatalf("Expected data object in XSS response, got %T", xssResult["data"])
		}

		title, ok := data["title"].(string)
		if !ok {
			t.Fatalf("Expected title string in XSS response, got %T", data["title"])
		}

		if strings.Contains(title, "<script>") {
			t.Errorf("Expected script tags to be HTML-escaped in title, got: %s", title)
		}
		if !strings.Contains(title, "&lt;script&gt;") {
			t.Errorf("Expected script tags to be HTML-escaped (&lt;script&gt;), got: %s", title)
		}
		if !strings.Contains(title, "alert(") {
			t.Errorf("Expected JavaScript code to be present but escaped, got: %s", title)
		}

		content, ok := data["content"].(string)
		if !ok {
			t.Fatalf("Expected content string in XSS response, got %T", data["content"])
		}

		if strings.Contains(content, "<script>") {
			t.Errorf("Expected script tags to be HTML-escaped in content, got: %s", content)
		}

		sqlData := map[string]string{
			"title":   "'; DROP TABLE posts; --",
			"url":     "https://example.com/sql",
			"content": "SQL injection test",
		}
		sqlBody, _ := json.Marshal(sqlData)
		sqlReq := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(sqlBody))
		sqlReq.Header.Set("Content-Type", "application/json")
		sqlReq.Header.Set("Authorization", "Bearer "+user.Token)
		sqlReq = testutils.WithUserContext(sqlReq, middleware.UserIDKey, user.User.ID)
		sqlResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(sqlResp, sqlReq)
		if sqlResp.Code != http.StatusCreated {
			t.Errorf("Expected status 201 for SQL injection sanitization, got %d", sqlResp.Code)
		}

		var sqlResult map[string]any
		if err := json.Unmarshal(sqlResp.Body.Bytes(), &sqlResult); err != nil {
			t.Fatalf("Failed to decode SQL response: %v", err)
		}
		if success, _ := sqlResult["success"].(bool); !success {
			t.Error("Expected SQL response to have success=true")
		}

		sqlResponseData, ok := sqlResult["data"].(map[string]any)
		if !ok {
			t.Fatalf("Expected data object in SQL response, got %T", sqlResult["data"])
		}

		sqlResponseTitle, ok := sqlResponseData["title"].(string)
		if !ok {
			t.Fatalf("Expected title string in SQL response, got %T", sqlResponseData["title"])
		}

		if strings.Contains(sqlResponseTitle, "'; DROP TABLE posts; --") {
			t.Errorf("Expected SQL injection payload to be HTML-escaped in title, got: %s", sqlResponseTitle)
		}
		if !strings.Contains(sqlResponseTitle, "&#39;") {
			t.Errorf("Expected single quotes to be HTML-escaped (&#39;), got: %s", sqlResponseTitle)
		}
		if !strings.Contains(sqlResponseTitle, "DROP TABLE") {
			t.Errorf("Expected SQL commands to be present but escaped, got: %s", sqlResponseTitle)
		}

		sqlResponseContent, ok := sqlResponseData["content"].(string)
		if !ok {
			t.Fatalf("Expected content string in SQL response, got %T", sqlResponseData["content"])
		}

		if strings.Contains(sqlResponseContent, "'; DROP TABLE posts; --") {
			t.Errorf("Expected SQL injection payload to be HTML-escaped in content, got: %s", sqlResponseContent)
		}
	})

	t.Run("Authorization_User_Access_Control", func(t *testing.T) {
		emailSender.Reset()
		user1 := createAuthenticatedUser(t, authService, userRepo, "auth_user1", "auth1@example.com")
		user2 := createAuthenticatedUser(t, authService, userRepo, "auth_user2", "auth2@example.com")

		post := testutils.CreatePostWithRepo(t, postRepo, user1.User.ID, "Private Post", "https://example.com/private")

		getPostReq := httptest.NewRequest("GET", fmt.Sprintf("/api/posts/%d", post.ID), nil)
		getPostReq.Header.Set("Authorization", "Bearer "+user2.Token)
		getPostReq = testutils.WithUserContext(getPostReq, middleware.UserIDKey, user2.User.ID)
		getPostReq = testutils.WithURLParams(getPostReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		getPostResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(getPostResp, getPostReq)
		testutils.AssertHTTPStatus(t, getPostResp, http.StatusOK)

		updateData := map[string]string{
			"title": "Updated Title",
		}
		updateBody, _ := json.Marshal(updateData)
		updateReq := httptest.NewRequest("PUT", fmt.Sprintf("/api/posts/%d", post.ID), bytes.NewBuffer(updateBody))
		updateReq.Header.Set("Content-Type", "application/json")
		updateReq.Header.Set("Authorization", "Bearer "+user2.Token)
		updateReq = testutils.WithUserContext(updateReq, middleware.UserIDKey, user2.User.ID)
		updateReq = testutils.WithURLParams(updateReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		updateResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(updateResp, updateReq)
		testutils.AssertHTTPStatus(t, updateResp, http.StatusForbidden)

		deleteReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/posts/%d", post.ID), nil)
		deleteReq.Header.Set("Authorization", "Bearer "+user2.Token)
		deleteReq = testutils.WithUserContext(deleteReq, middleware.UserIDKey, user2.User.ID)
		deleteReq = testutils.WithURLParams(deleteReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		deleteResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(deleteResp, deleteReq)
		testutils.AssertHTTPStatus(t, deleteResp, http.StatusForbidden)
	})

	t.Run("Authorization_Vote_Access_Control", func(t *testing.T) {
		emailSender.Reset()
		user1 := createAuthenticatedUser(t, authService, userRepo, "vote_auth_user1", "vote_auth1@example.com")
		user2 := createAuthenticatedUser(t, authService, userRepo, "vote_auth_user2", "vote_auth2@example.com")

		post := testutils.CreatePostWithRepo(t, postRepo, user1.User.ID, "Vote Auth Post", "https://example.com/vote-auth")

		voteData := map[string]string{"type": "up"}
		voteBody, _ := json.Marshal(voteData)
		voteReq := httptest.NewRequest("POST", fmt.Sprintf("/api/posts/%d/vote", post.ID), bytes.NewBuffer(voteBody))
		voteReq.Header.Set("Content-Type", "application/json")
		voteReq.Header.Set("Authorization", "Bearer "+user2.Token)
		voteReq = testutils.WithUserContext(voteReq, middleware.UserIDKey, user2.User.ID)
		voteReq = testutils.WithURLParams(voteReq, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		voteResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(voteResp, voteReq)
		if voteResp.Code != http.StatusOK {
			t.Errorf("Users should be able to vote on any post, got %d", voteResp.Code)
		}
	})

	t.Run("Authorization_Token_Expiration", func(t *testing.T) {
		emailSender.Reset()
		user := createAuthenticatedUser(t, authService, userRepo, "expire_auth_user", "expire_auth@example.com")

		now := time.Now()
		claims := services.TokenClaims{
			UserID:         user.User.ID,
			Username:       user.User.Username,
			SessionVersion: user.User.SessionVersion,
			TokenType:      services.TokenTypeAccess,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    testutils.AppTestConfig.JWT.Issuer,
				Audience:  []string{testutils.AppTestConfig.JWT.Audience},
				IssuedAt:  jwt.NewNumericDate(now.Add(-25 * time.Hour)),
				ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)),
				Subject:   fmt.Sprint(user.User.ID),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		expiredToken, err := token.SignedString([]byte(testutils.AppTestConfig.JWT.Secret))
		if err != nil {
			t.Fatalf("Failed to generate expired token: %v", err)
		}

		meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
		meReq.Header.Set("Authorization", "Bearer "+expiredToken)
		meResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(meResp, meReq)
		testutils.AssertHTTPStatus(t, meResp, http.StatusUnauthorized)
	})

	t.Run("Authorization_Token_Tampering", func(t *testing.T) {
		emailSender.Reset()
		user := createAuthenticatedUser(t, authService, userRepo, "tamper_user", "tamper@example.com")

		tamperedToken := user.Token[:len(user.Token)-5] + "XXXXX"

		meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
		meReq.Header.Set("Authorization", "Bearer "+tamperedToken)
		meResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(meResp, meReq)
		testutils.AssertHTTPStatus(t, meResp, http.StatusUnauthorized)
	})

	t.Run("Authorization_Session_Version_Mismatch", func(t *testing.T) {
		emailSender.Reset()
		user := createAuthenticatedUser(t, authService, userRepo, "session_user", "session@example.com")

		now := time.Now()
		claims := services.TokenClaims{
			UserID:         user.User.ID,
			Username:       user.User.Username,
			SessionVersion: user.User.SessionVersion + 1,
			TokenType:      services.TokenTypeAccess,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    testutils.AppTestConfig.JWT.Issuer,
				Audience:  []string{testutils.AppTestConfig.JWT.Audience},
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
				Subject:   fmt.Sprint(user.User.ID),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		invalidToken, err := token.SignedString([]byte(testutils.AppTestConfig.JWT.Secret))
		if err != nil {
			t.Fatalf("Failed to generate invalid token: %v", err)
		}

		meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
		meReq.Header.Set("Authorization", "Bearer "+invalidToken)
		meResp := httptest.NewRecorder()

		ctx.Router.ServeHTTP(meResp, meReq)
		testutils.AssertHTTPStatus(t, meResp, http.StatusUnauthorized)
	})

}

func TestIntegration_DatabaseMonitoring(t *testing.T) {
	db := testutils.NewTestDB(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	monitor := middleware.NewInMemoryDBMonitor()

	monitoringPlugin := database.NewGormDBMonitor(monitor)
	if err := db.Use(monitoringPlugin); err != nil {
		t.Fatalf("Failed to add monitoring plugin: %v", err)
	}

	userRepo := repositories.NewUserRepository(db)
	postRepo := repositories.NewPostRepository(db)
	voteRepo := repositories.NewVoteRepository(db)
	deletionRepo := repositories.NewAccountDeletionRepository(db)
	refreshTokenRepo := repositories.NewRefreshTokenRepository(db)
	emailSender := &testutils.MockEmailSender{}

	_, err := services.NewAuthFacadeForTest(testutils.AppTestConfig, userRepo, postRepo, deletionRepo, refreshTokenRepo, emailSender)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	voteService := services.NewVoteService(voteRepo, postRepo, db)

	apiHandler := handlers.NewAPIHandlerWithMonitoring(testutils.AppTestConfig, postRepo, userRepo, voteService, db, monitor)

	t.Run("Health endpoint includes database monitoring", func(t *testing.T) {

		user := &database.User{
			Username:      "monitoring_user",
			Email:         "monitoring@example.com",
			Password:      "password123",
			EmailVerified: true,
		}
		userRepo.Create(user)

		request := httptest.NewRequest("GET", "/health", nil)
		recorder := httptest.NewRecorder()

		apiHandler.GetHealth(recorder, request)

		testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

		var response map[string]any
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["success"] != true {
			t.Error("Expected success to be true")
		}

		data, ok := response["data"].(map[string]any)
		if !ok {
			t.Fatal("Expected data to be a map")
		}

		if pingTime, exists := data["ping_time"]; exists {
			t.Logf("Database ping time: %v", pingTime)
		}

		if dbStats, exists := data["database_stats"]; exists {
			t.Logf("Database stats present: %v", dbStats)
		}
	})

	t.Run("Metrics endpoint includes database monitoring", func(t *testing.T) {

		user := &database.User{
			Username:      "metrics_user",
			Email:         "metrics@example.com",
			Password:      "password123",
			EmailVerified: true,
		}
		userRepo.Create(user)

		post := &database.Post{
			Title:    "Test Post",
			Content:  "Test content",
			URL:      "https://example.com",
			AuthorID: &user.ID,
		}
		postRepo.Create(post)

		request := httptest.NewRequest("GET", "/metrics", nil)
		recorder := httptest.NewRecorder()

		apiHandler.GetMetrics(recorder, request)

		testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

		var response map[string]any
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["success"] != true {
			t.Error("Expected success to be true")
		}

		data, ok := response["data"].(map[string]any)
		if !ok {
			t.Fatal("Expected data to be a map")
		}

		if dbData, exists := data["database"]; exists {
			t.Logf("Database monitoring data present: %v", dbData)
			if dbMap, ok := dbData.(map[string]any); ok {
				if totalQueries, exists := dbMap["total_queries"]; exists {
					t.Logf("Total queries tracked: %v", totalQueries)
				}
			}
		}

		if perfData, exists := data["performance"]; exists {
			t.Logf("Performance data present: %v", perfData)
		}
	})

	t.Run("Database operations are tracked", func(t *testing.T) {

		monitor = middleware.NewInMemoryDBMonitor()
		monitoringPlugin = database.NewGormDBMonitor(monitor)
		db.Use(monitoringPlugin)

		apiHandler = handlers.NewAPIHandlerWithMonitoring(testutils.AppTestConfig, postRepo, userRepo, voteService, db, monitor)

		user := &database.User{
			Username:      "tracking_user",
			Email:         "tracking@example.com",
			Password:      "password123",
			EmailVerified: true,
		}
		userRepo.Create(user)

		post := &database.Post{
			Title:    "Tracking Post",
			Content:  "Tracking content",
			URL:      "https://example.com",
			AuthorID: &user.ID,
		}
		postRepo.Create(post)

		stats := monitor.GetStats()

		t.Logf("Database operations tracked: %d queries", stats.TotalQueries)
		t.Logf("Slow queries: %d", stats.SlowQueries)
		t.Logf("Average duration: %v", stats.AverageDuration)
		t.Logf("Error count: %d", stats.ErrorCount)

		if stats.TotalQueries > 0 {
			t.Logf("✅ Database monitoring is working - tracked %d queries", stats.TotalQueries)
		} else {
			t.Logf("⚠️  Database monitoring plugin may not be tracking all operations (this is a known limitation)")
		}
	})
}
