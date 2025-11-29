package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"goyco/internal/middleware"
	"goyco/internal/testutils"
)

func TestIntegration_PageHandlerFormWorkflows(t *testing.T) {
	ctx := setupPageHandlerTestContext(t)
	router := ctx.Router
	authService := ctx.AuthService

	t.Run("Settings_Email_Update_Form", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, authService, ctx.Suite.UserRepo, "settings_email_user", "settings_email@example.com")

		getRequest := httptest.NewRequest("GET", "/settings", nil)
		getRequest.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		getRecorder := httptest.NewRecorder()
		router.ServeHTTP(getRecorder, getRequest)

		csrfToken := getCSRFToken(t, router, "/settings", &http.Cookie{Name: "auth_token", Value: user.Token})

		requestBody := url.Values{}
		requestBody.Set("email", "newemail@example.com")
		requestBody.Set("csrf_token", csrfToken)

		request := httptest.NewRequest("POST", "/settings/email", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		request.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		assertStatusRange(t, recorder, http.StatusOK, http.StatusSeeOther)
	})

	t.Run("Settings_Username_Update_Form", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, authService, ctx.Suite.UserRepo, "settings_username_user", "settings_username@example.com")

		csrfToken := getCSRFToken(t, router, "/settings", &http.Cookie{Name: "auth_token", Value: user.Token})

		requestBody := url.Values{}
		requestBody.Set("username", "new_username")
		requestBody.Set("csrf_token", csrfToken)

		request := httptest.NewRequest("POST", "/settings/username", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		request.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		assertStatusRange(t, recorder, http.StatusOK, http.StatusSeeOther)
	})

	t.Run("Settings_Password_Update_Form", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		freshCtx := setupPageHandlerTestContext(t)
		freshCtx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, freshCtx.AuthService, freshCtx.Suite.UserRepo, "settings_password_user", "settings_password@example.com")

		csrfToken := getCSRFToken(t, freshCtx.Router, "/settings", &http.Cookie{Name: "auth_token", Value: user.Token})

		requestBody := url.Values{}
		requestBody.Set("current_password", "SecurePass123!")
		requestBody.Set("new_password", "NewSecurePass123!")
		requestBody.Set("csrf_token", csrfToken)

		request := httptest.NewRequest("POST", "/settings/password", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		request.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		recorder := httptest.NewRecorder()

		freshCtx.Router.ServeHTTP(recorder, request)

		assertStatusRange(t, recorder, http.StatusOK, http.StatusSeeOther)
	})

	t.Run("Logout_Page_Handler", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		freshCtx := setupPageHandlerTestContext(t)
		freshCtx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, freshCtx.AuthService, freshCtx.Suite.UserRepo, "logout_page_user", "logout_page@example.com")

		csrfToken := getCSRFToken(t, freshCtx.Router, "/settings", &http.Cookie{Name: "auth_token", Value: user.Token})

		requestBody := url.Values{}
		requestBody.Set("csrf_token", csrfToken)

		request := httptest.NewRequest("POST", "/logout", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		request.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		recorder := httptest.NewRecorder()

		freshCtx.Router.ServeHTTP(recorder, request)

		assertStatus(t, recorder, http.StatusSeeOther)
		assertCookieCleared(t, recorder, "auth_token")
	})

	t.Run("Resend_Verification_Page_Handler", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		freshCtx := setupPageHandlerTestContext(t)
		freshCtx.Suite.EmailSender.Reset()

		csrfToken := getCSRFToken(t, freshCtx.Router, "/resend-verification")

		requestBody := url.Values{}
		requestBody.Set("email", "resend_page@example.com")
		requestBody.Set("csrf_token", csrfToken)

		request := httptest.NewRequest("POST", "/resend-verification", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		recorder := httptest.NewRecorder()

		freshCtx.Router.ServeHTTP(recorder, request)

		assertStatusRange(t, recorder, http.StatusOK, http.StatusSeeOther)
	})

	t.Run("Post_Vote_Page_Handler", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		freshCtx := setupPageHandlerTestContext(t)
		freshCtx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, freshCtx.AuthService, freshCtx.Suite.UserRepo, "vote_page_user", "vote_page@example.com")

		post := testutils.CreatePostWithRepo(t, freshCtx.Suite.PostRepo, user.User.ID, "Vote Page Test", "https://example.com/vote-page")

		getRequest := httptest.NewRequest("GET", fmt.Sprintf("/posts/%d", post.ID), nil)
		getRecorder := httptest.NewRecorder()
		freshCtx.Router.ServeHTTP(getRecorder, getRequest)

		csrfToken := getCSRFToken(t, freshCtx.Router, fmt.Sprintf("/posts/%d", post.ID))

		requestBody := url.Values{}
		requestBody.Set("action", "up")
		requestBody.Set("csrf_token", csrfToken)

		request := httptest.NewRequest("POST", fmt.Sprintf("/posts/%d/vote", post.ID), strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		request.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		request = testutils.WithURLParams(request, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		recorder := httptest.NewRecorder()

		freshCtx.Router.ServeHTTP(recorder, request)

		assertStatusRange(t, recorder, http.StatusOK, http.StatusSeeOther)
	})

	t.Run("Login_Page_Handler_Workflow", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		freshCtx := setupPageHandlerTestContext(t)
		freshCtx.Suite.EmailSender.Reset()
		createAuthenticatedUser(t, freshCtx.AuthService, freshCtx.Suite.UserRepo, "login_page_user", "login_page@example.com")

		csrfToken := getCSRFToken(t, freshCtx.Router, "/login")

		requestBody := url.Values{}
		requestBody.Set("username", "login_page_user")
		requestBody.Set("password", "SecurePass123!")
		requestBody.Set("csrf_token", csrfToken)

		request := httptest.NewRequest("POST", "/login", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		recorder := httptest.NewRecorder()

		freshCtx.Router.ServeHTTP(recorder, request)

		assertStatus(t, recorder, http.StatusSeeOther)
		assertCookie(t, recorder, "auth_token", "")
	})

	t.Run("Email_Confirmation_Page_Handler", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		ctx.Suite.EmailSender.Reset()
		createAuthenticatedUser(t, authService, ctx.Suite.UserRepo, "confirm_page_user", "confirm_page@example.com")

		token := ctx.Suite.EmailSender.VerificationToken()
		if token == "" {
			token = "test-token"
		}

		request := httptest.NewRequest("GET", "/confirm?token="+url.QueryEscape(token), nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		assertStatusRange(t, recorder, http.StatusOK, http.StatusSeeOther)
	})
}
