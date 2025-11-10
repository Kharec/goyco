package services

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
	"goyco/internal/config"
	"goyco/internal/database"
)

type jwtMockUserRepo struct {
	users map[uint]*database.User
}

func (m *jwtMockUserRepo) GetByID(id uint) (*database.User, error) {
	if user, exists := m.users[id]; exists {
		return user, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *jwtMockUserRepo) GetByUsername(username string) (*database.User, error) {
	for _, user := range m.users {
		if user.Username == username {
			return user, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *jwtMockUserRepo) Update(user *database.User) error {
	if _, exists := m.users[user.ID]; !exists {
		return gorm.ErrRecordNotFound
	}
	m.users[user.ID] = user
	return nil
}

type jwtMockRefreshTokenRepo struct {
	tokens map[string]*database.RefreshToken
	nextID uint
}

func (m *jwtMockRefreshTokenRepo) Create(token *database.RefreshToken) error {
	if m.tokens == nil {
		m.tokens = make(map[string]*database.RefreshToken)
	}
	m.nextID++
	token.ID = m.nextID
	m.tokens[token.TokenHash] = token
	return nil
}

func (m *jwtMockRefreshTokenRepo) GetByTokenHash(tokenHash string) (*database.RefreshToken, error) {
	if token, exists := m.tokens[tokenHash]; exists {
		return token, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *jwtMockRefreshTokenRepo) DeleteByUserID(userID uint) error {
	for hash, token := range m.tokens {
		if token.UserID == userID {
			delete(m.tokens, hash)
		}
	}
	return nil
}

func (m *jwtMockRefreshTokenRepo) DeleteExpired() error {
	now := time.Now()
	for hash, token := range m.tokens {
		if token.ExpiresAt.Before(now) {
			delete(m.tokens, hash)
		}
	}
	return nil
}

func (m *jwtMockRefreshTokenRepo) DeleteByID(id uint) error {
	for hash, token := range m.tokens {
		if token.ID == id {
			delete(m.tokens, hash)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *jwtMockRefreshTokenRepo) GetByUserID(userID uint) ([]database.RefreshToken, error) {
	var tokens []database.RefreshToken
	for _, token := range m.tokens {
		if token.UserID == userID {
			tokens = append(tokens, *token)
		}
	}
	return tokens, nil
}

func (m *jwtMockRefreshTokenRepo) CountByUserID(userID uint) (int64, error) {
	var count int64
	for _, token := range m.tokens {
		if token.UserID == userID {
			count++
		}
	}
	return count, nil
}

func createTestJWTService() (*JWTService, *jwtMockUserRepo, *jwtMockRefreshTokenRepo) {
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

	userRepo := &jwtMockUserRepo{
		users: make(map[uint]*database.User),
	}
	refreshRepo := &jwtMockRefreshTokenRepo{
		tokens: make(map[string]*database.RefreshToken),
	}

	jwtService := NewJWTService(cfg, userRepo, refreshRepo)
	return jwtService, userRepo, refreshRepo
}

func createTestUser() *database.User {
	return &database.User{
		ID:             1,
		Username:       "testuser",
		Email:          "test@example.com",
		Password:       "hashedpassword",
		EmailVerified:  true,
		Locked:         false,
		SessionVersion: 1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func TestJWTService_GenerateAccessToken(t *testing.T) {
	jwtService, userRepo, _ := createTestJWTService()
	user := createTestUser()
	userRepo.users[user.ID] = user

	t.Run("Successful_Generation", func(t *testing.T) {
		token, err := jwtService.GenerateAccessToken(user)
		if err != nil {
			t.Fatalf("Expected successful token generation, got error: %v", err)
		}

		if token == "" {
			t.Error("Expected non-empty token")
		}

		parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
			return []byte(jwtService.config.Secret), nil
		})
		if err != nil {
			t.Fatalf("Failed to parse generated token: %v", err)
		}

		if !parsedToken.Valid {
			t.Error("Generated token should be valid")
		}

		if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
			if claims["sub"] != float64(user.ID) {
				t.Errorf("Expected subject %d, got %v", user.ID, claims["sub"])
			}
			if claims["username"] != user.Username {
				t.Errorf("Expected username %s, got %v", user.Username, claims["username"])
			}
			if claims["session_version"] != float64(user.SessionVersion) {
				t.Errorf("Expected session_version %d, got %v", user.SessionVersion, claims["session_version"])
			}
			if claims["type"] != TokenTypeAccess {
				t.Errorf("Expected type %s, got %v", TokenTypeAccess, claims["type"])
			}
			if claims["iss"] != jwtService.config.Issuer {
				t.Errorf("Expected issuer %s, got %v", jwtService.config.Issuer, claims["iss"])
			}
			if aud, ok := claims["aud"].([]any); !ok || len(aud) != 1 || aud[0] != jwtService.config.Audience {
				t.Errorf("Expected audience [%s], got %v", jwtService.config.Audience, claims["aud"])
			}
		}
	})

	t.Run("Nil_User", func(t *testing.T) {
		_, err := jwtService.GenerateAccessToken(nil)
		if err == nil {
			t.Error("Expected error for nil user")
		}
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("Expected ErrInvalidCredentials, got %v", err)
		}
	})
}

func TestJWTService_GenerateRefreshToken(t *testing.T) {
	jwtService, userRepo, refreshRepo := createTestJWTService()
	user := createTestUser()
	userRepo.users[user.ID] = user

	t.Run("Successful_Generation", func(t *testing.T) {
		token, err := jwtService.GenerateRefreshToken(user)
		if err != nil {
			t.Fatalf("Expected successful refresh token generation, got error: %v", err)
		}

		if token == "" {
			t.Error("Expected non-empty refresh token")
		}

		tokenHash := jwtService.hashToken(token)
		storedToken, err := refreshRepo.GetByTokenHash(tokenHash)
		if err != nil {
			t.Fatalf("Expected refresh token to be stored in database: %v", err)
		}

		if storedToken.UserID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, storedToken.UserID)
		}

		expectedExpiry := time.Now().Add(time.Duration(jwtService.config.RefreshExpiration) * time.Hour)
		if storedToken.ExpiresAt.Before(expectedExpiry.Add(-time.Minute)) || storedToken.ExpiresAt.After(expectedExpiry.Add(time.Minute)) {
			t.Errorf("Expected expiry around %v, got %v", expectedExpiry, storedToken.ExpiresAt)
		}
	})

	t.Run("Nil_User", func(t *testing.T) {
		_, err := jwtService.GenerateRefreshToken(nil)
		if err == nil {
			t.Error("Expected error for nil user")
		}
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("Expected ErrInvalidCredentials, got %v", err)
		}
	})
}

func TestJWTService_VerifyAccessToken(t *testing.T) {
	jwtService, userRepo, _ := createTestJWTService()
	user := createTestUser()
	userRepo.users[user.ID] = user

	t.Run("Valid_Token", func(t *testing.T) {
		token, err := jwtService.GenerateAccessToken(user)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		userID, err := jwtService.VerifyAccessToken(token)
		if err != nil {
			t.Fatalf("Expected successful token verification, got error: %v", err)
		}

		if userID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, userID)
		}
	})

	t.Run("Invalid_Token", func(t *testing.T) {
		_, err := jwtService.VerifyAccessToken("invalid-token")
		if err == nil {
			t.Error("Expected error for invalid token")
		}
	})

	t.Run("Empty_Token", func(t *testing.T) {
		_, err := jwtService.VerifyAccessToken("")
		if err == nil {
			t.Error("Expected error for empty token")
		}
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("Expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("User_Not_Found", func(t *testing.T) {
		token, err := jwtService.GenerateAccessToken(user)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		delete(userRepo.users, user.ID)

		_, err = jwtService.VerifyAccessToken(token)
		if err == nil {
			t.Error("Expected error for non-existent user")
		}
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("Expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("Locked_User", func(t *testing.T) {
		user.Locked = true
		userRepo.users[user.ID] = user

		token, err := jwtService.GenerateAccessToken(user)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		_, err = jwtService.VerifyAccessToken(token)
		if err == nil {
			t.Error("Expected error for locked user")
		}
		if !errors.Is(err, ErrAccountLocked) {
			t.Errorf("Expected ErrAccountLocked, got %v", err)
		}
	})

	t.Run("Session_Version_Mismatch", func(t *testing.T) {
		user.Locked = false
		user.SessionVersion = 2
		userRepo.users[user.ID] = user

		oldUser := *user
		oldUser.SessionVersion = 1
		token, err := jwtService.GenerateAccessToken(&oldUser)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		_, err = jwtService.VerifyAccessToken(token)
		if err == nil {
			t.Error("Expected error for session version mismatch")
		}
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("Expected ErrInvalidToken, got %v", err)
		}
	})
}

func TestJWTService_RefreshAccessToken(t *testing.T) {
	jwtService, userRepo, refreshRepo := createTestJWTService()
	user := createTestUser()
	userRepo.users[user.ID] = user

	t.Run("Successful_Refresh", func(t *testing.T) {

		refreshToken, err := jwtService.GenerateRefreshToken(user)
		if err != nil {
			t.Fatalf("Failed to generate refresh token: %v", err)
		}

		accessToken, err := jwtService.RefreshAccessToken(refreshToken)
		if err != nil {
			t.Fatalf("Expected successful token refresh, got error: %v", err)
		}

		if accessToken == "" {
			t.Error("Expected non-empty access token")
		}

		userID, err := jwtService.VerifyAccessToken(accessToken)
		if err != nil {
			t.Fatalf("Expected valid access token, got error: %v", err)
		}

		if userID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, userID)
		}
	})

	t.Run("Invalid_Refresh_Token", func(t *testing.T) {
		_, err := jwtService.RefreshAccessToken("invalid-refresh-token")
		if err == nil {
			t.Error("Expected error for invalid refresh token")
		}
		if !errors.Is(err, ErrRefreshTokenInvalid) {
			t.Errorf("Expected ErrRefreshTokenInvalid, got %v", err)
		}
	})

	t.Run("Expired_Refresh_Token", func(t *testing.T) {

		refreshToken := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "expired-token-hash",
			ExpiresAt: time.Now().Add(-time.Hour),
		}
		refreshRepo.tokens["expired-token-hash"] = refreshToken

		testToken := "test-expired-token"
		tokenHash := jwtService.hashToken(testToken)
		refreshToken.TokenHash = tokenHash
		refreshRepo.tokens[tokenHash] = refreshToken

		_, err := jwtService.RefreshAccessToken(testToken)
		if err == nil {
			t.Error("Expected error for expired refresh token")
		}
		if !errors.Is(err, ErrRefreshTokenExpired) {
			t.Errorf("Expected ErrRefreshTokenExpired, got %v", err)
		}
	})

	t.Run("User_Not_Found", func(t *testing.T) {

		refreshToken, err := jwtService.GenerateRefreshToken(user)
		if err != nil {
			t.Fatalf("Failed to generate refresh token: %v", err)
		}

		delete(userRepo.users, user.ID)

		_, err = jwtService.RefreshAccessToken(refreshToken)
		if err == nil {
			t.Error("Expected error for non-existent user")
		}
		if !errors.Is(err, ErrRefreshTokenInvalid) {
			t.Errorf("Expected ErrRefreshTokenInvalid, got %v", err)
		}
	})

	t.Run("Locked_User", func(t *testing.T) {
		user.Locked = true
		userRepo.users[user.ID] = user

		refreshToken, err := jwtService.GenerateRefreshToken(user)
		if err != nil {
			t.Fatalf("Failed to generate refresh token: %v", err)
		}

		_, err = jwtService.RefreshAccessToken(refreshToken)
		if err == nil {
			t.Error("Expected error for locked user")
		}
		if !errors.Is(err, ErrAccountLocked) {
			t.Errorf("Expected ErrAccountLocked, got %v", err)
		}
	})
}

func TestJWTService_RevokeRefreshToken(t *testing.T) {
	jwtService, userRepo, refreshRepo := createTestJWTService()
	user := createTestUser()
	userRepo.users[user.ID] = user

	t.Run("Successful_Revocation", func(t *testing.T) {

		refreshToken, err := jwtService.GenerateRefreshToken(user)
		if err != nil {
			t.Fatalf("Failed to generate refresh token: %v", err)
		}

		tokenHash := jwtService.hashToken(refreshToken)
		_, err = refreshRepo.GetByTokenHash(tokenHash)
		if err != nil {
			t.Fatalf("Expected refresh token to exist: %v", err)
		}

		err = jwtService.RevokeRefreshToken(refreshToken)
		if err != nil {
			t.Fatalf("Expected successful token revocation, got error: %v", err)
		}

		_, err = refreshRepo.GetByTokenHash(tokenHash)
		if err == nil {
			t.Error("Expected refresh token to be removed")
		}
	})

	t.Run("Non_Existent_Token", func(t *testing.T) {
		err := jwtService.RevokeRefreshToken("non-existent-token")
		if err != nil {
			t.Errorf("Expected no error for non-existent token, got %v", err)
		}
	})
}

func TestJWTService_RevokeAllRefreshTokens(t *testing.T) {
	jwtService, userRepo, refreshRepo := createTestJWTService()
	user := createTestUser()
	userRepo.users[user.ID] = user

	t.Run("Successful_Revocation", func(t *testing.T) {

		_, err := jwtService.GenerateRefreshToken(user)
		if err != nil {
			t.Fatalf("Failed to generate first refresh token: %v", err)
		}

		_, err = jwtService.GenerateRefreshToken(user)
		if err != nil {
			t.Fatalf("Failed to generate second refresh token: %v", err)
		}

		count, err := refreshRepo.CountByUserID(user.ID)
		if err != nil {
			t.Fatalf("Failed to count tokens: %v", err)
		}
		if count != 2 {
			t.Errorf("Expected 2 tokens, got %d", count)
		}

		err = jwtService.RevokeAllRefreshTokens(user.ID)
		if err != nil {
			t.Fatalf("Expected successful token revocation, got error: %v", err)
		}

		count, err = refreshRepo.CountByUserID(user.ID)
		if err != nil {
			t.Fatalf("Failed to count tokens: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 tokens, got %d", count)
		}
	})
}

func TestJWTService_CleanupExpiredTokens(t *testing.T) {
	jwtService, userRepo, refreshRepo := createTestJWTService()
	user := createTestUser()
	userRepo.users[user.ID] = user

	t.Run("Successful_Cleanup", func(t *testing.T) {

		expiredToken := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "expired-token-hash",
			ExpiresAt: time.Now().Add(-time.Hour),
		}
		refreshRepo.tokens["expired-token-hash"] = expiredToken

		validToken, err := jwtService.GenerateRefreshToken(user)
		if err != nil {
			t.Fatalf("Failed to generate valid refresh token: %v", err)
		}

		if len(refreshRepo.tokens) != 2 {
			t.Errorf("Expected 2 tokens, got %d", len(refreshRepo.tokens))
		}

		err = jwtService.CleanupExpiredTokens()
		if err != nil {
			t.Fatalf("Expected successful cleanup, got error: %v", err)
		}

		if len(refreshRepo.tokens) != 1 {
			t.Errorf("Expected 1 token after cleanup, got %d", len(refreshRepo.tokens))
		}

		tokenHash := jwtService.hashToken(validToken)
		_, exists := refreshRepo.tokens[tokenHash]
		if !exists {
			t.Error("Expected valid token to remain after cleanup")
		}
	})
}

func TestJWTService_KeyRotation(t *testing.T) {
	cfg := &config.JWTConfig{
		Secret:            "old-secret-key-that-is-long-enough-for-security",
		Expiration:        1,
		RefreshExpiration: 24,
		Issuer:            "test-issuer",
		Audience:          "test-audience",
		KeyRotation: config.KeyRotationConfig{
			Enabled:     true,
			CurrentKey:  "current-key-that-is-long-enough-for-security",
			PreviousKey: "previous-key-that-is-long-enough-for-security",
			KeyID:       "current-key-id",
		},
	}

	userRepo := &jwtMockUserRepo{
		users: make(map[uint]*database.User),
	}
	refreshRepo := &jwtMockRefreshTokenRepo{
		tokens: make(map[string]*database.RefreshToken),
	}

	jwtService := NewJWTService(cfg, userRepo, refreshRepo)
	user := createTestUser()
	userRepo.users[user.ID] = user

	t.Run("Generate_Token_With_Key_Rotation", func(t *testing.T) {
		token, err := jwtService.GenerateAccessToken(user)
		if err != nil {
			t.Fatalf("Expected successful token generation with key rotation, got error: %v", err)
		}

		parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
			return []byte(cfg.KeyRotation.CurrentKey), nil
		})
		if err != nil {
			t.Fatalf("Failed to parse token with key rotation: %v", err)
		}

		if !parsedToken.Valid {
			t.Error("Generated token should be valid")
		}

		if kid, ok := parsedToken.Header["kid"].(string); !ok || kid != cfg.KeyRotation.KeyID {
			t.Errorf("Expected key ID %s, got %v", cfg.KeyRotation.KeyID, parsedToken.Header["kid"])
		}
	})

	t.Run("Verify_Token_With_Current_Key", func(t *testing.T) {
		token, err := jwtService.GenerateAccessToken(user)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		userID, err := jwtService.VerifyAccessToken(token)
		if err != nil {
			t.Fatalf("Expected successful token verification with current key, got error: %v", err)
		}

		if userID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, userID)
		}
	})

	t.Run("Legacy_Token_Without_KID_Remains_Valid", func(t *testing.T) {
		legacyCfg := &config.JWTConfig{
			Secret:            "legacy-secret-key-that-is-long-enough-for-security",
			Expiration:        1,
			RefreshExpiration: 24,
			Issuer:            "legacy-issuer",
			Audience:          "legacy-audience",
			KeyRotation:       config.KeyRotationConfig{Enabled: false},
		}

		legacyUserRepo := &jwtMockUserRepo{users: map[uint]*database.User{user.ID: user}}
		legacyRefreshRepo := &jwtMockRefreshTokenRepo{tokens: make(map[string]*database.RefreshToken)}
		legacyService := NewJWTService(legacyCfg, legacyUserRepo, legacyRefreshRepo)

		legacyToken, err := legacyService.GenerateAccessToken(user)
		if err != nil {
			t.Fatalf("Failed to generate legacy token: %v", err)
		}

		legacyCfg.KeyRotation.Enabled = true
		legacyCfg.KeyRotation.CurrentKey = "rotated-current-key-that-is-long-enough-for-security"
		legacyCfg.KeyRotation.PreviousKey = legacyCfg.Secret
		legacyCfg.KeyRotation.KeyID = "rotated-key-id"

		parsedUserID, err := legacyService.VerifyAccessToken(legacyToken)
		if err != nil {
			t.Fatalf("Legacy token should remain valid after enabling rotation: %v", err)
		}
		if parsedUserID != user.ID {
			t.Fatalf("Expected user ID %d, got %d", user.ID, parsedUserID)
		}
	})

	t.Run("Legacy_Token_With_Previous_KID_Remains_Valid", func(t *testing.T) {
		rotCfg := &config.JWTConfig{
			Secret:            "unused-secret-key-that-is-long-enough-for-security",
			Expiration:        1,
			RefreshExpiration: 24,
			Issuer:            "rotation-issuer",
			Audience:          "rotation-audience",
			KeyRotation: config.KeyRotationConfig{
				Enabled:    true,
				CurrentKey: "rotation-key-v1-that-is-long-enough-for-security",
				KeyID:      "key-id-v1",
			},
		}

		rotUserRepo := &jwtMockUserRepo{users: map[uint]*database.User{user.ID: user}}
		rotRefreshRepo := &jwtMockRefreshTokenRepo{tokens: make(map[string]*database.RefreshToken)}
		rotService := NewJWTService(rotCfg, rotUserRepo, rotRefreshRepo)

		tokenV1, err := rotService.GenerateAccessToken(user)
		if err != nil {
			t.Fatalf("Failed to generate v1 token: %v", err)
		}

		rotCfg.KeyRotation.PreviousKey = rotCfg.KeyRotation.CurrentKey
		rotCfg.KeyRotation.CurrentKey = "rotation-key-v2-that-is-long-enough-for-security"
		rotCfg.KeyRotation.KeyID = "key-id-v2"

		parsedUserID, err := rotService.VerifyAccessToken(tokenV1)
		if err != nil {
			t.Fatalf("Token signed with previous key should remain valid: %v", err)
		}
		if parsedUserID != user.ID {
			t.Fatalf("Expected user ID %d, got %d", user.ID, parsedUserID)
		}
	})

	t.Run("Unknown_KID_Is_Rejected", func(t *testing.T) {
		cfg := &config.JWTConfig{
			Secret:            "unused-secret-key-that-is-long-enough-for-security",
			Expiration:        1,
			RefreshExpiration: 24,
			Issuer:            "issuer",
			Audience:          "audience",
			KeyRotation: config.KeyRotationConfig{
				Enabled:    true,
				CurrentKey: "current-key-for-unknown-kid-test-that-is-long-enough",
				KeyID:      "expected-key-id",
			},
		}

		repo := &jwtMockUserRepo{users: map[uint]*database.User{user.ID: user}}
		refreshRepo := &jwtMockRefreshTokenRepo{tokens: make(map[string]*database.RefreshToken)}
		service := NewJWTService(cfg, repo, refreshRepo)

		claims := TokenClaims{
			UserID:         user.ID,
			Username:       user.Username,
			SessionVersion: user.SessionVersion,
			TokenType:      TokenTypeAccess,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    cfg.Issuer,
				Audience:  []string{cfg.Audience},
				Subject:   fmt.Sprint(user.ID),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		token.Header["kid"] = "unexpected-key-id"
		tokenString, err := token.SignedString([]byte(cfg.KeyRotation.CurrentKey))
		if err != nil {
			t.Fatalf("Failed to sign token: %v", err)
		}

		_, err = service.VerifyAccessToken(tokenString)
		if !errors.Is(err, ErrInvalidKeyID) {
			t.Fatalf("Expected ErrInvalidKeyID, got %v", err)
		}
	})
}

func TestJWTService_ErrorHandling(t *testing.T) {
	jwtService, _, _ := createTestJWTService()

	t.Run("Invalid_Issuer", func(t *testing.T) {

		claims := TokenClaims{
			UserID:         1,
			Username:       "testuser",
			SessionVersion: 1,
			TokenType:      TokenTypeAccess,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    "wrong-issuer",
				Audience:  []string{jwtService.config.Audience},
				Subject:   "1",
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(jwtService.config.Secret))
		if err != nil {
			t.Fatalf("Failed to create test token: %v", err)
		}

		_, err = jwtService.VerifyAccessToken(tokenString)
		if err == nil {
			t.Error("Expected error for invalid issuer")
		}
		if !errors.Is(err, ErrInvalidIssuer) {
			t.Errorf("Expected ErrInvalidIssuer, got %v", err)
		}
	})

	t.Run("Invalid_Audience", func(t *testing.T) {

		claims := TokenClaims{
			UserID:         1,
			Username:       "testuser",
			SessionVersion: 1,
			TokenType:      TokenTypeAccess,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    jwtService.config.Issuer,
				Audience:  []string{"wrong-audience"},
				Subject:   "1",
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(jwtService.config.Secret))
		if err != nil {
			t.Fatalf("Failed to create test token: %v", err)
		}

		_, err = jwtService.VerifyAccessToken(tokenString)
		if err == nil {
			t.Error("Expected error for invalid audience")
		}
		if !errors.Is(err, ErrInvalidAudience) {
			t.Errorf("Expected ErrInvalidAudience, got %v", err)
		}
	})

	t.Run("Expired_Token", func(t *testing.T) {

		claims := TokenClaims{
			UserID:         1,
			Username:       "testuser",
			SessionVersion: 1,
			TokenType:      TokenTypeAccess,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    jwtService.config.Issuer,
				Audience:  []string{jwtService.config.Audience},
				Subject:   "1",
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(jwtService.config.Secret))
		if err != nil {
			t.Fatalf("Failed to create test token: %v", err)
		}

		_, err = jwtService.VerifyAccessToken(tokenString)
		if err == nil {
			t.Error("Expected error for expired token")
		}
		if !errors.Is(err, ErrTokenExpired) {
			t.Errorf("Expected ErrTokenExpired, got %v", err)
		}
	})

	t.Run("Subject_Mismatch", func(t *testing.T) {
		claims := TokenClaims{
			UserID:         1,
			Username:       "testuser",
			SessionVersion: 1,
			TokenType:      TokenTypeAccess,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    jwtService.config.Issuer,
				Audience:  []string{jwtService.config.Audience},
				Subject:   "999",
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(jwtService.config.Secret))
		if err != nil {
			t.Fatalf("Failed to create test token: %v", err)
		}

		_, err = jwtService.VerifyAccessToken(tokenString)
		if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("Expected ErrInvalidToken for subject mismatch, got %v", err)
		}
	})
}

func TestJWTService_HelperFunctions(t *testing.T) {
	jwtService, _, _ := createTestJWTService()

	t.Run("HashToken", func(t *testing.T) {
		token := "test-token"
		hash1 := jwtService.hashToken(token)
		hash2 := jwtService.hashToken(token)

		if hash1 != hash2 {
			t.Error("Hash should be deterministic")
		}

		if hash1 == token {
			t.Error("Hash should be different from original token")
		}

		hash3 := jwtService.hashToken("different-token")
		if hash1 == hash3 {
			t.Error("Different tokens should produce different hashes")
		}
	})

	t.Run("Contains", func(t *testing.T) {
		slice := []string{"item1", "item2", "item3"}

		if !slices.Contains(slice, "item1") {
			t.Error("Should contain item1")
		}

		if !slices.Contains(slice, "item2") {
			t.Error("Should contain item2")
		}

		if slices.Contains(slice, "item4") {
			t.Error("Should not contain item4")
		}

		if slices.Contains(slice, "") {
			t.Error("Should not contain empty string")
		}
	})
}

func TestJWTService_Integration(t *testing.T) {
	jwtService, userRepo, _ := createTestJWTService()
	user := createTestUser()
	userRepo.users[user.ID] = user

	t.Run("Complete_Flow", func(t *testing.T) {

		accessToken, err := jwtService.GenerateAccessToken(user)
		if err != nil {
			t.Fatalf("Failed to generate access token: %v", err)
		}

		refreshToken, err := jwtService.GenerateRefreshToken(user)
		if err != nil {
			t.Fatalf("Failed to generate refresh token: %v", err)
		}

		userID, err := jwtService.VerifyAccessToken(accessToken)
		if err != nil {
			t.Fatalf("Failed to verify access token: %v", err)
		}
		if userID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, userID)
		}

		newAccessToken, err := jwtService.RefreshAccessToken(refreshToken)
		if err != nil {
			t.Fatalf("Failed to refresh access token: %v", err)
		}

		userID, err = jwtService.VerifyAccessToken(newAccessToken)
		if err != nil {
			t.Fatalf("Failed to verify new access token: %v", err)
		}
		if userID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, userID)
		}

		err = jwtService.RevokeRefreshToken(refreshToken)
		if err != nil {
			t.Fatalf("Failed to revoke refresh token: %v", err)
		}

		_, err = jwtService.RefreshAccessToken(refreshToken)
		if err == nil {
			t.Error("Expected error when using revoked refresh token")
		}
		if !errors.Is(err, ErrRefreshTokenInvalid) {
			t.Errorf("Expected ErrRefreshTokenInvalid, got %v", err)
		}
	})
}
