package services

import (
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"goyco/internal/database"
	"goyco/internal/testutils"
)

func TestNewPasswordResetService(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})

	service := NewPasswordResetService(userRepo, emailService)

	if service == nil {
		t.Fatal("expected service to be created")
	}

	if service.userRepo != userRepo {
		t.Error("expected userRepo to be set")
	}

	if service.emailService != emailService {
		t.Error("expected emailService to be set")
	}
}

func TestPasswordResetService_RequestPasswordReset(t *testing.T) {
	tests := []struct {
		name            string
		usernameOrEmail string
		setupMocks      func() (*testutils.MockUserRepository, EmailSender)
		expectedError   bool
		shouldSendEmail bool
	}{
		{
			name:            "successful request by username",
			usernameOrEmail: "testuser",
			setupMocks: func() (*testutils.MockUserRepository, EmailSender) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:       1,
					Username: "testuser",
					Email:    "test@example.com",
				}
				userRepo.Create(user)

				emailSender := &testutils.MockEmailSender{}

				return userRepo, emailSender
			},
			expectedError:   false,
			shouldSendEmail: true,
		},
		{
			name:            "successful request by email",
			usernameOrEmail: "test@example.com",
			setupMocks: func() (*testutils.MockUserRepository, EmailSender) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:       1,
					Username: "testuser",
					Email:    "test@example.com",
				}
				userRepo.Create(user)

				emailSender := &testutils.MockEmailSender{}

				return userRepo, emailSender
			},
			expectedError:   false,
			shouldSendEmail: true,
		},
		{
			name:            "empty input",
			usernameOrEmail: "",
			setupMocks: func() (*testutils.MockUserRepository, EmailSender) {
				return testutils.NewMockUserRepository(), &testutils.MockEmailSender{}
			},
			expectedError:   true,
			shouldSendEmail: false,
		},
		{
			name:            "whitespace only input",
			usernameOrEmail: "   ",
			setupMocks: func() (*testutils.MockUserRepository, EmailSender) {
				return testutils.NewMockUserRepository(), &testutils.MockEmailSender{}
			},
			expectedError:   true,
			shouldSendEmail: false,
		},
		{
			name:            "user not found",
			usernameOrEmail: "nonexistent",
			setupMocks: func() (*testutils.MockUserRepository, EmailSender) {
				return testutils.NewMockUserRepository(), &testutils.MockEmailSender{}
			},
			expectedError:   false,
			shouldSendEmail: false,
		},
		{
			name:            "email service error",
			usernameOrEmail: "testuser",
			setupMocks: func() (*testutils.MockUserRepository, EmailSender) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:       1,
					Username: "testuser",
					Email:    "test@example.com",
				}
				userRepo.Create(user)

				var errorSender errorEmailSender
				errorSender.err = errors.New("email service error")
				emailSender := &errorSender

				return userRepo, emailSender
			},
			expectedError:   true,
			shouldSendEmail: false,
		},
		{
			name:            "prefers email over username",
			usernameOrEmail: "test@example.com",
			setupMocks: func() (*testutils.MockUserRepository, EmailSender) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:       1,
					Username: "testuser",
					Email:    "test@example.com",
				}
				userRepo.Create(user)

				emailSender := &testutils.MockEmailSender{}

				return userRepo, emailSender
			},
			expectedError:   false,
			shouldSendEmail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo, emailSender := tt.setupMocks()
			emailService, _ := NewEmailService(testutils.AppTestConfig, emailSender)

			service := NewPasswordResetService(userRepo, emailService)

			err := service.RequestPasswordReset(tt.usernameOrEmail)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if tt.shouldSendEmail {
					user, _ := userRepo.GetByUsername("testuser")
					if user == nil {
						user, _ = userRepo.GetByEmail("test@example.com")
					}
					if user != nil && user.PasswordResetToken == "" {
						t.Error("expected password reset token to be set")
					}
				}
			}
		})
	}
}

func TestPasswordResetService_ResetPassword(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		newPassword    string
		setupMocks     func() (*testutils.MockUserRepository, EmailSender)
		expectedError  bool
		verifyPassword bool
	}{
		{
			name:        "successful password reset",
			token:       "valid-token",
			newPassword: "NewSecurePass123!",
			setupMocks: func() (*testutils.MockUserRepository, EmailSender) {
				userRepo := testutils.NewMockUserRepository()
				expiresAt := time.Now().Add(time.Hour)
				user := &database.User{
					ID:                     1,
					Username:               "testuser",
					Email:                  "test@example.com",
					PasswordResetToken:     HashVerificationToken("valid-token"),
					PasswordResetExpiresAt: &expiresAt,
				}
				userRepo.Create(user)

				return userRepo, &testutils.MockEmailSender{}
			},
			expectedError:  false,
			verifyPassword: true,
		},
		{
			name:        "empty token",
			token:       "",
			newPassword: "NewSecurePass123!",
			setupMocks: func() (*testutils.MockUserRepository, EmailSender) {
				return testutils.NewMockUserRepository(), &testutils.MockEmailSender{}
			},
			expectedError:  true,
			verifyPassword: false,
		},
		{
			name:        "whitespace only token",
			token:       "   ",
			newPassword: "NewSecurePass123!",
			setupMocks: func() (*testutils.MockUserRepository, EmailSender) {
				return testutils.NewMockUserRepository(), &testutils.MockEmailSender{}
			},
			expectedError:  true,
			verifyPassword: false,
		},
		{
			name:        "invalid token",
			token:       "invalid-token",
			newPassword: "NewSecurePass123!",
			setupMocks: func() (*testutils.MockUserRepository, EmailSender) {
				return testutils.NewMockUserRepository(), &testutils.MockEmailSender{}
			},
			expectedError:  true,
			verifyPassword: false,
		},
		{
			name:        "expired token",
			token:       "expired-token",
			newPassword: "NewSecurePass123!",
			setupMocks: func() (*testutils.MockUserRepository, EmailSender) {
				userRepo := testutils.NewMockUserRepository()
				expiresAt := time.Now().Add(-time.Hour)
				user := &database.User{
					ID:                     1,
					Username:               "testuser",
					Email:                  "test@example.com",
					PasswordResetToken:     HashVerificationToken("expired-token"),
					PasswordResetExpiresAt: &expiresAt,
				}
				userRepo.Create(user)

				return userRepo, &testutils.MockEmailSender{}
			},
			expectedError:  true,
			verifyPassword: false,
		},
		{
			name:        "nil expiration date",
			token:       "valid-token",
			newPassword: "NewSecurePass123!",
			setupMocks: func() (*testutils.MockUserRepository, EmailSender) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:                 1,
					Username:           "testuser",
					Email:              "test@example.com",
					PasswordResetToken: HashVerificationToken("valid-token"),
				}
				userRepo.Create(user)

				return userRepo, &testutils.MockEmailSender{}
			},
			expectedError:  true,
			verifyPassword: false,
		},
		{
			name:        "invalid password",
			token:       "valid-token",
			newPassword: "short",
			setupMocks: func() (*testutils.MockUserRepository, EmailSender) {
				userRepo := testutils.NewMockUserRepository()
				expiresAt := time.Now().Add(time.Hour)
				user := &database.User{
					ID:                     1,
					Username:               "testuser",
					Email:                  "test@example.com",
					PasswordResetToken:     HashVerificationToken("valid-token"),
					PasswordResetExpiresAt: &expiresAt,
				}
				userRepo.Create(user)

				return userRepo, &testutils.MockEmailSender{}
			},
			expectedError:  true,
			verifyPassword: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo, emailSender := tt.setupMocks()
			emailService, _ := NewEmailService(testutils.AppTestConfig, emailSender)

			service := NewPasswordResetService(userRepo, emailService)

			err := service.ResetPassword(tt.token, tt.newPassword)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if tt.verifyPassword {
					user, _ := userRepo.GetByUsername("testuser")
					if user == nil {
						t.Fatal("expected user to exist")
					}

					if user.PasswordResetToken != "" {
						t.Error("expected password reset token to be cleared")
					}

					if user.PasswordResetExpiresAt != nil {
						t.Error("expected password reset expiration to be cleared")
					}

					if user.Password == "" {
						t.Error("expected password to be set")
					}

					err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(tt.newPassword))
					if err != nil {
						t.Errorf("password hash verification failed: %v", err)
					}
				}
			}
		})
	}
}

func TestPasswordResetService_ResetPassword_TokenClearedAfterExpiration(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	expiresAt := time.Now().Add(-time.Hour)
	user := &database.User{
		ID:                     1,
		Username:               "testuser",
		Email:                  "test@example.com",
		PasswordResetToken:     HashVerificationToken("expired-token"),
		PasswordResetExpiresAt: &expiresAt,
	}
	userRepo.Create(user)

	emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
	service := NewPasswordResetService(userRepo, emailService)

	err := service.ResetPassword("expired-token", "NewSecurePass123!")
	if err == nil {
		t.Error("expected error for expired token")
	}

	updatedUser, _ := userRepo.GetByID(1)
	if updatedUser == nil {
		t.Fatal("expected user to exist")
	}

	if updatedUser.PasswordResetToken != "" {
		t.Error("expected password reset token to be cleared after expiration")
	}

	if updatedUser.PasswordResetExpiresAt != nil {
		t.Error("expected password reset expiration to be cleared after expiration")
	}
}

func TestPasswordResetService_RequestPasswordReset_EmailFailureRollback(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	user := &database.User{
		ID:       1,
		Username: "testuser",
		Email:    "test@example.com",
	}
	userRepo.Create(user)

	emailSender := &errorEmailSender{err: errors.New("email service error")}

	emailService, _ := NewEmailService(testutils.AppTestConfig, emailSender)
	service := NewPasswordResetService(userRepo, emailService)

	err := service.RequestPasswordReset("testuser")
	if err == nil {
		t.Error("expected error when email fails")
	}

	updatedUser, _ := userRepo.GetByID(1)
	if updatedUser == nil {
		t.Fatal("expected user to exist")
	}

	if updatedUser.PasswordResetToken != "" {
		t.Error("expected password reset token to be rolled back on email failure")
	}

	if updatedUser.PasswordResetSentAt != nil {
		t.Error("expected password reset sent at to be rolled back on email failure")
	}

	if updatedUser.PasswordResetExpiresAt != nil {
		t.Error("expected password reset expiration to be rolled back on email failure")
	}
}
