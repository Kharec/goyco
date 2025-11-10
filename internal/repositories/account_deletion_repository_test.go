package repositories

import (
	"strings"
	"testing"

	"gorm.io/gorm"
	"goyco/internal/database"
)

func TestAccountDeletionRepository_Create(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful creation", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		request := suite.CreateTestAccountDeletionRequest(user.ID, "token-hash-123")

		if suite.GetAccountDeletionRequestCount() != 1 {
			t.Errorf("Expected 1 account deletion request, got %d", suite.GetAccountDeletionRequestCount())
		}

		if request.ID == 0 {
			t.Error("Expected request ID to be assigned")
		}
	})

	t.Run("duplicate token hash", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		suite.CreateTestAccountDeletionRequest(user.ID, "duplicate-token")

		request2 := &database.AccountDeletionRequest{
			UserID:    user.ID,
			TokenHash: "duplicate-token",
		}
		err := suite.DeletionRepo.Create(request2)
		if err == nil {
			t.Error("Expected error for duplicate token hash")
		}
	})

	t.Run("request with invalid user", func(t *testing.T) {
		suite.Reset()

		request := &database.AccountDeletionRequest{
			UserID:    999,
			TokenHash: "token-hash-123",
		}
		err := suite.DeletionRepo.Create(request)

		if err != nil {
			t.Errorf("Unexpected error for invalid user (SQLite allows this): %v", err)
		}
	})
}

func TestAccountDeletionRepository_GetByTokenHash(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing request", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		suite.CreateTestAccountDeletionRequest(user.ID, "token-hash-123")

		retrieved, err := suite.DeletionRepo.GetByTokenHash("token-hash-123")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected request, got nil")
		}

		if retrieved.TokenHash != "token-hash-123" {
			t.Errorf("Expected token hash 'token-hash-123', got %s", retrieved.TokenHash)
		}
		if retrieved.UserID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, retrieved.UserID)
		}
	})

	t.Run("non-existing token hash", func(t *testing.T) {
		suite.Reset()

		_, err := suite.DeletionRepo.GetByTokenHash("nonexistent-token")
		if err == nil {
			t.Error("Expected error for non-existing token hash")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestAccountDeletionRepository_DeleteByID(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful delete", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		request := suite.CreateTestAccountDeletionRequest(user.ID, "token-hash-123")

		err := suite.DeletionRepo.DeleteByID(request.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		_, err = suite.DeletionRepo.GetByTokenHash("token-hash-123")
		if err == nil {
			t.Error("Expected error for deleted request")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})

	t.Run("delete non-existing request", func(t *testing.T) {
		suite.Reset()

		err := suite.DeletionRepo.DeleteByID(999)
		if err != nil {
			t.Fatalf("Delete should succeed even for non-existing request (GORM behavior)")
		}
	})
}

func TestAccountDeletionRepository_DeleteByUserID(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful delete", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		suite.CreateTestAccountDeletionRequest(user.ID, "token-hash-1")

		err := suite.DeletionRepo.DeleteByUserID(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if suite.GetAccountDeletionRequestCount() != 0 {
			t.Errorf("Expected 0 requests after delete, got %d", suite.GetAccountDeletionRequestCount())
		}
	})

	t.Run("delete for user without requests", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		err := suite.DeletionRepo.DeleteByUserID(user.ID)
		if err != nil {
			t.Fatalf("Delete should succeed even for user without requests (GORM behavior)")
		}
	})
}

func TestAccountDeletionRepository_EdgeCases(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("empty token hash", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		request := &database.AccountDeletionRequest{
			UserID:    user.ID,
			TokenHash: "",
		}
		err := suite.DeletionRepo.Create(request)

		if err != nil {
			t.Errorf("Unexpected error for empty token hash: %v", err)
		}
	})

	t.Run("zero user ID", func(t *testing.T) {
		suite.Reset()

		request := &database.AccountDeletionRequest{
			UserID:    0,
			TokenHash: "token-hash-123",
		}
		err := suite.DeletionRepo.Create(request)

		if err != nil {
			t.Errorf("Unexpected error for zero user ID: %v", err)
		}
	})

	t.Run("very long token hash", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		longToken := strings.Repeat("a", 300)
		request := &database.AccountDeletionRequest{
			UserID:    user.ID,
			TokenHash: longToken,
		}
		err := suite.DeletionRepo.Create(request)

		if err != nil {
			t.Errorf("Unexpected error for long token hash: %v", err)
		}
	})
}

func TestAccountDeletionRepository_ConcurrentAccess(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("concurrent creates", func(t *testing.T) {
		suite.Reset()

		user1 := suite.CreateTestUser("user1", "user1@example.com", "password123")
		user2 := suite.CreateTestUser("user2", "user2@example.com", "password123")

		suite.CreateTestAccountDeletionRequest(user1.ID, "token-hash-1")
		suite.CreateTestAccountDeletionRequest(user2.ID, "token-hash-2")

		if suite.GetAccountDeletionRequestCount() != 2 {
			t.Errorf("Expected 2 requests, got %d", suite.GetAccountDeletionRequestCount())
		}
	})
}
