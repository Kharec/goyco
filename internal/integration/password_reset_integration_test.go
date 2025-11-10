package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"goyco/internal/database"
	"goyco/internal/middleware"
	"goyco/internal/testutils"
)

func TestIntegration_PasswordReset_CompleteFlow(t *testing.T) {
	ctx := setupTestContext(t)
	router := ctx.Router

	t.Run("API_PasswordReset_Request", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		user := &database.User{
			Username:      "reset_user",
			Email:         "reset@example.com",
			Password:      testutils.HashPassword("OldPassword123!"),
			EmailVerified: true,
		}
		if err := ctx.Suite.UserRepo.Create(user); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		reqBody := map[string]string{
			"username_or_email": "reset_user",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/forgot-password", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
		if response != nil {
			if success, ok := response["success"].(bool); !ok || !success {
				t.Error("Expected success=true")
			}
		}

		resetToken := ctx.Suite.EmailSender.PasswordResetToken()
		if resetToken == "" {
			t.Error("Expected password reset token to be generated")
		}
	})

	t.Run("API_PasswordReset_Complete", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		user := &database.User{
			Username:      "reset_complete_user",
			Email:         "reset_complete@example.com",
			Password:      testutils.HashPassword("OldPassword123!"),
			EmailVerified: true,
		}
		if err := ctx.Suite.UserRepo.Create(user); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		if err := ctx.AuthService.RequestPasswordReset("reset_complete_user"); err != nil {
			t.Fatalf("Failed to request password reset: %v", err)
		}

		resetToken := ctx.Suite.EmailSender.PasswordResetToken()
		if resetToken == "" {
			t.Fatal("Expected password reset token")
		}

		reqBody := map[string]string{
			"token":        resetToken,
			"new_password": "NewPassword123!",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/reset-password", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)

		loginResult, err := ctx.AuthService.Login("reset_complete_user", "NewPassword123!")
		if err != nil {
			t.Fatalf("Failed to login with new password: %v", err)
		}

		if loginResult.User.Username != "reset_complete_user" {
			t.Error("Expected login to succeed with new password")
		}
	})

	t.Run("Page_PasswordReset_Request", func(t *testing.T) {
		pageCtx := setupPageHandlerTestContext(t)
		pageRouter := pageCtx.Router
		pageCtx.Suite.EmailSender.Reset()

		user := &database.User{
			Username:      "page_reset_user",
			Email:         "page_reset@example.com",
			Password:      testutils.HashPassword("OldPassword123!"),
			EmailVerified: true,
		}
		if err := pageCtx.Suite.UserRepo.Create(user); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		csrfToken := getCSRFToken(t, pageRouter, "/forgot-password")

		reqBody := url.Values{}
		reqBody.Set("username_or_email", "page_reset_user")
		reqBody.Set("csrf_token", csrfToken)
		req := httptest.NewRequest("POST", "/forgot-password", strings.NewReader(reqBody.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		rec := httptest.NewRecorder()

		pageRouter.ServeHTTP(rec, req)

		assertStatusRange(t, rec, http.StatusOK, http.StatusSeeOther)

		resetToken := pageCtx.Suite.EmailSender.PasswordResetToken()
		if resetToken == "" {
			t.Error("Expected password reset token to be generated")
		}
	})

	t.Run("PasswordReset_TokenExpiration", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		user := &database.User{
			Username:      "expire_user",
			Email:         "expire@example.com",
			Password:      testutils.HashPassword("OldPassword123!"),
			EmailVerified: true,
		}
		if err := ctx.Suite.UserRepo.Create(user); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		if err := ctx.AuthService.RequestPasswordReset("expire_user"); err != nil {
			t.Fatalf("Failed to request password reset: %v", err)
		}

		resetToken := ctx.Suite.EmailSender.PasswordResetToken()
		hashedToken := testutils.HashVerificationToken(resetToken)

		user, err := ctx.Suite.UserRepo.GetByPasswordResetToken(hashedToken)
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}

		expiredTime := time.Now().Add(-25 * time.Hour)
		user.PasswordResetExpiresAt = &expiredTime
		if err := ctx.Suite.UserRepo.Update(user); err != nil {
			t.Fatalf("Failed to update user: %v", err)
		}

		reqBody := map[string]string{
			"token":        resetToken,
			"new_password": "NewPassword123!",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/reset-password", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusBadRequest)
	})

	t.Run("PasswordReset_InvalidToken", func(t *testing.T) {
		reqBody := map[string]string{
			"token":        "invalid-token",
			"new_password": "NewPassword123!",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/reset-password", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusBadRequest)
	})

	t.Run("PasswordReset_WeakPassword", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		user := &database.User{
			Username:      "weak_pass_user",
			Email:         "weak_pass@example.com",
			Password:      testutils.HashPassword("OldPassword123!"),
			EmailVerified: true,
		}
		if err := ctx.Suite.UserRepo.Create(user); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		if err := ctx.AuthService.RequestPasswordReset("weak_pass_user"); err != nil {
			t.Fatalf("Failed to request password reset: %v", err)
		}

		resetToken := ctx.Suite.EmailSender.PasswordResetToken()

		reqBody := map[string]string{
			"token":        resetToken,
			"new_password": "123",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/reset-password", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertErrorResponse(t, rec, http.StatusBadRequest)
	})

	t.Run("PasswordReset_EmailIntegration", func(t *testing.T) {
		middleware.StopAllRateLimiters()
		freshCtx := setupTestContext(t)
		freshCtx.Suite.EmailSender.Reset()

		user := &database.User{
			Username:      "email_reset_user",
			Email:         "email_reset@example.com",
			Password:      testutils.HashPassword("OldPassword123!"),
			EmailVerified: true,
		}
		if err := freshCtx.Suite.UserRepo.Create(user); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		reqBody := map[string]string{
			"username_or_email": "email_reset@example.com",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/forgot-password", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		freshCtx.Router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)

		resetToken := freshCtx.Suite.EmailSender.PasswordResetToken()
		if resetToken == "" {
			t.Error("Expected password reset token when using email")
		}
	})
}
