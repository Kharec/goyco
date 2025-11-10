package commands

import (
	"errors"
	"strings"
	"testing"
	"time"

	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/services"
	"goyco/internal/testutils"
)

func TestHandleUserCommand(t *testing.T) {
	cfg := testutils.NewTestConfig()

	t.Run("help requested", func(t *testing.T) {
		err := HandleUserCommand(cfg, "user", []string{"--help"})

		if err != nil {
			t.Errorf("unexpected error for help: %v", err)
		}
	})
}

func TestRunUserCommand(t *testing.T) {
	cfg := testutils.NewTestConfig()
	mockRepo := testutils.NewMockUserRepository()

	t.Run("missing subcommand", func(t *testing.T) {
		mockRefreshRepo := &mockRefreshTokenRepo{}
		err := runUserCommand(cfg, mockRepo, mockRefreshRepo, []string{})

		if err == nil {
			t.Error("expected error for missing subcommand")
		}

		if err.Error() != "missing user subcommand" {
			t.Errorf("expected specific error, got: %v", err)
		}
	})

	t.Run("unknown subcommand", func(t *testing.T) {
		mockRefreshRepo := &mockRefreshTokenRepo{}
		err := runUserCommand(cfg, mockRepo, mockRefreshRepo, []string{"unknown"})

		if err == nil {
			t.Error("expected error for unknown subcommand")
		}

		expectedErr := "unknown user subcommand: unknown"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("help subcommand", func(t *testing.T) {
		mockRefreshRepo := &mockRefreshTokenRepo{}
		err := runUserCommand(cfg, mockRepo, mockRefreshRepo, []string{"help"})

		if err != nil {
			t.Errorf("unexpected error for help: %v", err)
		}
	})
}

func TestUserCreate(t *testing.T) {
	cfg := testutils.NewTestConfig()

	t.Run("successful creation", func(t *testing.T) {
		mockRepo := testutils.NewMockUserRepository()
		err := userCreate(cfg, mockRepo, []string{
			"--username", "testuser",
			"--email", "test@example.com",
			"--password", "StrongPass123!",
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing username", func(t *testing.T) {
		mockRepo := testutils.NewMockUserRepository()
		err := userCreate(cfg, mockRepo, []string{
			"--email", "test@example.com",
			"--password", "StrongPass123!",
		})

		if err == nil {
			t.Error("expected error for missing username")
		}

		if err.Error() != "username, email, and password are required" {
			t.Errorf("expected specific error, got: %v", err)
		}
	})

	t.Run("missing email", func(t *testing.T) {
		mockRepo := testutils.NewMockUserRepository()
		err := userCreate(cfg, mockRepo, []string{
			"--username", "testuser",
			"--password", "StrongPass123!",
		})

		if err == nil {
			t.Error("expected error for missing email")
		}

		if err.Error() != "username, email, and password are required" {
			t.Errorf("expected specific error, got: %v", err)
		}
	})

	t.Run("missing password", func(t *testing.T) {
		mockRepo := testutils.NewMockUserRepository()
		err := userCreate(cfg, mockRepo, []string{
			"--username", "testuser",
			"--email", "test@example.com",
		})

		if err == nil {
			t.Error("expected error for missing password")
		}

		if err.Error() != "username, email, and password are required" {
			t.Errorf("expected specific error, got: %v", err)
		}
	})

	t.Run("password too short", func(t *testing.T) {
		mockRepo := testutils.NewMockUserRepository()
		err := userCreate(cfg, mockRepo, []string{
			"--username", "testuser",
			"--email", "test@example.com",
			"--password", "short",
		})

		if err == nil {
			t.Error("expected error for short password")
		}

		if !strings.Contains(err.Error(), "password must be at least 8 characters") {
			t.Errorf("expected password length error, got: %v", err)
		}
	})

	t.Run("missing username value", func(t *testing.T) {
		mockRepo := testutils.NewMockUserRepository()
		err := userCreate(cfg, mockRepo, []string{
			"--username",
			"--email", "test@example.com",
			"--password", "StrongPass123!",
		})

		if err == nil {
			t.Error("expected error for missing username value")
		}
	})

	t.Run("missing email value", func(t *testing.T) {
		mockRepo := testutils.NewMockUserRepository()
		err := userCreate(cfg, mockRepo, []string{
			"--username", "testuser",
			"--email",
			"--password", "StrongPass123!",
		})

		if err == nil {
			t.Error("expected error for missing email value")
		}
	})

	t.Run("missing password value", func(t *testing.T) {
		mockRepo := testutils.NewMockUserRepository()
		err := userCreate(cfg, mockRepo, []string{
			"--username", "testuser",
			"--email", "test@example.com",
			"--password",
		})

		if err == nil {
			t.Error("expected error for missing password value")
		}
	})

	t.Run("unknown flag", func(t *testing.T) {
		mockRepo := testutils.NewMockUserRepository()
		err := userCreate(cfg, mockRepo, []string{
			"--username", "testuser",
			"--email", "test@example.com",
			"--password", "StrongPass123!",
			"--unknown-flag",
		})

		if err == nil {
			t.Error("expected error for unknown flag")
		}
	})

	t.Run("duplicate flag", func(t *testing.T) {
		mockRepo := testutils.NewMockUserRepository()
		err := userCreate(cfg, mockRepo, []string{
			"--username", "testuser",
			"--email", "test@example.com",
			"--password", "StrongPass123!",
			"--username", "duplicate",
		})

		if err != nil {
			if !strings.Contains(err.Error(), "required") && !strings.Contains(err.Error(), "validation") {
				t.Errorf("unexpected error type: %v", err)
			}
		}
	})
}

func TestUserUpdate(t *testing.T) {
	mockRepo := testutils.NewMockUserRepository()

	testUser := &database.User{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "hashedpassword",
	}
	_ = mockRepo.Create(testUser)

	t.Run("successful update username", func(t *testing.T) {
		cfg := &config.Config{}
		mockRefreshRepo := &mockRefreshTokenRepo{}
		err := userUpdate(cfg, mockRepo, mockRefreshRepo, []string{
			"1",
			"--username", "newusername",
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("successful update email", func(t *testing.T) {
		cfg := &config.Config{}
		mockRefreshRepo := &mockRefreshTokenRepo{}
		err := userUpdate(cfg, mockRepo, mockRefreshRepo, []string{
			"1",
			"--email", "newemail@example.com",
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("successful update password", func(t *testing.T) {
		cfg := testutils.NewTestConfig()
		mockRefreshRepo := &mockRefreshTokenRepo{}
		err := userUpdate(cfg, mockRepo, mockRefreshRepo, []string{
			"1",
			"--password", "NewStrongPass123!",
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		cfg := &config.Config{}
		mockRefreshRepo := &mockRefreshTokenRepo{}
		err := userUpdate(cfg, mockRepo, mockRefreshRepo, []string{})

		if err == nil {
			t.Error("expected error for missing id")
		}

		if err.Error() != "user ID is required" {
			t.Errorf("expected specific error, got: %v", err)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		cfg := &config.Config{}
		mockRefreshRepo := &mockRefreshTokenRepo{}
		err := userUpdate(cfg, mockRepo, mockRefreshRepo, []string{
			"0",
			"--username", "newusername",
		})

		if err == nil {
			t.Error("expected error for invalid id")
		}

		if err.Error() != "user ID must be greater than 0" {
			t.Errorf("expected specific error, got: %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		cfg := &config.Config{}
		mockRefreshRepo := &mockRefreshTokenRepo{}
		err := userUpdate(cfg, mockRepo, mockRefreshRepo, []string{
			"999",
			"--username", "newusername",
		})

		if err == nil {
			t.Error("expected error for non-existent user")
		}

		expectedErr := "user 999 not found"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("password too short", func(t *testing.T) {
		cfg := &config.Config{}
		mockRefreshRepo := &mockRefreshTokenRepo{}
		err := userUpdate(cfg, mockRepo, mockRefreshRepo, []string{
			"1",
			"--password", "short",
		})

		if err == nil {
			t.Error("expected error for short password")
		}

		if !strings.Contains(err.Error(), "password must be at least 8 characters") {
			t.Errorf("expected password length error, got: %v", err)
		}
	})
}

func TestUserDelete(t *testing.T) {
	cfg := testutils.NewTestConfig()
	mockRepo := testutils.NewMockUserRepository()

	testUser := &database.User{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "hashedpassword",
	}
	_ = mockRepo.Create(testUser)

	t.Run("successful delete (keep posts)", func(t *testing.T) {
		err := userDelete(cfg, mockRepo, []string{"1"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("successful delete with posts", func(t *testing.T) {
		testUser2 := &database.User{
			Username: "testuser2",
			Email:    "test2@example.com",
			Password: "hashedpassword",
		}
		_ = mockRepo.Create(testUser2)

		err := userDelete(cfg, mockRepo, []string{"2", "--with-posts"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		err := userDelete(cfg, mockRepo, []string{})

		if err == nil {
			t.Error("expected error for missing id")
		}

		if err.Error() != "user ID is required" {
			t.Errorf("expected specific error, got: %v", err)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		err := userDelete(cfg, mockRepo, []string{"0"})

		if err == nil {
			t.Error("expected error for invalid id")
		}

		if err.Error() != "user ID must be greater than 0" {
			t.Errorf("expected specific error, got: %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		err := userDelete(cfg, mockRepo, []string{"999"})

		if err == nil {
			t.Error("expected error for non-existent user")
		}

		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	})

	t.Run("user already deleted", func(t *testing.T) {
		freshMockRepo := testutils.NewMockUserRepository()

		testUser := &database.User{
			Username: "deleteduser",
			Email:    "deleted@example.com",
			Password: "hashedpassword",
		}
		_ = freshMockRepo.Create(testUser)

		err := userDelete(cfg, freshMockRepo, []string{"1"})
		if err != nil {
			t.Errorf("unexpected error on first deletion: %v", err)
		}

		err = userDelete(cfg, freshMockRepo, []string{"1"})

		if err == nil {
			t.Error("expected error for already deleted user")
		}

		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	})
}

func TestUserList(t *testing.T) {
	mockRepo := testutils.NewMockUserRepository()

	testUsers := []*database.User{
		{
			Username:  "user1",
			Email:     "user1@example.com",
			Password:  "password1",
			CreatedAt: time.Now().Add(-2 * time.Hour),
		},
		{
			Username:  "user2",
			Email:     "user2@example.com",
			Password:  "password2",
			CreatedAt: time.Now().Add(-1 * time.Hour),
		},
	}

	for _, user := range testUsers {
		_ = mockRepo.Create(user)
	}

	t.Run("list all users", func(t *testing.T) {
		err := userList(mockRepo, []string{})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("list with limit", func(t *testing.T) {
		err := userList(mockRepo, []string{"--limit", "1"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("list with offset", func(t *testing.T) {
		err := userList(mockRepo, []string{"--offset", "1"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("list with all filters", func(t *testing.T) {
		err := userList(mockRepo, []string{"--limit", "1", "--offset", "0"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		emptyRepo := testutils.NewMockUserRepository()
		err := userList(emptyRepo, []string{})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		mockRepo.GetErr = errors.New("database error")
		err := userList(mockRepo, []string{})

		if err == nil {
			t.Error("expected error from repository")
		}

		expectedErr := "list users: database error"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("invalid limit type", func(t *testing.T) {
		err := userList(mockRepo, []string{"--limit", "abc"})

		if err == nil {
			t.Error("expected error for invalid limit type")
		}
	})

	t.Run("invalid offset type", func(t *testing.T) {
		err := userList(mockRepo, []string{"--offset", "xyz"})

		if err == nil {
			t.Error("expected error for invalid offset type")
		}
	})

	t.Run("unknown flag", func(t *testing.T) {
		err := userList(mockRepo, []string{"--unknown-flag"})

		if err == nil {
			t.Error("expected error for unknown flag")
		}
	})

	t.Run("missing limit value", func(t *testing.T) {
		err := userList(mockRepo, []string{"--limit"})

		if err == nil {
			t.Error("expected error for missing limit value")
		}
	})

	t.Run("missing offset value", func(t *testing.T) {
		err := userList(mockRepo, []string{"--offset"})

		if err == nil {
			t.Error("expected error for missing offset value")
		}
	})
}

func TestCheckUsernameAvailable(t *testing.T) {
	mockRepo := testutils.NewMockUserRepository()

	testUser := &database.User{
		Username: "existinguser",
		Email:    "test@example.com",
		Password: "password",
	}
	_ = mockRepo.Create(testUser)

	t.Run("username available", func(t *testing.T) {
		err := checkUsernameAvailable(mockRepo, "newuser", 0)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("username taken by different user", func(t *testing.T) {
		err := checkUsernameAvailable(mockRepo, "existinguser", 2)

		if err == nil {
			t.Error("expected error for taken username")
		}

		expectedErr := "username existinguser is already taken"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("username taken by same user (should be ok)", func(t *testing.T) {
		err := checkUsernameAvailable(mockRepo, "existinguser", 1)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestCheckEmailAvailable(t *testing.T) {
	mockRepo := testutils.NewMockUserRepository()

	testUser := &database.User{
		Username: "testuser",
		Email:    "existing@example.com",
		Password: "password",
	}
	_ = mockRepo.Create(testUser)

	t.Run("email available", func(t *testing.T) {
		err := checkEmailAvailable(mockRepo, "new@example.com", 0)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("email taken by different user", func(t *testing.T) {
		err := checkEmailAvailable(mockRepo, "existing@example.com", 2)

		if err == nil {
			t.Error("expected error for taken email")
		}

		expectedErr := "email existing@example.com is already registered"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("email taken by same user (should be ok)", func(t *testing.T) {
		err := checkEmailAvailable(mockRepo, "existing@example.com", 1)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestGenerateTemporaryPassword(t *testing.T) {
	for range 10 {
		password, err := generateTemporaryPassword()
		if err != nil {
			t.Fatalf("generateTemporaryPassword() error = %v", err)
		}

		if len(password) != 16 {
			t.Errorf("Password length = %d, want 16", len(password))
		}

		hasUpper := false
		hasLower := false
		hasDigit := false
		hasSpecial := false

		for _, char := range password {
			switch {
			case char >= 'A' && char <= 'Z':
				hasUpper = true
			case char >= 'a' && char <= 'z':
				hasLower = true
			case char >= '0' && char <= '9':
				hasDigit = true
			case char == '!' || char == '@' || char == '#' || char == '$' || char == '%' || char == '^' || char == '&' || char == '*':
				hasSpecial = true
			}
		}

		if !hasUpper {
			t.Errorf("Password %s missing uppercase letter", password)
		}
		if !hasLower {
			t.Errorf("Password %s missing lowercase letter", password)
		}
		if !hasDigit {
			t.Errorf("Password %s missing digit", password)
		}
		if !hasSpecial {
			t.Errorf("Password %s missing special character", password)
		}
	}
}

func TestGenerateTemporaryPassword_Uniqueness(t *testing.T) {
	passwords := make(map[string]bool)

	for range 100 {
		password, err := generateTemporaryPassword()
		if err != nil {
			t.Fatalf("generateTemporaryPassword() error = %v", err)
		}

		if passwords[password] {
			t.Errorf("Duplicate password generated: %s", password)
		}
		passwords[password] = true
	}
}

func TestResetUserPassword_WithoutEmail(t *testing.T) {

	tempPassword, err := generateTemporaryPassword()
	if err != nil {
		t.Fatalf("generateTemporaryPassword() error = %v", err)
	}

	if len(tempPassword) != 16 {
		t.Errorf("Password length = %d, want 16", len(tempPassword))
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, char := range tempPassword {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasDigit = true
		case char == '!' || char == '@' || char == '#' || char == '$' || char == '%' || char == '^' || char == '&' || char == '*':
			hasSpecial = true
		}
	}

	if !hasUpper {
		t.Error("Password missing uppercase letter")
	}
	if !hasLower {
		t.Error("Password missing lowercase letter")
	}
	if !hasDigit {
		t.Error("Password missing digit")
	}
	if !hasSpecial {
		t.Error("Password missing special character")
	}
}

type mockRefreshTokenRepo struct{}

func (m *mockRefreshTokenRepo) Create(token *database.RefreshToken) error { return nil }
func (m *mockRefreshTokenRepo) GetByTokenHash(tokenHash string) (*database.RefreshToken, error) {
	return nil, nil
}
func (m *mockRefreshTokenRepo) DeleteByUserID(userID uint) error { return nil }
func (m *mockRefreshTokenRepo) DeleteExpired() error             { return nil }
func (m *mockRefreshTokenRepo) DeleteByID(id uint) error         { return nil }
func (m *mockRefreshTokenRepo) GetByUserID(userID uint) ([]database.RefreshToken, error) {
	return nil, nil
}
func (m *mockRefreshTokenRepo) CountByUserID(userID uint) (int64, error) { return 0, nil }

func TestResetUserPassword_UserNotFound(t *testing.T) {
	mockRepo := testutils.NewMockUserRepository()
	mockRefreshRepo := &mockRefreshTokenRepo{}

	cfg := &config.Config{
		JWT: config.JWTConfig{Secret: "test-secret", Expiration: 24},
	}

	jwtService := services.NewJWTService(&cfg.JWT, mockRepo, mockRefreshRepo)
	mockSessionService := services.NewSessionService(jwtService, mockRepo)

	err := resetUserPassword(cfg, mockRepo, mockSessionService, 999)
	if err == nil {
		t.Error("Expected error for non-existent user, got nil")
	}

	expectedError := "user 999 not found"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestGeneratePasswordResetEmailBody(t *testing.T) {
	username := "testuser"
	title := "Test Title"
	tempPassword := "TempPass123!"
	baseURL := "https://example.com"
	adminEmail := "admin@example.com"

	body := generatePasswordResetEmailBody(username, tempPassword, baseURL, adminEmail, title)

	if !strings.Contains(body, username) {
		t.Error("Email body does not contain username")
	}

	if !strings.Contains(body, tempPassword) {
		t.Error("Email body does not contain temporary password")
	}

	if !strings.Contains(body, baseURL) {
		t.Error("Email body does not contain base URL")
	}

	if !strings.Contains(body, "IMPORTANT SECURITY NOTICE") {
		t.Error("Email body does not contain security notice")
	}

	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("Email body is not HTML")
	}

	if !strings.Contains(body, "mailto:"+adminEmail) {
		t.Error("Email body does not contain admin contact link")
	}
}
