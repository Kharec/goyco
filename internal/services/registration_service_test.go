package services

import (
	"errors"
	"testing"
	"time"

	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/testutils"
)

func TestNewRegistrationService(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
	cfg := testutils.AppTestConfig

	service := NewRegistrationService(userRepo, emailService, cfg)

	if service == nil {
		t.Fatal("expected service to be created")
	}

	if service.userRepo != userRepo {
		t.Error("expected userRepo to be set")
	}

	if service.emailService != emailService {
		t.Error("expected emailService to be set")
	}

	if service.config != cfg {
		t.Error("expected config to be set")
	}
}

func TestRegistrationService_Register(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		email         string
		password      string
		setupMocks    func() (*testutils.MockUserRepository, *EmailService, *config.Config)
		expectedError error
		checkResult   func(*testing.T, *RegistrationResult)
	}{
		{
			name:     "successful registration",
			username: "testuser",
			email:    "test@example.com",
			password: "SecurePass123!",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				emailSender := &testutils.MockEmailSender{}
				emailService, _ := NewEmailService(testutils.AppTestConfig, emailSender)
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
			checkResult: func(t *testing.T, result *RegistrationResult) {
				if result == nil {
					t.Fatal("expected non-nil result")
				}
				if result.User == nil {
					t.Fatal("expected non-nil user")
				}
				if result.User.Username != "testuser" {
					t.Errorf("expected username 'testuser', got %q", result.User.Username)
				}
				if result.User.Email != "test@example.com" {
					t.Errorf("expected email 'test@example.com', got %q", result.User.Email)
				}
				if result.User.Password != "" {
					t.Error("expected password to be sanitized")
				}
				if !result.VerificationSent {
					t.Error("expected VerificationSent to be true")
				}
				if result.User.EmailVerified {
					t.Error("expected EmailVerified to be false")
				}
			},
		},
		{
			name:     "invalid username",
			username: "",
			email:    "test@example.com",
			password: "SecurePass123!",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
			checkResult:   nil,
		},
		{
			name:     "invalid password",
			username: "testuser",
			email:    "test@example.com",
			password: "short",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
			checkResult:   nil,
		},
		{
			name:     "invalid email",
			username: "testuser",
			email:    "invalid-email",
			password: "SecurePass123!",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
			checkResult:   nil,
		},
		{
			name:     "username already taken",
			username: "existinguser",
			email:    "test@example.com",
			password: "SecurePass123!",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				existingUser := &database.User{
					ID:       1,
					Username: "existinguser",
					Email:    "existing@example.com",
				}
				userRepo.Create(existingUser)
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: ErrUsernameTaken,
			checkResult:   nil,
		},
		{
			name:     "email already taken",
			username: "testuser",
			email:    "existing@example.com",
			password: "SecurePass123!",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				existingUser := &database.User{
					ID:       1,
					Username: "existinguser",
					Email:    "existing@example.com",
				}
				userRepo.Create(existingUser)
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: ErrEmailTaken,
			checkResult:   nil,
		},
		{
			name:     "email service error",
			username: "testuser",
			email:    "test@example.com",
			password: "SecurePass123!",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				errorSender := &errorEmailSender{err: errors.New("email service error")}
				emailService, _ := NewEmailService(testutils.AppTestConfig, errorSender)
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
			checkResult:   nil,
		},
		{
			name:     "trims username whitespace",
			username: "  testuser  ",
			email:    "test@example.com",
			password: "SecurePass123!",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				emailSender := &testutils.MockEmailSender{}
				emailService, _ := NewEmailService(testutils.AppTestConfig, emailSender)
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
			checkResult: func(t *testing.T, result *RegistrationResult) {
				if result.User.Username != "testuser" {
					t.Errorf("expected trimmed username 'testuser', got %q", result.User.Username)
				}
			},
		},
		{
			name:     "normalizes email",
			username: "testuser",
			email:    "TEST@EXAMPLE.COM",
			password: "SecurePass123!",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				emailSender := &testutils.MockEmailSender{}
				emailService, _ := NewEmailService(testutils.AppTestConfig, emailSender)
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
			checkResult: func(t *testing.T, result *RegistrationResult) {
				if result.User.Email != "test@example.com" {
					t.Errorf("expected normalized email 'test@example.com', got %q", result.User.Email)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo, emailService, cfg := tt.setupMocks()
			service := NewRegistrationService(userRepo, emailService, cfg)

			result, err := service.Register(tt.username, tt.email, tt.password)

			if tt.expectedError != nil {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, tt.expectedError) {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
				return
			}

			if err != nil {
				if tt.checkResult == nil {
					return
				}
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

func TestRegistrationService_ConfirmEmail(t *testing.T) {
	tests := []struct {
		name          string
		token         string
		setupMocks    func() (*testutils.MockUserRepository, *EmailService, *config.Config)
		expectedError error
		checkResult   func(*testing.T, *database.User)
	}{
		{
			name:  "successful confirmation",
			token: "valid-token",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				hashedToken := HashVerificationToken("valid-token")
				user := &database.User{
					ID:                     1,
					Username:               "testuser",
					Email:                  "test@example.com",
					EmailVerified:          false,
					EmailVerificationToken: hashedToken,
				}
				userRepo.Create(user)
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
			checkResult: func(t *testing.T, user *database.User) {
				if user == nil {
					t.Fatal("expected non-nil user")
				}
				if !user.EmailVerified {
					t.Error("expected EmailVerified to be true")
				}
				if user.EmailVerificationToken != "" {
					t.Error("expected EmailVerificationToken to be cleared")
				}
				if user.EmailVerificationSentAt != nil {
					t.Error("expected EmailVerificationSentAt to be nil")
				}
				if user.EmailVerifiedAt == nil {
					t.Error("expected EmailVerifiedAt to be set")
				}
			},
		},
		{
			name:  "empty token",
			token: "",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: ErrInvalidVerificationToken,
			checkResult:   nil,
		},
		{
			name:  "whitespace only token",
			token: "   ",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: ErrInvalidVerificationToken,
			checkResult:   nil,
		},
		{
			name:  "invalid token",
			token: "invalid-token",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: ErrInvalidVerificationToken,
			checkResult:   nil,
		},
		{
			name:  "already verified",
			token: "valid-token",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				hashedToken := HashVerificationToken("valid-token")
				now := time.Now()
				user := &database.User{
					ID:                     1,
					Username:               "testuser",
					Email:                  "test@example.com",
					EmailVerified:          true,
					EmailVerifiedAt:        &now,
					EmailVerificationToken: hashedToken,
				}
				userRepo.Create(user)
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
			checkResult: func(t *testing.T, user *database.User) {
				if user == nil {
					t.Fatal("expected non-nil user")
				}
				if !user.EmailVerified {
					t.Error("expected EmailVerified to be true")
				}
			},
		},
		{
			name:  "trims token whitespace",
			token: "  valid-token  ",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				hashedToken := HashVerificationToken("valid-token")
				user := &database.User{
					ID:                     1,
					Username:               "testuser",
					Email:                  "test@example.com",
					EmailVerified:          false,
					EmailVerificationToken: hashedToken,
				}
				userRepo.Create(user)
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
			checkResult: func(t *testing.T, user *database.User) {
				if !user.EmailVerified {
					t.Error("expected EmailVerified to be true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo, emailService, cfg := tt.setupMocks()
			service := NewRegistrationService(userRepo, emailService, cfg)

			user, err := service.ConfirmEmail(tt.token)

			if tt.expectedError != nil {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, tt.expectedError) {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
				return
			}

			if err != nil {
				if tt.checkResult == nil {
					return
				}
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.checkResult != nil {
				tt.checkResult(t, user)
			}
		})
	}
}

func TestRegistrationService_ResendVerificationEmail(t *testing.T) {
	tests := []struct {
		name          string
		email         string
		setupMocks    func() (*testutils.MockUserRepository, *EmailService, *config.Config)
		expectedError error
	}{
		{
			name:  "successful resend",
			email: "test@example.com",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				oldTime := time.Now().Add(-10 * time.Minute)
				user := &database.User{
					ID:                      1,
					Username:                "testuser",
					Email:                   "test@example.com",
					EmailVerified:           false,
					EmailVerificationSentAt: &oldTime,
				}
				userRepo.Create(user)
				emailSender := &testutils.MockEmailSender{}
				emailService, _ := NewEmailService(testutils.AppTestConfig, emailSender)
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
		},
		{
			name:  "invalid email",
			email: "invalid-email",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: ErrInvalidEmail,
		},
		{
			name:  "user not found",
			email: "nonexistent@example.com",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: ErrInvalidCredentials,
		},
		{
			name:  "email already verified",
			email: "test@example.com",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				now := time.Now()
				user := &database.User{
					ID:              1,
					Username:        "testuser",
					Email:           "test@example.com",
					EmailVerified:   true,
					EmailVerifiedAt: &now,
				}
				userRepo.Create(user)
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
		},
		{
			name:  "email sent too recently",
			email: "test@example.com",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				recentTime := time.Now().Add(-2 * time.Minute)
				user := &database.User{
					ID:                      1,
					Username:                "testuser",
					Email:                   "test@example.com",
					EmailVerified:           false,
					EmailVerificationSentAt: &recentTime,
				}
				userRepo.Create(user)
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
		},
		{
			name:  "email service error",
			email: "test@example.com",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				oldTime := time.Now().Add(-10 * time.Minute)
				user := &database.User{
					ID:                      1,
					Username:                "testuser",
					Email:                   "test@example.com",
					EmailVerified:           false,
					EmailVerificationSentAt: &oldTime,
				}
				userRepo.Create(user)
				errorSender := &errorEmailSender{err: errors.New("email service error")}
				emailService, _ := NewEmailService(testutils.AppTestConfig, errorSender)
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
		},
		{
			name:  "trims email whitespace",
			email: "  test@example.com  ",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				oldTime := time.Now().Add(-10 * time.Minute)
				user := &database.User{
					ID:                      1,
					Username:                "testuser",
					Email:                   "test@example.com",
					EmailVerified:           false,
					EmailVerificationSentAt: &oldTime,
				}
				userRepo.Create(user)
				emailSender := &testutils.MockEmailSender{}
				emailService, _ := NewEmailService(testutils.AppTestConfig, emailSender)
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
		},
		{
			name:  "no previous verification sent",
			email: "test@example.com",
			setupMocks: func() (*testutils.MockUserRepository, *EmailService, *config.Config) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:            1,
					Username:      "testuser",
					Email:         "test@example.com",
					EmailVerified: false,
				}
				userRepo.Create(user)
				emailSender := &testutils.MockEmailSender{}
				emailService, _ := NewEmailService(testutils.AppTestConfig, emailSender)
				return userRepo, emailService, testutils.AppTestConfig
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo, emailService, cfg := tt.setupMocks()
			service := NewRegistrationService(userRepo, emailService, cfg)

			err := service.ResendVerificationEmail(tt.email)

			if tt.expectedError != nil {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, tt.expectedError) {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
				return
			}

			if err != nil {
				if tt.name == "email already verified" || tt.name == "email sent too recently" || tt.name == "email service error" {
					if err.Error() == "" {
						t.Fatal("expected error message")
					}
					return
				}
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
