package repositories

import (
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"goyco/internal/database"
)

func TestRefreshTokenRepository_Create(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful creation", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		token := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "token-hash-123",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		err := suite.RefreshTokenRepo.Create(token)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if token.ID == 0 {
			t.Error("Expected token ID to be assigned")
		}
	})

	t.Run("token with invalid user", func(t *testing.T) {
		suite.Reset()

		token := &database.RefreshToken{
			UserID:    999,
			TokenHash: "token-hash-123",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		err := suite.RefreshTokenRepo.Create(token)

		if err != nil {
			t.Errorf("Unexpected error for invalid user (SQLite allows this): %v", err)
		}
	})

	t.Run("duplicate token hash", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		token1 := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "duplicate-token",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		err := suite.RefreshTokenRepo.Create(token1)
		if err != nil {
			t.Fatalf("Expected no error for first token, got %v", err)
		}

		token2 := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "duplicate-token",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		err = suite.RefreshTokenRepo.Create(token2)
		if err == nil {
			t.Error("Expected error for duplicate token hash")
		}
	})
}

func TestRefreshTokenRepository_GetByTokenHash(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing token", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		token := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "token-hash-123",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		err := suite.RefreshTokenRepo.Create(token)
		if err != nil {
			t.Fatalf("Failed to create token: %v", err)
		}

		retrieved, err := suite.RefreshTokenRepo.GetByTokenHash("token-hash-123")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected token, got nil")
		}

		if retrieved.TokenHash != "token-hash-123" {
			t.Errorf("Expected token hash 'token-hash-123', got %s", retrieved.TokenHash)
		}
		if retrieved.UserID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, retrieved.UserID)
		}
	})

	t.Run("non-existing token", func(t *testing.T) {
		suite.Reset()

		_, err := suite.RefreshTokenRepo.GetByTokenHash("nonexistent-token")
		if err == nil {
			t.Error("Expected error for non-existing token")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestRefreshTokenRepository_DeleteByUserID(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful delete", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		token1 := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "token-hash-1",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		token2 := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "token-hash-2",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		suite.RefreshTokenRepo.Create(token1)
		suite.RefreshTokenRepo.Create(token2)

		err := suite.RefreshTokenRepo.DeleteByUserID(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		_, err = suite.RefreshTokenRepo.GetByTokenHash("token-hash-1")
		if err == nil {
			t.Error("Expected error for deleted token")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})

	t.Run("delete for user without tokens", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		err := suite.RefreshTokenRepo.DeleteByUserID(user.ID)
		if err != nil {
			t.Fatalf("Delete should succeed even for user without tokens (GORM behavior)")
		}
	})
}

func TestRefreshTokenRepository_DeleteExpired(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("delete expired tokens", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		expiredToken := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "expired-token",
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		}

		validToken := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "valid-token",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		suite.RefreshTokenRepo.Create(expiredToken)
		suite.RefreshTokenRepo.Create(validToken)

		err := suite.RefreshTokenRepo.DeleteExpired()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		_, err = suite.RefreshTokenRepo.GetByTokenHash("expired-token")
		if err == nil {
			t.Error("Expected error for deleted expired token")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}

		_, err = suite.RefreshTokenRepo.GetByTokenHash("valid-token")
		if err != nil {
			t.Errorf("Expected valid token to still exist, got error: %v", err)
		}
	})
}

func TestRefreshTokenRepository_DeleteByID(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful delete", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		token := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "token-hash-123",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		err := suite.RefreshTokenRepo.Create(token)
		if err != nil {
			t.Fatalf("Failed to create token: %v", err)
		}

		err = suite.RefreshTokenRepo.DeleteByID(token.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		_, err = suite.RefreshTokenRepo.GetByTokenHash("token-hash-123")
		if err == nil {
			t.Error("Expected error for deleted token")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})

	t.Run("delete non-existing token", func(t *testing.T) {
		suite.Reset()

		err := suite.RefreshTokenRepo.DeleteByID(999)
		if err != nil {
			t.Fatalf("Delete should succeed even for non-existing token (GORM behavior)")
		}
	})
}

func TestRefreshTokenRepository_GetByUserID(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("user with tokens", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		token1 := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "token-hash-1",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		token2 := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "token-hash-2",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		suite.RefreshTokenRepo.Create(token1)
		suite.RefreshTokenRepo.Create(token2)

		tokens, err := suite.RefreshTokenRepo.GetByUserID(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(tokens) != 2 {
			t.Errorf("Expected 2 tokens, got %d", len(tokens))
		}

		for _, token := range tokens {
			if token.UserID != user.ID {
				t.Errorf("Expected user ID %d, got %d", user.ID, token.UserID)
			}
		}
	})

	t.Run("user without tokens", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		tokens, err := suite.RefreshTokenRepo.GetByUserID(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(tokens) != 0 {
			t.Errorf("Expected 0 tokens, got %d", len(tokens))
		}
	})
}

func TestRefreshTokenRepository_CountByUserID(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("user with tokens", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		token1 := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "token-hash-1",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		token2 := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "token-hash-2",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		suite.RefreshTokenRepo.Create(token1)
		suite.RefreshTokenRepo.Create(token2)

		count, err := suite.RefreshTokenRepo.CountByUserID(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if count != 2 {
			t.Errorf("Expected 2 tokens, got %d", count)
		}
	})

	t.Run("user without tokens", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		count, err := suite.RefreshTokenRepo.CountByUserID(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if count != 0 {
			t.Errorf("Expected 0 tokens, got %d", count)
		}
	})
}

func TestRefreshTokenRepository_EdgeCases(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("empty token hash", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		token := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: "",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		err := suite.RefreshTokenRepo.Create(token)

		if err != nil {
			t.Errorf("Unexpected error for empty token hash: %v", err)
		}
	})

	t.Run("zero user ID", func(t *testing.T) {
		suite.Reset()

		token := &database.RefreshToken{
			UserID:    0,
			TokenHash: "token-hash-123",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		err := suite.RefreshTokenRepo.Create(token)

		if err != nil {
			t.Errorf("Unexpected error for zero user ID: %v", err)
		}
	})

	t.Run("very long token hash", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		longToken := strings.Repeat("a", 300)
		token := &database.RefreshToken{
			UserID:    user.ID,
			TokenHash: longToken,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		err := suite.RefreshTokenRepo.Create(token)

		if err != nil {
			t.Errorf("Unexpected error for long token hash: %v", err)
		}
	})
}
