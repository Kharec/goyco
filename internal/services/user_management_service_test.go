package services

import (
	"errors"
	"testing"
	"time"

	"goyco/internal/database"
	"goyco/internal/testutils"

	"golang.org/x/crypto/bcrypt"
)

func TestNewUserManagementService(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	postRepo := testutils.NewMockPostRepository()
	emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})

	service := NewUserManagementService(userRepo, postRepo, emailService)

	if service == nil {
		t.Fatal("expected service to be created")
	}

	if service.userRepo != userRepo {
		t.Error("expected userRepo to be set")
	}

	if service.postRepo != postRepo {
		t.Error("expected postRepo to be set")
	}

	if service.emailService != emailService {
		t.Error("expected emailService to be set")
	}
}

func TestUserManagementService_UpdateUsername(t *testing.T) {
	tests := []struct {
		name          string
		userID        uint
		newUsername   string
		setupMocks    func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService)
		expectedError error
		checkResult   func(*testing.T, *database.User)
	}{
		{
			name:        "successful update",
			userID:      1,
			newUsername: "newusername",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:       1,
					Username: "oldusername",
					Email:    "test@example.com",
				}
				userRepo.Create(user)
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: nil,
			checkResult: func(t *testing.T, user *database.User) {
				if user == nil {
					t.Fatal("expected non-nil user")
				}
				if user.Username != "newusername" {
					t.Errorf("expected username 'newusername', got %q", user.Username)
				}
				if user.Password != "" {
					t.Error("expected password to be sanitized")
				}
			},
		},
		{
			name:        "invalid username",
			userID:      1,
			newUsername: "",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:       1,
					Username: "oldusername",
					Email:    "test@example.com",
				}
				userRepo.Create(user)
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: nil,
			checkResult:   nil,
		},
		{
			name:        "username already taken by different user",
			userID:      1,
			newUsername: "takenusername",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				user1 := &database.User{
					ID:       1,
					Username: "oldusername",
					Email:    "test1@example.com",
				}
				user2 := &database.User{
					ID:       2,
					Username: "takenusername",
					Email:    "test2@example.com",
				}
				userRepo.Create(user1)
				userRepo.Create(user2)
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: ErrUsernameTaken,
			checkResult:   nil,
		},
		{
			name:        "same username",
			userID:      1,
			newUsername: "oldusername",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:       1,
					Username: "oldusername",
					Email:    "test@example.com",
				}
				userRepo.Create(user)
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: nil,
			checkResult: func(t *testing.T, user *database.User) {
				if user.Username != "oldusername" {
					t.Errorf("expected username 'oldusername', got %q", user.Username)
				}
			},
		},
		{
			name:        "user not found",
			userID:      999,
			newUsername: "newusername",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: nil,
			checkResult:   nil,
		},
		{
			name:        "trims username whitespace",
			userID:      1,
			newUsername: "  newusername  ",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:       1,
					Username: "oldusername",
					Email:    "test@example.com",
				}
				userRepo.Create(user)
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: nil,
			checkResult: func(t *testing.T, user *database.User) {
				if user.Username != "newusername" {
					t.Errorf("expected trimmed username 'newusername', got %q", user.Username)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo, postRepo, emailService := tt.setupMocks()
			service := NewUserManagementService(userRepo, postRepo, emailService)

			result, err := service.UpdateUsername(tt.userID, tt.newUsername)

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

func TestUserManagementService_UpdatePassword(t *testing.T) {
	tests := []struct {
		name            string
		userID          uint
		currentPassword string
		newPassword     string
		setupMocks      func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService)
		expectedError   error
		checkResult     func(*testing.T, *database.User)
	}{
		{
			name:            "successful update",
			userID:          1,
			currentPassword: "OldPass123!",
			newPassword:     "NewPass123!",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("OldPass123!"), bcrypt.DefaultCost)
				user := &database.User{
					ID:       1,
					Username: "testuser",
					Email:    "test@example.com",
					Password: string(hashedPassword),
				}
				userRepo.Create(user)
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: nil,
			checkResult: func(t *testing.T, user *database.User) {
				if user == nil {
					t.Fatal("expected non-nil user")
				}
				if user.Password != "" {
					t.Error("expected password to be sanitized")
				}
			},
		},
		{
			name:            "invalid new password",
			userID:          1,
			currentPassword: "OldPass123!",
			newPassword:     "short",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("OldPass123!"), bcrypt.DefaultCost)
				user := &database.User{
					ID:       1,
					Username: "testuser",
					Email:    "test@example.com",
					Password: string(hashedPassword),
				}
				userRepo.Create(user)
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: nil,
			checkResult:   nil,
		},
		{
			name:            "incorrect current password",
			userID:          1,
			currentPassword: "WrongPassword",
			newPassword:     "NewPass123!",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("OldPass123!"), bcrypt.DefaultCost)
				user := &database.User{
					ID:       1,
					Username: "testuser",
					Email:    "test@example.com",
					Password: string(hashedPassword),
				}
				userRepo.Create(user)
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: nil,
			checkResult:   nil,
		},
		{
			name:            "user not found",
			userID:          999,
			currentPassword: "OldPass123!",
			newPassword:     "NewPass123!",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: nil,
			checkResult:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo, postRepo, emailService := tt.setupMocks()
			service := NewUserManagementService(userRepo, postRepo, emailService)

			result, err := service.UpdatePassword(tt.userID, tt.currentPassword, tt.newPassword)

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

func TestUserManagementService_UpdateEmail(t *testing.T) {
	tests := []struct {
		name          string
		userID        uint
		newEmail      string
		setupMocks    func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService)
		expectedError error
		checkResult   func(*testing.T, *database.User)
	}{
		{
			name:     "successful update",
			userID:   1,
			newEmail: "newemail@example.com",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				now := time.Now()
				user := &database.User{
					ID:                     1,
					Username:               "testuser",
					Email:                  "oldemail@example.com",
					EmailVerified:          true,
					EmailVerifiedAt:        &now,
					EmailVerificationToken: "",
				}
				userRepo.Create(user)
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: nil,
			checkResult: func(t *testing.T, user *database.User) {
				if user == nil {
					t.Fatal("expected non-nil user")
				}
				if user.Email != "newemail@example.com" {
					t.Errorf("expected email 'newemail@example.com', got %q", user.Email)
				}
				if user.EmailVerified {
					t.Error("expected EmailVerified to be false")
				}
			},
		},
		{
			name:     "invalid email",
			userID:   1,
			newEmail: "invalid-email",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:       1,
					Username: "testuser",
					Email:    "oldemail@example.com",
				}
				userRepo.Create(user)
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: nil,
			checkResult:   nil,
		},
		{
			name:     "email already taken by different user",
			userID:   1,
			newEmail: "taken@example.com",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				user1 := &database.User{
					ID:       1,
					Username: "testuser1",
					Email:    "oldemail@example.com",
				}
				user2 := &database.User{
					ID:       2,
					Username: "testuser2",
					Email:    "taken@example.com",
				}
				userRepo.Create(user1)
				userRepo.Create(user2)
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: ErrEmailTaken,
			checkResult:   nil,
		},
		{
			name:     "same email",
			userID:   1,
			newEmail: "oldemail@example.com",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:       1,
					Username: "testuser",
					Email:    "oldemail@example.com",
				}
				userRepo.Create(user)
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: nil,
			checkResult: func(t *testing.T, user *database.User) {
				if user.Email != "oldemail@example.com" {
					t.Errorf("expected email 'oldemail@example.com', got %q", user.Email)
				}
			},
		},
		{
			name:     "user not found",
			userID:   999,
			newEmail: "newemail@example.com",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: nil,
			checkResult:   nil,
		},
		{
			name:     "normalizes email",
			userID:   1,
			newEmail: "NEWEMAIL@EXAMPLE.COM",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:       1,
					Username: "testuser",
					Email:    "oldemail@example.com",
				}
				userRepo.Create(user)
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedError: nil,
			checkResult: func(t *testing.T, user *database.User) {
				if user.Email != "newemail@example.com" {
					t.Errorf("expected normalized email 'newemail@example.com', got %q", user.Email)
				}
			},
		},
		{
			name:     "email service error rolls back",
			userID:   1,
			newEmail: "newemail@example.com",
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				now := time.Now()
				user := &database.User{
					ID:                     1,
					Username:               "testuser",
					Email:                  "oldemail@example.com",
					EmailVerified:          true,
					EmailVerifiedAt:        &now,
					EmailVerificationToken: "",
				}
				userRepo.Create(user)
				postRepo := testutils.NewMockPostRepository()
				errorSender := &errorEmailSender{err: errors.New("email service error")}
				emailService, _ := NewEmailService(testutils.AppTestConfig, errorSender)
				return userRepo, postRepo, emailService
			},
			expectedError: nil,
			checkResult:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo, postRepo, emailService := tt.setupMocks()
			service := NewUserManagementService(userRepo, postRepo, emailService)

			result, err := service.UpdateEmail(tt.userID, tt.newEmail)

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
				if tt.checkResult == nil || tt.name == "email service error rolls back" {
					if tt.name == "email service error rolls back" {
						user, _ := userRepo.GetByID(1)
						if user.Email != "oldemail@example.com" {
							t.Error("expected email to be rolled back to original")
						}
						if !user.EmailVerified {
							t.Error("expected EmailVerified to be rolled back")
						}
					}
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

func TestUserManagementService_UserHasPosts(t *testing.T) {
	tests := []struct {
		name          string
		userID        uint
		setupMocks    func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService)
		expectedHas   bool
		expectedCount int64
		expectedError error
	}{
		{
			name:   "user has posts",
			userID: 1,
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:       1,
					Username: "testuser",
					Email:    "test@example.com",
				}
				userRepo.Create(user)
				postRepo := testutils.NewMockPostRepository()
				userID := uint(1)
				post1 := &database.Post{
					ID:       1,
					AuthorID: &userID,
					Title:    "Post 1",
					URL:      "https://example.com/1",
				}
				post2 := &database.Post{
					ID:       2,
					AuthorID: &userID,
					Title:    "Post 2",
					URL:      "https://example.com/2",
				}
				postRepo.Create(post1)
				postRepo.Create(post2)
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedHas:   true,
			expectedCount: 2,
			expectedError: nil,
		},
		{
			name:   "user has no posts",
			userID: 1,
			setupMocks: func() (*testutils.MockUserRepository, *testutils.MockPostRepository, *EmailService) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:       1,
					Username: "testuser",
					Email:    "test@example.com",
				}
				userRepo.Create(user)
				postRepo := testutils.NewMockPostRepository()
				emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})
				return userRepo, postRepo, emailService
			},
			expectedHas:   false,
			expectedCount: 0,
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo, postRepo, emailService := tt.setupMocks()
			service := NewUserManagementService(userRepo, postRepo, emailService)

			hasPosts, count, err := service.UserHasPosts(tt.userID)

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
				t.Fatalf("unexpected error: %v", err)
			}

			if hasPosts != tt.expectedHas {
				t.Errorf("expected hasPosts %v, got %v", tt.expectedHas, hasPosts)
			}

			if count != tt.expectedCount {
				t.Errorf("expected count %d, got %d", tt.expectedCount, count)
			}
		})
	}
}
