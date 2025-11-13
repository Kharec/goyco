package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"goyco/internal/testutils"
)

func TestIntegration_PageHandler(t *testing.T) {
	ctx := setupPageHandlerTestContext(t)
	router := ctx.Router

	t.Run("Home_Page_Renders", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)

		if !strings.Contains(rec.Body.String(), "<html") {
			t.Error("Expected HTML content")
		}
	})

	t.Run("Login_Form_Renders", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/login", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)

		body := rec.Body.String()
		if !strings.Contains(body, "login") && !strings.Contains(body, "Login") {
			t.Error("Expected login form content")
		}
	})

	t.Run("Register_Form_Renders", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/register", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)

		body := rec.Body.String()
		if !strings.Contains(body, "register") && !strings.Contains(body, "Register") {
			t.Error("Expected register form content")
		}
	})

	t.Run("PageHandler_With_CSRF_Token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/register", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertCookie(t, rec, "csrf_token", "")
	})

	t.Run("PageHandler_Form_Submission", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		getReq := httptest.NewRequest("GET", "/register", nil)
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)

		cookies := getRec.Result().Cookies()
		var csrfCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "csrf_token" {
				csrfCookie = cookie
				break
			}
		}

		if csrfCookie == nil {
			t.Fatal("Expected CSRF cookie")
		}

		reqBody := url.Values{}
		reqBody.Set("username", "page_form_user")
		reqBody.Set("email", "page_form@example.com")
		reqBody.Set("password", "SecurePass123!")
		reqBody.Set("csrf_token", csrfCookie.Value)

		req := httptest.NewRequest("POST", "/register", strings.NewReader(reqBody.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(csrfCookie)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatusRange(t, rec, http.StatusOK, http.StatusSeeOther)
	})

	t.Run("PageHandler_Authenticated_Access", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createUserWithCleanup(t, ctx, "page_auth_user", "page_auth@example.com")

		req := httptest.NewRequest("GET", "/settings", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)
	})

	t.Run("PageHandler_Post_Display", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createUserWithCleanup(t, ctx, "page_post_user", "page_post@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Page Test Post", "https://example.com/page-test")

		req := httptest.NewRequest("GET", "/posts/"+fmt.Sprintf("%d", post.ID), nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)

		body := rec.Body.String()
		if !strings.Contains(body, "Page Test Post") {
			t.Error("Expected post title in page")
		}
	})

	t.Run("PageHandler_Search_Page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/search?q=test", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)
	})

	t.Run("PageHandler_Error_Handling", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/nonexistent", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusNotFound)
	})
}
