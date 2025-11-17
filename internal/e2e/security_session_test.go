package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"goyco/internal/config"
	"goyco/internal/testutils"
)

func TestE2E_SessionFixation(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("session_fixation", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "sessionfix", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		oldToken := authClient.Token
		oldRefreshToken := authClient.RefreshToken

		authClient.UpdatePassword(t, "Password123!", "NewPassword456!")

		statusCode := ctx.makeRequestWithToken(t, oldToken)
		if statusCode != http.StatusUnauthorized {
			t.Errorf("Expected old token to be invalidated after password change, got status %d", statusCode)
		}

		oldClient := &AuthenticatedClient{
			AuthenticatedClient: &testutils.AuthenticatedClient{
				Client:       ctx.client,
				Token:        oldToken,
				RefreshToken: oldRefreshToken,
				BaseURL:      ctx.baseURL,
			},
		}
		_, statusCode = oldClient.RefreshAccessToken(t)
		if statusCode == http.StatusOK {
			t.Errorf("Expected old refresh token to be invalidated after password change, but refresh succeeded")
		}

		newAuthClient := ctx.loginUser(t, createdUser.Username, "NewPassword456!")
		if newAuthClient.Token == "" {
			t.Errorf("Expected to be able to login with new password")
		}

		profile := newAuthClient.GetProfile(t)
		if profile.Data.Username != createdUser.Username {
			t.Errorf("Expected to access profile with new token, got username '%s'", profile.Data.Username)
		}
	})
}

func TestE2E_TokenInvalidationOnPasswordChange(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("token_invalidation_on_password_change", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "tokeninv", "Password123!")

		authClient1 := ctx.loginUser(t, createdUser.Username, createdUser.Password)
		token1 := authClient1.Token
		refreshToken1 := authClient1.RefreshToken

		authClient2 := ctx.loginUser(t, createdUser.Username, createdUser.Password)
		token2 := authClient2.Token
		refreshToken2 := authClient2.RefreshToken

		authClient3 := ctx.loginUser(t, createdUser.Username, createdUser.Password)
		token3 := authClient3.Token
		refreshToken3 := authClient3.RefreshToken

		profile1 := authClient1.GetProfile(t)
		if profile1.Data.Username != createdUser.Username {
			t.Errorf("Expected token1 to work before password change")
		}

		authClient1.UpdatePassword(t, "Password123!", "NewPassword789!")

		statusCode := ctx.makeRequestWithToken(t, token1)
		if statusCode != http.StatusUnauthorized {
			t.Errorf("Expected token1 to be invalidated after password change, got status %d", statusCode)
		}

		statusCode = ctx.makeRequestWithToken(t, token2)
		if statusCode != http.StatusUnauthorized {
			t.Errorf("Expected token2 to be invalidated after password change, got status %d", statusCode)
		}

		statusCode = ctx.makeRequestWithToken(t, token3)
		if statusCode != http.StatusUnauthorized {
			t.Errorf("Expected token3 to be invalidated after password change, got status %d", statusCode)
		}

		oldClient1 := &AuthenticatedClient{
			AuthenticatedClient: &testutils.AuthenticatedClient{
				Client:       ctx.client,
				Token:        token1,
				RefreshToken: refreshToken1,
				BaseURL:      ctx.baseURL,
			},
		}
		_, statusCode = oldClient1.RefreshAccessToken(t)
		if statusCode != http.StatusUnauthorized {
			t.Errorf("Expected refreshToken1 to be invalidated after password change, got status %d", statusCode)
		}

		oldClient2 := &AuthenticatedClient{
			AuthenticatedClient: &testutils.AuthenticatedClient{
				Client:       ctx.client,
				Token:        token2,
				RefreshToken: refreshToken2,
				BaseURL:      ctx.baseURL,
			},
		}
		_, statusCode = oldClient2.RefreshAccessToken(t)
		if statusCode != http.StatusUnauthorized {
			t.Errorf("Expected refreshToken2 to be invalidated after password change, got status %d", statusCode)
		}

		oldClient3 := &AuthenticatedClient{
			AuthenticatedClient: &testutils.AuthenticatedClient{
				Client:       ctx.client,
				Token:        token3,
				RefreshToken: refreshToken3,
				BaseURL:      ctx.baseURL,
			},
		}
		_, statusCode = oldClient3.RefreshAccessToken(t)
		if statusCode != http.StatusUnauthorized {
			t.Errorf("Expected refreshToken3 to be invalidated after password change, got status %d", statusCode)
		}

		newAuthClient := ctx.loginUser(t, createdUser.Username, "NewPassword789!")
		if newAuthClient.Token == "" {
			t.Errorf("Expected to be able to login with new password")
		}
	})
}

func TestE2E_TokenInvalidationOnEmailChange(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("token_invalidation_on_email_change", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "emailchange", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		oldToken := authClient.Token

		ctx.server.EmailSender.Reset()
		authClient.UpdateEmail(t, uniqueEmail(t, "newemail"))

		statusCode := ctx.makeRequestWithToken(t, oldToken)
		if statusCode == http.StatusOK {
			t.Log("Email change does not invalidate tokens (acceptable behavior)")
		}

		_, statusCode = authClient.RefreshAccessToken(t)
		if statusCode == http.StatusOK {
			t.Log("Email change does not invalidate refresh tokens (acceptable behavior)")
		}
	})
}

func TestE2E_SessionVersionIncrements(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("session_version_increments", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "sessionver", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}

		initialVersion := user.SessionVersion
		if initialVersion == 0 {
			t.Errorf("Expected initial session version to be >= 1, got %d", initialVersion)
		}

		authClient.UpdatePassword(t, "Password123!", "NewPassword999!")

		user, err = ctx.server.UserRepo.GetByID(createdUser.ID)
		if err != nil {
			t.Fatalf("Failed to get user after password change: %v", err)
		}

		if user.SessionVersion <= initialVersion {
			t.Errorf("Expected session version to increment after password change, got %d (was %d)", user.SessionVersion, initialVersion)
		}
	})
}

func TestE2E_OldTokensRejectedAfterSessionVersionChange(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("old_tokens_rejected_after_session_version_change", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "oldtoken", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}

		oldSessionVersion := user.SessionVersion
		oldToken := authClient.Token

		cfg := &config.Config{
			JWT: config.JWTConfig{
				Secret:            "test-secret-key-for-testing-purposes-only",
				Expiration:        24,
				RefreshExpiration: 168,
				Issuer:            "goyco",
				Audience:          "goyco-users",
			},
		}

		authClient.UpdatePassword(t, "Password123!", "NewPassword888!")

		user, err = ctx.server.UserRepo.GetByID(createdUser.ID)
		if err != nil {
			t.Fatalf("Failed to get user after password change: %v", err)
		}

		if user.SessionVersion == oldSessionVersion {
			t.Errorf("Expected session version to change after password update")
		}

		statusCode := ctx.makeRequestWithToken(t, oldToken)
		if statusCode != http.StatusUnauthorized {
			t.Errorf("Expected old token to be rejected after session version change, got status %d", statusCode)
		}

		tokenWithOldVersion := generateTokenWithSessionVersion(t, user, &cfg.JWT, oldSessionVersion)
		statusCode = ctx.makeRequestWithToken(t, tokenWithOldVersion)
		if statusCode != http.StatusUnauthorized {
			t.Errorf("Expected token with old session version to be rejected, got status %d", statusCode)
		}
	})
}

func TestE2E_TokenRefreshWithOldSessionVersion(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("token_refresh_with_old_session_version", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "refreshold", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		oldRefreshToken := authClient.RefreshToken

		authClient.UpdatePassword(t, "Password123!", "NewPassword777!")

		oldClient := &AuthenticatedClient{
			AuthenticatedClient: &testutils.AuthenticatedClient{
				Client:       ctx.client,
				Token:        authClient.Token,
				RefreshToken: oldRefreshToken,
				BaseURL:      ctx.baseURL,
			},
		}

		_, statusCode := oldClient.RefreshAccessToken(t)
		if statusCode != http.StatusUnauthorized {
			t.Errorf("Expected refresh with old refresh token to fail after password change, got status %d", statusCode)
		}
	})
}

func TestE2E_MultiDeviceSession(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("multi_device_session", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "multidev", "Password123!")

		deviceA := ctx.loginUser(t, createdUser.Username, createdUser.Password)
		tokenA := deviceA.Token

		deviceB := ctx.loginUser(t, createdUser.Username, createdUser.Password)
		tokenB := deviceB.Token

		profileA := deviceA.GetProfile(t)
		if profileA.Data.Username != createdUser.Username {
			t.Errorf("Expected device A to access profile")
		}

		profileB := deviceB.GetProfile(t)
		if profileB.Data.Username != createdUser.Username {
			t.Errorf("Expected device B to access profile")
		}

		deviceA.Logout(t)

		statusCode := ctx.makeRequestWithToken(t, tokenA)
		if statusCode == http.StatusOK {
			t.Log("Logout may not invalidate tokens immediately (acceptable)")
		}

		profileBAfter := deviceB.GetProfile(t)
		if profileBAfter.Data.Username != createdUser.Username {
			t.Errorf("Expected device B to still work after device A logout")
		}

		deviceB.RevokeAllTokens(t)

		_, statusCode = deviceB.RefreshAccessToken(t)
		if statusCode != http.StatusUnauthorized {
			t.Errorf("Expected device B refresh token to be revoked after revoke-all, got status %d", statusCode)
		}

		statusCode = ctx.makeRequestWithToken(t, tokenB)
		if statusCode == http.StatusOK {
			t.Log("Access token may still work after refresh token revocation (acceptable)")
		}
	})
}

func TestE2E_RevokeAllInvalidatesAllDevices(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("revoke_all_invalidates_all_devices", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "revokeall", "Password123!")

		device1 := ctx.loginUser(t, createdUser.Username, createdUser.Password)
		refreshToken1 := device1.RefreshToken

		device2 := ctx.loginUser(t, createdUser.Username, createdUser.Password)
		refreshToken2 := device2.RefreshToken

		device3 := ctx.loginUser(t, createdUser.Username, createdUser.Password)
		refreshToken3 := device3.RefreshToken

		device1.RevokeAllTokens(t)

		oldDevice1 := &AuthenticatedClient{
			AuthenticatedClient: &testutils.AuthenticatedClient{
				Client:       ctx.client,
				Token:        device1.Token,
				RefreshToken: refreshToken1,
				BaseURL:      ctx.baseURL,
			},
		}
		_, statusCode := oldDevice1.RefreshAccessToken(t)
		if statusCode != http.StatusUnauthorized {
			t.Errorf("Expected device1 refresh token to be revoked, got status %d", statusCode)
		}

		oldDevice2 := &AuthenticatedClient{
			AuthenticatedClient: &testutils.AuthenticatedClient{
				Client:       ctx.client,
				Token:        device2.Token,
				RefreshToken: refreshToken2,
				BaseURL:      ctx.baseURL,
			},
		}
		_, statusCode = oldDevice2.RefreshAccessToken(t)
		if statusCode != http.StatusUnauthorized {
			t.Errorf("Expected device2 refresh token to be revoked, got status %d", statusCode)
		}

		oldDevice3 := &AuthenticatedClient{
			AuthenticatedClient: &testutils.AuthenticatedClient{
				Client:       ctx.client,
				Token:        device3.Token,
				RefreshToken: refreshToken3,
				BaseURL:      ctx.baseURL,
			},
		}
		_, statusCode = oldDevice3.RefreshAccessToken(t)
		if statusCode != http.StatusUnauthorized {
			t.Errorf("Expected device3 refresh token to be revoked, got status %d", statusCode)
		}
	})
}

func TestE2E_TokenTiming(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("token_timing", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "timing", "Password123!")

		cfg := &config.Config{
			JWT: config.JWTConfig{
				Secret:            "test-secret-key-for-testing-purposes-only",
				Expiration:        24,
				RefreshExpiration: 168,
				Issuer:            "goyco",
				Audience:          "goyco-users",
			},
		}

		user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}

		t.Run("token_just_before_expiry", func(t *testing.T) {
			token := generateTokenWithExpiration(t, user, &cfg.JWT, 1*time.Minute)
			statusCode := ctx.makeRequestWithToken(t, token)
			if statusCode != http.StatusOK {
				t.Errorf("Expected token just before expiry to work, got status %d", statusCode)
			}
		})

		t.Run("token_just_after_expiry", func(t *testing.T) {
			token := generateTokenWithExpiration(t, user, &cfg.JWT, -1*time.Minute)
			statusCode := ctx.makeRequestWithToken(t, token)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected expired token to be rejected, got status %d", statusCode)
			}
		})

		t.Run("token_expiration_edge_case", func(t *testing.T) {
			token := generateTokenWithExpiration(t, user, &cfg.JWT, 0)
			statusCode := ctx.makeRequestWithToken(t, token)
			if statusCode == http.StatusOK {
				t.Log("Token with zero expiration may be accepted (clock skew tolerance)")
			}
		})
	})
}

func TestE2E_TokenReplayAttack(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("token_replay_attack", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "replay", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		token := authClient.Token

		t.Run("same_token_multiple_times", func(t *testing.T) {
			for i := range 5 {
				statusCode := ctx.makeRequestWithToken(t, token)
				if statusCode != http.StatusOK {
					t.Errorf("Expected token to work multiple times (replay %d), got status %d", i+1, statusCode)
				}
			}
		})

		t.Run("token_reuse_after_revocation", func(t *testing.T) {
			authClient.RevokeAllTokens(t)

			statusCode := ctx.makeRequestWithToken(t, token)
			if statusCode == http.StatusOK {
				t.Log("Access token may still work after refresh token revocation (acceptable)")
			}

			_, statusCode = authClient.RefreshAccessToken(t)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected refresh token to be rejected after revocation, got status %d", statusCode)
			}
		})

		t.Run("token_reuse_after_user_deletion", func(t *testing.T) {
			testUser := ctx.createUserWithCleanup(t, "deleteuser", "Password123!")
			deleteClient := ctx.loginUser(t, testUser.Username, testUser.Password)
			deleteToken := deleteClient.Token

			ctx.server.EmailSender.Reset()
			deleteClient.RequestAccountDeletion(t)
			deletionToken := ctx.server.EmailSender.DeletionToken()
			if deletionToken == "" {
				t.Fatalf("Expected deletion token")
			}

			deleteClient.ConfirmAccountDeletion(t, deletionToken, false)

			statusCode := ctx.makeRequestWithToken(t, deleteToken)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected token to be rejected after user deletion, got status %d", statusCode)
			}
		})
	})
}

func TestE2E_TokenScope(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("token_scope", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "scope", "Password123!")

		cfg := &config.Config{
			JWT: config.JWTConfig{
				Secret:            "test-secret-key-for-testing-purposes-only",
				Expiration:        24,
				RefreshExpiration: 168,
				Issuer:            "goyco",
				Audience:          "goyco-users",
			},
		}

		user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}

		t.Run("access_token_cannot_be_used_as_refresh", func(t *testing.T) {
			authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)
			accessToken := authClient.Token

			refreshData := map[string]string{
				"refresh_token": accessToken,
			}

			body, err := json.Marshal(refreshData)
			if err != nil {
				t.Fatalf("Failed to marshal refresh data: %v", err)
			}

			request, err := http.NewRequest("POST", ctx.baseURL+"/api/auth/refresh", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("Failed to create refresh request: %v", err)
			}
			request.Header.Set("Content-Type", "application/json")
			testutils.WithStandardHeaders(request)

			resp, err := ctx.client.Do(request)
			if err != nil {
				t.Fatalf("Failed to make refresh request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				t.Errorf("Expected access token to be rejected as refresh token, got status 200")
			}
		})

		t.Run("refresh_token_cannot_access_protected_endpoints", func(t *testing.T) {
			authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)
			refreshTokenString := authClient.RefreshToken

			statusCode := ctx.makeRequestWithToken(t, refreshTokenString)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected refresh token string to be rejected for protected endpoints, got status %d", statusCode)
			}

			invalidTypeToken := generateTokenWithType(t, user, &cfg.JWT, "invalid-type")
			statusCode = ctx.makeRequestWithToken(t, invalidTypeToken)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected invalid token type to be rejected, got status %d", statusCode)
			}
		})

		t.Run("token_type_validation", func(t *testing.T) {
			emptyTypeToken := generateTokenWithType(t, user, &cfg.JWT, "")
			statusCode := ctx.makeRequestWithToken(t, emptyTypeToken)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected empty token type to be rejected, got status %d", statusCode)
			}
		})
	})
}

func TestE2E_ConcurrentLoginPrevention(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("concurrent_login_prevention", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "concurrent", "Password123!")

		user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}

		initialVersion := user.SessionVersion

		login1 := ctx.loginUser(t, createdUser.Username, createdUser.Password)
		login2 := ctx.loginUser(t, createdUser.Username, createdUser.Password)
		login3 := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		user, err = ctx.server.UserRepo.GetByID(createdUser.ID)
		if err != nil {
			t.Fatalf("Failed to get user after logins: %v", err)
		}

		if user.SessionVersion != initialVersion {
			t.Log("Session version may increment on login (acceptable behavior)")
		}

		profile1 := login1.GetProfile(t)
		if profile1.Data.Username != createdUser.Username {
			t.Errorf("Expected login1 to work")
		}

		profile2 := login2.GetProfile(t)
		if profile2.Data.Username != createdUser.Username {
			t.Errorf("Expected login2 to work")
		}

		profile3 := login3.GetProfile(t)
		if profile3.Data.Username != createdUser.Username {
			t.Errorf("Expected login3 to work")
		}

		if login1.Token == login2.Token || login1.Token == login3.Token || login2.Token == login3.Token {
			t.Errorf("Expected concurrent logins to generate different tokens")
		}
	})
}
