package repositories

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"goyco/internal/database"
)

func TestUserRepository_Create(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful creation", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:      "testuser",
			Email:         "test@example.com",
			Password:      "password123",
			EmailVerified: true,
		}

		err := suite.UserRepo.Create(user)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if suite.GetUserCount() != 1 {
			t.Errorf("Expected 1 user, got %d", suite.GetUserCount())
		}

		if user.ID == 0 {
			t.Error("Expected user ID to be assigned")
		}
	})

	t.Run("duplicate username", func(t *testing.T) {
		suite.Reset()

		user1 := &database.User{
			Username:      "duplicate",
			Email:         "user1@example.com",
			Password:      "password123",
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user1)
		if err != nil {
			t.Fatalf("Expected no error for first user, got %v", err)
		}

		user2 := &database.User{
			Username:      "duplicate",
			Email:         "user2@example.com",
			Password:      "password123",
			EmailVerified: true,
		}
		err = suite.UserRepo.Create(user2)
		if err == nil {
			t.Error("Expected error for duplicate username")
		}
		if !errors.Is(err, gorm.ErrDuplicatedKey) && !strings.Contains(err.Error(), "duplicate") && !strings.Contains(err.Error(), "UNIQUE constraint") {
			t.Errorf("Expected duplicate key error, got %v", err)
		}
	})

	t.Run("duplicate email", func(t *testing.T) {
		suite.Reset()

		user1 := &database.User{
			Username:      "user1",
			Email:         "duplicate@example.com",
			Password:      "password123",
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user1)
		if err != nil {
			t.Fatalf("Expected no error for first user, got %v", err)
		}

		user2 := &database.User{
			Username:      "user2",
			Email:         "duplicate@example.com",
			Password:      "password123",
			EmailVerified: true,
		}
		err = suite.UserRepo.Create(user2)
		if err == nil {
			t.Error("Expected error for duplicate email")
		}
		if !errors.Is(err, gorm.ErrDuplicatedKey) && !strings.Contains(err.Error(), "duplicate") && !strings.Contains(err.Error(), "UNIQUE constraint") {
			t.Errorf("Expected duplicate key error, got %v", err)
		}
	})
}

func TestUserRepository_GetByID(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing user", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		retrieved, err := suite.UserRepo.GetByID(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected user, got nil")
		}

		if retrieved.ID != user.ID {
			t.Errorf("Expected ID %d, got %d", user.ID, retrieved.ID)
		}
		if retrieved.Username != user.Username {
			t.Errorf("Expected username %s, got %s", user.Username, retrieved.Username)
		}
		if retrieved.Email != user.Email {
			t.Errorf("Expected email %s, got %s", user.Email, retrieved.Email)
		}
	})

	t.Run("non-existing user", func(t *testing.T) {
		suite.Reset()

		_, err := suite.UserRepo.GetByID(999)
		if err == nil {
			t.Error("Expected error for non-existing user")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestUserRepository_GetByUsername(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing user", func(t *testing.T) {
		suite.Reset()

		suite.CreateTestUser("testuser", "test@example.com", "password123")

		retrieved, err := suite.UserRepo.GetByUsername("testuser")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected user, got nil")
		}

		if retrieved.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got %s", retrieved.Username)
		}
	})

	t.Run("non-existing user", func(t *testing.T) {
		suite.Reset()

		_, err := suite.UserRepo.GetByUsername("nonexistent")
		if err == nil {
			t.Error("Expected error for non-existing username")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestUserRepository_GetByUsernameIncludingDeleted(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing user", func(t *testing.T) {
		suite.Reset()

		suite.CreateTestUser("testuser", "test@example.com", "password123")

		retrieved, err := suite.UserRepo.GetByUsernameIncludingDeleted("testuser")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected user, got nil")
		}

		if retrieved.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got %s", retrieved.Username)
		}
	})

	t.Run("deleted user", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("deleteduser", "deleted@example.com", "password123")

		err := suite.UserRepo.Delete(user.ID)
		if err != nil {
			t.Fatalf("Failed to delete user: %v", err)
		}

		retrieved, err := suite.UserRepo.GetByUsernameIncludingDeleted("deleteduser")
		if err != nil {
			t.Fatalf("Expected no error for deleted user, got %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected deleted user, got nil")
		}

		if retrieved.Username != "deleteduser" {
			t.Errorf("Expected username 'deleteduser', got %s", retrieved.Username)
		}
	})
}

func TestUserRepository_GetByEmail(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing user", func(t *testing.T) {
		suite.Reset()

		suite.CreateTestUser("testuser", "test@example.com", "password123")

		retrieved, err := suite.UserRepo.GetByEmail("test@example.com")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected user, got nil")
		}

		if retrieved.Email != "test@example.com" {
			t.Errorf("Expected email 'test@example.com', got %s", retrieved.Email)
		}
	})

	t.Run("non-existing email", func(t *testing.T) {
		suite.Reset()

		_, err := suite.UserRepo.GetByEmail("nonexistent@example.com")
		if err == nil {
			t.Error("Expected error for non-existing email")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})

	t.Run("email normalization to lowercase", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:      "testuser",
			Email:         "Test@Example.COM",
			Password:      "password123",
			EmailVerified: true,
		}

		err := suite.UserRepo.Create(user)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if user.Email != "test@example.com" {
			t.Errorf("Expected email to be normalized to lowercase, got %s", user.Email)
		}

		retrieved, err := suite.UserRepo.GetByEmail("test@example.com")
		if err != nil {
			t.Fatalf("Expected to find user with normalized email, got error: %v", err)
		}

		if retrieved.Email != "test@example.com" {
			t.Errorf("Expected normalized email, got %s", retrieved.Email)
		}
	})
}

func TestUserRepository_GetByVerificationToken(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing user with token", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:      "testuser",
			Email:         "test@example.com",
			Password:      "password123",
			EmailVerified: false,
		}
		user.EmailVerificationToken = "verification-token-123"
		err := suite.UserRepo.Create(user)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		retrieved, err := suite.UserRepo.GetByVerificationToken("verification-token-123")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected user, got nil")
		}

		if retrieved.EmailVerificationToken != "verification-token-123" {
			t.Errorf("Expected token 'verification-token-123', got %s", retrieved.EmailVerificationToken)
		}
	})

	t.Run("non-existing token", func(t *testing.T) {
		suite.Reset()

		_, err := suite.UserRepo.GetByVerificationToken("nonexistent-token")
		if err == nil {
			t.Error("Expected error for non-existing token")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestUserRepository_GetByPasswordResetToken(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing user with token", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:           "testuser",
			Email:              "test@example.com",
			Password:           "password123",
			EmailVerified:      true,
			PasswordResetToken: "reset-token-123",
		}
		err := suite.UserRepo.Create(user)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		retrieved, err := suite.UserRepo.GetByPasswordResetToken("reset-token-123")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected user, got nil")
		}

		if retrieved.PasswordResetToken != "reset-token-123" {
			t.Errorf("Expected token 'reset-token-123', got %s", retrieved.PasswordResetToken)
		}
	})

	t.Run("non-existing token", func(t *testing.T) {
		suite.Reset()

		_, err := suite.UserRepo.GetByPasswordResetToken("nonexistent-token")
		if err == nil {
			t.Error("Expected error for non-existing token")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestUserRepository_GetAll(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("empty database", func(t *testing.T) {
		suite.Reset()

		users, err := suite.UserRepo.GetAll(10, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(users) != 0 {
			t.Errorf("Expected 0 users, got %d", len(users))
		}
	})

	t.Run("with users", func(t *testing.T) {
		suite.Reset()

		suite.CreateTestUser("user1", "user1@example.com", "password123")
		suite.CreateTestUser("user2", "user2@example.com", "password123")
		suite.CreateTestUser("user3", "user3@example.com", "password123")

		retrieved, err := suite.UserRepo.GetAll(10, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 3 {
			t.Errorf("Expected 3 users, got %d", len(retrieved))
		}
	})

	t.Run("with limit", func(t *testing.T) {
		suite.Reset()

		suite.CreateTestUser("user4", "user4@example.com", "password123")
		suite.CreateTestUser("user5", "user5@example.com", "password123")
		suite.CreateTestUser("user6", "user6@example.com", "password123")

		retrieved, err := suite.UserRepo.GetAll(2, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 users, got %d", len(retrieved))
		}
	})

	t.Run("with offset", func(t *testing.T) {
		suite.Reset()

		suite.CreateTestUser("user7", "user7@example.com", "password123")
		suite.CreateTestUser("user8", "user8@example.com", "password123")
		suite.CreateTestUser("user9", "user9@example.com", "password123")

		retrieved, err := suite.UserRepo.GetAll(2, 1)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 users, got %d", len(retrieved))
		}
	})
}

func TestUserRepository_Update(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful update", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		user.Username = "updateduser"
		user.Email = "updated@example.com"
		err := suite.UserRepo.Update(user)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		retrieved, err := suite.UserRepo.GetByID(user.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve user: %v", err)
		}

		if retrieved.Username != "updateduser" {
			t.Errorf("Expected username 'updateduser', got %s", retrieved.Username)
		}
		if retrieved.Email != "updated@example.com" {
			t.Errorf("Expected email 'updated@example.com', got %s", retrieved.Email)
		}
	})

	t.Run("update non-existing user", func(t *testing.T) {
		suite.Reset()

		initialCount, err := suite.UserRepo.Count()
		if err != nil {
			t.Fatalf("Failed to get initial count: %v", err)
		}

		user := &database.User{
			ID:            999,
			Username:      "nonexistent",
			Email:         "nonexistent@example.com",
			Password:      "password123",
			EmailVerified: true,
		}
		err = suite.UserRepo.Update(user)
		if err != nil {
			t.Fatalf("Update should succeed even for non-existing user (GORM behavior)")
		}

		finalCount, err := suite.UserRepo.Count()
		if err != nil {
			t.Fatalf("Failed to get final count: %v", err)
		}
		if finalCount != initialCount {
			t.Errorf("Expected count to remain %d, got %d (update should not create new record)", initialCount, finalCount)
		}

		_, err = suite.UserRepo.GetByID(999)
		if err == nil {
			t.Error("Expected error for non-existing user after update")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestUserRepository_Delete(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful soft delete", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		err := suite.UserRepo.Delete(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		_, err = suite.UserRepo.GetByID(user.ID)
		if err == nil {
			t.Error("Expected error for soft deleted user")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}

		retrieved, err := suite.UserRepo.GetByUsernameIncludingDeleted("testuser")
		if err != nil {
			t.Fatalf("Expected no error for deleted user with including deleted method, got %v", err)
		}
		if retrieved == nil {
			t.Fatal("Expected deleted user to be found with including deleted method")
		}
	})

	t.Run("delete non-existing user", func(t *testing.T) {
		suite.Reset()

		err := suite.UserRepo.Delete(999)
		if err != nil {
			t.Fatalf("Delete should succeed even for non-existing user (GORM behavior)")
		}
	})
}

func TestUserRepository_HardDelete(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful hard delete", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		err := suite.UserRepo.HardDelete(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		_, err = suite.UserRepo.GetByID(user.ID)
		if err == nil {
			t.Error("Expected error for hard deleted user")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}

		_, err = suite.UserRepo.GetByUsernameIncludingDeleted("testuser")
		if err == nil {
			t.Error("Expected error for hard deleted user with including deleted method")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestUserRepository_GetPosts(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("user with posts", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		suite.CreateTestPost(user.ID, "Post 1", "https://example.com/1", "Content 1")
		suite.CreateTestPost(user.ID, "Post 2", "https://example.com/2", "Content 2")

		retrieved, err := suite.UserRepo.GetPosts(user.ID, 10, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 posts, got %d", len(retrieved))
		}
	})

	t.Run("user without posts", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("noposts", "noposts@example.com", "password123")

		retrieved, err := suite.UserRepo.GetPosts(user.ID, 10, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 0 {
			t.Errorf("Expected 0 posts, got %d", len(retrieved))
		}
	})

	t.Run("with limit and offset", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("pagination", "pagination@example.com", "password123")

		for i := 0; i < 5; i++ {
			suite.CreateTestPost(user.ID,
				"Post "+strconv.Itoa(i),
				"https://example.com/"+strconv.Itoa(i),
				"Content "+strconv.Itoa(i))
		}

		retrieved, err := suite.UserRepo.GetPosts(user.ID, 2, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 posts, got %d", len(retrieved))
		}

		retrieved, err = suite.UserRepo.GetPosts(user.ID, 2, 1)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 posts with offset, got %d", len(retrieved))
		}
	})
}

func TestUserRepository_EdgeCases(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("empty username", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:      "",
			Email:         "test@example.com",
			Password:      "password123",
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user)
		if err == nil {
			t.Error("Expected validation error for empty username")
		}
	})

	t.Run("empty email", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:      "testuser",
			Email:         "",
			Password:      "password123",
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user)
		if err == nil {
			t.Error("Expected validation error for empty email")
		}
	})

	t.Run("invalid email format", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:      "testuser",
			Email:         "invalid-email",
			Password:      "password123",
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user)
		if err == nil {
			t.Error("Expected validation error for invalid email format")
		}
	})

	t.Run("very long username", func(t *testing.T) {
		suite.Reset()

		longUsername := strings.Repeat("a", 300)
		user := &database.User{
			Username:      longUsername,
			Email:         "test@example.com",
			Password:      "password123",
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user)

		if err != nil {
			t.Errorf("Unexpected error for long username: %v", err)
		}
	})

	t.Run("nil pointer handling", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		if post.AuthorID == nil {
			t.Error("Expected AuthorID to be set")
		}
	})

	t.Run("maximum length email", func(t *testing.T) {
		suite.Reset()

		longEmail := strings.Repeat("a", 200) + "@example.com"
		if len(longEmail) > 255 {
			longEmail = strings.Repeat("a", 200) + "@ex.com"
		}
		user := &database.User{
			Username:      "testuser",
			Email:         longEmail,
			Password:      "password123",
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user)
		if err != nil {
			t.Errorf("Unexpected error for long email: %v", err)
		}
	})
}

func TestUserRepository_GetByIDIncludingDeleted(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing user", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		retrieved, err := suite.UserRepo.GetByIDIncludingDeleted(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected user, got nil")
		}

		if retrieved.ID != user.ID {
			t.Errorf("Expected ID %d, got %d", user.ID, retrieved.ID)
		}
	})

	t.Run("deleted user", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		err := suite.UserRepo.Delete(user.ID)
		if err != nil {
			t.Fatalf("Failed to delete user: %v", err)
		}

		retrieved, err := suite.UserRepo.GetByIDIncludingDeleted(user.ID)
		if err != nil {
			t.Fatalf("Expected no error for deleted user, got %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected deleted user, got nil")
		}

		if retrieved.ID != user.ID {
			t.Errorf("Expected ID %d, got %d", user.ID, retrieved.ID)
		}
	})

	t.Run("non-existing user", func(t *testing.T) {
		suite.Reset()

		_, err := suite.UserRepo.GetByIDIncludingDeleted(999)
		if err == nil {
			t.Error("Expected error for non-existing user")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestUserRepository_SoftDeleteWithPosts(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful soft delete with posts", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		user2 := suite.CreateTestUser("voter", "voter@example.com", "password123")
		suite.CreateTestVote(user2.ID, post.ID, database.VoteUp)

		err := suite.UserRepo.SoftDeleteWithPosts(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		_, err = suite.UserRepo.GetByID(user.ID)
		if err == nil {
			t.Error("Expected error for soft deleted user")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}

		retrievedPost, err := suite.PostRepo.GetByID(post.ID)
		if err != nil {
			t.Fatalf("Expected to find post, got %v", err)
		}

		if retrievedPost.AuthorID != nil {
			t.Error("Expected post author to be null after user deletion")
		}
	})

	t.Run("soft delete user without posts", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		err := suite.UserRepo.SoftDeleteWithPosts(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		_, err = suite.UserRepo.GetByID(user.ID)
		if err == nil {
			t.Error("Expected error for soft deleted user")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestUserRepository_Lock(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful lock", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		err := suite.UserRepo.Lock(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		retrieved, err := suite.UserRepo.GetByID(user.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve user: %v", err)
		}

		if !retrieved.Locked {
			t.Error("Expected user to be locked")
		}
	})

	t.Run("lock non-existing user", func(t *testing.T) {
		suite.Reset()

		err := suite.UserRepo.Lock(999)
		if err != nil {
			t.Fatalf("Lock should succeed even for non-existing user (GORM behavior)")
		}
	})
}

func TestUserRepository_Unlock(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful unlock", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		err := suite.UserRepo.Lock(user.ID)
		if err != nil {
			t.Fatalf("Failed to lock user: %v", err)
		}

		err = suite.UserRepo.Unlock(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		retrieved, err := suite.UserRepo.GetByID(user.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve user: %v", err)
		}

		if retrieved.Locked {
			t.Error("Expected user to be unlocked")
		}
	})

	t.Run("unlock non-existing user", func(t *testing.T) {
		suite.Reset()

		err := suite.UserRepo.Unlock(999)
		if err != nil {
			t.Fatalf("Unlock should succeed even for non-existing user (GORM behavior)")
		}
	})
}

func TestUserRepository_GetDeletedUsers(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("with deleted users", func(t *testing.T) {
		suite.Reset()

		user1 := suite.CreateTestUser("user1", "user1@example.com", "password123")
		user2 := suite.CreateTestUser("user2", "user2@example.com", "password123")
		user3 := suite.CreateTestUser("user3", "user3@example.com", "password123")

		suite.UserRepo.Delete(user1.ID)
		suite.UserRepo.Delete(user2.ID)

		deletedUsers, err := suite.UserRepo.GetDeletedUsers()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(deletedUsers) != 2 {
			t.Errorf("Expected 2 deleted users, got %d", len(deletedUsers))
		}

		deletedIDs := make(map[uint]bool)
		for _, user := range deletedUsers {
			deletedIDs[user.ID] = true
		}

		if !deletedIDs[user1.ID] {
			t.Error("Expected user1 to be in deleted users list")
		}
		if !deletedIDs[user2.ID] {
			t.Error("Expected user2 to be in deleted users list")
		}
		if deletedIDs[user3.ID] {
			t.Error("Expected user3 to not be in deleted users list")
		}
	})

	t.Run("no deleted users", func(t *testing.T) {
		suite.Reset()

		suite.CreateTestUser("user1", "user1@example.com", "password123")

		deletedUsers, err := suite.UserRepo.GetDeletedUsers()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(deletedUsers) != 0 {
			t.Errorf("Expected 0 deleted users, got %d", len(deletedUsers))
		}
	})
}

func TestUserRepository_HardDeleteAll(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("hard delete all users", func(t *testing.T) {
		suite.Reset()

		user1 := suite.CreateTestUser("user1", "user1@example.com", "password123")
		user2 := suite.CreateTestUser("user2", "user2@example.com", "password123")

		post := suite.CreateTestPost(user1.ID, "Test Post", "https://example.com", "Test content")
		suite.CreateTestVote(user2.ID, post.ID, database.VoteUp)

		deletedCount, err := suite.UserRepo.HardDeleteAll()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if deletedCount < 2 {
			t.Errorf("Expected at least 2 records deleted, got %d", deletedCount)
		}

		users, err := suite.UserRepo.GetAll(10, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(users) != 0 {
			t.Errorf("Expected 0 users, got %d", len(users))
		}
	})

	t.Run("hard delete with no users", func(t *testing.T) {
		suite.Reset()

		deletedCount, err := suite.UserRepo.HardDeleteAll()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if deletedCount != 0 {
			t.Errorf("Expected 0 records deleted, got %d", deletedCount)
		}
	})
}

func TestUserRepository_Count(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("empty database", func(t *testing.T) {
		suite.Reset()

		count, err := suite.UserRepo.Count()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if count != 0 {
			t.Errorf("Expected count 0, got %d", count)
		}
	})

	t.Run("with users", func(t *testing.T) {
		suite.Reset()

		suite.CreateTestUser("user1", "user1@example.com", "password123")
		suite.CreateTestUser("user2", "user2@example.com", "password123")
		suite.CreateTestUser("user3", "user3@example.com", "password123")

		count, err := suite.UserRepo.Count()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if count != 3 {
			t.Errorf("Expected count 3, got %d", count)
		}
	})

	t.Run("with deleted users", func(t *testing.T) {
		suite.Reset()

		user1 := suite.CreateTestUser("user1", "user1@example.com", "password123")
		suite.CreateTestUser("user2", "user2@example.com", "password123")

		suite.UserRepo.Delete(user1.ID)

		count, err := suite.UserRepo.Count()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if count != 1 {
			t.Errorf("Expected count 1, got %d", count)
		}
	})
}

func TestUserRepository_ConcurrentAccess(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("concurrent creates", func(t *testing.T) {
		suite.Reset()

		done := make(chan bool, 10)
		errors := make(chan error, 10)

		for i := 0; i < 10; i++ {
			go func(id int) {
				defer func() { done <- true }()
				user := &database.User{
					Username:      fmt.Sprintf("concurrent%d", id),
					Email:         fmt.Sprintf("concurrent%d@example.com", id),
					Password:      "password123",
					EmailVerified: true,
				}
				var err error
				for retries := 0; retries < 5; retries++ {
					err = suite.UserRepo.Create(user)
					if err == nil {
						break
					}
					if strings.Contains(err.Error(), "locked") {
						time.Sleep(time.Millisecond * time.Duration(10*(retries+1)))
						continue
					}
					break
				}
				if err != nil {
					errors <- err
				}
			}(i)
		}

		for i := 0; i < 10; i++ {
			<-done
		}
		close(errors)

		for err := range errors {
			if !strings.Contains(err.Error(), "locked") {
				t.Errorf("Concurrent create failed: %v", err)
			}
		}

		count, err := suite.UserRepo.Count()
		if err != nil {
			t.Fatalf("Failed to count users: %v", err)
		}
		if count < 7 {
			t.Errorf("Expected at least 7 users (SQLite concurrency limitation), got %d", count)
		}
	})

	t.Run("concurrent updates", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("concurrent_update", "update@example.com", "password123")

		done := make(chan bool, 5)
		for i := 0; i < 5; i++ {
			go func(id int) {
				defer func() { done <- true }()
				user.Username = fmt.Sprintf("updated%d", id)
				_ = suite.UserRepo.Update(user)
			}(i)
		}

		for i := 0; i < 5; i++ {
			<-done
		}

		retrieved, err := suite.UserRepo.GetByID(user.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve user: %v", err)
		}
		if retrieved == nil {
			t.Error("Expected user to exist after concurrent updates")
		}
	})

	t.Run("concurrent deletes", func(t *testing.T) {
		suite.Reset()

		user1 := suite.CreateTestUser("delete1", "delete1@example.com", "password123")
		user2 := suite.CreateTestUser("delete2", "delete2@example.com", "password123")

		done := make(chan bool, 2)
		go func() {
			var err error
			for retries := 0; retries < 5; retries++ {
				err = suite.UserRepo.Delete(user1.ID)
				if err == nil {
					break
				}
				if strings.Contains(err.Error(), "locked") {
					time.Sleep(time.Millisecond * time.Duration(10*(retries+1)))
					continue
				}
				break
			}
			done <- true
		}()
		go func() {
			var err error
			for retries := 0; retries < 5; retries++ {
				err = suite.UserRepo.Delete(user2.ID)
				if err == nil {
					break
				}
				if strings.Contains(err.Error(), "locked") {
					time.Sleep(time.Millisecond * time.Duration(10*(retries+1)))
					continue
				}
				break
			}
			done <- true
		}()

		<-done
		<-done

		count, err := suite.UserRepo.Count()
		if err != nil {
			t.Fatalf("Failed to count users: %v", err)
		}
		if count > 0 {
			var err error
			for retries := 0; retries < 5; retries++ {
				count, err = suite.UserRepo.Count()
				if err == nil && count == 0 {
					break
				}
				time.Sleep(time.Millisecond * time.Duration(10*(retries+1)))
			}
			if count > 0 {
				t.Errorf("Expected 0 users after concurrent deletes (SQLite concurrency limitation), got %d", count)
			}
		}
	})
}

func TestUserRepository_WithTx(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("transaction repository", func(t *testing.T) {
		suite.Reset()

		tx := suite.DB.Begin()
		defer tx.Rollback()

		txUserRepo := suite.UserRepo.WithTx(tx)

		user := &database.User{
			Username:      "txuser",
			Email:         "tx@example.com",
			Password:      "password123",
			EmailVerified: true,
		}

		err := txUserRepo.Create(user)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		retrieved, err := txUserRepo.GetByID(user.ID)
		if err != nil {
			t.Fatalf("Expected to find user in transaction, got %v", err)
		}

		if retrieved.Username != "txuser" {
			t.Errorf("Expected username 'txuser', got %s", retrieved.Username)
		}
	})
}
