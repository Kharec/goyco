package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"goyco/internal/repositories"
	"goyco/internal/testutils"
)

func TestE2E_APIRegistration(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("api_registration_flow", func(t *testing.T) {
		var createdUser *TestUser

		t.Cleanup(func() {
			if createdUser != nil {
				ctx.server.UserRepo.Delete(createdUser.ID)
			}
		})

		tempUser := ctx.createUserWithCleanup(t, "temp", "TempPass123!")
		tempAuthClient := ctx.loginUser(t, tempUser.Username, tempUser.Password)

		ctx.server.EmailSender.Reset()

		newUsername := uniqueUsername(t, "newuser")
		newEmail := uniqueEmail(t, "newuser")
		registerResp := tempAuthClient.RegisterUser(t, newUsername, newEmail, "NewPass123!")

		if !registerResp.Success {
			t.Errorf("Expected registration to be successful, got failure: %s", registerResp.Message)
		}

		ctx.loginExpectStatus(t, newUsername, "NewPass123!", http.StatusForbidden)

		confirmationToken := ctx.server.EmailSender.VerificationToken()
		if confirmationToken == "" {
			t.Fatalf("expected registration to trigger verification token")
		}

		ctx.confirmEmail(t, confirmationToken)

		authClient := ctx.loginUser(t, newUsername, "NewPass123!")
		if authClient.Token == "" {
			t.Errorf("Expected to be able to login with registered user")
		}

		user, err := ctx.server.UserRepo.GetByUsername(newUsername)
		if err != nil {
			t.Fatalf("Failed to load registered user: %v", err)
		}

		createdUser = &TestUser{
			ID:            user.ID,
			Username:      user.Username,
			Email:         user.Email,
			Password:      "NewPass123!",
			EmailVerified: user.EmailVerified,
		}
	})
}

func TestE2E_PasswordResetFlow(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("password_reset_flow", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "resetuser", "OldPassword123!")
		originalPassword := createdUser.Password

		t.Run("request_password_reset", func(t *testing.T) {
			ctx.server.EmailSender.Reset()

			statusCode := testutils.RequestPasswordReset(t, ctx.client, ctx.baseURL, createdUser.Email, testutils.GenerateTestIP())
			if statusCode != http.StatusOK {
				t.Errorf("Expected password reset request to succeed with status 200, got %d", statusCode)
			}

			resetToken := ctx.server.EmailSender.PasswordResetToken()
			if resetToken == "" {
				t.Errorf("Expected password reset email to contain a reset token, but token is empty")
			}
		})

		t.Run("reset_password_with_valid_token", func(t *testing.T) {
			ctx.server.EmailSender.Reset()

			statusCode := testutils.RequestPasswordReset(t, ctx.client, ctx.baseURL, createdUser.Username, testutils.GenerateTestIP())
			if statusCode != http.StatusOK {
				t.Errorf("Expected password reset request to succeed, got status %d", statusCode)
			}

			resetToken := ctx.server.EmailSender.PasswordResetToken()
			if resetToken == "" {
				t.Fatalf("Expected password reset email to contain a reset token")
			}

			statusCode = testutils.ResetPassword(t, ctx.client, ctx.baseURL, resetToken, "NewPassword456!", testutils.GenerateTestIP())
			if statusCode != http.StatusOK {
				t.Errorf("Expected password reset to succeed with status 200, got %d", statusCode)
			}
		})

		t.Run("login_with_new_password_works", func(t *testing.T) {
			ctx.loginExpectStatus(t, createdUser.Username, originalPassword, http.StatusUnauthorized)

			newAuthClient := ctx.loginUser(t, createdUser.Username, "NewPassword456!")
			if newAuthClient.Token == "" {
				t.Errorf("Expected to be able to login with new password, but login failed")
			}

			profile := newAuthClient.GetProfile(t)
			if profile.Data.Username != createdUser.Username {
				t.Errorf("Expected to access profile with new password, got username '%s'", profile.Data.Username)
			}
		})

		t.Run("old_password_no_longer_works", func(t *testing.T) {
			statusCode := ctx.loginExpectStatusWithIP(t, createdUser.Username, originalPassword, http.StatusUnauthorized, testutils.GenerateTestIP())
			failIfRateLimited(t, statusCode, "old password login check")

			newAuthClient := ctx.loginUser(t, createdUser.Username, "NewPassword456!")
			if newAuthClient.Token == "" {
				t.Errorf("Expected to still be able to login with new password after checking old password")
			}
		})

		t.Run("expired_invalid_tokens_rejected", func(t *testing.T) {
			statusCode := testutils.ResetPassword(t, ctx.client, ctx.baseURL, "", "AnotherPassword789!", testutils.GenerateTestIP())
			if statusCode != http.StatusBadRequest && statusCode != http.StatusTooManyRequests {
				t.Errorf("Expected empty token to be rejected with status 400 or 429, got %d", statusCode)
			}

			statusCode = testutils.ResetPassword(t, ctx.client, ctx.baseURL, "invalid-token-12345", "AnotherPassword789!", testutils.GenerateTestIP())
			if statusCode != http.StatusBadRequest && statusCode != http.StatusTooManyRequests {
				t.Errorf("Expected invalid token to be rejected with status 400 or 429, got %d", statusCode)
			}

			statusCode = testutils.ResetPassword(t, ctx.client, ctx.baseURL, "not-a-valid-token", "AnotherPassword789!", testutils.GenerateTestIP())
			if statusCode != http.StatusBadRequest && statusCode != http.StatusTooManyRequests {
				t.Errorf("Expected malformed token to be rejected with status 400 or 429, got %d", statusCode)
			}

			ctx.server.EmailSender.Reset()
			resetStatus := testutils.RequestPasswordReset(t, ctx.client, ctx.baseURL, createdUser.Email, testutils.GenerateTestIP())
			failIfRateLimited(t, resetStatus, "password reset token generation")
			validToken := ctx.server.EmailSender.PasswordResetToken()
			if validToken == "" {
				t.Fatalf("Expected password reset token but got empty")
			}

			statusCode = testutils.ResetPassword(t, ctx.client, ctx.baseURL, validToken, "AnotherPassword789!", testutils.GenerateTestIP())
			if statusCode != http.StatusOK {
				t.Errorf("Expected valid token to work, got status %d", statusCode)
			}

			statusCode = testutils.ResetPassword(t, ctx.client, ctx.baseURL, validToken, "YetAnotherPassword000!", testutils.GenerateTestIP())
			if statusCode != http.StatusBadRequest && statusCode != http.StatusTooManyRequests {
				t.Errorf("Expected used token to be rejected with status 400 or 429, got %d", statusCode)
			}
		})

		t.Run("password_reset_by_username", func(t *testing.T) {
			ctx.server.EmailSender.Reset()

			statusCode := testutils.RequestPasswordReset(t, ctx.client, ctx.baseURL, createdUser.Username, testutils.GenerateTestIP())
			failIfRateLimited(t, statusCode, "password reset by username")
			if statusCode != http.StatusOK {
				t.Errorf("Expected password reset request by username to succeed, got status %d", statusCode)
			}

			resetToken := ctx.server.EmailSender.PasswordResetToken()
			if resetToken == "" {
				t.Errorf("Expected password reset email to contain a reset token when using username")
			}

			statusCode = testutils.ResetPassword(t, ctx.client, ctx.baseURL, resetToken, "FinalPassword999!", testutils.GenerateTestIP())
			if statusCode != http.StatusOK {
				t.Errorf("Expected password reset with username-requested token to succeed, got status %d", statusCode)
			}

			newAuthClient := ctx.loginUser(t, createdUser.Username, "FinalPassword999!")
			if newAuthClient.Token == "" {
				t.Errorf("Expected to be able to login with password reset via username")
			}
		})

		t.Run("password_reset_nonexistent_user", func(t *testing.T) {
			ctx.server.EmailSender.Reset()

			statusCode := testutils.RequestPasswordReset(t, ctx.client, ctx.baseURL, "nonexistent@example.com", testutils.GenerateTestIP())
			if statusCode != http.StatusOK && statusCode != http.StatusTooManyRequests {
				t.Errorf("Expected password reset request for non-existent user to return 200 or 429 (security), got %d", statusCode)
			}

			if statusCode == http.StatusOK {
				resetToken := ctx.server.EmailSender.PasswordResetToken()
				if resetToken != "" {
					t.Errorf("Expected no password reset token for non-existent user, but got token: %s", resetToken)
				}
			}
		})
	})
}

func TestE2E_RefreshTokenFlow(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("refresh_token_flow", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "refreshtokenuser", "Password123!")

		t.Run("login_get_tokens", func(t *testing.T) {
			authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

			if authClient.Token == "" {
				t.Errorf("Expected access token to be set after login, but it's empty")
			}

			if authClient.RefreshToken == "" {
				t.Errorf("Expected refresh token to be set after login, but it's empty")
			}

			profile := authClient.GetProfile(t)
			if profile.Data.Username != createdUser.Username {
				t.Errorf("Expected to access profile with access token, got username '%s'", profile.Data.Username)
			}
		})

		t.Run("refresh_access_token", func(t *testing.T) {
			authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)
			originalAccessToken := authClient.Token
			originalRefreshToken := authClient.RefreshToken

			newAccessToken, statusCode := authClient.RefreshAccessToken(t)
			if statusCode != http.StatusOK {
				t.Errorf("Expected refresh token to succeed with status 200, got %d", statusCode)
			}

			if newAccessToken == "" {
				t.Errorf("Expected new access token to be returned, but it's empty")
			}

			if newAccessToken == originalAccessToken {
				t.Logf("New access token is identical to original (may occur if generated within same second)")
			}

			if authClient.RefreshToken == originalRefreshToken {
				t.Errorf("Expected refresh token to rotate")
			}

			profile := authClient.GetProfile(t)
			if profile.Data.Username != createdUser.Username {
				t.Errorf("Expected to access profile with new access token, got username '%s'", profile.Data.Username)
			}

			oldAuthClient := &AuthenticatedClient{
				AuthenticatedClient: &testutils.AuthenticatedClient{
					Client:       ctx.client,
					Token:        originalAccessToken,
					RefreshToken: originalRefreshToken,
					BaseURL:      ctx.baseURL,
				},
			}
			profile = oldAuthClient.GetProfile(t)
			if profile.Data.Username != createdUser.Username {
				t.Errorf("Expected old access token to still work (until expiration), got username '%s'", profile.Data.Username)
			}
		})

		t.Run("invalid_refresh_token_rejected", func(t *testing.T) {
			authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

			invalidClient := &AuthenticatedClient{
				AuthenticatedClient: &testutils.AuthenticatedClient{
					Client:       ctx.client,
					Token:        authClient.Token,
					RefreshToken: "invalid-refresh-token-12345",
					BaseURL:      ctx.baseURL,
				},
			}

			_, statusCode := invalidClient.RefreshAccessToken(t)
			if statusCode != http.StatusUnauthorized && statusCode != http.StatusBadRequest {
				t.Errorf("Expected invalid refresh token to be rejected with status 401 or 400, got %d", statusCode)
			}
		})

		t.Run("empty_refresh_token_rejected", func(t *testing.T) {
			authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

			emptyClient := &AuthenticatedClient{
				AuthenticatedClient: &testutils.AuthenticatedClient{
					Client:       ctx.client,
					Token:        authClient.Token,
					RefreshToken: "",
					BaseURL:      ctx.baseURL,
				},
			}

			_, statusCode := emptyClient.RefreshAccessToken(t)
			if statusCode != http.StatusBadRequest && statusCode != http.StatusUnauthorized {
				t.Errorf("Expected empty refresh token to be rejected with status 400 or 401, got %d", statusCode)
			}
		})

		t.Run("expired_refresh_token_rejected", func(t *testing.T) {
			authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)
			refreshTokenString := authClient.RefreshToken

			refreshToken, err := ctx.server.RefreshTokenRepo.GetByTokenHash(tokenHash(refreshTokenString))
			if err != nil {
				t.Fatalf("Failed to find refresh token in database: %v", err)
			}

			refreshToken.ExpiresAt = time.Now().Add(-1 * time.Hour)
			if err := ctx.server.DB.Save(refreshToken).Error; err != nil {
				t.Fatalf("Failed to expire refresh token: %v", err)
			}

			expiredClient := &AuthenticatedClient{
				AuthenticatedClient: &testutils.AuthenticatedClient{
					Client:       ctx.client,
					Token:        authClient.Token,
					RefreshToken: refreshTokenString,
					BaseURL:      ctx.baseURL,
				},
			}

			_, statusCode := expiredClient.RefreshAccessToken(t)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected expired refresh token to be rejected with status 401, got %d", statusCode)
			}

			_, err = ctx.server.RefreshTokenRepo.GetByTokenHash(tokenHash(refreshTokenString))
			if err == nil {
				t.Errorf("Expected expired refresh token to be deleted from database after failed refresh attempt")
			}
		})

		t.Run("multiple_refresh_operations", func(t *testing.T) {
			loginStatus := ctx.loginExpectStatusWithIP(t, createdUser.Username, createdUser.Password, http.StatusOK, testutils.GenerateTestIP())
			failIfRateLimited(t, loginStatus, "initial login before refresh loop")

			authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

			for i := range 3 {
				newAccessToken, statusCode := authClient.RefreshAccessToken(t, testutils.GenerateTestIP())
				failIfRateLimited(t, statusCode, fmt.Sprintf("refresh operation %d", i+1))
				if statusCode != http.StatusOK {
					t.Errorf("Expected refresh token operation %d to succeed, got status %d", i+1, statusCode)
				}

				if newAccessToken != "" {
					profile := authClient.GetProfile(t)
					if profile.Data.Username != createdUser.Username {
						t.Errorf("Expected to access profile with refreshed token %d, got username '%s'", i+1, profile.Data.Username)
					}
				}
			}

			if authClient.RefreshToken == "" {
				t.Errorf("Expected refresh token to still be available after multiple refreshes")
			}
		})
	})
}

func TestE2E_TokenRevocationFlow(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("token_revocation_flow", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "revokeuser", "Password123!")

		t.Run("single_token_revocation", func(t *testing.T) {
			authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)
			refreshToken1 := authClient.RefreshToken

			_, statusCode := authClient.RefreshAccessToken(t)
			if statusCode != http.StatusOK {
				t.Errorf("Expected refresh token to work before revocation, got status %d", statusCode)
			}

			statusCode = authClient.RevokeToken(t, refreshToken1)
			if statusCode != http.StatusOK {
				t.Errorf("Expected token revocation to succeed with status 200, got %d", statusCode)
			}

			revokedClient := &AuthenticatedClient{
				AuthenticatedClient: &testutils.AuthenticatedClient{
					Client:       ctx.client,
					Token:        authClient.Token,
					RefreshToken: refreshToken1,
					BaseURL:      ctx.baseURL,
				},
			}
			_, statusCode = revokedClient.RefreshAccessToken(t)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected revoked refresh token to be rejected with status 401, got %d", statusCode)
			}
		})

		t.Run("revoked_token_cannot_be_used", func(t *testing.T) {
			authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)
			refreshToken2 := authClient.RefreshToken

			statusCode := authClient.RevokeToken(t, refreshToken2)
			if statusCode != http.StatusOK {
				t.Errorf("Expected token revocation to succeed, got status %d", statusCode)
			}

			revokedClient := &AuthenticatedClient{
				AuthenticatedClient: &testutils.AuthenticatedClient{
					Client:       ctx.client,
					Token:        authClient.Token,
					RefreshToken: refreshToken2,
					BaseURL:      ctx.baseURL,
				},
			}

			_, statusCode = revokedClient.RefreshAccessToken(t)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected revoked refresh token (first attempt) to be rejected with status 401, got %d", statusCode)
			}

			_, statusCode = revokedClient.RefreshAccessToken(t)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected revoked refresh token (second attempt) to be rejected with status 401, got %d", statusCode)
			}

			_, err := ctx.server.RefreshTokenRepo.GetByTokenHash(tokenHash(refreshToken2))
			if err == nil {
				t.Errorf("Expected revoked refresh token to be deleted from database")
			}
		})

		t.Run("revoke_all_tokens", func(t *testing.T) {
			authClient1 := ctx.loginUser(t, createdUser.Username, createdUser.Password)
			refreshToken1 := authClient1.RefreshToken

			authClient2 := ctx.loginUser(t, createdUser.Username, createdUser.Password)
			refreshToken2 := authClient2.RefreshToken

			authClient3 := ctx.loginUser(t, createdUser.Username, createdUser.Password)
			refreshToken3 := authClient3.RefreshToken

			_, statusCode := authClient1.RefreshAccessToken(t)
			if statusCode != http.StatusOK {
				t.Errorf("Expected refresh token 1 to work before revocation, got status %d", statusCode)
			}

			_, statusCode = authClient2.RefreshAccessToken(t)
			if statusCode != http.StatusOK {
				t.Errorf("Expected refresh token 2 to work before revocation, got status %d", statusCode)
			}

			_, statusCode = authClient3.RefreshAccessToken(t)
			if statusCode != http.StatusOK {
				t.Errorf("Expected refresh token 3 to work before revocation, got status %d", statusCode)
			}

			statusCode = authClient1.RevokeAllTokens(t)
			if statusCode != http.StatusOK {
				t.Errorf("Expected revoke-all to succeed with status 200, got %d", statusCode)
			}

			_, statusCode = authClient1.RefreshAccessToken(t)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected refresh token 1 to be rejected after revoke-all, got status %d", statusCode)
			}

			_, statusCode = authClient2.RefreshAccessToken(t)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected refresh token 2 to be rejected after revoke-all, got status %d", statusCode)
			}

			_, statusCode = authClient3.RefreshAccessToken(t)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected refresh token 3 to be rejected after revoke-all, got status %d", statusCode)
			}

			_, err := ctx.server.RefreshTokenRepo.GetByTokenHash(tokenHash(refreshToken1))
			if err == nil {
				t.Errorf("Expected refresh token 1 to be deleted from database after revoke-all")
			}

			_, err = ctx.server.RefreshTokenRepo.GetByTokenHash(tokenHash(refreshToken2))
			if err == nil {
				t.Errorf("Expected refresh token 2 to be deleted from database after revoke-all")
			}

			_, err = ctx.server.RefreshTokenRepo.GetByTokenHash(tokenHash(refreshToken3))
			if err == nil {
				t.Errorf("Expected refresh token 3 to be deleted from database after revoke-all")
			}
		})

		t.Run("revoked_refresh_tokens_cannot_be_used", func(t *testing.T) {
			loginStatus := ctx.loginExpectStatus(t, createdUser.Username, createdUser.Password, http.StatusOK)
			skipIfRateLimited(t, loginStatus, "revoked refresh tokens test")

			authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)
			refreshToken := authClient.RefreshToken
			originalAccessToken := authClient.Token

			_, statusCode := authClient.RefreshAccessToken(t)
			if statusCode != http.StatusOK {
				t.Errorf("Expected refresh token to work before revocation, got status %d", statusCode)
			}

			statusCode = authClient.RevokeToken(t, refreshToken)
			if statusCode != http.StatusOK {
				t.Errorf("Expected token revocation to succeed, got status %d", statusCode)
			}

			revokedClient := &AuthenticatedClient{
				AuthenticatedClient: &testutils.AuthenticatedClient{
					Client:       ctx.client,
					Token:        originalAccessToken,
					RefreshToken: refreshToken,
					BaseURL:      ctx.baseURL,
				},
			}

			_, statusCode = revokedClient.RefreshAccessToken(t)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected revoked refresh token to be rejected with status 401, got %d", statusCode)
			}

			profile := revokedClient.GetProfile(t)
			if profile.Data.Username != createdUser.Username {
				t.Errorf("Expected access token to still work after refresh token revocation, got username '%s'", profile.Data.Username)
			}

			_, statusCode = revokedClient.RefreshAccessToken(t)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected revoked refresh token to still be rejected on second attempt, got status %d", statusCode)
			}
		})

		t.Run("unauthorized_revocation_attempts", func(t *testing.T) {
			loginStatus := ctx.loginExpectStatus(t, createdUser.Username, createdUser.Password, http.StatusOK)
			skipIfRateLimited(t, loginStatus, "unauthorized revocation test")

			authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)
			refreshToken := authClient.RefreshToken

			unauthenticatedClient := &http.Client{Transport: ctx.client.Transport}
			statusCode := testutils.RevokeToken(t, unauthenticatedClient, ctx.baseURL, refreshToken, "")
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected unauthenticated revocation to be rejected with status 401, got %d", statusCode)
			}

			statusCode = testutils.RevokeAllTokens(t, unauthenticatedClient, ctx.baseURL, "")
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected unauthenticated revoke-all to be rejected with status 401, got %d", statusCode)
			}
		})

		t.Run("revoke_nonexistent_token", func(t *testing.T) {
			loginStatus := ctx.loginExpectStatus(t, createdUser.Username, createdUser.Password, http.StatusOK)
			skipIfRateLimited(t, loginStatus, "revoke nonexistent token test")

			authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

			statusCode := authClient.RevokeToken(t, "non-existent-token-12345")
			if statusCode != http.StatusOK {
				t.Errorf("Expected revoking non-existent token to succeed (idempotent), got status %d", statusCode)
			}
		})
	})
}

func TestE2E_AccountDeletionConfirmationFlow(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("account_deletion_confirmation_flow", func(t *testing.T) {
		var createdUser *TestUser

		t.Run("deletion_request_generates_token", func(t *testing.T) {
			createdUser = ctx.createUserWithCleanup(t, "deleteuser", "Password123!")
			authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

			ctx.server.EmailSender.Reset()

			deletionResp := authClient.RequestAccountDeletion(t)
			if !deletionResp.Success {
				t.Errorf("Expected account deletion request to succeed, got failure: %s", deletionResp.Message)
			}

			deletionToken := ctx.server.EmailSender.DeletionToken()
			if deletionToken == "" {
				t.Errorf("Expected account deletion email to contain a deletion token, but token is empty")
			}

			_, err := ctx.server.UserRepo.GetByID(createdUser.ID)
			if err != nil {
				t.Errorf("Expected user to still exist before confirmation, got error: %v", err)
			}
		})

		t.Run("confirmation_with_valid_token_deletes_account", func(t *testing.T) {
			testUser := ctx.createUserWithCleanup(t, "confirmdeleteuser", "Password123!")
			authClient := ctx.loginUser(t, testUser.Username, testUser.Password)

			ctx.server.EmailSender.Reset()

			authClient.RequestAccountDeletion(t)

			deletionToken := ctx.server.EmailSender.DeletionToken()
			if deletionToken == "" {
				t.Fatalf("Expected account deletion email to contain a deletion token")
			}

			statusCode := authClient.ConfirmAccountDeletion(t, deletionToken, false)
			if statusCode != http.StatusOK {
				t.Errorf("Expected account deletion confirmation to succeed with status 200, got %d", statusCode)
			}

			_, err := ctx.server.UserRepo.GetByID(testUser.ID)
			if err == nil {
				t.Errorf("Expected user to be deleted after confirmation")
			}
		})

		t.Run("confirmation_with_invalid_token_fails", func(t *testing.T) {
			testUser := ctx.createUserWithCleanup(t, "invalidtokenuser", "Password123!")
			authClient := ctx.loginUser(t, testUser.Username, testUser.Password)

			statusCode := authClient.ConfirmAccountDeletion(t, "invalid-token-12345", false)
			if statusCode != http.StatusBadRequest && statusCode != http.StatusTooManyRequests {
				t.Errorf("Expected invalid token confirmation to fail with status 400 or 429, got %d", statusCode)
			}

			statusCode = authClient.ConfirmAccountDeletion(t, "", false)
			if statusCode != http.StatusBadRequest && statusCode != http.StatusTooManyRequests {
				t.Errorf("Expected empty token confirmation to fail with status 400 or 429, got %d", statusCode)
			}

			statusCode = authClient.ConfirmAccountDeletion(t, "not-a-valid-token", false)
			if statusCode != http.StatusBadRequest && statusCode != http.StatusTooManyRequests {
				t.Errorf("Expected malformed token confirmation to fail with status 400 or 429, got %d", statusCode)
			}

			if statusCode != http.StatusTooManyRequests {
				_, err := ctx.server.UserRepo.GetByID(testUser.ID)
				if err != nil {
					t.Errorf("Expected user to still exist after invalid confirmation attempt, got error: %v", err)
				}
			}
		})

		t.Run("deleted_account_cannot_login", func(t *testing.T) {
			testUser := ctx.createUserWithCleanup(t, "deleteduser", "Password123!")
			authClient := ctx.loginUser(t, testUser.Username, testUser.Password)

			ctx.server.EmailSender.Reset()

			authClient.RequestAccountDeletion(t)

			deletionToken := ctx.server.EmailSender.DeletionToken()
			if deletionToken == "" {
				t.Fatalf("Expected account deletion email to contain a deletion token")
			}

			statusCode := authClient.ConfirmAccountDeletion(t, deletionToken, false)
			if statusCode != http.StatusOK {
				t.Errorf("Expected account deletion confirmation to succeed, got status %d", statusCode)
			}

			statusCode = ctx.loginExpectStatus(t, testUser.Username, testUser.Password, http.StatusUnauthorized)
			failIfRateLimited(t, statusCode, "deleted account login")

			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected login with deleted account to fail with status 401, got %d", statusCode)
			}
		})

		t.Run("delete_or_keep_posts_option", func(t *testing.T) {
			testCases := []struct {
				name        string
				deletePosts bool
				postContent string
				shouldExist bool
			}{
				{"keep_posts", false, "This post should be kept", true},
				{"delete_posts", true, "This post should be deleted", false},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					testUser := ctx.createUserWithCleanup(t, tc.name+"user", "Password123!")
					authClient := ctx.loginUser(t, testUser.Username, testUser.Password)

					testPost := authClient.CreatePost(t, "Test Post", fmt.Sprintf("https://example.com/test-%s", tc.name), tc.postContent)

					ctx.server.EmailSender.Reset()

					authClient.RequestAccountDeletion(t)

					deletionToken := ctx.server.EmailSender.DeletionToken()
					if deletionToken == "" {
						t.Fatalf("Expected account deletion email to contain a deletion token")
					}

					statusCode := authClient.ConfirmAccountDeletion(t, deletionToken, tc.deletePosts)
					if statusCode != http.StatusOK {
						t.Errorf("Expected account deletion confirmation to succeed, got status %d", statusCode)
					}

					post, err := ctx.server.PostRepo.GetByID(testPost.ID)
					if tc.shouldExist {
						if err != nil {
							t.Errorf("Expected post to still exist when deletePosts=false, got error: %v", err)
						}
						if post != nil && post.Title != testPost.Title {
							t.Errorf("Expected post to have original title, got '%s'", post.Title)
						}
					} else {
						if err == nil {
							t.Errorf("Expected post to be deleted when deletePosts=true, but post still exists")
						}
					}

					_, err = ctx.server.UserRepo.GetByID(testUser.ID)
					if err == nil {
						t.Errorf("Expected user to be deleted")
					}
				})
			}
		})

		t.Run("expired_token_cannot_be_used", func(t *testing.T) {
			testUser := ctx.createUserWithCleanup(t, "expiredtokenuser", "Password123!")
			authClient := ctx.loginUser(t, testUser.Username, testUser.Password)

			ctx.server.EmailSender.Reset()

			authClient.RequestAccountDeletion(t)

			deletionToken := ctx.server.EmailSender.DeletionToken()
			if deletionToken == "" {
				t.Fatalf("Expected account deletion email to contain a deletion token")
			}

			_, err := ctx.server.UserRepo.GetByID(testUser.ID)
			if err != nil {
				t.Errorf("Expected user to still exist before expired token test")
			}

			deletionRepo := repositories.NewAccountDeletionRepository(ctx.server.DB)
			tokenHash := tokenHash(deletionToken)
			deletionReq, err := deletionRepo.GetByTokenHash(tokenHash)
			if err != nil {
				t.Fatalf("Failed to get deletion request: %v", err)
			}

			deletionReq.ExpiresAt = time.Now().Add(-1 * time.Hour)
			if err := ctx.server.DB.Save(deletionReq).Error; err != nil {
				t.Fatalf("Failed to expire deletion token: %v", err)
			}

			statusCode := authClient.ConfirmAccountDeletion(t, deletionToken, false)
			if statusCode != http.StatusBadRequest && statusCode != http.StatusUnauthorized {
				t.Errorf("Expected expired deletion token to be rejected with status 400 or 401, got %d", statusCode)
			}

			_, err = ctx.server.UserRepo.GetByID(testUser.ID)
			if err != nil {
				t.Errorf("Expected user to still exist after using expired token, got error: %v", err)
			}
		})
	})
}
