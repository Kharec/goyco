package services

import (
	"errors"
	"testing"
	"time"

	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/testutils"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type sessionMockRefreshTokenRepo struct {
	tokens            map[string]*database.RefreshToken
	createErr         error
	deleteByUserIDErr error
	deleteExpiredErr  error
	getByTokenHashErr error
}

func newSessionMockRefreshTokenRepo() *sessionMockRefreshTokenRepo {
	return &sessionMockRefreshTokenRepo{
		tokens: make(map[string]*database.RefreshToken),
	}
}

func (m *sessionMockRefreshTokenRepo) Create(token *database.RefreshToken) error {
	if m.createErr != nil {
		return m.createErr
	}
	if m.tokens == nil {
		m.tokens = make(map[string]*database.RefreshToken)
	}
	m.tokens[token.TokenHash] = token
	return nil
}

func (m *sessionMockRefreshTokenRepo) GetByTokenHash(tokenHash string) (*database.RefreshToken, error) {
	if m.getByTokenHashErr != nil {
		return nil, m.getByTokenHashErr
	}
	if token, ok := m.tokens[tokenHash]; ok {
		return token, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *sessionMockRefreshTokenRepo) DeleteByUserID(userID uint) error {
	if m.deleteByUserIDErr != nil {
		return m.deleteByUserIDErr
	}
	for hash, token := range m.tokens {
		if token.UserID == userID {
			delete(m.tokens, hash)
		}
	}
	return nil
}

func (m *sessionMockRefreshTokenRepo) DeleteExpired() error {
	if m.deleteExpiredErr != nil {
		return m.deleteExpiredErr
	}
	now := time.Now()
	for hash, token := range m.tokens {
		if token.ExpiresAt.Before(now) {
			delete(m.tokens, hash)
		}
	}
	return nil
}

func (m *sessionMockRefreshTokenRepo) DeleteByID(id uint) error {
	for hash, token := range m.tokens {
		if token.ID == id {
			delete(m.tokens, hash)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *sessionMockRefreshTokenRepo) GetByUserID(userID uint) ([]database.RefreshToken, error) {
	var tokens []database.RefreshToken
	for _, token := range m.tokens {
		if token.UserID == userID {
			tokens = append(tokens, *token)
		}
	}
	return tokens, nil
}

func (m *sessionMockRefreshTokenRepo) CountByUserID(userID uint) (int64, error) {
	var count int64
	for _, token := range m.tokens {
		if token.UserID == userID {
			count++
		}
	}
	return count, nil
}

func createSessionTestJWTService(userRepo *testutils.MockUserRepository) (*JWTService, *sessionMockRefreshTokenRepo) {
	cfg := &config.JWTConfig{
		Secret:            "test-secret-key-that-is-long-enough-for-security",
		Expiration:        1,
		RefreshExpiration: 24,
		Issuer:            "test-issuer",
		Audience:          "test-audience",
		KeyRotation: config.KeyRotationConfig{
			Enabled:     false,
			CurrentKey:  "",
			PreviousKey: "",
			KeyID:       "",
		},
	}

	refreshRepo := newSessionMockRefreshTokenRepo()
	jwtService := NewJWTService(cfg, userRepo, refreshRepo)
	return jwtService, refreshRepo
}

func createTestUserWithPassword(password string) *database.User {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return &database.User{
		ID:             1,
		Username:       "testuser",
		Email:          "test@example.com",
		Password:       string(hashedPassword),
		EmailVerified:  true,
		Locked:         false,
		SessionVersion: 1,
	}
}

func TestNewSessionService(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	jwtService, _ := createSessionTestJWTService(userRepo)

	service := NewSessionService(jwtService, userRepo)

	if service == nil {
		t.Fatal("expected service to be created")
	}

	if service.jwtService != jwtService {
		t.Error("expected jwtService to be set")
	}

	if service.userRepo != userRepo {
		t.Error("expected userRepo to be set")
	}
}

func TestSessionService_Login(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		password      string
		setupMocks    func() (*JWTService, *testutils.MockUserRepository)
		expectedError error
		checkResult   func(*testing.T, *AuthResult)
	}{
		{
			name:     "successful login",
			username: "testuser",
			password: "SecurePass123!",
			setupMocks: func() (*JWTService, *testutils.MockUserRepository) {
				userRepo := testutils.NewMockUserRepository()
				user := createTestUserWithPassword("SecurePass123!")
				userRepo.Create(user)
				jwtService, _ := createSessionTestJWTService(userRepo)
				return jwtService, userRepo
			},
			expectedError: nil,
			checkResult: func(t *testing.T, result *AuthResult) {
				if result == nil {
					t.Fatal("expected non-nil result")
				}
				if result.AccessToken == "" {
					t.Error("expected non-empty access token")
				}
				if result.RefreshToken == "" {
					t.Error("expected non-empty refresh token")
				}
				if result.User == nil {
					t.Fatal("expected non-nil user")
				}
				if result.User.Username != "testuser" {
					t.Errorf("expected username 'testuser', got %q", result.User.Username)
				}
				if result.User.Password != "" {
					t.Error("expected password to be sanitized")
				}
			},
		},
		{
			name:     "empty username",
			username: "",
			password: "SecurePass123!",
			setupMocks: func() (*JWTService, *testutils.MockUserRepository) {
				userRepo := testutils.NewMockUserRepository()
				jwtService, _ := createSessionTestJWTService(userRepo)
				return jwtService, userRepo
			},
			expectedError: ErrInvalidCredentials,
			checkResult:   nil,
		},
		{
			name:     "whitespace only username",
			username: "   ",
			password: "SecurePass123!",
			setupMocks: func() (*JWTService, *testutils.MockUserRepository) {
				userRepo := testutils.NewMockUserRepository()
				jwtService, _ := createSessionTestJWTService(userRepo)
				return jwtService, userRepo
			},
			expectedError: ErrInvalidCredentials,
			checkResult:   nil,
		},
		{
			name:     "user not found",
			username: "nonexistent",
			password: "SecurePass123!",
			setupMocks: func() (*JWTService, *testutils.MockUserRepository) {
				userRepo := testutils.NewMockUserRepository()
				jwtService, _ := createSessionTestJWTService(userRepo)
				return jwtService, userRepo
			},
			expectedError: ErrInvalidCredentials,
			checkResult:   nil,
		},
		{
			name:     "email not verified",
			username: "testuser",
			password: "SecurePass123!",
			setupMocks: func() (*JWTService, *testutils.MockUserRepository) {
				userRepo := testutils.NewMockUserRepository()
				user := createTestUserWithPassword("SecurePass123!")
				user.EmailVerified = false
				userRepo.Create(user)
				jwtService, _ := createSessionTestJWTService(userRepo)
				return jwtService, userRepo
			},
			expectedError: ErrEmailNotVerified,
			checkResult:   nil,
		},
		{
			name:     "account locked",
			username: "testuser",
			password: "SecurePass123!",
			setupMocks: func() (*JWTService, *testutils.MockUserRepository) {
				userRepo := testutils.NewMockUserRepository()
				user := createTestUserWithPassword("SecurePass123!")
				user.Locked = true
				userRepo.Create(user)
				jwtService, _ := createSessionTestJWTService(userRepo)
				return jwtService, userRepo
			},
			expectedError: ErrAccountLocked,
			checkResult:   nil,
		},
		{
			name:     "invalid password",
			username: "testuser",
			password: "WrongPassword",
			setupMocks: func() (*JWTService, *testutils.MockUserRepository) {
				userRepo := testutils.NewMockUserRepository()
				user := createTestUserWithPassword("SecurePass123!")
				userRepo.Create(user)
				jwtService, _ := createSessionTestJWTService(userRepo)
				return jwtService, userRepo
			},
			expectedError: ErrInvalidCredentials,
			checkResult:   nil,
		},
		{
			name:     "trims username whitespace",
			username: "  testuser  ",
			password: "SecurePass123!",
			setupMocks: func() (*JWTService, *testutils.MockUserRepository) {
				userRepo := testutils.NewMockUserRepository()
				user := createTestUserWithPassword("SecurePass123!")
				userRepo.Create(user)
				jwtService, _ := createSessionTestJWTService(userRepo)
				return jwtService, userRepo
			},
			expectedError: nil,
			checkResult: func(t *testing.T, result *AuthResult) {
				if result.User.Username != "testuser" {
					t.Errorf("expected trimmed username 'testuser', got %q", result.User.Username)
				}
				if result.AccessToken == "" {
					t.Error("expected non-empty access token")
				}
				if result.RefreshToken == "" {
					t.Error("expected non-empty refresh token")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jwtService, userRepo := tt.setupMocks()
			service := NewSessionService(jwtService, userRepo)

			result, err := service.Login(tt.username, tt.password)

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

func TestSessionService_VerifyToken(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	user := createTestUserWithPassword("SecurePass123!")
	userRepo.Create(user)
	jwtService, _ := createSessionTestJWTService(userRepo)

	t.Run("successful verification", func(t *testing.T) {
		service := NewSessionService(jwtService, userRepo)
		token, err := jwtService.GenerateAccessToken(user)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		userID, err := service.VerifyToken(token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if userID != user.ID {
			t.Errorf("expected user ID %d, got %d", user.ID, userID)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		service := NewSessionService(jwtService, userRepo)
		_, err := service.VerifyToken("invalid-token")
		if err == nil {
			t.Fatal("expected error for invalid token")
		}
	})

	t.Run("empty token", func(t *testing.T) {
		service := NewSessionService(jwtService, userRepo)
		_, err := service.VerifyToken("")
		if err == nil {
			t.Fatal("expected error for empty token")
		}
	})
}

func TestSessionService_RefreshAccessToken(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	user := createTestUserWithPassword("SecurePass123!")
	userRepo.Create(user)
	jwtService, _ := createSessionTestJWTService(userRepo)

	t.Run("successful refresh", func(t *testing.T) {
		service := NewSessionService(jwtService, userRepo)
		refreshToken, err := jwtService.GenerateRefreshToken(user)
		if err != nil {
			t.Fatalf("failed to generate refresh token: %v", err)
		}

		result, err := service.RefreshAccessToken(refreshToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.AccessToken == "" {
			t.Error("expected non-empty access token")
		}
		if result.RefreshToken != refreshToken {
			t.Errorf("expected refresh token to remain unchanged")
		}
		if result.User == nil {
			t.Fatal("expected non-nil user")
		}
	})

	t.Run("invalid refresh token", func(t *testing.T) {
		service := NewSessionService(jwtService, userRepo)
		_, err := service.RefreshAccessToken("invalid-refresh-token")
		if err == nil {
			t.Fatal("expected error for invalid refresh token")
		}
		if !errors.Is(err, ErrRefreshTokenInvalid) {
			t.Errorf("expected ErrRefreshTokenInvalid, got %v", err)
		}
	})
}

func TestSessionService_RevokeRefreshToken(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	user := createTestUserWithPassword("SecurePass123!")
	userRepo.Create(user)
	jwtService, _ := createSessionTestJWTService(userRepo)

	t.Run("successful revocation", func(t *testing.T) {
		service := NewSessionService(jwtService, userRepo)
		refreshToken, err := jwtService.GenerateRefreshToken(user)
		if err != nil {
			t.Fatalf("failed to generate refresh token: %v", err)
		}

		err = service.RevokeRefreshToken(refreshToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = service.RefreshAccessToken(refreshToken)
		if err == nil {
			t.Fatal("expected error when using revoked refresh token")
		}
	})
}

func TestSessionService_RevokeAllUserTokens(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	user := createTestUserWithPassword("SecurePass123!")
	userRepo.Create(user)
	jwtService, _ := createSessionTestJWTService(userRepo)

	t.Run("successful revocation", func(t *testing.T) {
		service := NewSessionService(jwtService, userRepo)
		refreshToken, err := jwtService.GenerateRefreshToken(user)
		if err != nil {
			t.Fatalf("failed to generate refresh token: %v", err)
		}

		err = service.RevokeAllUserTokens(user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = service.RefreshAccessToken(refreshToken)
		if err == nil {
			t.Fatal("expected error when using revoked refresh token")
		}
	})
}

func TestSessionService_InvalidateAllSessions(t *testing.T) {
	tests := []struct {
		name          string
		userID        uint
		setupMocks    func() (*JWTService, *testutils.MockUserRepository)
		expectedError error
		checkResult   func(*testing.T, *testutils.MockUserRepository)
	}{
		{
			name:   "successful invalidation",
			userID: 1,
			setupMocks: func() (*JWTService, *testutils.MockUserRepository) {
				userRepo := testutils.NewMockUserRepository()
				user := &database.User{
					ID:             1,
					Username:       "testuser",
					Email:          "test@example.com",
					SessionVersion: 1,
				}
				userRepo.Create(user)
				jwtService, _ := createSessionTestJWTService(userRepo)
				return jwtService, userRepo
			},
			expectedError: nil,
			checkResult: func(t *testing.T, userRepo *testutils.MockUserRepository) {
				user, err := userRepo.GetByID(1)
				if err != nil {
					t.Fatalf("failed to get user: %v", err)
				}
				if user.SessionVersion != 2 {
					t.Errorf("expected SessionVersion to be 2, got %d", user.SessionVersion)
				}
			},
		},
		{
			name:   "user not found",
			userID: 999,
			setupMocks: func() (*JWTService, *testutils.MockUserRepository) {
				userRepo := testutils.NewMockUserRepository()
				jwtService, _ := createSessionTestJWTService(userRepo)
				return jwtService, userRepo
			},
			expectedError: nil,
			checkResult:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jwtService, userRepo := tt.setupMocks()
			service := NewSessionService(jwtService, userRepo)

			err := service.InvalidateAllSessions(tt.userID)

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
				if tt.name == "user not found" {
					if err.Error() == "" {
						t.Fatal("expected error message")
					}
					return
				}
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.checkResult != nil {
				tt.checkResult(t, userRepo)
			}
		})
	}
}

func TestSessionService_CleanupExpiredTokens(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	jwtService, _ := createSessionTestJWTService(userRepo)

	t.Run("successful cleanup", func(t *testing.T) {
		service := NewSessionService(jwtService, userRepo)
		err := service.CleanupExpiredTokens()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
