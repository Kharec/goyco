package integration

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"goyco/internal/middleware"
	"goyco/internal/testutils"
)

func TestIntegration_ErrorPropagation(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("Invalid_JSON_Error_Propagation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "json_error_user", "json_error@example.com")

		request := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer([]byte("invalid json{")))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+user.Token)
		request = testutils.WithUserContext(request, middleware.UserIDKey, user.User.ID)
		recorder := httptest.NewRecorder()

		ctx.Router.ServeHTTP(recorder, request)

		assertErrorResponse(t, recorder, http.StatusBadRequest)
	})

	t.Run("Validation_Error_Propagation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		reqBody := map[string]any{
			"username": "",
			"email":    "invalid-email",
			"password": "weak",
		}
		request := makePostRequestWithJSON(t, ctx.Router, "/api/auth/register", reqBody)

		assertErrorResponse(t, request, http.StatusBadRequest)
	})

	t.Run("Database_Error_Propagation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "db_error_user", "db_error@example.com")

		reqBody := map[string]any{
			"title":   "Test Post",
			"url":     "https://example.com/test",
			"content": "Test content",
		}

		request := makePostRequest(t, ctx.Router, "/api/posts", reqBody, user, nil)

		if request.Code == http.StatusInternalServerError {
			assertErrorResponse(t, request, http.StatusInternalServerError)
		} else {
			assertStatus(t, request, http.StatusCreated)
		}
	})

	t.Run("NotFound_Error_Propagation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "notfound_error_user", "notfound_error@example.com")

		request := makeAuthenticatedGetRequest(t, ctx.Router, "/api/posts/999999", user, map[string]string{"id": "999999"})

		assertErrorResponse(t, request, http.StatusNotFound)
	})

	t.Run("Unauthorized_Error_Propagation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		reqBody := map[string]any{
			"title":   "Test Post",
			"url":     "https://example.com/test",
			"content": "Test content",
		}

		request := makePostRequestWithJSON(t, ctx.Router, "/api/posts", reqBody)

		assertErrorResponse(t, request, http.StatusUnauthorized)
	})

	t.Run("Forbidden_Error_Propagation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		owner := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "forbidden_owner", "forbidden_owner@example.com")
		otherUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "forbidden_other", "forbidden_other@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, owner.User.ID, "Forbidden Post", "https://example.com/forbidden")

		updateBody := map[string]any{
			"title":   "Updated Title",
			"content": "Updated content",
		}

		request := makePutRequest(t, ctx.Router, fmt.Sprintf("/api/posts/%d", post.ID), updateBody, otherUser, map[string]string{"id": fmt.Sprintf("%d", post.ID)})

		assertErrorResponse(t, request, http.StatusForbidden)
	})

	t.Run("Service_Error_Propagation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		reqBody := map[string]any{
			"username": "existing_user",
			"email":    "existing@example.com",
			"password": "SecurePass123!",
		}

		request := makePostRequestWithJSON(t, ctx.Router, "/api/auth/register", reqBody)

		assertStatus(t, request, http.StatusCreated)

		request = makePostRequestWithJSON(t, ctx.Router, "/api/auth/register", reqBody)

		assertStatusRange(t, request, http.StatusBadRequest, http.StatusConflict)
		assertErrorResponse(t, request, request.Code)
	})

	t.Run("Middleware_Error_Propagation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		request := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer([]byte("{}")))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer expired.invalid.token")
		recorder := httptest.NewRecorder()

		ctx.Router.ServeHTTP(recorder, request)

		assertErrorResponse(t, recorder, http.StatusUnauthorized)
	})

	t.Run("Handler_Error_Response_Format", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		request := makeGetRequest(t, ctx.Router, "/api/nonexistent")

		if request.Code == http.StatusNotFound {
			if request.Header().Get("Content-Type") == "application/json" {
				assertErrorResponse(t, request, http.StatusNotFound)
			} else if request.Body.Len() == 0 {
				t.Error("Expected error response body")
			}
		}
	})
}
