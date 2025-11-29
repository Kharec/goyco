package integration

import (
	"net/http"
	"testing"

	"goyco/internal/database"
	"goyco/internal/testutils"
)

func TestIntegration_EmailService(t *testing.T) {
	ctx := setupTestContext(t)
	router := ctx.Router

	t.Run("Registration_Email_Sent", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		reqBody := map[string]any{
			"username": "email_reg_user",
			"email":    "email_reg@example.com",
			"password": "SecurePass123!",
		}
		request := makePostRequestWithJSON(t, router, "/api/auth/register", reqBody)

		assertStatus(t, request, http.StatusCreated)

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

		reqBody := map[string]any{
			"username_or_email": "email_reset_user",
		}
		makePostRequestWithJSON(t, router, "/api/auth/forgot-password", reqBody)

		token := ctx.Suite.EmailSender.PasswordResetToken()
		if token == "" {
			t.Error("Expected password reset email to be sent")
		}
	})

	t.Run("AccountDeletion_Email_Sent", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "email_del_user", "email_del@example.com")

		request := makeDeleteRequest(t, router, "/api/auth/account", user, nil)

		assertStatus(t, request, http.StatusOK)

		token := ctx.Suite.EmailSender.DeletionToken()
		if token == "" {
			t.Error("Expected account deletion email to be sent")
		}
	})

	t.Run("EmailChange_Verification_Sent", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "email_change_user", "email_change@example.com")

		reqBody := map[string]any{
			"email": "newemail@example.com",
		}
		makePutRequest(t, router, "/api/auth/email", reqBody, user, nil)

		token := ctx.Suite.EmailSender.VerificationToken()
		if token == "" {
			t.Error("Expected email change verification to be sent")
		}
	})

	t.Run("Email_Template_Content", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()

		reqBody := map[string]any{
			"username": "template_user",
			"email":    "template@example.com",
			"password": "SecurePass123!",
		}
		makePostRequestWithJSON(t, router, "/api/auth/register", reqBody)

		token := ctx.Suite.EmailSender.VerificationToken()
		if token == "" {
			t.Fatal("Expected verification token")
		}

		if len(token) < 10 {
			t.Error("Expected token to have reasonable format")
		}
	})
}
