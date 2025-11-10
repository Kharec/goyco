package integration

import (
	"bytes"
	"encoding/json"
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

		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer([]byte("invalid json{")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusBadRequest)
	})

	t.Run("Validation_Error_Propagation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		reqBody := map[string]string{
			"username": "",
			"email":    "invalid-email",
			"password": "weak",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusBadRequest)
	})

	t.Run("Database_Error_Propagation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "db_error_user", "db_error@example.com")

		reqBody := map[string]string{
			"title":   "Test Post",
			"url":     "https://example.com/test",
			"content": "Test content",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		if rec.Code == http.StatusInternalServerError {
			assertErrorResponse(t, rec, http.StatusInternalServerError)
		} else {
			assertStatus(t, rec, http.StatusCreated)
		}
	})

	t.Run("NotFound_Error_Propagation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "notfound_error_user", "notfound_error@example.com")

		req := httptest.NewRequest("GET", "/api/posts/999999", nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		req = testutils.WithURLParams(req, map[string]string{"id": "999999"})
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusNotFound)
	})

	t.Run("Unauthorized_Error_Propagation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		reqBody := map[string]string{
			"title":   "Test Post",
			"url":     "https://example.com/test",
			"content": "Test content",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusUnauthorized)
	})

	t.Run("Forbidden_Error_Propagation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		owner := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "forbidden_owner", "forbidden_owner@example.com")
		otherUser := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "forbidden_other", "forbidden_other@example.com")

		post := testutils.CreatePostWithRepo(t, ctx.Suite.PostRepo, owner.User.ID, "Forbidden Post", "https://example.com/forbidden")

		updateBody := map[string]string{
			"title":   "Updated Title",
			"content": "Updated content",
		}
		body, _ := json.Marshal(updateBody)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/posts/%d", post.ID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+otherUser.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, otherUser.User.ID)
		req = testutils.WithURLParams(req, map[string]string{"id": fmt.Sprintf("%d", post.ID)})
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusForbidden)
	})

	t.Run("Service_Error_Propagation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		reqBody := map[string]string{
			"username": "existing_user",
			"email":    "existing@example.com",
			"password": "SecurePass123!",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		ctx.Router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusCreated)

		req = httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		ctx.Router.ServeHTTP(rec, req)

		assertStatusRange(t, rec, http.StatusBadRequest, http.StatusConflict)
		assertErrorResponse(t, rec, rec.Code)
	})

	t.Run("Middleware_Error_Propagation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer expired.invalid.token")
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusUnauthorized)
	})

	t.Run("Handler_Error_Response_Format", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		req := httptest.NewRequest("GET", "/api/nonexistent", nil)
		rec := httptest.NewRecorder()

		ctx.Router.ServeHTTP(rec, req)

		if rec.Code == http.StatusNotFound {
			if rec.Header().Get("Content-Type") == "application/json" {
				assertErrorResponse(t, rec, http.StatusNotFound)
			} else {
				if rec.Body.Len() == 0 {
					t.Error("Expected error response body")
				}
			}
		}
	})
}
