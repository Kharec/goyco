package repositories

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"goyco/internal/database"
)

func TestPostRepository_Create(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful creation", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		if suite.GetPostCount() != 1 {
			t.Errorf("Expected 1 post, got %d", suite.GetPostCount())
		}

		if post.ID == 0 {
			t.Error("Expected post ID to be assigned")
		}
	})

	t.Run("post with invalid author", func(t *testing.T) {
		suite.Reset()

		post := &database.Post{
			Title:    "Test Post",
			URL:      "https://example.com",
			Content:  "Test content",
			AuthorID: suite.CreateInvalidUserID(),
		}
		err := suite.PostRepo.Create(post)

		if err != nil {
			t.Errorf("Unexpected error for invalid author (SQLite allows this): %v", err)
		}
	})

	t.Run("duplicate URL", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		suite.CreateTestPost(user.ID, "Post 1", "https://example.com", "Content 1")

		post2 := &database.Post{
			Title:    "Post 2",
			URL:      "https://example.com",
			Content:  "Content 2",
			AuthorID: &user.ID,
		}
		err := suite.PostRepo.Create(post2)
		if err == nil {
			t.Error("Expected error for duplicate URL")
		}
		if !errors.Is(err, gorm.ErrDuplicatedKey) && !strings.Contains(err.Error(), "duplicate") && !strings.Contains(err.Error(), "UNIQUE constraint") {
			t.Errorf("Expected duplicate key error, got %v", err)
		}
	})
}

func TestPostRepository_GetByID(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("existing post", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		retrieved, err := suite.PostRepo.GetByID(post.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected post, got nil")
		}

		if retrieved.ID != post.ID {
			t.Errorf("Expected ID %d, got %d", post.ID, retrieved.ID)
		}
		if retrieved.Title != post.Title {
			t.Errorf("Expected title %s, got %s", post.Title, retrieved.Title)
		}
		if retrieved.URL != post.URL {
			t.Errorf("Expected URL %s, got %s", post.URL, retrieved.URL)
		}
		if retrieved.AuthorID == nil || post.AuthorID == nil {
			t.Error("Expected both AuthorID pointers to be non-nil")
		} else if *retrieved.AuthorID != *post.AuthorID {
			t.Errorf("Expected author ID %d, got %d", *post.AuthorID, *retrieved.AuthorID)
		}
	})

	t.Run("non-existing post", func(t *testing.T) {
		suite.Reset()

		_, err := suite.PostRepo.GetByID(999)
		if err == nil {
			t.Error("Expected error for non-existing post")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestPostRepository_GetAll(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("empty database", func(t *testing.T) {
		suite.Reset()

		posts, err := suite.PostRepo.GetAll(10, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(posts) != 0 {
			t.Errorf("Expected 0 posts, got %d", len(posts))
		}
	})

	t.Run("with posts", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		suite.CreateTestPost(user.ID, "Post 1", "https://example.com/1", "Content 1")
		suite.CreateTestPost(user.ID, "Post 2", "https://example.com/2", "Content 2")
		suite.CreateTestPost(user.ID, "Post 3", "https://example.com/3", "Content 3")

		retrieved, err := suite.PostRepo.GetAll(10, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 3 {
			t.Errorf("Expected 3 posts, got %d", len(retrieved))
		}
	})

	t.Run("with limit", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser2", "test2@example.com", "password123")

		for i := range 5 {
			suite.CreateTestPost(user.ID,
				"Post "+strconv.Itoa(i),
				"https://example.com/"+strconv.Itoa(i),
				"Content "+strconv.Itoa(i))
		}

		retrieved, err := suite.PostRepo.GetAll(2, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 posts, got %d", len(retrieved))
		}
	})

	t.Run("with offset", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser3", "test3@example.com", "password123")

		for i := range 5 {
			suite.CreateTestPost(user.ID,
				"Post "+strconv.Itoa(i),
				"https://example.com/"+strconv.Itoa(i),
				"Content "+strconv.Itoa(i))
		}

		retrieved, err := suite.PostRepo.GetAll(2, 1)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 posts, got %d", len(retrieved))
		}
	})
}

func TestPostRepository_GetByUserID(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("user with posts", func(t *testing.T) {
		suite.Reset()

		user1 := suite.CreateTestUser("user1", "user1@example.com", "password123")
		user2 := suite.CreateTestUser("user2", "user2@example.com", "password123")

		suite.CreateTestPost(user1.ID, "User1 Post", "https://example.com/1", "Content 1")
		suite.CreateTestPost(user2.ID, "User2 Post", "https://example.com/2", "Content 2")
		suite.CreateTestPost(user1.ID, "User1 Post 2", "https://example.com/3", "Content 3")

		retrieved, err := suite.PostRepo.GetByUserID(user1.ID, 10, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 posts for user1, got %d", len(retrieved))
		}

		for _, post := range retrieved {
			if post.AuthorID == nil || *post.AuthorID != user1.ID {
				t.Errorf("Expected author ID %d, got %v", user1.ID, post.AuthorID)
			}
		}
	})

	t.Run("user without posts", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("noposts", "noposts@example.com", "password123")

		retrieved, err := suite.PostRepo.GetByUserID(user.ID, 10, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 0 {
			t.Errorf("Expected 0 posts, got %d", len(retrieved))
		}
	})
}

func TestPostRepository_Update(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful update", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		post.Title = "Updated Post"
		post.Content = "Updated content"
		post.Score = 10
		post.UpVotes = 12
		post.DownVotes = 2
		err := suite.PostRepo.Update(post)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		retrieved, err := suite.PostRepo.GetByID(post.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve post: %v", err)
		}

		if retrieved.Title != "Updated Post" {
			t.Errorf("Expected title 'Updated Post', got %s", retrieved.Title)
		}
		if retrieved.Content != "Updated content" {
			t.Errorf("Expected content 'Updated content', got %s", retrieved.Content)
		}
		if retrieved.Score != 10 {
			t.Errorf("Expected score 10, got %d", retrieved.Score)
		}
		if retrieved.UpVotes != 12 {
			t.Errorf("Expected up votes 12, got %d", retrieved.UpVotes)
		}
		if retrieved.DownVotes != 2 {
			t.Errorf("Expected down votes 2, got %d", retrieved.DownVotes)
		}
	})
}

func TestPostRepository_Delete(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("successful delete", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		err := suite.PostRepo.Delete(post.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		_, err = suite.PostRepo.GetByID(post.ID)
		if err == nil {
			t.Error("Expected error for deleted post")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})
}

func TestPostRepository_Count(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("empty database", func(t *testing.T) {
		suite.Reset()

		count, err := suite.PostRepo.Count()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if count != 0 {
			t.Errorf("Expected count 0, got %d", count)
		}
	})

	t.Run("with posts", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		for i := range 5 {
			suite.CreateTestPost(user.ID,
				"Post "+strconv.Itoa(i),
				"https://example.com/"+strconv.Itoa(i),
				"Content "+strconv.Itoa(i))
		}

		count, err := suite.PostRepo.Count()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if count != 5 {
			t.Errorf("Expected count 5, got %d", count)
		}
	})
}

func TestPostRepository_GetTopPosts(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("with posts", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		post1 := &database.Post{
			Title:    "Low Score",
			URL:      "https://example.com/1",
			Content:  "Content 1",
			AuthorID: &user.ID,
			Score:    1,
		}
		post2 := &database.Post{
			Title:    "High Score",
			URL:      "https://example.com/2",
			Content:  "Content 2",
			AuthorID: &user.ID,
			Score:    10,
		}
		post3 := &database.Post{
			Title:    "Medium Score",
			URL:      "https://example.com/3",
			Content:  "Content 3",
			AuthorID: &user.ID,
			Score:    5,
		}

		suite.PostRepo.Create(post1)
		suite.PostRepo.Create(post2)
		suite.PostRepo.Create(post3)

		retrieved, err := suite.PostRepo.GetTopPosts(2)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 top posts, got %d", len(retrieved))
		}

		for i := 0; i < len(retrieved)-1; i++ {
			if retrieved[i].Score < retrieved[i+1].Score {
				t.Errorf("Posts not in descending order: %d < %d", retrieved[i].Score, retrieved[i+1].Score)
			}
		}
	})
}

func TestPostRepository_GetNewestPosts(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("with posts", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		post1 := &database.Post{
			Title:    "Old Post",
			URL:      "https://example.com/1",
			Content:  "Content 1",
			AuthorID: &user.ID,
		}
		post2 := &database.Post{
			Title:    "New Post",
			URL:      "https://example.com/2",
			Content:  "Content 2",
			AuthorID: &user.ID,
		}

		post1.CreatedAt = time.Now().Add(-2 * time.Hour)
		post2.CreatedAt = time.Now().Add(-1 * time.Hour)

		suite.PostRepo.Create(post1)
		suite.PostRepo.Create(post2)

		retrieved, err := suite.PostRepo.GetNewestPosts(2)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 newest posts, got %d", len(retrieved))
		}

		if retrieved[0].CreatedAt.Before(retrieved[1].CreatedAt) {
			t.Error("Expected posts to be ordered by creation time (newest first)")
		}
	})
}

func TestPostRepository_Search(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("with matching posts", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		suite.CreateTestPost(user.ID, "Golang Tutorial", "https://example.com/1", "Learn Go programming")
		suite.CreateTestPost(user.ID, "Python Guide", "https://example.com/2", "Learn Python programming")
		suite.CreateTestPost(user.ID, "Go Best Practices", "https://example.com/3", "Advanced Go techniques")

		retrieved, err := suite.PostRepo.Search("Go", 10, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 posts matching 'Go', got %d", len(retrieved))
		}

		for _, post := range retrieved {
			if post.Title != "Golang Tutorial" && post.Title != "Go Best Practices" {
				t.Errorf("Unexpected post in search results: %s", post.Title)
			}
		}
	})

	t.Run("case insensitive search", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		suite.CreateTestPost(user.ID, "Golang Tutorial", "https://example.com/1", "Learn Go programming")

		retrieved, err := suite.PostRepo.Search("golang", 10, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 1 {
			t.Errorf("Expected 1 post matching 'golang', got %d", len(retrieved))
		}

		if retrieved[0].Title != "Golang Tutorial" {
			t.Errorf("Expected 'Golang Tutorial', got %s", retrieved[0].Title)
		}
	})

	t.Run("no matching posts", func(t *testing.T) {
		suite.Reset()

		retrieved, err := suite.PostRepo.Search("nonexistent", 10, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 0 {
			t.Errorf("Expected 0 posts matching 'nonexistent', got %d", len(retrieved))
		}
	})

	t.Run("search with limit and offset", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser2", "test2@example.com", "password123")

		for i := range 5 {
			suite.CreateTestPost(user.ID,
				"Go Post "+strconv.Itoa(i),
				"https://example.com/go"+strconv.Itoa(i),
				"Go content "+strconv.Itoa(i))
		}

		retrieved, err := suite.PostRepo.Search("Go", 2, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 posts with limit, got %d", len(retrieved))
		}

		retrieved, err = suite.PostRepo.Search("Go", 2, 1)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 posts with offset, got %d", len(retrieved))
		}
	})
}

func TestPostRepository_CountByUserID(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("user with posts", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		suite.CreateTestPost(user.ID, "Post 1", "https://example.com/1", "Content 1")
		suite.CreateTestPost(user.ID, "Post 2", "https://example.com/2", "Content 2")
		suite.CreateTestPost(user.ID, "Post 3", "https://example.com/3", "Content 3")

		count, err := suite.PostRepo.CountByUserID(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if count != 3 {
			t.Errorf("Expected 3 posts, got %d", count)
		}
	})

	t.Run("user without posts", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		count, err := suite.PostRepo.CountByUserID(user.ID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if count != 0 {
			t.Errorf("Expected 0 posts, got %d", count)
		}
	})
}

func TestPostRepository_WithTx(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("transaction repository", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")

		tx := suite.DB.Begin()
		defer tx.Rollback()

		txPostRepo := suite.PostRepo.WithTx(tx)

		post := &database.Post{
			Title:    "Transaction Post",
			URL:      "https://example.com",
			Content:  "Test content",
			AuthorID: &user.ID,
		}

		err := txPostRepo.Create(post)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		retrieved, err := txPostRepo.GetByID(post.ID)
		if err != nil {
			t.Fatalf("Expected to find post in transaction, got %v", err)
		}

		if retrieved.Title != "Transaction Post" {
			t.Errorf("Expected title 'Transaction Post', got %s", retrieved.Title)
		}
	})
}

func TestPostRepository_GetPostsByDeletedUsers(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("posts by deleted users", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		err := suite.UserRepo.Delete(user.ID)
		if err != nil {
			t.Fatalf("Failed to delete user: %v", err)
		}

		posts, err := suite.PostRepo.GetPostsByDeletedUsers()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(posts) != 1 {
			t.Errorf("Expected 1 post by deleted user, got %d", len(posts))
		}

		if posts[0].ID != post.ID {
			t.Errorf("Expected post ID %d, got %d", post.ID, posts[0].ID)
		}
	})

	t.Run("no posts by deleted users", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		posts, err := suite.PostRepo.GetPostsByDeletedUsers()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(posts) != 0 {
			t.Errorf("Expected 0 posts by deleted users, got %d", len(posts))
		}
	})
}

func TestPostRepository_HardDeletePostsByDeletedUsers(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("hard delete posts by deleted users", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		post := suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		err := suite.UserRepo.Delete(user.ID)
		if err != nil {
			t.Fatalf("Failed to delete user: %v", err)
		}

		deletedCount, err := suite.PostRepo.HardDeletePostsByDeletedUsers()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if deletedCount != 1 {
			t.Errorf("Expected 1 post deleted, got %d", deletedCount)
		}

		_, err = suite.PostRepo.GetByID(post.ID)
		if err == nil {
			t.Error("Expected error for hard deleted post")
		}
		if err != gorm.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}
	})

	t.Run("no posts to delete", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		suite.CreateTestPost(user.ID, "Test Post", "https://example.com", "Test content")

		deletedCount, err := suite.PostRepo.HardDeletePostsByDeletedUsers()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if deletedCount != 0 {
			t.Errorf("Expected 0 posts deleted, got %d", deletedCount)
		}
	})
}

func TestPostRepository_HardDeleteAll(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("hard delete all posts", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser", "test@example.com", "password123")
		suite.CreateTestPost(user.ID, "Post 1", "https://example.com/1", "Content 1")
		suite.CreateTestPost(user.ID, "Post 2", "https://example.com/2", "Content 2")

		deletedCount, err := suite.PostRepo.HardDeleteAll()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if deletedCount != 2 {
			t.Errorf("Expected 2 posts deleted, got %d", deletedCount)
		}

		posts, err := suite.PostRepo.GetAll(10, 0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(posts) != 0 {
			t.Errorf("Expected 0 posts, got %d", len(posts))
		}
	})

	t.Run("hard delete with no posts", func(t *testing.T) {
		suite.Reset()

		deletedCount, err := suite.PostRepo.HardDeleteAll()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if deletedCount != 0 {
			t.Errorf("Expected 0 posts deleted, got %d", deletedCount)
		}
	})
}

func TestPostRepository_EdgeCases(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("empty URL", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser2", "test2@example.com", "password123")

		post := &database.Post{
			Title:    "Test Post",
			URL:      "",
			Content:  "Test content",
			AuthorID: &user.ID,
		}
		err := suite.PostRepo.Create(post)

		if err != nil {
			t.Errorf("Unexpected error for empty URL: %v", err)
		}
	})

	t.Run("invalid URL format", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser3", "test3@example.com", "password123")

		post := &database.Post{
			Title:    "Test Post",
			URL:      "not-a-url",
			Content:  "Test content",
			AuthorID: &user.ID,
		}
		err := suite.PostRepo.Create(post)

		if err != nil {
			t.Errorf("Unexpected error for invalid URL format: %v", err)
		}
	})

	t.Run("very long title", func(t *testing.T) {
		suite.Reset()

		user := suite.CreateTestUser("testuser4", "test4@example.com", "password123")

		longTitle := strings.Repeat("a", 300)
		post := &database.Post{
			Title:    longTitle,
			URL:      "https://example.com",
			Content:  "Test content",
			AuthorID: &user.ID,
		}
		err := suite.PostRepo.Create(post)

		if err != nil {
			t.Errorf("Unexpected error for long title: %v", err)
		}
	})
}
