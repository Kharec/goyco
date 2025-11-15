package database

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := "file:memdb_" + t.Name() + "?mode=memory&cache=shared&_journal_mode=WAL&_synchronous=NORMAL"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	err = db.AutoMigrate(
		&User{},
		&Post{},
		&Vote{},
		&AccountDeletionRequest{},
		&RefreshToken{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	if execErr := db.Exec("PRAGMA busy_timeout = 5000").Error; execErr != nil {
		t.Fatalf("Failed to configure busy timeout: %v", execErr)
	}
	if execErr := db.Exec("PRAGMA foreign_keys = ON").Error; execErr != nil {
		t.Fatalf("Failed to enable foreign keys: %v", execErr)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("Failed to access SQL DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	return db
}

func createTestUser(t *testing.T, db *gorm.DB) *User {
	t.Helper()

	uniqueID := time.Now().UnixNano()
	user := &User{
		Username:      fmt.Sprintf("testuser%d", uniqueID),
		Email:         fmt.Sprintf("test%d@example.com", uniqueID),
		Password:      "hashedpassword123",
		EmailVerified: true,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	return user
}

func createTestPost(t *testing.T, db *gorm.DB, authorID uint) *Post {
	t.Helper()
	post := &Post{
		Title:    "Test Post " + t.Name(),
		URL:      "https://example.com/test" + t.Name(),
		Content:  "Test content",
		AuthorID: &authorID,
	}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("Failed to create test post: %v", err)
	}
	return post
}

func TestUser_Model(t *testing.T) {
	db := newTestDB(t)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()

	t.Run("create_user", func(t *testing.T) {
		user := &User{
			Username:      "testuser",
			Email:         "test@example.com",
			Password:      "hashedpassword",
			EmailVerified: true,
		}

		if err := db.Create(user).Error; err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		if user.ID == 0 {
			t.Error("Expected user ID to be set")
		}

		if user.CreatedAt.IsZero() {
			t.Error("Expected CreatedAt to be set")
		}

		if user.UpdatedAt.IsZero() {
			t.Error("Expected UpdatedAt to be set")
		}
	})

	t.Run("user_constraints", func(t *testing.T) {

		user1 := &User{
			Username:      "duplicate",
			Email:         "user1@example.com",
			Password:      "hashedpassword",
			EmailVerified: true,
		}

		user2 := &User{
			Username:      "duplicate",
			Email:         "user2@example.com",
			Password:      "hashedpassword",
			EmailVerified: true,
		}

		if err := db.Create(user1).Error; err != nil {
			t.Fatalf("Failed to create first user: %v", err)
		}

		if err := db.Create(user2).Error; err == nil {
			t.Error("Expected error when creating user with duplicate username")
		}

		user3 := &User{
			Username:      "unique",
			Email:         "user1@example.com",
			Password:      "hashedpassword",
			EmailVerified: true,
		}

		if err := db.Create(user3).Error; err == nil {
			t.Error("Expected error when creating user with duplicate email")
		}
	})

	t.Run("user_relationships", func(t *testing.T) {
		user := &User{
			Username:      "author",
			Email:         "author@example.com",
			Password:      "hashedpassword",
			EmailVerified: true,
		}

		if err := db.Create(user).Error; err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		post1 := &Post{
			Title:    "Post 1",
			URL:      "https://example.com/1",
			Content:  "Content 1",
			AuthorID: &user.ID,
		}

		post2 := &Post{
			Title:    "Post 2",
			URL:      "https://example.com/2",
			Content:  "Content 2",
			AuthorID: &user.ID,
		}

		if err := db.Create(post1).Error; err != nil {
			t.Fatalf("Failed to create post 1: %v", err)
		}

		if err := db.Create(post2).Error; err != nil {
			t.Fatalf("Failed to create post 2: %v", err)
		}

		var foundUser User
		if err := db.Preload("Posts").First(&foundUser, user.ID).Error; err != nil {
			t.Fatalf("Failed to load user with posts: %v", err)
		}

		if len(foundUser.Posts) != 2 {
			t.Errorf("Expected 2 posts, got %d", len(foundUser.Posts))
		}
	})
}

func TestPost_Model(t *testing.T) {
	db := newTestDB(t)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()

	t.Run("create_post", func(t *testing.T) {
		user := createTestUser(t, db)

		post := &Post{
			Title:    "Test Post",
			URL:      "https://example.com/test",
			Content:  "Test content",
			AuthorID: &user.ID,
		}

		if err := db.Create(post).Error; err != nil {
			t.Fatalf("Failed to create post: %v", err)
		}

		if post.ID == 0 {
			t.Error("Expected post ID to be set")
		}

		if post.CreatedAt.IsZero() {
			t.Error("Expected CreatedAt to be set")
		}

		if post.UpdatedAt.IsZero() {
			t.Error("Expected UpdatedAt to be set")
		}

		if post.UpVotes != 0 {
			t.Error("Expected UpVotes to be 0 by default")
		}

		if post.DownVotes != 0 {
			t.Error("Expected DownVotes to be 0 by default")
		}

		if post.Score != 0 {
			t.Error("Expected Score to be 0 by default")
		}
	})

	t.Run("post_constraints", func(t *testing.T) {
		user := createTestUser(t, db)

		post1 := &Post{
			Title:    "Post 1",
			URL:      "https://example.com/unique",
			Content:  "Content 1",
			AuthorID: &user.ID,
		}

		post2 := &Post{
			Title:    "Post 2",
			URL:      "https://example.com/unique",
			Content:  "Content 2",
			AuthorID: &user.ID,
		}

		if err := db.Create(post1).Error; err != nil {
			t.Fatalf("Failed to create first post: %v", err)
		}

		if err := db.Create(post2).Error; err == nil {
			t.Error("Expected error when creating post with duplicate URL")
		}
	})

	t.Run("post_relationships", func(t *testing.T) {
		user1 := createTestUser(t, db)
		user2 := createTestUser(t, db)

		post := createTestPost(t, db, user1.ID)

		vote1 := &Vote{
			UserID: &user1.ID,
			PostID: post.ID,
			Type:   VoteUp,
		}

		vote2 := &Vote{
			UserID: &user2.ID,
			PostID: post.ID,
			Type:   VoteDown,
		}

		if err := db.Create(vote1).Error; err != nil {
			t.Fatalf("Failed to create vote 1: %v", err)
		}

		if err := db.Create(vote2).Error; err != nil {
			t.Fatalf("Failed to create vote 2: %v", err)
		}

		var foundPost Post
		if err := db.Preload("Votes").First(&foundPost, post.ID).Error; err != nil {
			t.Fatalf("Failed to load post with votes: %v", err)
		}

		if len(foundPost.Votes) != 2 {
			t.Errorf("Expected 2 votes, got %d", len(foundPost.Votes))
		}
	})
}

func TestVote_Model(t *testing.T) {
	db := newTestDB(t)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()

	t.Run("create_vote", func(t *testing.T) {
		user := createTestUser(t, db)
		post := createTestPost(t, db, user.ID)

		vote := &Vote{
			UserID: &user.ID,
			PostID: post.ID,
			Type:   VoteUp,
		}

		if err := db.Create(vote).Error; err != nil {
			t.Fatalf("Failed to create vote: %v", err)
		}

		if vote.ID == 0 {
			t.Error("Expected vote ID to be set")
		}

		if vote.CreatedAt.IsZero() {
			t.Error("Expected CreatedAt to be set")
		}

		if vote.UpdatedAt.IsZero() {
			t.Error("Expected UpdatedAt to be set")
		}
	})

	t.Run("vote_constraints", func(t *testing.T) {
		user := createTestUser(t, db)
		post := createTestPost(t, db, user.ID)

		vote1 := &Vote{
			UserID: &user.ID,
			PostID: post.ID,
			Type:   VoteUp,
		}

		vote2 := &Vote{
			UserID: &user.ID,
			PostID: post.ID,
			Type:   VoteDown,
		}

		if err := db.Create(vote1).Error; err != nil {
			t.Fatalf("Failed to create first vote: %v", err)
		}

		if err := db.Create(vote2).Error; err == nil {
			t.Error("Expected error when creating vote with duplicate user-post combination")
		}
	})

	t.Run("vote_types", func(t *testing.T) {
		user := createTestUser(t, db)

		voteTypes := []VoteType{VoteUp, VoteDown, VoteNone}

		for i, voteType := range voteTypes {

			post := &Post{
				Title:    "Test Post " + string(rune(i)),
				URL:      "https://example.com/test" + string(rune(i)),
				Content:  "Test content",
				AuthorID: &user.ID,
			}

			if err := db.Create(post).Error; err != nil {
				t.Fatalf("Failed to create post %d: %v", i, err)
			}

			vote := &Vote{
				UserID: &user.ID,
				PostID: post.ID,
				Type:   voteType,
			}

			if err := db.Create(vote).Error; err != nil {
				t.Fatalf("Failed to create vote with type %s: %v", voteType, err)
			}
		}
	})

	t.Run("vote_relationships", func(t *testing.T) {
		user := createTestUser(t, db)
		post := createTestPost(t, db, user.ID)

		vote := &Vote{
			UserID: &user.ID,
			PostID: post.ID,
			Type:   VoteUp,
		}

		if err := db.Create(vote).Error; err != nil {
			t.Fatalf("Failed to create vote: %v", err)
		}

		var foundVote Vote
		if err := db.Preload("User").Preload("Post").First(&foundVote, vote.ID).Error; err != nil {
			t.Fatalf("Failed to load vote with relationships: %v", err)
		}

		if foundVote.User.ID != user.ID {
			t.Error("Expected vote to be associated with correct user")
		}

		if foundVote.Post.ID != post.ID {
			t.Error("Expected vote to be associated with correct post")
		}
	})
}

func TestRefreshToken_Model(t *testing.T) {
	db := newTestDB(t)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()

	t.Run("create_refresh_token", func(t *testing.T) {
		user := createTestUser(t, db)

		token := &RefreshToken{
			UserID:    user.ID,
			TokenHash: "hashedtoken123",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		if err := db.Create(token).Error; err != nil {
			t.Fatalf("Failed to create refresh token: %v", err)
		}

		if token.ID == 0 {
			t.Error("Expected token ID to be set")
		}

		if token.CreatedAt.IsZero() {
			t.Error("Expected CreatedAt to be set")
		}

		if token.UpdatedAt.IsZero() {
			t.Error("Expected UpdatedAt to be set")
		}
	})

	t.Run("refresh_token_constraints", func(t *testing.T) {
		user := createTestUser(t, db)

		token1 := &RefreshToken{
			UserID:    user.ID,
			TokenHash: "uniquehash",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		token2 := &RefreshToken{
			UserID:    user.ID,
			TokenHash: "uniquehash",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		if err := db.Create(token1).Error; err != nil {
			t.Fatalf("Failed to create first token: %v", err)
		}

		if err := db.Create(token2).Error; err == nil {
			t.Error("Expected error when creating token with duplicate hash")
		}
	})
}

func TestAccountDeletionRequest_Model(t *testing.T) {
	db := newTestDB(t)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()

	t.Run("create_account_deletion_request", func(t *testing.T) {
		user := createTestUser(t, db)

		request := &AccountDeletionRequest{
			UserID:    user.ID,
			TokenHash: "deletiontoken123",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		if err := db.Create(request).Error; err != nil {
			t.Fatalf("Failed to create account deletion request: %v", err)
		}

		if request.ID == 0 {
			t.Error("Expected request ID to be set")
		}

		if request.CreatedAt.IsZero() {
			t.Error("Expected CreatedAt to be set")
		}
	})

	t.Run("account_deletion_request_constraints", func(t *testing.T) {
		user := createTestUser(t, db)

		request1 := &AccountDeletionRequest{
			UserID:    user.ID,
			TokenHash: "token1",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		request2 := &AccountDeletionRequest{
			UserID:    user.ID,
			TokenHash: "token2",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		if err := db.Create(request1).Error; err != nil {
			t.Fatalf("Failed to create first request: %v", err)
		}

		if err := db.Create(request2).Error; err == nil {
			t.Error("Expected error when creating request with duplicate user")
		}
	})
}

func TestVoteType_Constants(t *testing.T) {
	t.Run("vote_type_constants", func(t *testing.T) {
		if VoteUp != "up" {
			t.Errorf("Expected VoteUp to be 'up', got '%s'", VoteUp)
		}

		if VoteDown != "down" {
			t.Errorf("Expected VoteDown to be 'down', got '%s'", VoteDown)
		}

		if VoteNone != "none" {
			t.Errorf("Expected VoteNone to be 'none', got '%s'", VoteNone)
		}
	})
}

func TestModel_SoftDelete(t *testing.T) {
	db := newTestDB(t)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()

	t.Run("user_soft_delete", func(t *testing.T) {
		user := createTestUser(t, db)

		if err := db.Delete(user).Error; err != nil {
			t.Fatalf("Failed to soft delete user: %v", err)
		}

		var foundUser User
		if err := db.First(&foundUser, user.ID).Error; err == nil {
			t.Error("Expected user to be soft deleted")
		}

		if err := db.Unscoped().First(&foundUser, user.ID).Error; err != nil {
			t.Fatalf("Expected to find soft deleted user with Unscoped: %v", err)
		}

		if foundUser.DeletedAt.Time.IsZero() {
			t.Error("Expected DeletedAt to be set")
		}
	})

	t.Run("post_soft_delete", func(t *testing.T) {
		user := createTestUser(t, db)
		post := createTestPost(t, db, user.ID)

		if err := db.Delete(post).Error; err != nil {
			t.Fatalf("Failed to soft delete post: %v", err)
		}

		var foundPost Post
		if err := db.First(&foundPost, post.ID).Error; err == nil {
			t.Error("Expected post to be soft deleted")
		}

		if err := db.Unscoped().First(&foundPost, post.ID).Error; err != nil {
			t.Fatalf("Expected to find soft deleted post with Unscoped: %v", err)
		}

		if foundPost.DeletedAt.Time.IsZero() {
			t.Error("Expected DeletedAt to be set")
		}
	})
}
