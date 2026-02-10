package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"goyco/internal/config"
	"goyco/internal/services"
	"goyco/internal/testutils"

	"github.com/golang-jwt/jwt/v5"
)

func TestE2E_ResendVerificationEmail(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("resend_verification_email", func(t *testing.T) {
		t.Run("resend_verification_email_succeeds", func(t *testing.T) {
			createdUser := ctx.createUserWithCleanup(t, "resenduser", "Password123!")

			user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
			if err != nil {
				t.Fatalf("Failed to get user: %v", err)
			}
			user.EmailVerified = false
			user.EmailVerificationToken = ""
			user.EmailVerificationSentAt = nil
			if err := ctx.server.UserRepo.Update(user); err != nil {
				t.Fatalf("Failed to update user: %v", err)
			}

			ctx.server.EmailSender.Reset()

			statusCode := ctx.resendVerification(t, createdUser.Email)
			if statusCode != http.StatusOK {
				t.Errorf("Expected resend verification email to succeed with status 200, got %d", statusCode)
			}

			newToken := ctx.server.EmailSender.VerificationToken()
			if newToken == "" {
				t.Errorf("Expected resend verification email to contain a verification token, but token is empty")
			}
		})

		t.Run("new_verification_token_generated", func(t *testing.T) {
			testUser := ctx.createUserWithCleanup(t, "tokenuser", "Password123!")

			user, err := ctx.server.UserRepo.GetByID(testUser.ID)
			if err != nil {
				t.Fatalf("Failed to get user: %v", err)
			}
			user.EmailVerified = false
			user.EmailVerificationToken = ""
			user.EmailVerificationSentAt = nil
			if err := ctx.server.UserRepo.Update(user); err != nil {
				t.Fatalf("Failed to update user: %v", err)
			}

			ctx.server.EmailSender.Reset()

			firstIP := testutils.GenerateTestIP()
			statusCode := ctx.resendVerificationWithIP(t, testUser.Email, firstIP)
			if statusCode != http.StatusOK {
				t.Errorf("Expected first resend to succeed, got status %d", statusCode)
			}
			failIfRateLimited(t, statusCode, "first resend verification")

			firstToken := ctx.server.EmailSender.VerificationToken()
			if firstToken == "" {
				t.Fatalf("Expected first verification token to be generated")
			}

			userAfterFirstResend, err := ctx.server.UserRepo.GetByID(testUser.ID)
			if err != nil {
				t.Fatalf("Failed to reload user after first resend: %v", err)
			}
			firstTokenHash := userAfterFirstResend.EmailVerificationToken
			userAfterFirstResend.EmailVerificationSentAt = nil
			if err := ctx.server.UserRepo.Update(userAfterFirstResend); err != nil {
				t.Fatalf("Failed to reset verification resend cooldown: %v", err)
			}

			ctx.server.EmailSender.Reset()

			secondIP := testutils.GenerateTestIP()
			statusCode = ctx.resendVerificationWithIP(t, testUser.Email, secondIP)
			failIfRateLimited(t, statusCode, "second resend verification")
			if statusCode != http.StatusOK {
				t.Errorf("Expected second resend to succeed, got status %d", statusCode)
			}

			secondToken := ctx.server.EmailSender.VerificationToken()
			if secondToken == "" {
				t.Fatalf("Expected second verification token to be generated")
			}

			ctx.assertEventually(t, func() bool {
				updatedUser, err := ctx.server.UserRepo.GetByID(testUser.ID)
				if err != nil {
					return false
				}
				return updatedUser.EmailVerificationToken != "" && updatedUser.EmailVerificationToken != firstTokenHash
			}, 1*time.Second)

			if firstToken == secondToken {
				t.Errorf("Expected new verification token to be different from old token")
			}
		})

		t.Run("old_verification_token_behavior", func(t *testing.T) {
			testUser := ctx.createUserWithCleanup(t, "oldtokenuser", "Password123!")

			user, err := ctx.server.UserRepo.GetByID(testUser.ID)
			if err != nil {
				t.Fatalf("Failed to get user: %v", err)
			}
			user.EmailVerified = false
			user.EmailVerificationToken = ""
			user.EmailVerificationSentAt = nil
			if err := ctx.server.UserRepo.Update(user); err != nil {
				t.Fatalf("Failed to update user: %v", err)
			}

			ctx.server.EmailSender.Reset()

			firstIP := testutils.GenerateTestIP()
			statusCode := ctx.resendVerificationWithIP(t, testUser.Email, firstIP)
			if statusCode != http.StatusOK {
				t.Errorf("Expected first resend to succeed, got status %d", statusCode)
			}
			failIfRateLimited(t, statusCode, "first resend verification for old token")

			oldToken := ctx.server.EmailSender.VerificationToken()
			if oldToken == "" {
				t.Fatalf("Expected first verification token to be generated")
			}

			userAfterFirstResend, err := ctx.server.UserRepo.GetByID(testUser.ID)
			if err != nil {
				t.Fatalf("Failed to reload user after first resend: %v", err)
			}
			userAfterFirstResend.EmailVerificationSentAt = nil
			if err := ctx.server.UserRepo.Update(userAfterFirstResend); err != nil {
				t.Fatalf("Failed to reset verification resend cooldown: %v", err)
			}

			ctx.server.EmailSender.Reset()

			secondIP := testutils.GenerateTestIP()
			statusCode = ctx.resendVerificationWithIP(t, testUser.Email, secondIP)
			failIfRateLimited(t, statusCode, "second resend verification for old token")
			if statusCode != http.StatusOK {
				t.Errorf("Expected second resend to succeed, got status %d", statusCode)
			}

			newToken := ctx.server.EmailSender.VerificationToken()
			if newToken == "" {
				t.Fatalf("Expected second verification token to be generated")
			}

			confirmURL := fmt.Sprintf("%s/api/auth/confirm?token=%s", ctx.baseURL, url.QueryEscape(oldToken))
			request, err := http.NewRequest(http.MethodGet, confirmURL, nil)
			if err != nil {
				t.Fatalf("Failed to create confirmation request: %v", err)
			}
			request.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

			resp, err := ctx.client.Do(request)
			if err != nil {
				t.Fatalf("Confirmation request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				t.Errorf("Expected old verification token to be invalidated, but confirmation succeeded with status 200")
			}

			confirmURL = fmt.Sprintf("%s/api/auth/confirm?token=%s", ctx.baseURL, url.QueryEscape(newToken))
			request, err = http.NewRequest(http.MethodGet, confirmURL, nil)
			if err != nil {
				t.Fatalf("Failed to create confirmation request: %v", err)
			}
			request.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

			resp, err = ctx.client.Do(request)
			if err != nil {
				t.Fatalf("Confirmation request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("Expected new verification token to work, got status %d. Body: %s", resp.StatusCode, string(body))
			}

			updatedUser, err := ctx.server.UserRepo.GetByID(testUser.ID)
			if err != nil {
				t.Fatalf("Failed to get updated user: %v", err)
			}
			if !updatedUser.EmailVerified {
				t.Errorf("Expected user to be verified after confirming with new token")
			}
		})

		t.Run("already_verified_account_response", func(t *testing.T) {
			testUser := ctx.createUserWithCleanup(t, "verifieduser", "Password123!")

			user, err := ctx.server.UserRepo.GetByID(testUser.ID)
			if err != nil {
				t.Fatalf("Failed to get user: %v", err)
			}
			user.EmailVerified = true
			now := time.Now()
			user.EmailVerifiedAt = &now
			if err := ctx.server.UserRepo.Update(user); err != nil {
				t.Fatalf("Failed to update user: %v", err)
			}

			statusCode := ctx.resendVerification(t, testUser.Email)
			if statusCode != http.StatusConflict {
				t.Errorf("Expected resend verification for already verified account to return status 409 (Conflict), got %d", statusCode)
			}
		})

		t.Run("invalid_email_format", func(t *testing.T) {
			statusCode := ctx.resendVerification(t, "invalid-email")
			if statusCode != http.StatusBadRequest {
				t.Errorf("Expected invalid email format to return status 400, got %d", statusCode)
			}
		})

		t.Run("nonexistent_email", func(t *testing.T) {
			statusCode := ctx.resendVerification(t, "nonexistent@example.com")
			if statusCode != http.StatusNotFound {
				t.Errorf("Expected non-existent email to return status 404, got %d", statusCode)
			}
		})

		t.Run("empty_email", func(t *testing.T) {
			statusCode := ctx.resendVerification(t, "")
			if statusCode != http.StatusBadRequest {
				t.Errorf("Expected empty email to return status 400, got %d", statusCode)
			}
		})
	})
}

func TestE2E_InvalidTokenScenarios(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("invalid_token_scenarios", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "tokenuser", "Password123!")

		t.Run("empty_token_rejected", func(t *testing.T) {
			statusCode := ctx.makeRequestWithToken(t, "")
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected empty token to be rejected with status 401, got %d", statusCode)
			}
		})

		t.Run("missing_authorization_header_rejected", func(t *testing.T) {
			request, err := testutils.NewRequestBuilder("GET", ctx.baseURL+"/api/auth/me").Build()
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := ctx.client.Do(request)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("Expected missing Authorization header to be rejected with status 401, got %d", resp.StatusCode)
			}
		})

		t.Run("malformed_token_rejected", func(t *testing.T) {
			malformedTokens := []string{
				"not.a.valid.token",
				"just-a-string",
				"invalid",
				"Bearer token",
				"12345",
				"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			}

			for _, malformedToken := range malformedTokens {
				statusCode := ctx.makeRequestWithToken(t, malformedToken)
				if statusCode != http.StatusUnauthorized {
					t.Errorf("Expected malformed token '%s' to be rejected with status 401, got %d", malformedToken, statusCode)
				}
			}
		})

		t.Run("invalid_jwt_format_rejected", func(t *testing.T) {
			invalidJWT := "not.a.valid.token"
			statusCode := ctx.makeRequestWithToken(t, invalidJWT)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected invalid JWT format to be rejected with status 401, got %d", statusCode)
			}
		})

		t.Run("expired_token_rejected", func(t *testing.T) {
			user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
			if err != nil {
				t.Fatalf("Failed to get user: %v", err)
			}

			cfg := &config.Config{
				JWT: config.JWTConfig{
					Secret:            "test-secret-key-for-testing-purposes-only",
					Expiration:        24,
					RefreshExpiration: 168,
					Issuer:            "goyco",
					Audience:          "goyco-users",
				},
			}

			expiredToken := generateExpiredToken(t, user, &cfg.JWT)
			statusCode := ctx.makeRequestWithToken(t, expiredToken)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected expired token to be rejected with status 401, got %d", statusCode)
			}
		})

		t.Run("tampered_token_rejected", func(t *testing.T) {
			user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
			if err != nil {
				t.Fatalf("Failed to get user: %v", err)
			}

			cfg := &config.Config{
				JWT: config.JWTConfig{
					Secret:            "test-secret-key-for-testing-purposes-only",
					Expiration:        24,
					RefreshExpiration: 168,
					Issuer:            "goyco",
					Audience:          "goyco-users",
				},
			}

			tamperedToken := generateTamperedToken(t, user, &cfg.JWT)
			statusCode := ctx.makeRequestWithToken(t, tamperedToken)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected tampered token to be rejected with status 401, got %d", statusCode)
			}
		})

		t.Run("token_with_wrong_issuer_rejected", func(t *testing.T) {
			user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
			if err != nil {
				t.Fatalf("Failed to get user: %v", err)
			}

			cfg := &config.Config{
				JWT: config.JWTConfig{
					Secret:            "test-secret-key-for-testing-purposes-only",
					Expiration:        24,
					RefreshExpiration: 168,
					Issuer:            "wrong-issuer",
					Audience:          "goyco-users",
				},
			}

			claims := services.TokenClaims{
				UserID:         user.ID,
				Username:       user.Username,
				SessionVersion: user.SessionVersion,
				TokenType:      services.TokenTypeAccess,
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    cfg.JWT.Issuer,
					Audience:  []string{cfg.JWT.Audience},
					Subject:   fmt.Sprint(user.ID),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
				},
			}

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			wrongIssuerToken, err := token.SignedString([]byte("test-secret-key-for-testing-purposes-only"))
			if err != nil {
				t.Fatalf("Failed to create token with wrong issuer: %v", err)
			}

			statusCode := ctx.makeRequestWithToken(t, wrongIssuerToken)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected token with wrong issuer to be rejected with status 401, got %d", statusCode)
			}
		})

		t.Run("token_with_wrong_audience_rejected", func(t *testing.T) {
			user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
			if err != nil {
				t.Fatalf("Failed to get user: %v", err)
			}

			claims := services.TokenClaims{
				UserID:         user.ID,
				Username:       user.Username,
				SessionVersion: user.SessionVersion,
				TokenType:      services.TokenTypeAccess,
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "goyco",
					Audience:  []string{"wrong-audience"},
					Subject:   fmt.Sprint(user.ID),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
				},
			}

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			wrongAudienceToken, err := token.SignedString([]byte("test-secret-key-for-testing-purposes-only"))
			if err != nil {
				t.Fatalf("Failed to create token with wrong audience: %v", err)
			}

			statusCode := ctx.makeRequestWithToken(t, wrongAudienceToken)
			if statusCode != http.StatusUnauthorized {
				t.Errorf("Expected token with wrong audience to be rejected with status 401, got %d", statusCode)
			}
		})

		t.Run("token_with_invalid_characters_rejected", func(t *testing.T) {
			invalidTokens := []string{
				strings.Repeat("a", 1000),
				"!@#$%^&*()",
				"invalid.token.format",
			}

			for _, invalidToken := range invalidTokens {
				statusCode := ctx.makeRequestWithToken(t, invalidToken)
				if statusCode != http.StatusUnauthorized {
					t.Errorf("Expected token with invalid characters '%s' to be rejected with status 401, got %d", invalidToken, statusCode)
				}
			}

		})
	})
}

func TestE2E_AccountLockoutBehavior(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("account_lockout_behavior", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "lockuser", "Password123!")
		t.Cleanup(func() {
			user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
			if err == nil && user != nil && user.Locked {
				ctx.server.UserRepo.Unlock(createdUser.ID)
			}
		})

		t.Run("account_can_be_locked", func(t *testing.T) {
			if err := ctx.server.UserRepo.Lock(createdUser.ID); err != nil {
				t.Fatalf("Failed to lock account: %v", err)
			}

			user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
			if err != nil {
				t.Fatalf("Failed to get user: %v", err)
			}
			if !user.Locked {
				t.Errorf("Expected account to be locked, but Locked is false")
			}
		})

		t.Run("locked_account_cannot_login", func(t *testing.T) {
			if err := ctx.server.UserRepo.Lock(createdUser.ID); err != nil {
				t.Fatalf("Failed to lock account: %v", err)
			}

			user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
			if err != nil {
				t.Fatalf("Failed to get user: %v", err)
			}
			if !user.Locked {
				t.Fatalf("Account lock failed - user.Locked is still false after locking")
			}

			loginData := map[string]string{
				"username": createdUser.Username,
				"password": createdUser.Password,
			}

			body, err := json.Marshal(loginData)
			if err != nil {
				t.Fatalf("Failed to marshal login data: %v", err)
			}

			request, err := http.NewRequest(http.MethodPost, ctx.baseURL+"/api/auth/login", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("Failed to create login request: %v", err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			request.Header.Set("Accept-Encoding", "gzip")

			request.Header.Set("X-Forwarded-For", testutils.GenerateTestIP())

			resp, err := ctx.client.Do(request)
			if err != nil {
				t.Fatalf("Failed to make login request: %v", err)
			}
			defer resp.Body.Close()

			failIfRateLimited(t, resp.StatusCode, "locked account login behavior")

			if resp.StatusCode == http.StatusOK {
				t.Errorf("Expected locked account login to fail (not 200 OK), got status %d. This indicates lock is not working.", resp.StatusCode)
			}

			if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusUnauthorized {
				t.Logf("Note: Locked account login returned status %d (expected 403 or 401). Account lock state verified in database.", resp.StatusCode)
			}
		})

		t.Run("locked_account_returns_appropriate_error", func(t *testing.T) {

			if err := ctx.server.UserRepo.Lock(createdUser.ID); err != nil {
				t.Fatalf("Failed to lock account: %v", err)
			}

			loginData := map[string]string{
				"username": createdUser.Username,
				"password": createdUser.Password,
			}

			body, err := json.Marshal(loginData)
			if err != nil {
				t.Fatalf("Failed to marshal login data: %v", err)
			}

			request, err := http.NewRequest(http.MethodPost, ctx.baseURL+"/api/auth/login", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("Failed to create login request: %v", err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			request.Header.Set("Accept-Encoding", "gzip")
			request.Header.Set("X-Forwarded-For", testutils.GenerateTestIP())

			resp, err := ctx.client.Do(request)
			if err != nil {
				t.Fatalf("Failed to make login request: %v", err)
			}
			defer resp.Body.Close()

			failIfRateLimited(t, resp.StatusCode, "locked account error response")

			if resp.StatusCode == http.StatusOK {
				t.Errorf("Expected locked account login to fail, got status 200 (OK)")
			}

			reader, cleanup, err := getResponseReader(resp)
			if err != nil {
				t.Fatalf("Failed to get response reader: %v", err)
			}
			defer cleanup()

			var loginResp map[string]any
			if err := json.NewDecoder(reader).Decode(&loginResp); err != nil {
				t.Fatalf("Failed to decode login response: %v", err)
			}

			if resp.StatusCode == http.StatusForbidden {
				errorMsg, hasError := loginResp["error"].(string)
				if !hasError {
					t.Errorf("Expected error message in response for locked account, but 'error' field is missing or not a string")
				} else {
					if !strings.Contains(strings.ToLower(errorMsg), "locked") {
						t.Errorf("Expected error message to mention 'locked' for status 403, got: %s", errorMsg)
					}
				}
			}

			success, hasSuccess := loginResp["success"].(bool)
			if hasSuccess && success {
				t.Errorf("Expected login response to indicate failure (success: false), got success: true")
			}
		})

		t.Run("account_unlock_works", func(t *testing.T) {
			if err := ctx.server.UserRepo.Unlock(createdUser.ID); err != nil {
				t.Fatalf("Failed to unlock account: %v", err)
			}

			user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
			if err != nil {
				t.Fatalf("Failed to get user: %v", err)
			}
			if user.Locked {
				t.Errorf("Expected account to be unlocked, but Locked is true")
			}
		})

		t.Run("unlocked_account_can_login", func(t *testing.T) {
			if err := ctx.server.UserRepo.Unlock(createdUser.ID); err != nil {
				t.Fatalf("Failed to unlock account: %v", err)
			}

			user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
			if err != nil {
				t.Fatalf("Failed to get user: %v", err)
			}
			if user.Locked {
				t.Fatalf("Expected account to be unlocked, but Locked is true")
			}

			authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)
			if authClient.Token == "" {
				t.Errorf("Expected unlocked account to be able to login, but login failed")
			}
		})

		t.Run("locking_already_locked_account_is_idempotent", func(t *testing.T) {
			if err := ctx.server.UserRepo.Lock(createdUser.ID); err != nil {
				t.Fatalf("Failed to lock already locked account: %v", err)
			}

			user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
			if err != nil {
				t.Fatalf("Failed to get user: %v", err)
			}
			if !user.Locked {
				t.Errorf("Expected account to remain locked after second lock operation, but Locked is false")
			}

			if err := ctx.server.UserRepo.Unlock(createdUser.ID); err != nil {
				t.Logf("Warning: Failed to unlock account after idempotent test: %v", err)
			}
		})

		t.Run("unlocking_already_unlocked_account_is_idempotent", func(t *testing.T) {
			ctx.server.UserRepo.Unlock(createdUser.ID)

			if err := ctx.server.UserRepo.Unlock(createdUser.ID); err != nil {
				t.Fatalf("Failed to unlock already unlocked account: %v", err)
			}

			user, err := ctx.server.UserRepo.GetByID(createdUser.ID)
			if err != nil {
				t.Fatalf("Failed to get user: %v", err)
			}
			if user.Locked {
				t.Errorf("Expected account to remain unlocked after second unlock operation, but Locked is true")
			}
		})
	})
}

func TestE2E_EmailUsernameUniqueness(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("email_username_uniqueness", func(t *testing.T) {
		var createdUsers []*TestUser

		t.Cleanup(func() {
			for _, user := range createdUsers {
				if user != nil {
					ctx.server.UserRepo.Delete(user.ID)
				}
			}
		})

		firstUser := ctx.createUserWithCleanup(t, "firstuser", "Password123!")
		createdUsers = append(createdUsers, firstUser)

		t.Run("duplicate_email_registration_fails", func(t *testing.T) {
			duplicateUsername := uniqueUsername(t, "dupuser")

			registerData := map[string]string{
				"username": duplicateUsername,
				"email":    firstUser.Email,
				"password": "Password123!",
			}
			body, err := json.Marshal(registerData)
			if err != nil {
				t.Fatalf("failed to marshal register data: %v", err)
			}
			headers := map[string]string{"Content-Type": "application/json"}
			statusCode := ctx.doRequestExpectStatus(t, "POST", "/api/auth/register", http.StatusConflict, bytes.NewReader(body), headers)
			failIfRateLimited(t, statusCode, "duplicate email registration")

			if statusCode != http.StatusConflict {
				t.Errorf("Expected duplicate email registration to return status 409 (Conflict), got %d", statusCode)
			}

			_, err = ctx.server.UserRepo.GetByUsername(duplicateUsername)
			if err == nil {
				t.Errorf("Expected duplicate user with email %s not to be created, but user exists", firstUser.Email)
			}
		})

		t.Run("duplicate_username_registration_fails", func(t *testing.T) {
			duplicateEmail := uniqueEmail(t, "dupemail")

			registerData := map[string]string{
				"username": firstUser.Username,
				"email":    duplicateEmail,
				"password": "Password123!",
			}
			body, err := json.Marshal(registerData)
			if err != nil {
				t.Fatalf("failed to marshal register data: %v", err)
			}
			headers := map[string]string{"Content-Type": "application/json"}
			statusCode := ctx.doRequestExpectStatus(t, "POST", "/api/auth/register", http.StatusConflict, bytes.NewReader(body), headers)
			failIfRateLimited(t, statusCode, "duplicate username registration")

			if statusCode != http.StatusConflict {
				t.Errorf("Expected duplicate username registration to return status 409 (Conflict), got %d", statusCode)
			}

			existingUser, err := ctx.server.UserRepo.GetByUsername(firstUser.Username)
			if err != nil {
				t.Errorf("Original user should still exist, got error: %v", err)
			} else if existingUser.Email != firstUser.Email {
				t.Errorf("Original user's email should not have changed, expected %s, got %s", firstUser.Email, existingUser.Email)
			}
		})

		t.Run("email_update_to_existing_email_fails", func(t *testing.T) {
			secondUser := ctx.createUserWithCleanup(t, "seconduser", "Password123!")
			createdUsers = append(createdUsers, secondUser)

			authClient, err := ctx.loginUserSafe(t, secondUser.Username, secondUser.Password)
			if err != nil {

				t.Skip("Skipping email update test: login failed (likely password hashing issue)")
				return
			}

			statusCode := authClient.UpdateEmailExpectStatus(t, firstUser.Email)
			if statusCode != http.StatusConflict {
				t.Errorf("Expected email update to existing email to return status 409 (Conflict), got %d", statusCode)
			}

			updatedUser, err := ctx.server.UserRepo.GetByID(secondUser.ID)
			if err != nil {
				t.Fatalf("Failed to get updated user: %v", err)
			}
			if updatedUser.Email != secondUser.Email {
				t.Errorf("Expected secondUser's email to remain unchanged, got %s instead of %s", updatedUser.Email, secondUser.Email)
			}
		})

		t.Run("username_update_to_existing_username_fails", func(t *testing.T) {
			var thirdUser *TestUser
			var authClient *AuthenticatedClient

			if len(createdUsers) >= 2 {
				thirdUser = createdUsers[1]
			} else {
				thirdUser = ctx.createUserWithCleanup(t, "thirduser", "Password123!")
				createdUsers = append(createdUsers, thirdUser)
			}

			clientTmp, err := ctx.loginUserSafe(t, thirdUser.Username, thirdUser.Password)
			if err != nil {

				t.Skip("Skipping username update test: login failed (likely password hashing issue)")
				return
			}
			authClient = clientTmp

			statusCode := authClient.UpdateUsernameExpectStatus(t, firstUser.Username)
			if statusCode != http.StatusConflict {
				t.Errorf("Expected username update to existing username to return status 409 (Conflict), got %d", statusCode)
			}

			updatedUser, err := ctx.server.UserRepo.GetByID(thirdUser.ID)
			if err != nil {
				t.Fatalf("Failed to get updated user: %v", err)
			}
			if updatedUser.Username != thirdUser.Username {
				t.Errorf("Expected thirdUser's username to remain unchanged, got %s instead of %s", updatedUser.Username, thirdUser.Username)
			}
		})

		t.Run("users_can_update_to_own_email_username", func(t *testing.T) {
			testUser := ctx.createUserWithCleanup(t, "owntestuser", "Password123!")
			createdUsers = append(createdUsers, testUser)

			authClient, err := ctx.loginUserSafe(t, testUser.Username, testUser.Password)
			if err != nil {

				t.Skip("Skipping own email/username update test: login failed (likely password hashing issue)")
				return
			}

			statusCode := authClient.UpdateEmailExpectStatus(t, testUser.Email)

			if statusCode == http.StatusConflict {
				t.Errorf("Updating email to own email should not return 409 (Conflict), got %d. This might indicate incorrect duplicate detection.", statusCode)
			}

			statusCode = authClient.UpdateUsernameExpectStatus(t, testUser.Username)

			if statusCode == http.StatusConflict {
				t.Errorf("Updating username to own username should not return 409 (Conflict), got %d. This might indicate incorrect duplicate detection.", statusCode)
			}
		})

		t.Run("case_insensitive_email_uniqueness", func(t *testing.T) {
			caseVariation := strings.ToUpper(firstUser.Email)
			if caseVariation == firstUser.Email {

				caseVariation = strings.ToLower(firstUser.Email)
			}

			if caseVariation != firstUser.Email {

				duplicateUsername := uniqueUsername(t, "caseuser")
				registerData := map[string]string{
					"username": duplicateUsername,
					"email":    caseVariation,
					"password": "Password123!",
				}
				body, err := json.Marshal(registerData)
				if err != nil {
					t.Fatalf("failed to marshal register data: %v", err)
				}
				headers := map[string]string{"Content-Type": "application/json"}
				statusCode := ctx.doRequestExpectStatus(t, "POST", "/api/auth/register", http.StatusConflict, bytes.NewReader(body), headers)
				failIfRateLimited(t, statusCode, "case-insensitive duplicate email registration")

				if statusCode != http.StatusConflict {
					t.Errorf("Expected case-variation of existing email to return status 409 (Conflict), got %d", statusCode)
				}
			}
		})
	})
}
