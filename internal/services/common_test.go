package services

import (
	"errors"
	"testing"

	"goyco/internal/config"
	"goyco/internal/repositories"
	"goyco/internal/testutils"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestTrimString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no_whitespace",
			input:    "test",
			expected: "test",
		},
		{
			name:     "leading_spaces",
			input:    "  test",
			expected: "test",
		},
		{
			name:     "trailing_spaces",
			input:    "test  ",
			expected: "test",
		},
		{
			name:     "leading_and_trailing_spaces",
			input:    "  test  ",
			expected: "test",
		},
		{
			name:     "only_spaces",
			input:    "   ",
			expected: "",
		},
		{
			name:     "empty_string",
			input:    "",
			expected: "",
		},
		{
			name:     "tabs_and_newlines",
			input:    "\t\n  test  \n\t",
			expected: "test",
		},
		{
			name:     "internal_spaces_preserved",
			input:    "  test  string  ",
			expected: "test  string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimString(tt.input)
			if result != tt.expected {
				t.Errorf("TrimString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHashPassword(t *testing.T) {
	t.Run("successful_hash", func(t *testing.T) {
		password := "testpassword123"
		hashed, err := HashPassword(password, DefaultBcryptCost)
		if err != nil {
			t.Fatalf("HashPassword() error = %v, want no error", err)
		}
		if hashed == "" {
			t.Error("HashPassword() returned empty string")
		}
		if hashed == password {
			t.Error("HashPassword() returned plain password")
		}

		err = bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
		if err != nil {
			t.Errorf("HashPassword() produced invalid hash: %v", err)
		}
	})

	t.Run("different_passwords_produce_different_hashes", func(t *testing.T) {
		password1 := "password1"
		password2 := "password2"

		hash1, err := HashPassword(password1, DefaultBcryptCost)
		if err != nil {
			t.Fatalf("HashPassword() error = %v", err)
		}

		hash2, err := HashPassword(password2, DefaultBcryptCost)
		if err != nil {
			t.Fatalf("HashPassword() error = %v", err)
		}

		if hash1 == hash2 {
			t.Error("Different passwords produced same hash")
		}
	})

	t.Run("same_password_produces_different_hashes", func(t *testing.T) {
		password := "samepassword"
		hash1, err := HashPassword(password, DefaultBcryptCost)
		if err != nil {
			t.Fatalf("HashPassword() error = %v", err)
		}

		hash2, err := HashPassword(password, DefaultBcryptCost)
		if err != nil {
			t.Fatalf("HashPassword() error = %v", err)
		}

		if hash1 == hash2 {
			t.Error("Same password produced same hash (should be different due to salt)")
		}

		if err := bcrypt.CompareHashAndPassword([]byte(hash1), []byte(password)); err != nil {
			t.Errorf("First hash doesn't verify: %v", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash2), []byte(password)); err != nil {
			t.Errorf("Second hash doesn't verify: %v", err)
		}
	})

	t.Run("custom_cost", func(t *testing.T) {
		password := "testpassword"
		customCost := 12

		hashed, err := HashPassword(password, customCost)
		if err != nil {
			t.Fatalf("HashPassword() error = %v", err)
		}

		err = bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
		if err != nil {
			t.Errorf("HashPassword() with custom cost produced invalid hash: %v", err)
		}
	})

	t.Run("zero_cost_uses_default", func(t *testing.T) {
		password := "testpassword"

		hashed, err := HashPassword(password, 0)
		if err != nil {
			t.Fatalf("HashPassword() error = %v", err)
		}

		err = bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
		if err != nil {
			t.Errorf("HashPassword() with zero cost produced invalid hash: %v", err)
		}
	})

	t.Run("negative_cost_uses_default", func(t *testing.T) {
		password := "testpassword"

		hashed, err := HashPassword(password, -1)
		if err != nil {
			t.Fatalf("HashPassword() error = %v", err)
		}

		err = bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
		if err != nil {
			t.Errorf("HashPassword() with negative cost produced invalid hash: %v", err)
		}
	})

	t.Run("empty_password", func(t *testing.T) {
		hashed, err := HashPassword("", DefaultBcryptCost)
		if err != nil {
			t.Fatalf("HashPassword() error = %v", err)
		}

		err = bcrypt.CompareHashAndPassword([]byte(hashed), []byte(""))
		if err != nil {
			t.Errorf("HashPassword() with empty password produced invalid hash: %v", err)
		}
	})
}

func TestIsRecordNotFound(t *testing.T) {
	t.Run("gorm_record_not_found", func(t *testing.T) {
		err := gorm.ErrRecordNotFound
		if !IsRecordNotFound(err) {
			t.Error("IsRecordNotFound() = false, want true for gorm.ErrRecordNotFound")
		}
	})

	t.Run("wrapped_gorm_record_not_found", func(t *testing.T) {
		err := errors.New("some context")
		wrappedErr := errors.Join(err, gorm.ErrRecordNotFound)
		if !IsRecordNotFound(wrappedErr) {
			t.Error("IsRecordNotFound() = false, want true for wrapped gorm.ErrRecordNotFound")
		}
	})

	t.Run("other_error", func(t *testing.T) {
		err := errors.New("some other error")
		if IsRecordNotFound(err) {
			t.Error("IsRecordNotFound() = true, want false for other error")
		}
	})

	t.Run("nil_error", func(t *testing.T) {
		if IsRecordNotFound(nil) {
			t.Error("IsRecordNotFound() = true, want false for nil error")
		}
	})
}

func TestHandleUniqueConstraintError(t *testing.T) {
	t.Run("nil_error", func(t *testing.T) {
		err := HandleUniqueConstraintError(nil)
		if err != nil {
			t.Errorf("HandleUniqueConstraintError(nil) = %v, want nil", err)
		}
	})

	t.Run("non_pq_error", func(t *testing.T) {
		originalErr := errors.New("some other error")
		err := HandleUniqueConstraintError(originalErr)
		if err != originalErr {
			t.Errorf("HandleUniqueConstraintError() = %v, want %v", err, originalErr)
		}
	})

	t.Run("pq_error_wrong_code", func(t *testing.T) {
		pqErr := &pq.Error{
			Code:    "23503",
			Message: "some error",
		}
		err := HandleUniqueConstraintError(pqErr)
		if err != pqErr {
			t.Errorf("HandleUniqueConstraintError() = %v, want %v", err, pqErr)
		}
	})

	t.Run("username_constraint_in_constraint_field", func(t *testing.T) {
		pqErr := &pq.Error{
			Code:       "23505",
			Constraint: "users_username_key",
			Message:    "duplicate key value violates unique constraint",
		}
		err := HandleUniqueConstraintError(pqErr)
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("HandleUniqueConstraintError() = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("username_constraint_in_message", func(t *testing.T) {
		pqErr := &pq.Error{
			Code:       "23505",
			Constraint: "some_constraint",
			Message:    "duplicate key value violates unique constraint users.username",
		}
		err := HandleUniqueConstraintError(pqErr)
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("HandleUniqueConstraintError() = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("username_constraint_case_insensitive", func(t *testing.T) {
		pqErr := &pq.Error{
			Code:       "23505",
			Constraint: "USERS_USERNAME_KEY",
			Message:    "DUPLICATE KEY VALUE VIOLATES UNIQUE CONSTRAINT",
		}
		err := HandleUniqueConstraintError(pqErr)
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("HandleUniqueConstraintError() = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("email_constraint_in_constraint_field", func(t *testing.T) {
		pqErr := &pq.Error{
			Code:       "23505",
			Constraint: "users_email_key",
			Message:    "duplicate key value violates unique constraint",
		}
		err := HandleUniqueConstraintError(pqErr)
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("HandleUniqueConstraintError() = %v, want ErrEmailTaken", err)
		}
	})

	t.Run("email_constraint_in_message", func(t *testing.T) {
		pqErr := &pq.Error{
			Code:       "23505",
			Constraint: "some_constraint",
			Message:    "duplicate key value violates unique constraint users.email",
		}
		err := HandleUniqueConstraintError(pqErr)
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("HandleUniqueConstraintError() = %v, want ErrEmailTaken", err)
		}
	})

	t.Run("email_constraint_case_insensitive", func(t *testing.T) {
		pqErr := &pq.Error{
			Code:       "23505",
			Constraint: "USERS_EMAIL_KEY",
			Message:    "DUPLICATE KEY VALUE VIOLATES UNIQUE CONSTRAINT",
		}
		err := HandleUniqueConstraintError(pqErr)
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("HandleUniqueConstraintError() = %v, want ErrEmailTaken", err)
		}
	})

	t.Run("unknown_unique_constraint_defaults_to_username", func(t *testing.T) {
		pqErr := &pq.Error{
			Code:       "23505",
			Constraint: "some_other_constraint",
			Message:    "duplicate key value violates unique constraint",
		}
		err := HandleUniqueConstraintError(pqErr)
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("HandleUniqueConstraintError() = %v, want ErrUsernameTaken (default)", err)
		}
	})
}

func TestHandleUniqueConstraintErrorWithMessage(t *testing.T) {
	t.Run("nil_error", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(nil)
		if err != nil {
			t.Errorf("HandleUniqueConstraintErrorWithMessage(nil) = %v, want nil", err)
		}
	})

	t.Run("pq_error_username_handled_by_HandleUniqueConstraintError", func(t *testing.T) {
		pqErr := &pq.Error{
			Code:       "23505",
			Constraint: "users_username_key",
			Message:    "duplicate key value violates unique constraint",
		}
		err := HandleUniqueConstraintErrorWithMessage(pqErr)
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("pq_error_email_handled_by_HandleUniqueConstraintError", func(t *testing.T) {
		pqErr := &pq.Error{
			Code:       "23505",
			Constraint: "users_email_key",
			Message:    "duplicate key value violates unique constraint",
		}
		err := HandleUniqueConstraintErrorWithMessage(pqErr)
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrEmailTaken", err)
		}
	})

	t.Run("message_based_duplicate_key_username", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("duplicate key value violates unique constraint username"))
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("message_based_unique_constraint_username", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("unique constraint failed on username"))
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("message_based_violates_unique_constraint_username", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("violates unique constraint users_username_key"))
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("message_based_unique_constraint_failed_username", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("unique constraint failed on users.username"))
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("message_based_constraint_failed_username", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("constraint failed on username"))
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("message_based_constraint_and_unique_username", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("constraint unique username failed"))
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("message_based_duplicate_key_email", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("duplicate key value violates unique constraint email"))
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrEmailTaken", err)
		}
	})

	t.Run("message_based_unique_constraint_email", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("unique constraint failed on users_email_key"))
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrEmailTaken", err)
		}
	})

	t.Run("message_based_violates_unique_constraint_email", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("violates unique constraint users.email"))
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrEmailTaken", err)
		}
	})

	t.Run("message_based_case_insensitive_username", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("DUPLICATE KEY VALUE VIOLATES UNIQUE CONSTRAINT USERNAME"))
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("message_based_case_insensitive_email", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("DUPLICATE KEY VALUE VIOLATES UNIQUE CONSTRAINT EMAIL"))
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrEmailTaken", err)
		}
	})

	t.Run("non_unique_error_passed_through", func(t *testing.T) {
		originalErr := errors.New("some other database error")
		err := HandleUniqueConstraintErrorWithMessage(originalErr)
		if err != originalErr {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want %v", err, originalErr)
		}
	})

	t.Run("unique_error_without_username_or_email_defaults_to_username", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("duplicate key value violates unique constraint"))
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrUsernameTaken (default)", err)
		}
	})

	t.Run("wrapped_pq_error", func(t *testing.T) {
		pqErr := &pq.Error{
			Code:       "23505",
			Constraint: "users_username_key",
			Message:    "duplicate key value violates unique constraint",
		}
		wrappedErr := errors.New("context: " + pqErr.Error())
		err := HandleUniqueConstraintErrorWithMessage(wrappedErr)
		if err == nil {
			t.Error("HandleUniqueConstraintErrorWithMessage() returned nil, expected error")
		}
	})
}

func TestHandleUniqueConstraintErrorWithMessage_EdgeCases(t *testing.T) {
	t.Run("mixed_case_username_in_message", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("duplicate key value violates unique constraint UserName"))
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("mixed_case_email_in_message", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("duplicate key value violates unique constraint Email"))
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrEmailTaken", err)
		}
	})

	t.Run("username_substring_not_matched", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("duplicate key value violates unique constraint usernames"))
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("email_substring_not_matched", func(t *testing.T) {
		err := HandleUniqueConstraintErrorWithMessage(errors.New("duplicate key value violates unique constraint emails"))
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("HandleUniqueConstraintErrorWithMessage() = %v, want ErrEmailTaken", err)
		}
	})
}

func BenchmarkTrimString(b *testing.B) {
	input := "  test string with spaces  "
	for i := 0; i < b.N; i++ {
		_ = TrimString(input)
	}
}

func BenchmarkHashPassword(b *testing.B) {
	password := "testpassword123"
	for i := 0; i < b.N; i++ {
		_, _ = HashPassword(password, DefaultBcryptCost)
	}
}

func BenchmarkIsRecordNotFound(b *testing.B) {
	err := gorm.ErrRecordNotFound
	for i := 0; i < b.N; i++ {
		_ = IsRecordNotFound(err)
	}
}

func TestNewAuthFacadeForTest(t *testing.T) {
	t.Run("successful_creation", func(t *testing.T) {
		db := testutils.NewTestDB(t)
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		userRepo := repositories.NewUserRepository(db)
		postRepo := repositories.NewPostRepository(db)
		deletionRepo := repositories.NewAccountDeletionRepository(db)
		refreshTokenRepo := repositories.NewRefreshTokenRepository(db)
		emailSender := &testutils.MockEmailSender{}

		authFacade, err := NewAuthFacadeForTest(testutils.AppTestConfig, userRepo, postRepo, deletionRepo, refreshTokenRepo, emailSender)
		if err != nil {
			t.Fatalf("NewAuthFacadeForTest() error = %v, want no error", err)
		}

		if authFacade == nil {
			t.Fatal("NewAuthFacadeForTest() returned nil AuthFacade")
		}
	})

	t.Run("successful_creation_with_custom_config", func(t *testing.T) {
		db := testutils.NewTestDB(t)
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		customConfig := &config.Config{
			JWT: config.JWTConfig{
				Secret:     "custom-secret-key-for-testing-purposes-only",
				Expiration: 48,
			},
			App: config.AppConfig{
				BaseURL:    "http://localhost:3000",
				BcryptCost: 12,
			},
		}

		userRepo := repositories.NewUserRepository(db)
		postRepo := repositories.NewPostRepository(db)
		deletionRepo := repositories.NewAccountDeletionRepository(db)
		refreshTokenRepo := repositories.NewRefreshTokenRepository(db)
		emailSender := &testutils.MockEmailSender{}

		authFacade, err := NewAuthFacadeForTest(customConfig, userRepo, postRepo, deletionRepo, refreshTokenRepo, emailSender)
		if err != nil {
			t.Fatalf("NewAuthFacadeForTest() error = %v, want no error", err)
		}

		if authFacade == nil {
			t.Fatal("NewAuthFacadeForTest() returned nil AuthFacade")
		}
	})

	t.Run("error_on_empty_base_url", func(t *testing.T) {
		db := testutils.NewTestDB(t)
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		invalidConfig := &config.Config{
			JWT: config.JWTConfig{
				Secret:     "test-secret-key",
				Expiration: 24,
			},
			App: config.AppConfig{
				BaseURL:    "",
				BcryptCost: 10,
			},
		}

		userRepo := repositories.NewUserRepository(db)
		postRepo := repositories.NewPostRepository(db)
		deletionRepo := repositories.NewAccountDeletionRepository(db)
		refreshTokenRepo := repositories.NewRefreshTokenRepository(db)
		emailSender := &testutils.MockEmailSender{}

		authFacade, err := NewAuthFacadeForTest(invalidConfig, userRepo, postRepo, deletionRepo, refreshTokenRepo, emailSender)
		if err == nil {
			t.Fatal("NewAuthFacadeForTest() expected error for empty base URL, got nil")
		}

		if authFacade != nil {
			t.Fatal("NewAuthFacadeForTest() expected nil AuthFacade on error, got non-nil")
		}

		if err.Error() != "create email service: APP_BASE_URL is required and must be externally reachable" {
			t.Errorf("NewAuthFacadeForTest() error = %v, want error about APP_BASE_URL", err)
		}
	})

	t.Run("error_on_whitespace_only_base_url", func(t *testing.T) {
		db := testutils.NewTestDB(t)
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		invalidConfig := &config.Config{
			JWT: config.JWTConfig{
				Secret:     "test-secret-key",
				Expiration: 24,
			},
			App: config.AppConfig{
				BaseURL:    "   ",
				BcryptCost: 10,
			},
		}

		userRepo := repositories.NewUserRepository(db)
		postRepo := repositories.NewPostRepository(db)
		deletionRepo := repositories.NewAccountDeletionRepository(db)
		refreshTokenRepo := repositories.NewRefreshTokenRepository(db)
		emailSender := &testutils.MockEmailSender{}

		authFacade, err := NewAuthFacadeForTest(invalidConfig, userRepo, postRepo, deletionRepo, refreshTokenRepo, emailSender)
		if err == nil {
			t.Fatal("NewAuthFacadeForTest() expected error for whitespace-only base URL, got nil")
		}

		if authFacade != nil {
			t.Fatal("NewAuthFacadeForTest() expected nil AuthFacade on error, got non-nil")
		}
	})

	t.Run("facade_can_perform_operations", func(t *testing.T) {
		db := testutils.NewTestDB(t)
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		userRepo := repositories.NewUserRepository(db)
		postRepo := repositories.NewPostRepository(db)
		deletionRepo := repositories.NewAccountDeletionRepository(db)
		refreshTokenRepo := repositories.NewRefreshTokenRepository(db)
		emailSender := &testutils.MockEmailSender{}

		authFacade, err := NewAuthFacadeForTest(testutils.AppTestConfig, userRepo, postRepo, deletionRepo, refreshTokenRepo, emailSender)
		if err != nil {
			t.Fatalf("NewAuthFacadeForTest() error = %v, want no error", err)
		}

		result, err := authFacade.Register("testuser", "test@example.com", "SecurePass123!")
		if err != nil {
			t.Fatalf("AuthFacade.Register() error = %v, want no error", err)
		}

		if result == nil {
			t.Fatal("AuthFacade.Register() returned nil result")
		}

		if result.User.Username != "testuser" {
			t.Errorf("AuthFacade.Register() username = %v, want 'testuser'", result.User.Username)
		}

		if result.User.Email != "test@example.com" {
			t.Errorf("AuthFacade.Register() email = %v, want 'test@example.com'", result.User.Email)
		}
	})

	t.Run("facade_can_login", func(t *testing.T) {
		db := testutils.NewTestDB(t)
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		userRepo := repositories.NewUserRepository(db)
		postRepo := repositories.NewPostRepository(db)
		deletionRepo := repositories.NewAccountDeletionRepository(db)
		refreshTokenRepo := repositories.NewRefreshTokenRepository(db)
		emailSender := &testutils.MockEmailSender{}

		authFacade, err := NewAuthFacadeForTest(testutils.AppTestConfig, userRepo, postRepo, deletionRepo, refreshTokenRepo, emailSender)
		if err != nil {
			t.Fatalf("NewAuthFacadeForTest() error = %v, want no error", err)
		}

		password := "SecurePass123!"
		_, err = authFacade.Register("loginuser", "login@example.com", password)
		if err != nil {
			t.Fatalf("AuthFacade.Register() error = %v, want no error", err)
		}

		user, err := userRepo.GetByUsername("loginuser")
		if err != nil {
			t.Fatalf("GetByUsername() error = %v", err)
		}
		user.EmailVerified = true
		if err := userRepo.Update(user); err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		authResult, err := authFacade.Login("loginuser", password)
		if err != nil {
			t.Fatalf("AuthFacade.Login() error = %v, want no error", err)
		}

		if authResult == nil {
			t.Fatal("AuthFacade.Login() returned nil result")
		}

		if authResult.AccessToken == "" {
			t.Error("AuthFacade.Login() returned empty access token")
		}

		if authResult.RefreshToken == "" {
			t.Error("AuthFacade.Login() returned empty refresh token")
		}

		if authResult.User == nil {
			t.Error("AuthFacade.Login() returned nil user")
		}
	})

	t.Run("multiple_facades_independent", func(t *testing.T) {
		db1 := testutils.NewTestDB(t)
		defer func() {
			sqlDB, _ := db1.DB()
			sqlDB.Close()
		}()

		db2 := testutils.NewTestDB(t)
		defer func() {
			sqlDB, _ := db2.DB()
			sqlDB.Close()
		}()

		userRepo1 := repositories.NewUserRepository(db1)
		postRepo1 := repositories.NewPostRepository(db1)
		deletionRepo1 := repositories.NewAccountDeletionRepository(db1)
		refreshTokenRepo1 := repositories.NewRefreshTokenRepository(db1)
		emailSender1 := &testutils.MockEmailSender{}

		userRepo2 := repositories.NewUserRepository(db2)
		postRepo2 := repositories.NewPostRepository(db2)
		deletionRepo2 := repositories.NewAccountDeletionRepository(db2)
		refreshTokenRepo2 := repositories.NewRefreshTokenRepository(db2)
		emailSender2 := &testutils.MockEmailSender{}

		authFacade1, err := NewAuthFacadeForTest(testutils.AppTestConfig, userRepo1, postRepo1, deletionRepo1, refreshTokenRepo1, emailSender1)
		if err != nil {
			t.Fatalf("NewAuthFacadeForTest() error = %v", err)
		}

		authFacade2, err := NewAuthFacadeForTest(testutils.AppTestConfig, userRepo2, postRepo2, deletionRepo2, refreshTokenRepo2, emailSender2)
		if err != nil {
			t.Fatalf("NewAuthFacadeForTest() error = %v", err)
		}

		if authFacade1 == authFacade2 {
			t.Error("NewAuthFacadeForTest() returned same instance for different repositories")
		}

		_, err = authFacade1.Register("user1", "user1@example.com", "Pass123!")
		if err != nil {
			t.Fatalf("AuthFacade1.Register() error = %v", err)
		}

		_, err = authFacade2.Register("user2", "user2@example.com", "Pass123!")
		if err != nil {
			t.Fatalf("AuthFacade2.Register() error = %v", err)
		}

		user1, err := userRepo1.GetByUsername("user1")
		if err != nil {
			t.Fatalf("GetByUsername() error = %v", err)
		}
		user1.EmailVerified = true
		if err := userRepo1.Update(user1); err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		user2, err := userRepo2.GetByUsername("user2")
		if err != nil {
			t.Fatalf("GetByUsername() error = %v", err)
		}
		user2.EmailVerified = true
		if err := userRepo2.Update(user2); err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		_, err = authFacade1.Login("user1", "Pass123!")
		if err != nil {
			t.Errorf("AuthFacade1.Login() error = %v, should find user1", err)
		}

		_, err = authFacade2.Login("user2", "Pass123!")
		if err != nil {
			t.Errorf("AuthFacade2.Login() error = %v, should find user2", err)
		}
	})
}
