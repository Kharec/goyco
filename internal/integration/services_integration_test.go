package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"goyco/internal/database"
	"goyco/internal/repositories"
	"goyco/internal/services"
	"goyco/internal/testutils"

	"github.com/golang-jwt/jwt/v5"
)

func TestIntegration_Services(t *testing.T) {
	suite := testutils.NewServiceSuite(t)

	authService, err := services.NewAuthFacadeForTest(testutils.AppTestConfig, suite.UserRepo, suite.PostRepo, suite.DeletionRepo, suite.RefreshTokenRepo, suite.EmailSender)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	voteService := services.NewVoteService(suite.VoteRepo, suite.PostRepo, suite.DB)
	emailSender := suite.EmailSender
	userRepo := suite.UserRepo
	deletionRepo := suite.DeletionRepo
	postRepo := suite.PostRepo
	titleFetcher := suite.TitleFetcher

	t.Run("Auth_Complete_User_Lifecycle", func(t *testing.T) {
		emailSender.Reset()
		registerResult, err := authService.Register("lifecycle_user", "lifecycle@example.com", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to register user: %v", err)
		}

		if registerResult.User.Username != "lifecycle_user" {
			t.Errorf("Expected username 'lifecycle_user', got '%s'", registerResult.User.Username)
		}

		verificationToken := setupVerificationTokenForTest(t, emailSender, userRepo, "lifecycle_user")

		_, err = authService.ConfirmEmail(verificationToken)
		if err != nil {
			t.Fatalf("Failed to confirm email: %v", err)
		}

		loginResult, err := authService.Login("lifecycle_user", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to login user: %v", err)
		}

		if loginResult.User.Username != "lifecycle_user" {
			t.Errorf("Expected username 'lifecycle_user', got '%s'", loginResult.User.Username)
		}

		updateResult, err := authService.UpdateUsername(loginResult.User.ID, "updated_lifecycle_user")
		if err != nil {
			t.Fatalf("Failed to update username: %v", err)
		}

		if updateResult.Username != "updated_lifecycle_user" {
			t.Errorf("Expected updated username, got '%s'", updateResult.Username)
		}

		emailSender.Reset()
		emailResult, err := authService.UpdateEmail(loginResult.User.ID, "updated@example.com")
		if err != nil {
			t.Fatalf("Failed to update email: %v", err)
		}

		if emailResult.Email != "updated@example.com" {
			t.Errorf("Expected updated email, got '%s'", emailResult.Email)
		}

		updatedToken := setupVerificationTokenForTest(t, emailSender, userRepo, "updated_lifecycle_user")

		_, err = authService.ConfirmEmail(updatedToken)
		if err != nil {
			t.Fatalf("Failed to confirm updated email: %v", err)
		}

		_, err = authService.UpdatePassword(loginResult.User.ID, "SecurePass123!", "NewSecurePass123!")
		if err != nil {
			t.Fatalf("Failed to update password: %v", err)
		}

		_, err = authService.Login("updated_lifecycle_user", "NewSecurePass123!")
		if err != nil {
			t.Fatalf("Failed to login with new password: %v", err)
		}
	})

	t.Run("Auth_Security_Validation", func(t *testing.T) {
		emailSender.Reset()
		_, err := authService.Register("weak_user", "weak@example.com", "123")
		if err == nil {
			t.Error("Expected error for weak password")
		}

		_, err = authService.Register("invalid_user", "not-an-email", "SecurePass123!")
		if err == nil {
			t.Error("Expected error for invalid email")
		}

		_, err = authService.Register("duplicate_user", "duplicate1@example.com", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to register first user: %v", err)
		}

		_, err = authService.Register("duplicate_user", "duplicate2@example.com", "SecurePass123!")
		if err == nil {
			t.Error("Expected error for duplicate username")
		}

		_, err = authService.Register("another_user", "duplicate1@example.com", "SecurePass123!")
		if err == nil {
			t.Error("Expected error for duplicate email")
		}
	})

	t.Run("Auth_Account_Deletion_Workflow", func(t *testing.T) {
		emailSender.Reset()
		registerResult, err := authService.Register("deletion_user", "deletion@example.com", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to register user: %v", err)
		}

		verificationToken := setupVerificationTokenForTest(t, emailSender, userRepo, "deletion_user")

		_, err = authService.ConfirmEmail(verificationToken)
		if err != nil {
			t.Fatalf("Failed to confirm email: %v", err)
		}

		err = authService.RequestAccountDeletion(registerResult.User.ID)
		if err != nil {
			t.Fatalf("Failed to request account deletion: %v", err)
		}

		deletionToken := setupDeletionTokenForTest(t, emailSender, deletionRepo, registerResult.User.ID)

		err = authService.ConfirmAccountDeletion(deletionToken)
		if err != nil {
			t.Fatalf("Failed to confirm account deletion: %v", err)
		}

		if err := authService.ConfirmAccountDeletion(deletionToken); !errors.Is(err, services.ErrInvalidDeletionToken) {
			t.Fatalf("Expected token reuse to return ErrInvalidDeletionToken, got %v", err)
		}
	})

	t.Run("Auth_Locked_User_Session_Invalidation", func(t *testing.T) {
		user := &database.User{
			Username:      "locked_user",
			Email:         "locked@example.com",
			Password:      "$2a$10$abcdefghijklmnopqrstuvwxyz",
			EmailVerified: true,
		}
		if err := userRepo.Create(user); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		now := time.Now()
		claims := services.TokenClaims{
			UserID:         user.ID,
			Username:       user.Username,
			SessionVersion: user.SessionVersion,
			TokenType:      services.TokenTypeAccess,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    testutils.AppTestConfig.JWT.Issuer,
				Audience:  []string{testutils.AppTestConfig.JWT.Audience},
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
				Subject:   fmt.Sprint(user.ID),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(testutils.AppTestConfig.JWT.Secret))
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		userID, err := authService.VerifyToken(tokenString)
		if err != nil {
			t.Fatalf("Token should be valid before locking: %v", err)
		}
		if userID != user.ID {
			t.Fatalf("Expected user ID %d, got %d", user.ID, userID)
		}

		if err := userRepo.Lock(user.ID); err != nil {
			t.Fatalf("Failed to lock user: %v", err)
		}

		_, err = authService.VerifyToken(tokenString)
		if !errors.Is(err, services.ErrAccountLocked) {
			t.Fatalf("Expected ErrAccountLocked, got %v", err)
		}

		if err := userRepo.Unlock(user.ID); err != nil {
			t.Fatalf("Failed to unlock user: %v", err)
		}

		userID, err = authService.VerifyToken(tokenString)
		if err != nil {
			t.Fatalf("Token should be valid after unlock: %v", err)
		}
		if userID != user.ID {
			t.Fatalf("Expected user ID %d, got %d", user.ID, userID)
		}

		userRepo.HardDelete(user.ID)
	})

	t.Run("Auth_Password_Change_Session_Invalidation", func(t *testing.T) {
		user := &database.User{
			Username:       "password_test_user",
			Email:          "password_test@example.com",
			Password:       "$2a$10$abcdefghijklmnopqrstuvwxyz",
			EmailVerified:  true,
			SessionVersion: 1,
		}
		if err := userRepo.Create(user); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		now := time.Now()
		claims := services.TokenClaims{
			UserID:         user.ID,
			Username:       user.Username,
			SessionVersion: 1,
			TokenType:      services.TokenTypeAccess,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    testutils.AppTestConfig.JWT.Issuer,
				Audience:  []string{testutils.AppTestConfig.JWT.Audience},
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
				Subject:   fmt.Sprint(user.ID),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(testutils.AppTestConfig.JWT.Secret))
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		userID, err := authService.VerifyToken(tokenString)
		if err != nil {
			t.Fatalf("Token should be valid before password change: %v", err)
		}
		if userID != user.ID {
			t.Fatalf("Expected user ID %d, got %d", user.ID, userID)
		}

		if err := authService.InvalidateAllSessions(user.ID); err != nil {
			t.Fatalf("Failed to invalidate sessions: %v", err)
		}

		_, err = authService.VerifyToken(tokenString)
		if err == nil {
			t.Fatalf("Token should be invalid after session invalidation")
		}

		userRepo.HardDelete(user.ID)
	})

	t.Run("Auth_Email_Change_Verification_Template", func(t *testing.T) {
		user := &database.User{
			Username:       "email_change_user",
			Email:          "old@example.com",
			Password:       "$2a$10$abcdefghijklmnopqrstuvwxyz",
			EmailVerified:  true,
			SessionVersion: 1,
		}
		if err := userRepo.Create(user); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		emailService, err := services.NewEmailService(testutils.AppTestConfig, suite.EmailSender)
		if err != nil {
			t.Fatalf("Failed to create email service: %v", err)
		}
		verificationURL := "https://example.com/confirm?token=test123"
		body := emailService.GenerateEmailChangeVerificationEmailBody(user.Username, verificationURL)

		if !strings.Contains(body, "Confirm your new email address") {
			t.Error("Email should contain 'Confirm your new email address'")
		}
		if !strings.Contains(body, "You've requested to change your email address") {
			t.Error("Email should contain email change specific message")
		}
		if !strings.Contains(body, "Confirm New Email Address") {
			t.Error("Email should contain 'Confirm New Email Address' button text")
		}
		if !strings.Contains(body, "your new email address will be active") {
			t.Error("Email should mention that new email will be active")
		}
		if !strings.Contains(body, "If you didn't request this email change") {
			t.Error("Email should contain security warning about email change")
		}

		userRepo.HardDelete(user.ID)
	})

	t.Run("Vote_Service_Complete_Workflow", func(t *testing.T) {
		emailSender.Reset()
		user := createTestUserWithAuth(authService, emailSender, suite.UserRepo, "vote_user", "vote@example.com")
		post := testutils.CreatePostWithRepo(t, postRepo, user.ID, "Vote Test Post", "https://example.com/vote-test")

		voteRequest := services.VoteRequest{
			UserID: user.ID,
			PostID: post.ID,
			Type:   "up",
		}
		voteResult, err := voteService.CastVote(voteRequest)
		if err != nil {
			t.Fatalf("Failed to cast vote: %v", err)
		}

		if voteResult.Type != database.VoteUp {
			t.Errorf("Expected vote type 'up', got '%v'", voteResult.Type)
		}

		userVote, err := voteService.GetUserVote(user.ID, post.ID, "127.0.0.1", "test")
		if err != nil {
			t.Fatalf("Failed to get user vote: %v", err)
		}

		if userVote == nil || userVote.Type != database.VoteUp {
			t.Errorf("Expected user vote type 'up', got '%v'", userVote)
		}

		votes, err := voteService.GetPostVotes(post.ID)
		if err != nil {
			t.Fatalf("Failed to get post votes: %v", err)
		}

		totalVotes := len(votes)
		if totalVotes != 1 {
			t.Errorf("Expected 1 vote, got %d", totalVotes)
		}

		voteRequest = services.VoteRequest{
			UserID: user.ID,
			PostID: post.ID,
			Type:   "down",
		}
		voteResult, err = voteService.CastVote(voteRequest)
		if err != nil {
			t.Fatalf("Failed to change vote: %v", err)
		}

		if voteResult.Type != database.VoteDown {
			t.Errorf("Expected vote type 'down', got '%v'", voteResult.Type)
		}

		removeRequest := services.VoteRequest{
			UserID: user.ID,
			PostID: post.ID,
			Type:   database.VoteNone,
		}
		_, err = voteService.CastVote(removeRequest)
		if err != nil {
			t.Fatalf("Failed to remove vote: %v", err)
		}

		_, err = voteService.GetUserVote(user.ID, post.ID, "127.0.0.1", "test")
		if err == nil {
			t.Error("Expected error when getting removed vote")
		}
	})

	t.Run("Vote_Service_Concurrent_Operations", func(t *testing.T) {
		emailSender.Reset()
		users := make([]*database.User, 5)
		for i := range 5 {
			users[i] = createTestUserWithAuth(authService, emailSender, suite.UserRepo, fmt.Sprintf("concurrent_user_%d", i), fmt.Sprintf("concurrent%d@example.com", i))
		}

		post := testutils.CreatePostWithRepo(t, postRepo, users[0].ID, "Concurrent Vote Post", "https://example.com/concurrent-vote")

		var wg sync.WaitGroup
		errors := make(chan error, len(users))

		for i, user := range users {
			wg.Add(1)
			go func(index int, u *database.User) {
				defer wg.Done()

				voteType := database.VoteUp
				if index%2 == 0 {
					voteType = database.VoteDown
				}

				voteRequest := services.VoteRequest{
					UserID: u.ID,
					PostID: post.ID,
					Type:   voteType,
				}
				_, err := voteService.CastVote(voteRequest)
				if err != nil {
					errors <- fmt.Errorf("failed to cast vote for user %d: %v", index, err)
				}
			}(i, user)
		}

		wg.Wait()
		close(errors)

		var errs []error
		for err := range errors {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			t.Fatalf("concurrent vote failures: %v", errs)
		}

		votes, err := voteService.GetPostVotes(post.ID)
		if err != nil {
			t.Fatalf("Failed to get post votes: %v", err)
		}

		totalVotes := len(votes)
		if totalVotes != 5 {
			t.Errorf("Expected 5 votes, got %d", totalVotes)
		}
	})

	t.Run("Title_Fetcher_Functionality", func(t *testing.T) {
		emailSender.Reset()
		titleFetcher.SetTitle("Mock Title")
		title, err := titleFetcher.FetchTitle(context.Background(), "https://example.com/test")
		if err != nil {
			t.Fatalf("Failed to fetch title: %v", err)
		}

		if title != "Mock Title" {
			t.Errorf("Expected title 'Mock Title', got '%s'", title)
		}
	})

	t.Run("Error_Handling_Invalid_Operations", func(t *testing.T) {
		emailSender.Reset()
		user := createTestUserWithAuth(authService, emailSender, suite.UserRepo, "error_user", "error@example.com")
		voteRequest := services.VoteRequest{
			UserID: user.ID,
			PostID: 99999,
			Type:   "up",
		}
		_, err := voteService.CastVote(voteRequest)
		if err == nil {
			t.Error("Expected error when voting on non-existent post")
		}

		post := testutils.CreatePostWithRepo(t, postRepo, user.ID, "Error Test Post", "https://example.com/error-test")
		voteRequest = services.VoteRequest{
			UserID: 99999,
			PostID: post.ID,
			Type:   "up",
		}
		_, err = voteService.CastVote(voteRequest)
		if err == nil {
			t.Error("Expected error when voting with non-existent user")
		}

		voteRequest = services.VoteRequest{
			UserID: user.ID,
			PostID: post.ID,
			Type:   "invalid",
		}
		_, err = voteService.CastVote(voteRequest)
		if err == nil {
			t.Error("Expected error for invalid vote type")
		}
	})

	t.Run("Data_Consistency_Cross_Services", func(t *testing.T) {
		emailSender.Reset()
		user := createTestUserWithAuth(authService, emailSender, suite.UserRepo, "consistency_user", "consistency@example.com")

		post := testutils.CreatePostWithRepo(t, postRepo, user.ID, "Consistency Test Post", "https://example.com/consistency")

		voters := make([]*database.User, 3)
		for i := range 3 {
			voters[i] = createTestUserWithAuth(authService, emailSender, suite.UserRepo, fmt.Sprintf("voter_%d", i), fmt.Sprintf("voter%d@example.com", i))
		}

		for i, voter := range voters {
			voteType := database.VoteUp
			if i%2 == 0 {
				voteType = database.VoteDown
			}
			voteRequest := services.VoteRequest{
				UserID: voter.ID,
				PostID: post.ID,
				Type:   voteType,
			}
			_, err := voteService.CastVote(voteRequest)
			if err != nil {
				t.Fatalf("Failed to cast vote %d: %v", i, err)
			}
		}

		votes, err := voteService.GetPostVotes(post.ID)
		if err != nil {
			t.Fatalf("Failed to get post votes: %v", err)
		}

		totalVotes := len(votes)
		if totalVotes != 3 {
			t.Errorf("Expected 3 votes, got %d", totalVotes)
		}

		for i, voter := range voters {
			userVote, err := voteService.GetUserVote(voter.ID, post.ID, "127.0.0.1", "test")
			if err != nil {
				t.Fatalf("Failed to get user vote %d: %v", i, err)
			}

			expectedType := database.VoteUp
			if i%2 == 0 {
				expectedType = database.VoteDown
			}

			if userVote.Type != expectedType {
				t.Errorf("Expected vote type '%v' for user %d, got '%v'", expectedType, i, userVote.Type)
			}
		}
	})

	t.Run("EmailSender_Integration", func(t *testing.T) {
		sender := testutils.GetSMTPSenderFromEnv(t)

		recipient := os.Getenv("SMTP_TEST_RECIPIENT")
		if strings.TrimSpace(recipient) == "" {
			recipient = sender.From
		}

		subject := fmt.Sprintf("Test Subject %d", time.Now().UnixNano())
		body := fmt.Sprintf("Test Body sent at %s", time.Now().Format(time.RFC3339))

		err := sender.Send(recipient, subject, body)
		if err != nil {
			t.Errorf("Send failed: %v", err)
		}
	})

	t.Run("EmailSender_HTML_Email", func(t *testing.T) {
		sender := testutils.GetSMTPSenderFromEnv(t)

		recipient := os.Getenv("SMTP_TEST_RECIPIENT")
		if strings.TrimSpace(recipient) == "" {
			recipient = sender.From
		}

		htmlBody := "<html><body><h1>Test</h1><p>This is a test email.</p></body></html>"
		err := sender.Send(recipient, "HTML Test Subject", htmlBody)
		if err != nil {
			t.Errorf("Send failed: %v", err)
		}
	})

	t.Run("EmailSender_Async_Email", func(t *testing.T) {
		sender := testutils.GetSMTPSenderFromEnv(t)

		recipient := os.Getenv("SMTP_TEST_RECIPIENT")
		if strings.TrimSpace(recipient) == "" {
			recipient = sender.From
		}

		asyncBody := fmt.Sprintf("Async Test Body sent at %s", time.Now().Format(time.RFC3339))
		err := sender.Send(recipient, "Async Test Subject", asyncBody)
		if err != nil {
			t.Errorf("Send failed: %v", err)
		}
	})

	t.Run("Refresh_Token_Complete_Workflow", func(t *testing.T) {
		emailSender.Reset()
		user := createTestUserWithAuth(authService, emailSender, suite.UserRepo, "refresh_user", "refresh@example.com")

		loginResult, err := authService.Login("refresh_user", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		if loginResult.RefreshToken == "" {
			t.Fatal("Login should return a refresh token")
		}

		newAccessToken, err := authService.RefreshAccessToken(loginResult.RefreshToken)
		if err != nil {
			t.Fatalf("Failed to refresh access token: %v", err)
		}

		if newAccessToken.AccessToken == "" {
			t.Fatal("Refresh should return a new access token")
		}

		if newAccessToken.AccessToken == loginResult.AccessToken {
			t.Error("New access token should be different from original")
		}

		userID, err := authService.VerifyToken(newAccessToken.AccessToken)
		if err != nil {
			t.Fatalf("New access token should be valid: %v", err)
		}

		if userID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, userID)
		}
	})

	t.Run("Refresh_Token_Expiration", func(t *testing.T) {
		emailSender.Reset()
		createTestUserWithAuth(authService, emailSender, suite.UserRepo, "expire_user", "expire@example.com")

		loginResult, err := authService.Login("expire_user", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		refreshToken, err := suite.RefreshTokenRepo.GetByTokenHash(testutils.HashVerificationToken(loginResult.RefreshToken))
		if err != nil {
			t.Fatalf("Failed to get refresh token: %v", err)
		}

		refreshToken.ExpiresAt = time.Now().Add(-1 * time.Hour)
		if err := suite.DB.Model(refreshToken).Update("expires_at", refreshToken.ExpiresAt).Error; err != nil {
			t.Fatalf("Failed to update token expiration: %v", err)
		}

		_, err = authService.RefreshAccessToken(loginResult.RefreshToken)
		if err == nil {
			t.Error("Expected error for expired refresh token")
		}
	})

	t.Run("Refresh_Token_Revocation", func(t *testing.T) {
		emailSender.Reset()
		createTestUserWithAuth(authService, emailSender, suite.UserRepo, "revoke_user", "revoke@example.com")

		loginResult, err := authService.Login("revoke_user", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		err = authService.RevokeRefreshToken(loginResult.RefreshToken)
		if err != nil {
			t.Fatalf("Failed to revoke refresh token: %v", err)
		}

		_, err = authService.RefreshAccessToken(loginResult.RefreshToken)
		if err == nil {
			t.Error("Expected error for revoked refresh token")
		}
	})

	t.Run("Refresh_Token_Multiple_Tokens", func(t *testing.T) {
		emailSender.Reset()
		user := createTestUserWithAuth(authService, emailSender, suite.UserRepo, "multi_token_user", "multi@example.com")

		login1, err := authService.Login("multi_token_user", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed first login: %v", err)
		}

		login2, err := authService.Login("multi_token_user", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed second login: %v", err)
		}

		if login1.RefreshToken == login2.RefreshToken {
			t.Error("Each login should generate a unique refresh token")
		}

		accessToken1, err := authService.RefreshAccessToken(login1.RefreshToken)
		if err != nil {
			t.Fatalf("Failed to refresh with first token: %v", err)
		}

		accessToken2, err := authService.RefreshAccessToken(login2.RefreshToken)
		if err != nil {
			t.Fatalf("Failed to refresh with second token: %v", err)
		}

		if accessToken1.AccessToken == accessToken2.AccessToken {
			t.Error("Different refresh tokens should generate different access tokens")
		}

		userID1, err := authService.VerifyToken(accessToken1.AccessToken)
		if err != nil {
			t.Fatalf("First access token should be valid: %v", err)
		}

		userID2, err := authService.VerifyToken(accessToken2.AccessToken)
		if err != nil {
			t.Fatalf("Second access token should be valid: %v", err)
		}

		if userID1 != user.ID || userID2 != user.ID {
			t.Error("Both tokens should belong to the same user")
		}
	})

	t.Run("Refresh_Token_Revoke_All", func(t *testing.T) {
		emailSender.Reset()
		user := createTestUserWithAuth(authService, emailSender, suite.UserRepo, "revoke_all_user", "revoke_all@example.com")

		login1, err := authService.Login("revoke_all_user", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed first login: %v", err)
		}

		login2, err := authService.Login("revoke_all_user", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed second login: %v", err)
		}

		err = authService.RevokeAllUserTokens(user.ID)
		if err != nil {
			t.Fatalf("Failed to revoke all tokens: %v", err)
		}

		_, err = authService.RefreshAccessToken(login1.RefreshToken)
		if err == nil {
			t.Error("Expected error for revoked refresh token")
		}

		_, err = authService.RefreshAccessToken(login2.RefreshToken)
		if err == nil {
			t.Error("Expected error for revoked refresh token")
		}
	})

}

func createTestUserWithAuth(authService interface {
	Register(username, email, password string) (*services.RegistrationResult, error)
	ConfirmEmail(token string) (*database.User, error)
}, emailSender interface {
	Reset()
	VerificationToken() string
}, userRepo repositories.UserRepository, username, email string) *database.User {
	emailSender.Reset()

	_, err := authService.Register(username, email, "SecurePass123!")
	if err != nil {
		panic(fmt.Sprintf("Failed to register user: %v", err))
	}

	verificationToken := emailSender.VerificationToken()
	if verificationToken == "" {
		panic("Failed to capture verification token during test setup")
	}

	hashedToken := testutils.HashVerificationToken(verificationToken)

	user, err := userRepo.GetByUsername(username)
	if err != nil {
		panic(fmt.Sprintf("Failed to get user: %v", err))
	}
	user.EmailVerificationToken = hashedToken
	if err := userRepo.Update(user); err != nil {
		panic(fmt.Sprintf("Failed to update user with hashed token: %v", err))
	}

	confirmResult, err := authService.ConfirmEmail(verificationToken)
	if err != nil {
		panic(fmt.Sprintf("Failed to confirm email: %v", err))
	}

	return confirmResult
}

func setupVerificationTokenForTest(t *testing.T, emailSender *testutils.MockEmailSender, userRepo repositories.UserRepository, username string) string {
	t.Helper()

	verificationToken := emailSender.VerificationToken()
	if verificationToken == "" {
		t.Fatal("Expected verification token to be generated")
	}

	hashedToken := testutils.HashVerificationToken(verificationToken)

	user, err := userRepo.GetByUsername(username)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	user.EmailVerificationToken = hashedToken
	if err := userRepo.Update(user); err != nil {
		t.Fatalf("Failed to update user with hashed token: %v", err)
	}

	return verificationToken
}

func setupDeletionTokenForTest(t *testing.T, emailSender *testutils.MockEmailSender, deletionRepo repositories.AccountDeletionRepository, userID uint) string {
	t.Helper()

	deletionToken := emailSender.DeletionToken()
	if deletionToken == "" {
		t.Fatal("Expected deletion token to be generated")
	}

	hashedToken := testutils.HashVerificationToken(deletionToken)

	if err := deletionRepo.DeleteByUserID(userID); err != nil {
		t.Fatalf("Cannot delete user %d", userID)
	}

	req := &database.AccountDeletionRequest{
		UserID:    userID,
		TokenHash: hashedToken,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := deletionRepo.Create(req); err != nil {
		t.Fatalf("Failed to create account deletion request: %v", err)
	}

	return deletionToken
}
