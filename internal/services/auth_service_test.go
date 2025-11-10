package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"goyco/internal/database"
	"goyco/internal/repositories"
	"goyco/internal/testutils"
)

func testHashVerificationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func setupAuthService(t *testing.T) (*AuthFacade, *testutils.ServiceSuite) {
	t.Helper()
	suite := testutils.NewServiceSuite(t)
	authService, err := NewAuthFacadeForTest(testutils.AppTestConfig, suite.UserRepo, suite.PostRepo, suite.DeletionRepo, suite.RefreshTokenRepo, suite.EmailSender)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}
	return authService, suite
}

func TestAuthService_Unit_Register(t *testing.T) {
	t.Run("Successful_Registration", func(t *testing.T) {
		authService, _ := setupAuthService(t)

		result, err := authService.Register("testuser", "test@example.com", "SecurePass123!")
		if err != nil {
			t.Fatalf("Expected successful registration, got error: %v", err)
		}

		if result.User.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", result.User.Username)
		}

		if result.User.Email != "test@example.com" {
			t.Errorf("Expected email 'test@example.com', got '%s'", result.User.Email)
		}

		if result.User.EmailVerified {
			t.Error("Expected email to be unverified initially")
		}

		if !result.VerificationSent {
			t.Error("Expected verification to be sent")
		}
	})

	t.Run("Duplicate_Username", func(t *testing.T) {
		authService, suite := setupAuthService(t)

		existingUser := &database.User{
			Username:      "existinguser",
			Email:         "existing@example.com",
			Password:      "hashed",
			EmailVerified: true,
		}
		if err := suite.UserRepo.Create(existingUser); err != nil {
			t.Fatalf("Failed to create existing user: %v", err)
		}

		_, err := authService.Register("existinguser", "test@example.com", "SecurePass123!")
		if err == nil {
			t.Error("Expected error for duplicate username")
		}

		if !strings.Contains(err.Error(), "username already exists") {
			t.Errorf("Expected username conflict error, got: %v", err)
		}
	})

	t.Run("Duplicate_Email", func(t *testing.T) {
		authService, suite := setupAuthService(t)

		existingUser := &database.User{
			Username:      "existinguser",
			Email:         "existing@example.com",
			Password:      "hashed",
			EmailVerified: true,
		}
		if err := suite.UserRepo.Create(existingUser); err != nil {
			t.Fatalf("Failed to create existing user: %v", err)
		}

		_, err := authService.Register("newuser", "existing@example.com", "SecurePass123!")
		if err == nil {
			t.Error("Expected error for duplicate email")
		}

		if !strings.Contains(err.Error(), "email already exists") {
			t.Errorf("Expected email conflict error, got: %v", err)
		}
	})

	t.Run("Weak_Password", func(t *testing.T) {
		authService, _ := setupAuthService(t)

		_, err := authService.Register("testuser", "test@example.com", "123")
		if err == nil {
			t.Error("Expected error for weak password")
		}

		if !strings.Contains(strings.ToLower(err.Error()), "password") {
			t.Errorf("Expected password validation error, got: %v", err)
		}
	})

	t.Run("Invalid_Email", func(t *testing.T) {
		authService, _ := setupAuthService(t)

		_, err := authService.Register("testuser", "invalid-email", "SecurePass123!")
		if err == nil {
			t.Error("Expected error for invalid email")
		}

		if !strings.Contains(err.Error(), "email") {
			t.Errorf("Expected email validation error, got: %v", err)
		}
	})
}

func TestAuthService_Unit_Login(t *testing.T) {
	t.Run("Successful_Login", func(t *testing.T) {
		authService, suite := setupAuthService(t)

		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("SecurePass123!"), bcrypt.DefaultCost)
		testUser := &database.User{
			Username:      "testuser",
			Email:         "test@example.com",
			Password:      string(hashedPassword),
			EmailVerified: true,
		}
		if err := suite.UserRepo.Create(testUser); err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		result, err := authService.Login("testuser", "SecurePass123!")
		if err != nil {
			t.Fatalf("Expected successful login, got error: %v", err)
		}

		if result.User.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", result.User.Username)
		}

		if result.AccessToken == "" {
			t.Error("Expected access token to be generated")
		}

		if result.RefreshToken == "" {
			t.Error("Expected refresh token to be generated")
		}

		token, err := jwt.Parse(result.AccessToken, func(token *jwt.Token) (any, error) {
			return []byte(testutils.AppTestConfig.JWT.Secret), nil
		})
		if err != nil {
			t.Fatalf("Failed to parse token: %v", err)
		}

		if !token.Valid {
			t.Error("Expected valid JWT token")
		}
	})

	t.Run("Invalid_Credentials", func(t *testing.T) {
		authService, _ := setupAuthService(t)

		_, err := authService.Login("nonexistent", "SecurePass123!")
		if err == nil {
			t.Error("Expected error for invalid credentials")
		}

		if !strings.Contains(err.Error(), "invalid credentials") {
			t.Errorf("Expected invalid credentials error, got: %v", err)
		}
	})

	t.Run("Wrong_Password", func(t *testing.T) {
		authService, suite := setupAuthService(t)

		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
		testUser := &database.User{
			Username:      "testuser",
			Email:         "test@example.com",
			Password:      string(hashedPassword),
			EmailVerified: true,
		}
		if err := suite.UserRepo.Create(testUser); err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		_, err := authService.Login("testuser", "wrongpassword")
		if err == nil {
			t.Error("Expected error for wrong password")
		}

		if !strings.Contains(err.Error(), "invalid credentials") {
			t.Errorf("Expected invalid credentials error, got: %v", err)
		}
	})

	t.Run("Unverified_Email", func(t *testing.T) {
		authService, suite := setupAuthService(t)

		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("SecurePass123!"), bcrypt.DefaultCost)
		testUser := &database.User{
			Username:      "testuser",
			Email:         "test@example.com",
			Password:      string(hashedPassword),
			EmailVerified: false,
		}
		if err := suite.UserRepo.Create(testUser); err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		_, err := authService.Login("testuser", "SecurePass123!")
		if err == nil {
			t.Error("Expected error for unverified email")
		}

		if !strings.Contains(err.Error(), "email not verified") {
			t.Errorf("Expected email verification error, got: %v", err)
		}
	})
}

func TestAuthService_Unit_ConfirmEmail(t *testing.T) {
	t.Run("Successful_Email_Confirmation", func(t *testing.T) {
		authService, suite := setupAuthService(t)

		rawToken := "valid-token"
		hashedToken := testHashVerificationToken(rawToken)

		testUser := &database.User{
			Username:               "testuser",
			Email:                  "test@example.com",
			Password:               "hashed",
			EmailVerified:          false,
			EmailVerificationToken: hashedToken,
		}
		if err := suite.UserRepo.Create(testUser); err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		result, err := authService.ConfirmEmail("valid-token")
		if err != nil {
			t.Fatalf("Expected successful email confirmation, got error: %v", err)
		}

		if !result.EmailVerified {
			t.Error("Expected email to be verified")
		}

		if result.EmailVerificationToken != "" {
			t.Error("Expected verification token to be cleared")
		}
	})

	t.Run("Invalid_Token", func(t *testing.T) {
		authService, _ := setupAuthService(t)

		_, err := authService.ConfirmEmail("invalid-token")
		if err == nil {
			t.Error("Expected error for invalid token")
		}

		if !strings.Contains(err.Error(), "invalid verification token") {
			t.Errorf("Expected invalid verification token error, got: %v", err)
		}
	})
}

func TestAuthService_Integration_Complete_Workflow(t *testing.T) {
	suite := testutils.NewServiceSuite(t)

	authService, err := NewAuthFacadeForTest(testutils.AppTestConfig, suite.UserRepo, suite.PostRepo, suite.DeletionRepo, suite.RefreshTokenRepo, suite.EmailSender)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	t.Run("Complete_User_Lifecycle", func(t *testing.T) {
		suite.EmailSender.Reset()
		registerResult, err := authService.Register("lifecycle_user", "lifecycle@example.com", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to register user: %v", err)
		}

		if registerResult.User.Username != "lifecycle_user" {
			t.Errorf("Expected username 'lifecycle_user', got '%s'", registerResult.User.Username)
		}

		if registerResult.User.EmailVerified {
			t.Error("Expected email to be unverified initially")
		}

		verificationToken := setupVerificationTokenForTest(t, suite.EmailSender, suite.UserRepo, "lifecycle_user")

		confirmResult, err := authService.ConfirmEmail(verificationToken)
		if err != nil {
			t.Fatalf("Failed to confirm email: %v", err)
		}

		if !confirmResult.EmailVerified {
			t.Error("Expected email to be verified after confirmation")
		}

		loginResult, err := authService.Login("lifecycle_user", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		if loginResult.User.Username != "lifecycle_user" {
			t.Errorf("Expected username 'lifecycle_user', got '%s'", loginResult.User.Username)
		}

		if loginResult.AccessToken == "" {
			t.Error("Expected access token to be generated")
		}

		if loginResult.RefreshToken == "" {
			t.Error("Expected refresh token to be generated")
		}

		updateResult, err := authService.UpdateUsername(loginResult.User.ID, "updated_lifecycle_user")
		if err != nil {
			t.Fatalf("Failed to update username: %v", err)
		}

		if updateResult.Username != "updated_lifecycle_user" {
			t.Errorf("Expected updated username, got '%s'", updateResult.Username)
		}

		suite.EmailSender.Reset()
		emailResult, err := authService.UpdateEmail(loginResult.User.ID, "updated@example.com")
		if err != nil {
			t.Fatalf("Failed to update email: %v", err)
		}

		if emailResult.Email != "updated@example.com" {
			t.Errorf("Expected updated email, got '%s'", emailResult.Email)
		}

		newVerificationToken := setupVerificationTokenForTest(t, suite.EmailSender, suite.UserRepo, "updated_lifecycle_user")

		_, err = authService.ConfirmEmail(newVerificationToken)
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

	t.Run("Account_Deletion_Workflow", func(t *testing.T) {
		suite.EmailSender.Reset()

		registerResult, err := authService.Register("deletion_user", "deletion@example.com", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to register user: %v", err)
		}

		verificationToken := setupVerificationTokenForTest(t, suite.EmailSender, suite.UserRepo, "deletion_user")

		_, err = authService.ConfirmEmail(verificationToken)
		if err != nil {
			t.Fatalf("Failed to confirm email: %v", err)
		}

		err = authService.RequestAccountDeletion(registerResult.User.ID)
		if err != nil {
			t.Fatalf("Failed to request account deletion: %v", err)
		}

		deletionToken := setupDeletionTokenForTest(t, suite.EmailSender, suite.DeletionRepo, registerResult.User.ID)

		err = authService.ConfirmAccountDeletion(deletionToken)
		if err != nil {
			t.Fatalf("Failed to confirm account deletion: %v", err)
		}

		if err := authService.ConfirmAccountDeletion(deletionToken); !errors.Is(err, ErrInvalidDeletionToken) {
			t.Fatalf("Expected token reuse to fail with ErrInvalidDeletionToken, got %v", err)
		}
	})

	t.Run("Password_Reset_Workflow", func(t *testing.T) {
		suite.EmailSender.Reset()

		_, err := authService.Register("reset_user", "reset@example.com", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to register user: %v", err)
		}

		verificationToken := setupVerificationTokenForTest(t, suite.EmailSender, suite.UserRepo, "reset_user")

		_, err = authService.ConfirmEmail(verificationToken)
		if err != nil {
			t.Fatalf("Failed to confirm email: %v", err)
		}

		err = authService.RequestPasswordReset("reset@example.com")
		if err != nil {
			t.Fatalf("Failed to request password reset: %v", err)
		}

		resetToken := setupPasswordResetTokenForTest(t, suite.EmailSender, suite.UserRepo, "reset@example.com")

		err = authService.ResetPassword(resetToken, "NewSecurePass123!")
		if err != nil {
			t.Fatalf("Failed to reset password: %v", err)
		}

		if err := authService.ResetPassword(resetToken, "AnotherPass123!"); err == nil {
			t.Fatal("expected reset token reuse to fail")
		}

		_, err = authService.Login("reset_user", "NewSecurePass123!")
		if err != nil {
			t.Fatalf("Failed to login with new password: %v", err)
		}
	})
}

func TestAuthService_Integration_Error_Handling(t *testing.T) {
	suite := testutils.NewServiceSuite(t)

	authService, err := NewAuthFacadeForTest(testutils.AppTestConfig, suite.UserRepo, suite.PostRepo, suite.DeletionRepo, suite.RefreshTokenRepo, suite.EmailSender)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	t.Run("Validation_Errors", func(t *testing.T) {
		_, err := authService.Register("weak_user", "weak@example.com", "123")
		if err == nil {
			t.Error("Expected error for weak password")
		}

		_, err = authService.Register("invalid_user", "not-an-email", "SecurePass123!")
		if err == nil {
			t.Error("Expected error for invalid email")
		}

		_, err = authService.Register("", "test@example.com", "SecurePass123!")
		if err == nil {
			t.Error("Expected error for empty username")
		}
	})

	t.Run("Duplicate_Constraints", func(t *testing.T) {
		_, err := authService.Register("duplicate_user", "duplicate1@example.com", "SecurePass123!")
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

	t.Run("Duplicate_LongUsername", func(t *testing.T) {
		longUsername := strings.Repeat("x", 50)
		user := &database.User{
			Username:      longUsername,
			Email:         "longuser@example.com",
			Password:      testutils.HashPassword("SecurePass123!"),
			EmailVerified: true,
		}
		if err := suite.UserRepo.Create(user); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		_, err := authService.Register(longUsername, "longuser2@example.com", "SecurePass123!")
		if !errors.Is(err, ErrUsernameTaken) {
			t.Fatalf("expected ErrUsernameTaken for duplicate long username, got %v", err)
		}
	})

	t.Run("Authentication_Errors", func(t *testing.T) {
		_, err := authService.Login("nonexistent", "password")
		if err == nil {
			t.Error("Expected error for non-existent user")
		}

		_, err = authService.Register("auth_user", "auth@example.com", "SecurePass123!")
		if err != nil {
			t.Fatalf("Failed to register user: %v", err)
		}

		verificationToken := suite.EmailSender.VerificationToken()
		if verificationToken == "" {
			t.Fatal("Expected verification token to be generated")
		}

		_, err = authService.ConfirmEmail(verificationToken)
		if err != nil {
			t.Fatalf("Failed to confirm email: %v", err)
		}

		_, err = authService.Login("auth_user", "wrongpassword")
		if err == nil {
			t.Error("Expected error for wrong password")
		}
	})

	t.Run("Pre_Verified_User_Operations", func(t *testing.T) {
		user := createTestUserWithAuth(authService, suite.EmailSender, suite.UserRepo, "preverified_user", "preverified@example.com")

		if user.Username != "preverified_user" {
			t.Errorf("Expected username 'preverified_user', got '%s'", user.Username)
		}

		if user.Email != "preverified@example.com" {
			t.Errorf("Expected email 'preverified@example.com', got '%s'", user.Email)
		}

		if !user.EmailVerified {
			t.Error("Expected user to be email verified")
		}

		loginResult, err := authService.Login("preverified_user", "SecurePass123!")
		if err != nil {
			t.Fatalf("Expected successful login for pre-verified user, got error: %v", err)
		}

		if loginResult.User.ID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, loginResult.User.ID)
		}

		updateResult, err := authService.UpdateUsername(user.ID, "updated_preverified_user")
		if err != nil {
			t.Fatalf("Expected successful username update, got error: %v", err)
		}

		if updateResult.Username != "updated_preverified_user" {
			t.Errorf("Expected updated username, got '%s'", updateResult.Username)
		}
	})
}

func createTestUserWithAuth(authService interface {
	Register(username, email, password string) (*RegistrationResult, error)
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

func setupPasswordResetTokenForTest(t *testing.T, emailSender *testutils.MockEmailSender, userRepo repositories.UserRepository, email string) string {
	t.Helper()

	resetToken := emailSender.PasswordResetToken()
	if resetToken == "" {
		t.Fatal("Expected password reset token to be generated")
	}

	hashedToken := testutils.HashVerificationToken(resetToken)

	user, err := userRepo.GetByEmail(email)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	user.PasswordResetToken = hashedToken
	if err := userRepo.Update(user); err != nil {
		t.Fatalf("Failed to update user with hashed reset token: %v", err)
	}

	return resetToken
}
