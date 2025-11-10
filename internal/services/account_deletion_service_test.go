package services

import (
	"errors"
	"testing"
	"time"

	"goyco/internal/database"
	"goyco/internal/testutils"

	"gorm.io/gorm"
)

type errorEmailSender struct {
	err error
}

func (e *errorEmailSender) Send(to, subject, body string) error {
	return e.err
}

type mockAccountDeletionRepository struct {
	requests            map[uint]*database.AccountDeletionRequest
	requestsByTokenHash map[string]*database.AccountDeletionRequest
	nextID              uint
	createErr           error
	getByTokenHashErr   error
	deleteByIDErr       error
	deleteByUserIDErr   error
}

func newMockAccountDeletionRepository() *mockAccountDeletionRepository {
	return &mockAccountDeletionRepository{
		requests:            make(map[uint]*database.AccountDeletionRequest),
		requestsByTokenHash: make(map[string]*database.AccountDeletionRequest),
		nextID:              1,
	}
}

func (m *mockAccountDeletionRepository) Create(req *database.AccountDeletionRequest) error {
	if m.createErr != nil {
		return m.createErr
	}

	req.ID = m.nextID
	m.nextID++

	reqCopy := *req
	m.requests[req.ID] = &reqCopy
	m.requestsByTokenHash[req.TokenHash] = &reqCopy
	return nil
}

func (m *mockAccountDeletionRepository) GetByTokenHash(hash string) (*database.AccountDeletionRequest, error) {
	if m.getByTokenHashErr != nil {
		return nil, m.getByTokenHashErr
	}

	if req, ok := m.requestsByTokenHash[hash]; ok {
		reqCopy := *req
		return &reqCopy, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockAccountDeletionRepository) DeleteByID(id uint) error {
	if m.deleteByIDErr != nil {
		return m.deleteByIDErr
	}

	if req, ok := m.requests[id]; ok {
		delete(m.requests, id)
		delete(m.requestsByTokenHash, req.TokenHash)
		return nil
	}
	return gorm.ErrRecordNotFound
}

func (m *mockAccountDeletionRepository) DeleteByUserID(userID uint) error {
	if m.deleteByUserIDErr != nil {
		return m.deleteByUserIDErr
	}

	for id, req := range m.requests {
		if req.UserID == userID {
			delete(m.requests, id)
			delete(m.requestsByTokenHash, req.TokenHash)
		}
	}
	return nil
}

func TestNewAccountDeletionService(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	postRepo := testutils.NewMockPostRepository()
	deletionRepo := newMockAccountDeletionRepository()
	emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})

	service := NewAccountDeletionService(userRepo, postRepo, deletionRepo, emailService)

	if service == nil {
		t.Fatal("expected service to be created")
	}

	if service.userRepo != userRepo {
		t.Error("expected userRepo to be set")
	}

	if service.postRepo != postRepo {
		t.Error("expected postRepo to be set")
	}

	if service.deletionRepo != deletionRepo {
		t.Error("expected deletionRepo to be set")
	}

	if service.emailService != emailService {
		t.Error("expected emailService to be set")
	}
}

func TestAccountDeletionService_GetUserIDFromDeletionToken(t *testing.T) {
	tests := []struct {
		name          string
		token         string
		setupRepo     func() *mockAccountDeletionRepository
		expectedID    uint
		expectedError error
	}{
		{
			name:  "successful retrieval",
			token: "valid-token",
			setupRepo: func() *mockAccountDeletionRepository {
				repo := newMockAccountDeletionRepository()
				req := &database.AccountDeletionRequest{
					UserID:    1,
					TokenHash: HashVerificationToken("valid-token"),
					ExpiresAt: time.Now().Add(time.Hour),
				}
				repo.Create(req)
				return repo
			},
			expectedID:    1,
			expectedError: nil,
		},
		{
			name:          "empty token",
			token:         "",
			setupRepo:     func() *mockAccountDeletionRepository { return newMockAccountDeletionRepository() },
			expectedID:    0,
			expectedError: ErrInvalidDeletionToken,
		},
		{
			name:          "whitespace only token",
			token:         "   ",
			setupRepo:     func() *mockAccountDeletionRepository { return newMockAccountDeletionRepository() },
			expectedID:    0,
			expectedError: ErrInvalidDeletionToken,
		},
		{
			name:  "token not found",
			token: "invalid-token",
			setupRepo: func() *mockAccountDeletionRepository {
				return newMockAccountDeletionRepository()
			},
			expectedID:    0,
			expectedError: ErrInvalidDeletionToken,
		},
		{
			name:  "expired token",
			token: "expired-token",
			setupRepo: func() *mockAccountDeletionRepository {
				repo := newMockAccountDeletionRepository()
				req := &database.AccountDeletionRequest{
					UserID:    1,
					TokenHash: HashVerificationToken("expired-token"),
					ExpiresAt: time.Now().Add(-time.Hour),
				}
				repo.Create(req)
				return repo
			},
			expectedID:    0,
			expectedError: ErrInvalidDeletionToken,
		},
		{
			name:  "repository error",
			token: "valid-token",
			setupRepo: func() *mockAccountDeletionRepository {
				repo := newMockAccountDeletionRepository()
				repo.getByTokenHashErr = errors.New("database error")
				return repo
			},
			expectedID:    0,
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := testutils.NewMockUserRepository()
			postRepo := testutils.NewMockPostRepository()
			deletionRepo := tt.setupRepo()
			emailService, _ := NewEmailService(testutils.AppTestConfig, &testutils.MockEmailSender{})

			service := NewAccountDeletionService(userRepo, postRepo, deletionRepo, emailService)

			userID, err := service.GetUserIDFromDeletionToken(tt.token)

			if tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if tt.name == "repository error" || tt.name == "nil repository" {
					if err == nil {
						t.Error("expected error but got none")
					}
				} else if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if userID != tt.expectedID {
				t.Errorf("expected userID %d, got %d", tt.expectedID, userID)
			}
		})
	}
}

func TestAccountDeletionService_RequestAccountDeletion(t *testing.T) {
	tests := []struct {
		name          string
		userID        uint
		setupMocks    func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender)
		expectedError bool
		checkToken    bool
	}{
		{
			name:   "successful request",
			userID: 1,
			setupMocks: func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{ID: 1, Username: "testuser", Email: "test@example.com"}
				userRepo.Create(user)

				deletionRepo := newMockAccountDeletionRepository()
				emailSender := &testutils.MockEmailSender{}

				return userRepo, deletionRepo, emailSender
			},
			expectedError: false,
			checkToken:    true,
		},
		{
			name:   "invalid user ID",
			userID: 0,
			setupMocks: func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender) {
				return testutils.NewMockUserRepository(), newMockAccountDeletionRepository(), &testutils.MockEmailSender{}
			},
			expectedError: true,
			checkToken:    false,
		},
		{
			name:   "user not found",
			userID: 999,
			setupMocks: func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender) {
				return testutils.NewMockUserRepository(), newMockAccountDeletionRepository(), &testutils.MockEmailSender{}
			},
			expectedError: true,
			checkToken:    false,
		},
		{
			name:   "email service error",
			userID: 1,
			setupMocks: func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{ID: 1, Username: "testuser", Email: "test@example.com"}
				userRepo.Create(user)

				deletionRepo := newMockAccountDeletionRepository()
				var errorSender errorEmailSender
				errorSender.err = errors.New("email service error")
				emailSender := &errorSender

				return userRepo, deletionRepo, emailSender
			},
			expectedError: true,
			checkToken:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo, deletionRepo, emailSender := tt.setupMocks()
			postRepo := testutils.NewMockPostRepository()
			emailService, _ := NewEmailService(testutils.AppTestConfig, emailSender)

			service := NewAccountDeletionService(userRepo, postRepo, deletionRepo, emailService)

			err := service.RequestAccountDeletion(tt.userID)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if tt.checkToken {
					if len(deletionRepo.requests) == 0 {
						t.Error("expected deletion request to be created")
					}
				}
			}
		})
	}
}

func TestAccountDeletionService_ConfirmAccountDeletion(t *testing.T) {
	tests := []struct {
		name          string
		token         string
		setupMocks    func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender)
		expectedError error
	}{
		{
			name:  "successful deletion",
			token: "valid-token",
			setupMocks: func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{ID: 1, Username: "testuser", Email: "test@example.com"}
				userRepo.Create(user)

				deletionRepo := newMockAccountDeletionRepository()
				req := &database.AccountDeletionRequest{
					UserID:    1,
					TokenHash: HashVerificationToken("valid-token"),
					ExpiresAt: time.Now().Add(time.Hour),
				}
				deletionRepo.Create(req)

				emailSender := &testutils.MockEmailSender{}

				return userRepo, deletionRepo, emailSender
			},
			expectedError: nil,
		},
		{
			name:  "empty token",
			token: "",
			setupMocks: func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender) {
				return testutils.NewMockUserRepository(), newMockAccountDeletionRepository(), &testutils.MockEmailSender{}
			},
			expectedError: ErrInvalidDeletionToken,
		},
		{
			name:  "token not found",
			token: "invalid-token",
			setupMocks: func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender) {
				return testutils.NewMockUserRepository(), newMockAccountDeletionRepository(), &testutils.MockEmailSender{}
			},
			expectedError: ErrInvalidDeletionToken,
		},
		{
			name:  "expired token",
			token: "expired-token",
			setupMocks: func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender) {
				deletionRepo := newMockAccountDeletionRepository()
				req := &database.AccountDeletionRequest{
					UserID:    1,
					TokenHash: HashVerificationToken("expired-token"),
					ExpiresAt: time.Now().Add(-time.Hour),
				}
				deletionRepo.Create(req)

				return testutils.NewMockUserRepository(), deletionRepo, &testutils.MockEmailSender{}
			},
			expectedError: ErrInvalidDeletionToken,
		},
		{
			name:  "user not found",
			token: "valid-token",
			setupMocks: func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender) {
				deletionRepo := newMockAccountDeletionRepository()
				req := &database.AccountDeletionRequest{
					UserID:    999,
					TokenHash: HashVerificationToken("valid-token"),
					ExpiresAt: time.Now().Add(time.Hour),
				}
				deletionRepo.Create(req)

				return testutils.NewMockUserRepository(), deletionRepo, &testutils.MockEmailSender{}
			},
			expectedError: ErrInvalidDeletionToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo, deletionRepo, emailSender := tt.setupMocks()
			postRepo := testutils.NewMockPostRepository()
			emailService, _ := NewEmailService(testutils.AppTestConfig, emailSender)

			service := NewAccountDeletionService(userRepo, postRepo, deletionRepo, emailService)

			err := service.ConfirmAccountDeletion(tt.token)

			if tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestAccountDeletionService_ConfirmAccountDeletionWithPosts(t *testing.T) {
	tests := []struct {
		name          string
		token         string
		deletePosts   bool
		setupMocks    func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender)
		expectedError error
	}{
		{
			name:        "successful deletion without posts",
			token:       "valid-token",
			deletePosts: false,
			setupMocks: func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{ID: 1, Username: "testuser", Email: "test@example.com"}
				userRepo.Create(user)

				deletionRepo := newMockAccountDeletionRepository()
				req := &database.AccountDeletionRequest{
					UserID:    1,
					TokenHash: HashVerificationToken("valid-token"),
					ExpiresAt: time.Now().Add(time.Hour),
				}
				deletionRepo.Create(req)

				emailSender := &testutils.MockEmailSender{}

				return userRepo, deletionRepo, emailSender
			},
			expectedError: nil,
		},
		{
			name:        "successful deletion with posts",
			token:       "valid-token",
			deletePosts: true,
			setupMocks: func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{ID: 1, Username: "testuser", Email: "test@example.com"}
				userRepo.Create(user)

				deletionRepo := newMockAccountDeletionRepository()
				req := &database.AccountDeletionRequest{
					UserID:    1,
					TokenHash: HashVerificationToken("valid-token"),
					ExpiresAt: time.Now().Add(time.Hour),
				}
				deletionRepo.Create(req)

				emailSender := &testutils.MockEmailSender{}

				return userRepo, deletionRepo, emailSender
			},
			expectedError: nil,
		},
		{
			name:        "empty token",
			token:       "",
			deletePosts: false,
			setupMocks: func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender) {
				return testutils.NewMockUserRepository(), newMockAccountDeletionRepository(), &testutils.MockEmailSender{}
			},
			expectedError: ErrInvalidDeletionToken,
		},
		{
			name:        "expired token",
			token:       "expired-token",
			deletePosts: false,
			setupMocks: func() (*testutils.MockUserRepository, *mockAccountDeletionRepository, EmailSender) {
				deletionRepo := newMockAccountDeletionRepository()
				req := &database.AccountDeletionRequest{
					UserID:    1,
					TokenHash: HashVerificationToken("expired-token"),
					ExpiresAt: time.Now().Add(-time.Hour),
				}
				deletionRepo.Create(req)

				return testutils.NewMockUserRepository(), deletionRepo, &testutils.MockEmailSender{}
			},
			expectedError: ErrInvalidDeletionToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo, deletionRepo, emailSender := tt.setupMocks()
			postRepo := testutils.NewMockPostRepository()
			emailService, _ := NewEmailService(testutils.AppTestConfig, emailSender)

			service := NewAccountDeletionService(userRepo, postRepo, deletionRepo, emailService)

			err := service.ConfirmAccountDeletionWithPosts(tt.token, tt.deletePosts)

			if tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestAccountDeletionService_UserHasPosts(t *testing.T) {
}
