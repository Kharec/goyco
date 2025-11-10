package repositories

import (
	"sync"
	"testing"

	"gorm.io/gorm"
	"goyco/internal/database"
)

func TestVoteRepository_Create(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful creation", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		suite.CreateTestVote(user.ID, post.ID, database.VoteUp)

		if suite.GetVoteCount() != 1 {
			t.Errorf("Expected 1 vote, got %d", suite.GetVoteCount())
		}

	})

	t.Run("duplicate vote", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		suite.CreateTestVote(user.ID, post.ID, database.VoteUp)

		vote2 := &database.Vote{
			UserID: &user.ID,
			PostID: post.ID,
			Type:   database.VoteDown,
		}
		err := suite.VoteRepo.Create(vote2)
		if err == nil {
			t.Error("Expected duplicate vote constraint error")
		}
	})

	t.Run("vote with invalid user", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		vote := &database.Vote{
			UserID: suite.CreateInvalidUserID(),
			PostID: post.ID,
			Type:   database.VoteUp,
		}
		err := suite.VoteRepo.Create(vote)

		if err != nil {
			t.Errorf("Unexpected error for invalid user (SQLite allows this): %v", err)
		}
	})

	t.Run("vote with invalid post", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		vote := &database.Vote{
			UserID: &user.ID,
			PostID: 999,
			Type:   database.VoteUp,
		}
		err := suite.VoteRepo.Create(vote)

		if err != nil {
			t.Errorf("Unexpected error for invalid post (SQLite allows this): %v", err)
		}
	})
}

func TestVoteRepository_GetByID(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing vote", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")
		vote := suite.CreateTestVote(user.ID, post.ID, database.VoteUp)

		retrieved, err := suite.VoteRepo.GetByID(vote.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected vote, got nil")
		}

		if retrieved.ID != vote.ID {
			t.Errorf("Expected ID %d, got %d", vote.ID, retrieved.ID)
		}
		if vote.UserID == nil {
			t.Fatal("Test setup error: vote.UserID is nil")
		}
		if retrieved.UserID == nil {
			t.Fatalf("Expected user ID %d, got nil", *vote.UserID)
		}
		if *retrieved.UserID != *vote.UserID {
			t.Errorf("Expected user ID %d, got %d", *vote.UserID, *retrieved.UserID)
		}
		if retrieved.PostID != vote.PostID {
			t.Errorf("Expected post ID %d, got %d", vote.PostID, retrieved.PostID)
		}
		if retrieved.Type != vote.Type {
			t.Errorf("Expected type %v, got %v", vote.Type, retrieved.Type)
		}
	})

	t.Run("non-existing vote", func(t *testing.T) {
		suite.Reset()

		_, err := suite.VoteRepo.GetByID(999)
		if err == nil {
			t.Error("Expected error for non-existing vote")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestVoteRepository_GetByUserAndPost(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing vote", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")
		suite.CreateTestVote(user.ID, post.ID, database.VoteUp)

		retrieved, err := suite.VoteRepo.GetByUserAndPost(user.ID, post.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected vote, got nil")
		}

		if retrieved.UserID == nil || *retrieved.UserID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, retrieved.UserID)
		}
		if retrieved.PostID != post.ID {
			t.Errorf("Expected post ID %d, got %d", post.ID, retrieved.PostID)
		}
		if retrieved.Type != database.VoteUp {
			t.Errorf("Expected type %v, got %v", database.VoteUp, retrieved.Type)
		}
	})

	t.Run("non-existing vote", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		_, err := suite.VoteRepo.GetByUserAndPost(user.ID, post.ID)
		if err == nil {
			t.Error("Expected error for non-existing vote")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestVoteRepository_GetByPostID(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("post with votes", func(t *testing.T) {
		suite.Reset()

		user1 := suite.CreateTestUser("user1", "user1@example.com", "password123")
		user2 := suite.CreateTestUser("user2", "user2@example.com", "password123")
		post := suite.CreateTestPost(user1.ID, "Test Post", "https://example.com", "Test content")

		suite.CreateTestVote(user1.ID, post.ID, database.VoteUp)
		suite.CreateTestVote(user2.ID, post.ID, database.VoteDown)

		retrieved, err := suite.VoteRepo.GetByPostID(post.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 votes, got %d", len(retrieved))
		}

		for _, vote := range retrieved {
			if vote.PostID != post.ID {
				t.Errorf("Expected post ID %d, got %d", post.ID, vote.PostID)
			}
		}
	})

	t.Run("post without votes", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		retrieved, err := suite.VoteRepo.GetByPostID(post.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 0 {
			t.Errorf("Expected 0 votes, got %d", len(retrieved))
		}
	})
}

func TestVoteRepository_GetByUserID(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("user with votes", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post1 := suite.CreateTestPost(user.ID, "Post 1", "https://example.com/1", "Content 1")
		post2 := suite.CreateTestPost(user.ID, "Post 2", "https://example.com/2", "Content 2")

		suite.CreateTestVote(user.ID, post1.ID, database.VoteUp)
		suite.CreateTestVote(user.ID, post2.ID, database.VoteDown)

		retrieved, err := suite.VoteRepo.GetByUserID(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 votes, got %d", len(retrieved))
		}

		for _, vote := range retrieved {
			if vote.UserID == nil || *vote.UserID != user.ID {
				t.Errorf("Expected user ID %d, got %d", user.ID, vote.UserID)
			}
		}
	})

	t.Run("user without votes", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		retrieved, err := suite.VoteRepo.GetByUserID(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 0 {
			t.Errorf("Expected 0 votes, got %d", len(retrieved))
		}
	})
}

func TestVoteRepository_Update(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful update", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")
		vote := suite.CreateTestVote(user.ID, post.ID, database.VoteUp)

		vote.Type = database.VoteDown
		err := suite.VoteRepo.Update(vote)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		retrieved, err := suite.VoteRepo.GetByID(vote.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve vote: %v", err)
		}

		if retrieved.Type != database.VoteDown {
			t.Errorf("Expected type %v, got %v", database.VoteDown, retrieved.Type)
		}
	})
}

func TestVoteRepository_Delete(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful delete", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")
		vote := suite.CreateTestVote(user.ID, post.ID, database.VoteUp)

		err := suite.VoteRepo.Delete(vote.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		_, err = suite.VoteRepo.GetByID(vote.ID)
		if err == nil {
			t.Error("Expected error for deleted vote")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestVoteRepository_CountByPostID(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("post with votes", func(t *testing.T) {
		suite.Reset()

		user1 := suite.CreateTestUser("user1", "user1@example.com", "password123")
		user2 := suite.CreateTestUser("user2", "user2@example.com", "password123")
		user3 := suite.CreateTestUser("user3", "user3@example.com", "password123")
		post := suite.CreateTestPost(user1.ID, "Test Post", "https://example.com", "Test content")

		suite.CreateTestVote(user1.ID, post.ID, database.VoteUp)
		suite.CreateTestVote(user2.ID, post.ID, database.VoteDown)
		suite.CreateTestVote(user3.ID, post.ID, database.VoteUp)

		count, err := suite.VoteRepo.CountByPostID(post.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if count != 3 {
			t.Errorf("Expected 3 votes, got %d", count)
		}
	})

	t.Run("post without votes", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		count, err := suite.VoteRepo.CountByPostID(post.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if count != 0 {
			t.Errorf("Expected 0 votes, got %d", count)
		}
	})
}

func TestVoteRepository_CountByUserID(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("user with votes", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post1 := suite.CreateTestPost(user.ID, "Post 1", "https://example.com/1", "Content 1")
		post2 := suite.CreateTestPost(user.ID, "Post 2", "https://example.com/2", "Content 2")

		suite.CreateTestVote(user.ID, post1.ID, database.VoteUp)
		suite.CreateTestVote(user.ID, post2.ID, database.VoteDown)

		count, err := suite.VoteRepo.CountByUserID(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if count != 2 {
			t.Errorf("Expected 2 votes, got %d", count)
		}
	})

	t.Run("user without votes", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		count, err := suite.VoteRepo.CountByUserID(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if count != 0 {
			t.Errorf("Expected 0 votes, got %d", count)
		}
	})
}

func TestVoteRepository_EdgeCases(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("invalid vote type", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		vote := &database.Vote{
			UserID: &user.ID,
			PostID: post.ID,
			Type:   "invalid",
		}
		err := suite.VoteRepo.Create(vote)

		if err != nil {
			t.Errorf("Unexpected error for invalid vote type: %v", err)
		}
	})

	t.Run("zero user ID", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		vote := &database.Vote{
			UserID: nil,
			PostID: post.ID,
			Type:   database.VoteUp,
		}
		err := suite.VoteRepo.Create(vote)

		if err != nil {
			t.Errorf("Unexpected error for nil user ID: %v", err)
		}
	})

	t.Run("zero post ID", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		vote := &database.Vote{
			UserID: &user.ID,
			PostID: 0,
			Type:   database.VoteUp,
		}
		err := suite.VoteRepo.Create(vote)

		if err != nil {
			t.Errorf("Unexpected error for zero post ID: %v", err)
		}
	})
}

func TestVoteRepository_CreateOrUpdate(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("create new vote", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		vote := &database.Vote{
			UserID: &user.ID,
			PostID: post.ID,
			Type:   database.VoteUp,
		}

		err := suite.VoteRepo.CreateOrUpdate(vote)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if vote.ID == 0 {
			t.Error("Expected vote ID to be assigned")
		}
	})

	t.Run("update existing vote", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		initialVote := suite.CreateTestVote(user.ID, post.ID, database.VoteUp)

		updateVote := &database.Vote{
			UserID: &user.ID,
			PostID: post.ID,
			Type:   database.VoteDown,
		}

		err := suite.VoteRepo.CreateOrUpdate(updateVote)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		retrieved, err := suite.VoteRepo.GetByID(initialVote.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve vote: %v", err)
		}

		if retrieved.Type != database.VoteDown {
			t.Errorf("Expected vote type %v, got %v", database.VoteDown, retrieved.Type)
		}
	})

	t.Run("create vote with vote hash", func(t *testing.T) {
		suite.Reset()

		post := suite.CreateTestPost(1, "Test Post", "https://example.com", "Test content")

		vote := &database.Vote{
			VoteHash: func() *string { s := "vote-hash-123"; return &s }(),
			PostID:   post.ID,
			Type:     database.VoteUp,
		}

		err := suite.VoteRepo.CreateOrUpdate(vote)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if vote.ID == 0 {
			t.Error("Expected vote ID to be assigned")
		}
	})

	t.Run("update vote with vote hash", func(t *testing.T) {
		suite.Reset()

		post := suite.CreateTestPost(1, "Test Post", "https://example.com", "Test content")

		initialVote := &database.Vote{
			VoteHash: func() *string { s := "vote-hash-123"; return &s }(),
			PostID:   post.ID,
			Type:     database.VoteUp,
		}
		err := suite.VoteRepo.Create(initialVote)
		if err != nil {
			t.Fatalf("Failed to create initial vote: %v", err)
		}

		updateVote := &database.Vote{
			VoteHash: func() *string { s := "vote-hash-123"; return &s }(),
			PostID:   post.ID,
			Type:     database.VoteDown,
		}

		err = suite.VoteRepo.CreateOrUpdate(updateVote)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		retrieved, err := suite.VoteRepo.GetByVoteHash("vote-hash-123")
		if err != nil {
			t.Fatalf("Failed to retrieve vote: %v", err)
		}

		if retrieved.Type != database.VoteDown {
			t.Errorf("Expected vote type %v, got %v", database.VoteDown, retrieved.Type)
		}
	})

	t.Run("vote without user_id or vote_hash", func(t *testing.T) {
		suite.Reset()

		post := suite.CreateTestPost(1, "Test Post", "https://example.com", "Test content")

		vote := &database.Vote{
			PostID: post.ID,
			Type:   database.VoteUp,
		}

		err := suite.VoteRepo.CreateOrUpdate(vote)
		if err == nil {
			t.Error("Expected error for vote without user_id or vote_hash")
		}
	})
}

func TestVoteRepository_GetByVoteHash(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing vote with hash", func(t *testing.T) {
		suite.Reset()

		post := suite.CreateTestPost(1, "Test Post", "https://example.com", "Test content")

		vote := &database.Vote{
			VoteHash: func() *string { s := "vote-hash-123"; return &s }(),
			PostID:   post.ID,
			Type:     database.VoteUp,
		}
		err := suite.VoteRepo.Create(vote)
		if err != nil {
			t.Fatalf("Failed to create vote: %v", err)
		}

		retrieved, err := suite.VoteRepo.GetByVoteHash("vote-hash-123")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected vote, got nil")
		}

		if retrieved.VoteHash == nil || *retrieved.VoteHash != "vote-hash-123" {
			t.Errorf("Expected vote hash 'vote-hash-123', got %v", retrieved.VoteHash)
		}
		if retrieved.PostID != post.ID {
			t.Errorf("Expected post ID %d, got %d", post.ID, retrieved.PostID)
		}
		if retrieved.Type != database.VoteUp {
			t.Errorf("Expected type %v, got %v", database.VoteUp, retrieved.Type)
		}
	})

	t.Run("non-existing vote hash", func(t *testing.T) {
		suite.Reset()

		_, err := suite.VoteRepo.GetByVoteHash("nonexistent-hash")
		if err == nil {
			t.Error("Expected error for non-existing vote hash")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestVoteRepository_WithTx(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("transaction repository", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		tx := suite.DB.Begin()
		defer tx.Rollback()

		txVoteRepo := suite.VoteRepo.WithTx(tx)

		vote := &database.Vote{
			UserID: &user.ID,
			PostID: post.ID,
			Type:   database.VoteUp,
		}

		err := txVoteRepo.Create(vote)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		retrieved, err := txVoteRepo.GetByID(vote.ID)
		if err != nil {
			t.Fatalf("Expected to find vote in transaction, got %v", err)
		}

		if retrieved.Type != database.VoteUp {
			t.Errorf("Expected vote type %v, got %v", database.VoteUp, retrieved.Type)
		}
	})
}

func TestVoteRepository_Count(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("empty database", func(t *testing.T) {
		suite.Reset()

		count, err := suite.VoteRepo.Count()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if count != 0 {
			t.Errorf("Expected count 0, got %d", count)
		}
	})

	t.Run("with votes", func(t *testing.T) {
		suite.Reset()

		user1 := suite.CreateTestUser("user1", "user1@example.com", "password123")
		user2 := suite.CreateTestUser("user2", "user2@example.com", "password123")
		post := suite.CreateTestPost(user1.ID, "Test Post", "https://example.com", "Test content")

		suite.CreateTestVote(user1.ID, post.ID, database.VoteUp)
		suite.CreateTestVote(user2.ID, post.ID, database.VoteDown)

		count, err := suite.VoteRepo.Count()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if count != 2 {
			t.Errorf("Expected count 2, got %d", count)
		}
	})
}

func TestVoteRepository_ConcurrentAccess(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("concurrent votes on same post with consistency check", func(t *testing.T) {
		suite.Reset()

		user1 := suite.CreateTestUser("user1", "user1@example.com", "password123")
		user2 := suite.CreateTestUser("user2", "user2@example.com", "password123")
		user3 := suite.CreateTestUser("user3", "user3@example.com", "password123")
		post := suite.CreateTestPost(user1.ID, "Test Post", "https://example.com", "Test content")

		var wg sync.WaitGroup
		errors := make(chan error, 3)

		wg.Add(3)
		go func() {
			defer wg.Done()
			if err := suite.VoteRepo.Create(&database.Vote{UserID: &user1.ID, PostID: post.ID, Type: database.VoteUp}); err != nil {
				errors <- err
			}
		}()
		go func() {
			defer wg.Done()
			if err := suite.VoteRepo.Create(&database.Vote{UserID: &user2.ID, PostID: post.ID, Type: database.VoteDown}); err != nil {
				errors <- err
			}
		}()
		go func() {
			defer wg.Done()
			if err := suite.VoteRepo.Create(&database.Vote{UserID: &user3.ID, PostID: post.ID, Type: database.VoteUp}); err != nil {
				errors <- err
			}
		}()

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("Concurrent vote creation failed: %v", err)
		}

		count, err := suite.VoteRepo.CountByPostID(post.ID)
		if err != nil {
			t.Fatalf("Failed to count votes: %v", err)
		}
		if count != 3 {
			t.Errorf("Expected 3 votes, got %d", count)
		}

		votes, err := suite.VoteRepo.GetByPostID(post.ID)
		if err != nil {
			t.Fatalf("Failed to get votes: %v", err)
		}
		if len(votes) != 3 {
			t.Errorf("Expected 3 votes, got %d", len(votes))
		}

		uniqueUsers := make(map[uint]bool)
		for _, vote := range votes {
			if vote.UserID != nil {
				if uniqueUsers[*vote.UserID] {
					t.Errorf("Duplicate vote from user %d", *vote.UserID)
				}
				uniqueUsers[*vote.UserID] = true
			}
		}
	})
}
