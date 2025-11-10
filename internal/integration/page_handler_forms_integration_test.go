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

		getReq := httptest.NewRequest("GET", "/settings", nil)
		getReq.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)

		csrfToken := getCSRFToken(t, router, "/settings", &http.Cookie{Name: "auth_token", Value: user.Token})

		reqBody := url.Values{}
		reqBody.Set("email", "newemail@example.com")
		reqBody.Set("csrf_token", csrfToken)

		req := httptest.NewRequest("POST", "/settings/email", strings.NewReader(reqBody.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatusRange(t, rec, http.StatusOK, http.StatusSeeOther)
	})

	t.Run("Settings_Username_Update_Form", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, authService, ctx.Suite.UserRepo, "settings_username_user", "settings_username@example.com")

		csrfToken := getCSRFToken(t, router, "/settings", &http.Cookie{Name: "auth_token", Value: user.Token})

		reqBody := url.Values{}
		reqBody.Set("username", "new_username")
		reqBody.Set("csrf_token", csrfToken)

		req := httptest.NewRequest("POST", "/settings/username", strings.NewReader(reqBody.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatusRange(t, rec, http.StatusOK, http.StatusSeeOther)
	})

	t.Run("Settings_Password_Update_Form", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		freshCtx := setupPageHandlerTestContext(t)
		freshCtx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, freshCtx.AuthService, freshCtx.Suite.UserRepo, "settings_password_user", "settings_password@example.com")

		csrfToken := getCSRFToken(t, freshCtx.Router, "/settings", &http.Cookie{Name: "auth_token", Value: user.Token})

		reqBody := url.Values{}
		reqBody.Set("current_password", "SecurePass123!")
		reqBody.Set("new_password", "NewSecurePass123!")
		reqBody.Set("csrf_token", csrfToken)

		req := httptest.NewRequest("POST", "/settings/password", strings.NewReader(reqBody.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		rec := httptest.NewRecorder()

		freshCtx.Router.ServeHTTP(rec, req)

		assertStatusRange(t, rec, http.StatusOK, http.StatusSeeOther)
	})

	t.Run("Logout_Page_Handler", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		freshCtx := setupPageHandlerTestContext(t)
		freshCtx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, freshCtx.AuthService, freshCtx.Suite.UserRepo, "logout_page_user", "logout_page@example.com")

		csrfToken := getCSRFToken(t, freshCtx.Router, "/settings", &http.Cookie{Name: "auth_token", Value: user.Token})

		reqBody := url.Values{}
		reqBody.Set("csrf_token", csrfToken)

		req := httptest.NewRequest("POST", "/logout", strings.NewReader(reqBody.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		rec := httptest.NewRecorder()

		freshCtx.Router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusSeeOther)
		assertCookieCleared(t, rec, "auth_token")
	})

	t.Run("Resend_Verification_Page_Handler", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		freshCtx := setupPageHandlerTestContext(t)
		freshCtx.Suite.EmailSender.Reset()

		csrfToken := getCSRFToken(t, freshCtx.Router, "/resend-verification")

		reqBody := url.Values{}
		reqBody.Set("email", "resend_page@example.com")
		reqBody.Set("csrf_token", csrfToken)

		req := httptest.NewRequest("POST", "/resend-verification", strings.NewReader(reqBody.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		rec := httptest.NewRecorder()

		freshCtx.Router.ServeHTTP(rec, req)

		assertStatusRange(t, rec, http.StatusOK, http.StatusSeeOther)
	})

	t.Run("Post_Vote_Page_Handler", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		freshCtx := setupPageHandlerTestContext(t)
		freshCtx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, freshCtx.AuthService, freshCtx.Suite.UserRepo, "vote_page_user", "vote_page@example.com")

		post := testutils.CreatePostWithRepo(t, freshCtx.Suite.PostRepo, user.User.ID, "Vote Page Test", "https://example.com/vote-page")

		getReq := httptest.NewRequest("GET", fmt.Sprintf("/posts/%d", post.ID), nil)
		getRec := httptest.NewRecorder()
		freshCtx.Router.ServeHTTP(getRec, getReq)

		csrfToken := getCSRFToken(t, freshCtx.Router, fmt.Sprintf("/posts/%d", post.ID))

		reqBody := url.Values{}
		reqBody.Set("action", "up")
		reqBody.Set("csrf_token", csrfToken)

		req := httptest.NewRequest("POST", fmt.Sprintf("/posts/%d/vote", post.ID), strings.NewReader(reqBody.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		rec := httptest.NewRecorder()

		freshCtx.Router.ServeHTTP(rec, req)

		assertStatusRange(t, rec, http.StatusOK, http.StatusSeeOther)
	})

	t.Run("Login_Page_Handler_Workflow", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		freshCtx := setupPageHandlerTestContext(t)
		freshCtx.Suite.EmailSender.Reset()
		createAuthenticatedUser(t, freshCtx.AuthService, freshCtx.Suite.UserRepo, "login_page_user", "login_page@example.com")

		csrfToken := getCSRFToken(t, freshCtx.Router, "/login")

		reqBody := url.Values{}
		reqBody.Set("username", "login_page_user")
		reqBody.Set("password", "SecurePass123!")
		reqBody.Set("csrf_token", csrfToken)

		req := httptest.NewRequest("POST", "/login", strings.NewReader(reqBody.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		rec := httptest.NewRecorder()

		freshCtx.Router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusSeeOther)
		cookies := rec.Result().Cookies()
		authCookieSet := false
		for _, cookie := range cookies {
			if cookie.Name == "auth_token" && cookie.Value != "" {
				authCookieSet = true
				break
			}
		}
		if !authCookieSet {
			t.Error("Expected auth cookie to be set on login")
		}
	})

	t.Run("Email_Confirmation_Page_Handler", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		ctx.Suite.EmailSender.Reset()
		createAuthenticatedUser(t, authService, ctx.Suite.UserRepo, "confirm_page_user", "confirm_page@example.com")

		token := ctx.Suite.EmailSender.VerificationToken()
		if token == "" {
			token = "test-token"
		}

		req := httptest.NewRequest("GET", "/confirm?token="+url.QueryEscape(token), nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatusRange(t, rec, http.StatusOK, http.StatusSeeOther)
	})
}
