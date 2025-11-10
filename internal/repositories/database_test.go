package repositories

import (
	"fmt"
	"testing"

	"goyco/internal/database"

	"gorm.io/gorm"
)

func TestDatabase_AssertUserExists(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing user", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		retrieved := suite.AssertUserExists(user.ID)
		if retrieved == nil {
			t.Fatal("Expected user, got nil")
		}

		if retrieved.ID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, retrieved.ID)
		}
	})
}

func TestDatabase_AssertUserNotExists(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("non-existing user", func(t *testing.T) {
		suite.Reset()

		suite.AssertUserNotExists(999)
	})
}

func TestDatabase_AssertPostExists(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing post", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		retrieved := suite.AssertPostExists(post.ID)
		if retrieved == nil {
			t.Fatal("Expected post, got nil")
		}

		if retrieved.ID != post.ID {
			t.Errorf("Expected post ID %d, got %d", post.ID, retrieved.ID)
		}
	})
}

func TestDatabase_AssertPostNotExists(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("non-existing post", func(t *testing.T) {
		suite.Reset()

		suite.AssertPostNotExists(999)
	})
}

func TestDatabase_AssertVoteExists(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing vote", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")
		vote := suite.CreateTestVote(user.ID, post.ID, database.VoteUp)

		retrieved := suite.AssertVoteExists(vote.ID)
		if retrieved == nil {
			t.Fatal("Expected vote, got nil")
		}

		if retrieved.ID != vote.ID {
			t.Errorf("Expected vote ID %d, got %d", vote.ID, retrieved.ID)
		}
	})
}

func TestDatabase_AssertVoteNotExists(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("non-existing vote", func(t *testing.T) {
		suite.Reset()

		suite.AssertVoteNotExists(999)
	})
}

func TestDatabase_AssertAccountDeletionRequestExists(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing request", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		request := suite.CreateTestAccountDeletionRequest(user.ID, "token-hash-123")

		retrieved := suite.AssertAccountDeletionRequestExists("token-hash-123")
		if retrieved == nil {
			t.Fatal("Expected request, got nil")
		}

		if retrieved.ID != request.ID {
			t.Errorf("Expected request ID %d, got %d", request.ID, retrieved.ID)
		}
	})
}

func TestDatabase_AssertAccountDeletionRequestNotExists(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("non-existing request", func(t *testing.T) {
		suite.Reset()

		suite.AssertAccountDeletionRequestNotExists("nonexistent-token")
	})
}

func TestDatabase_GetUserCount(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("empty database", func(t *testing.T) {
		suite.Reset()

		count := suite.GetUserCount()
		if count != 0 {
			t.Errorf("Expected 0 users, got %d", count)
		}
	})

	t.Run("with users", func(t *testing.T) {
		suite.Reset()

		suite.CreateTestUser("user1", "user1@example.com", "password123")
		suite.CreateTestUser("user2", "user2@example.com", "password123")

		count := suite.GetUserCount()
		if count != 2 {
			t.Errorf("Expected 2 users, got %d", count)
		}
	})
}

func TestDatabase_GetPostCount(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("empty database", func(t *testing.T) {
		suite.Reset()

		count := suite.GetPostCount()
		if count != 0 {
			t.Errorf("Expected 0 posts, got %d", count)
		}
	})

	t.Run("with posts", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		suite.CreateTestPost(user.ID, "Post 1", "https://example.com/1", "Content 1")
		suite.CreateTestPost(user.ID, "Post 2", "https://example.com/2", "Content 2")

		count := suite.GetPostCount()
		if count != 2 {
			t.Errorf("Expected 2 posts, got %d", count)
		}
	})
}

func TestDatabase_GetVoteCount(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("empty database", func(t *testing.T) {
		suite.Reset()

		count := suite.GetVoteCount()
		if count != 0 {
			t.Errorf("Expected 0 votes, got %d", count)
		}
	})

	t.Run("with votes", func(t *testing.T) {
		suite.Reset()

		user1 := suite.CreateTestUser("user1", "user1@example.com", "password123")
		user2 := suite.CreateTestUser("user2", "user2@example.com", "password123")
		post := suite.CreateTestPost(user1.ID, "Test Post", "https://example.com", "Test content")

		suite.CreateTestVote(user1.ID, post.ID, database.VoteUp)
		suite.CreateTestVote(user2.ID, post.ID, database.VoteDown)

		count := suite.GetVoteCount()
		if count != 2 {
			t.Errorf("Expected 2 votes, got %d", count)
		}
	})
}

func TestDatabase_GetAccountDeletionRequestCount(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("empty database", func(t *testing.T) {
		suite.Reset()

		count := suite.GetAccountDeletionRequestCount()
		if count != 0 {
			t.Errorf("Expected 0 requests, got %d", count)
		}
	})

	t.Run("with requests", func(t *testing.T) {
		suite.Reset()

		user1 := suite.CreateTestUser("user1", "user1@example.com", "password123")
		user2 := suite.CreateTestUser("user2", "user2@example.com", "password123")
		suite.CreateTestAccountDeletionRequest(user1.ID, "token-hash-1")
		suite.CreateTestAccountDeletionRequest(user2.ID, "token-hash-2")

		count := suite.GetAccountDeletionRequestCount()
		if count != 2 {
			t.Errorf("Expected 2 requests, got %d", count)
		}
	})
}

func TestDatabase_Reset(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("reset clears all data", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")
		suite.CreateTestVote(user.ID, post.ID, database.VoteUp)
		suite.CreateTestAccountDeletionRequest(user.ID, "token-hash-123")

		if suite.GetUserCount() != 1 {
			t.Errorf("Expected 1 user before reset, got %d", suite.GetUserCount())
		}
		if suite.GetPostCount() != 1 {
			t.Errorf("Expected 1 post before reset, got %d", suite.GetPostCount())
		}
		if suite.GetVoteCount() != 1 {
			t.Errorf("Expected 1 vote before reset, got %d", suite.GetVoteCount())
		}
		if suite.GetAccountDeletionRequestCount() != 1 {
			t.Errorf("Expected 1 request before reset, got %d", suite.GetAccountDeletionRequestCount())
		}

		suite.Reset()

		if suite.GetUserCount() != 0 {
			t.Errorf("Expected 0 users after reset, got %d", suite.GetUserCount())
		}
		if suite.GetPostCount() != 0 {
			t.Errorf("Expected 0 posts after reset, got %d", suite.GetPostCount())
		}
		if suite.GetVoteCount() != 0 {
			t.Errorf("Expected 0 votes after reset, got %d", suite.GetVoteCount())
		}
		if suite.GetAccountDeletionRequestCount() != 0 {
			t.Errorf("Expected 0 requests after reset, got %d", suite.GetAccountDeletionRequestCount())
		}
	})
}

func TestDatabase_CreateTestUser(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful creation", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		if user.ID == 0 {
			t.Error("Expected user ID to be assigned")
		}

		if user.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got %s", user.Username)
		}

		if user.Email != "test@example.com" {
			t.Errorf("Expected email 'test@example.com', got %s", user.Email)
		}

		if !user.EmailVerified {
			t.Error("Expected user to be email verified")
		}
	})
}

func TestDatabase_CreateTestPost(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful creation", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		if post.ID == 0 {
			t.Error("Expected post ID to be assigned")
		}

		if post.Title != "Test Post" {
			t.Errorf("Expected title 'Test Post', got %s", post.Title)
		}

		if post.URL != "https://example.com" {
			t.Errorf("Expected URL 'https://example.com', got %s", post.URL)
		}

		if post.Content != "Test content" {
			t.Errorf("Expected content 'Test content', got %s", post.Content)
		}

		if post.AuthorID == nil || *post.AuthorID != user.ID {
			t.Errorf("Expected author ID %d, got %v", user.ID, post.AuthorID)
		}
	})
}

func TestDatabase_CreateTestVote(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful creation", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")
		vote := suite.CreateTestVote(user.ID, post.ID, database.VoteUp)

		if vote.ID == 0 {
			t.Error("Expected vote ID to be assigned")
		}

		if vote.UserID == nil || *vote.UserID != user.ID {
			t.Errorf("Expected user ID %d, got %v", user.ID, vote.UserID)
		}

		if vote.PostID != post.ID {
			t.Errorf("Expected post ID %d, got %d", post.ID, vote.PostID)
		}

		if vote.Type != database.VoteUp {
			t.Errorf("Expected vote type %v, got %v", database.VoteUp, vote.Type)
		}
	})
}

func TestDatabase_CreateTestAccountDeletionRequest(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful creation", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		request := suite.CreateTestAccountDeletionRequest(user.ID, "token-hash-123")

		if request.ID == 0 {
			t.Error("Expected request ID to be assigned")
		}

		if request.UserID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, request.UserID)
		}

		if request.TokenHash != "token-hash-123" {
			t.Errorf("Expected token hash 'token-hash-123', got %s", request.TokenHash)
		}
	})
}

func TestApplyPagination(t *testing.T) {
	suite := NewTestSuite(t)

	tests := []struct {
		name             string
		limit            int
		offset           int
		setupQuery       func(*gorm.DB) *gorm.DB
		verifyPagination func(*testing.T, *gorm.DB, int, int)
	}{
		{
			name:   "limit > 0 and offset > 0",
			limit:  10,
			offset: 5,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 20; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				if len(users) != limit {
					t.Errorf("Expected %d users, got %d", limit, len(users))
				}
			},
		},
		{
			name:   "limit > 0 and offset = 0",
			limit:  5,
			offset: 0,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 10; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				if len(users) != limit {
					t.Errorf("Expected %d users, got %d", limit, len(users))
				}
			},
		},
		{
			name:   "limit = 0 (should not apply limit)",
			limit:  0,
			offset: 5,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 10; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				expected := 5
				if len(users) != expected {
					t.Errorf("Expected %d users with offset %d, got %d", expected, offset, len(users))
				}
			},
		},
		{
			name:   "offset = 0 (should not apply offset)",
			limit:  10,
			offset: 0,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 15; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				if len(users) != limit {
					t.Errorf("Expected %d users, got %d", limit, len(users))
				}
			},
		},
		{
			name:   "limit = 0 and offset = 0 (should not apply pagination)",
			limit:  0,
			offset: 0,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 10; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				if len(users) != 10 {
					t.Errorf("Expected all 10 users, got %d", len(users))
				}
			},
		},
		{
			name:   "negative limit (should not apply limit)",
			limit:  -5,
			offset: 10,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 20; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				expected := 10
				if len(users) != expected {
					t.Errorf("Expected %d users with offset %d, got %d", expected, offset, len(users))
				}
			},
		},
		{
			name:   "negative offset (should not apply offset)",
			limit:  10,
			offset: -5,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 15; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				if len(users) != limit {
					t.Errorf("Expected %d users, got %d", limit, len(users))
				}
			},
		},
		{
			name:   "large limit and offset values",
			limit:  1000,
			offset: 500,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 2000; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				if len(users) != limit {
					t.Errorf("Expected %d users, got %d", limit, len(users))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suite.Reset()

			baseQuery := tt.setupQuery(suite.DB)
			paginatedQuery := ApplyPagination(baseQuery, tt.limit, tt.offset)

			tt.verifyPagination(t, paginatedQuery, tt.limit, tt.offset)
		})
	}
}
