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
		request := httptest.NewRequest("GET", "/", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		assertStatus(t, recorder, http.StatusOK)

		if !strings.Contains(recorder.Body.String(), "<html") {
			t.Error("Expected HTML content")
		}
	})

	t.Run("Login_Form_Renders", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/login", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		assertStatus(t, recorder, http.StatusOK)

		body := recorder.Body.String()
		if !strings.Contains(body, "login") && !strings.Contains(body, "Login") {
			t.Error("Expected login form content")
		}
	})

	t.Run("Register_Form_Renders", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/register", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)
		assertStatus(t, recorder, http.StatusOK)

		body := recorder.Body.String()
		if !strings.Contains(body, "register") && !strings.Contains(body, "Register") {
			t.Error("Expected register form content")
		}
	})

	t.Run("PageHandler_With_CSRF_Token", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/register", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		assertCookie(t, recorder, "csrf_token", "")
	})

	t.Run("PageHandler_Form_Submission", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		getRequest := httptest.NewRequest("GET", "/register", nil)
		getRecorder := httptest.NewRecorder()
		router.ServeHTTP(getRecorder, getRequest)

		cookies := getRecorder.Result().Cookies()
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

		requestBody := url.Values{}
		requestBody.Set("username", "page_form_user")
		requestBody.Set("email", "page_form@example.com")
		requestBody.Set("password", "SecurePass123!")
		requestBody.Set("csrf_token", csrfCookie.Value)

		request := httptest.NewRequest("POST", "/register", strings.NewReader(requestBody.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(csrfCookie)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		assertStatusRange(t, recorder, http.StatusOK, http.StatusSeeOther)
	})

	t.Run("PageHandler_Authenticated_Access", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createUserWithCleanup(t, ctx, "page_auth_user", "page_auth@example.com")

		request := httptest.NewRequest("GET", "/settings", nil)
		request.AddCookie(&http.Cookie{Name: "auth_token", Value: user.Token})
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		assertStatus(t, recorder, http.StatusOK)
	})

	t.Run("PageHandler_Post_Display", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createUserWithCleanup(t, ctx, "page_post_user", "page_post@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, user.User.ID, "Page Test Post", "https://example.com/page-test")

		request := httptest.NewRequest("GET", "/posts/"+fmt.Sprintf("%d", post.ID), nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		assertStatus(t, recorder, http.StatusOK)

		body := recorder.Body.String()
		if !strings.Contains(body, "Page Test Post") {
			t.Error("Expected post title in page")
		}
	})

	t.Run("PageHandler_Search_Page", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/search?q=test", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		assertStatus(t, recorder, http.StatusOK)
	})

	t.Run("PageHandler_Error_Handling", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/nonexistent", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		assertStatus(t, recorder, http.StatusNotFound)
	})
}
