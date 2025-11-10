package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"goyco/internal/database"
	"goyco/internal/middleware"
	"goyco/internal/testutils"
)

func TestIntegration_EmailService(t *testing.T) {
	ctx := setupTestContext(t)
	router := ctx.Router

	t.Run("Registration_Email_Sent", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		reqBody := map[string]string{
			"username": "email_reg_user",
			"email":    "email_reg@example.com",
			"password": "SecurePass123!",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusCreated)

		token := ctx.Suite.EmailSender.VerificationToken()
		if token == "" {
			t.Error("Expected verification email to be sent")
		}
	})

	t.Run("PasswordReset_Email_Sent", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		user := &database.User{
			Username:      "email_reset_user",
			Email:         "email_reset@example.com",
			Password:      testutils.HashPassword("OldPassword123!"),
			EmailVerified: true,
		}
		if err := ctx.Suite.UserRepo.Create(user); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		reqBody := map[string]string{
			"username_or_email": "email_reset_user",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/forgot-password", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		token := ctx.Suite.EmailSender.PasswordResetToken()
		if token == "" {
			t.Error("Expected password reset email to be sent")
		}
	})

	t.Run("AccountDeletion_Email_Sent", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "email_del_user", "email_del@example.com")

		reqBody := map[string]string{}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("DELETE", "/api/auth/account", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)

		token := ctx.Suite.EmailSender.DeletionToken()
		if token == "" {
			t.Error("Expected account deletion email to be sent")
		}
	})

	t.Run("EmailChange_Verification_Sent", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "email_change_user", "email_change@example.com")

		reqBody := map[string]string{
			"email": "newemail@example.com",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PUT", "/api/auth/email", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		token := ctx.Suite.EmailSender.VerificationToken()
		if token == "" {
			t.Error("Expected email change verification to be sent")
		}
	})

	t.Run("Email_Template_Content", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		reqBody := map[string]string{
			"username": "template_user",
			"email":    "template@example.com",
			"password": "SecurePass123!",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		token := ctx.Suite.EmailSender.VerificationToken()
		if token == "" {
			t.Fatal("Expected verification token")
		}

		if len(token) < 10 {
			t.Error("Expected token to have reasonable format")
		}
	})
}
