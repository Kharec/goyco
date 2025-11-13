package integration

import (
	"fmt"
	"testing"

	"goyco/internal/database"
	"goyco/internal/repositories"

	"golang.org/x/crypto/bcrypt"
)

func TestIntegration_Repositories(t *testing.T) {
	suite := repositories.NewTestSuite(t)

	t.Run("User_Lifecycle", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:      "lifecycle_user",
			Email:         "lifecycle@example.com",
			Password:      hashPassword("SecurePass123!"),
			EmailVerified: false,
		}
		err := suite.UserRepo.Create(user)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		retrieved, err := suite.UserRepo.GetByID(user.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve user: %v", err)
		}
		if retrieved.Username != "lifecycle_user" {
			t.Errorf("Expected username 'lifecycle_user', got '%s'", retrieved.Username)
		}

		retrieved.EmailVerified = true
		err = suite.UserRepo.Update(retrieved)
		if err != nil {
			t.Fatalf("Failed to update user: %v", err)
		}

		updated, err := suite.UserRepo.GetByID(user.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve updated user: %v", err)
		}
		if !updated.EmailVerified {
			t.Error("Expected email to be verified")
		}

		err = suite.UserRepo.Delete(user.ID)
		if err != nil {
			t.Fatalf("Failed to delete user: %v", err)
		}

		_, err = suite.UserRepo.GetByID(user.ID)
		if err == nil {
			t.Error("Expected user to be deleted")
		}
	})

	t.Run("Post_Lifecycle", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:      "post_author",
			Email:         "author@example.com",
			Password:      hashPassword("SecurePass123!"),
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		post := &database.Post{
			Title:    "Integration Test Post",
			URL:      "https://example.com/integration-test",
			Content:  "This is a comprehensive integration test post",
			AuthorID: &user.ID,
		}
		err = suite.PostRepo.Create(post)
		if err != nil {
			t.Fatalf("Failed to create post: %v", err)
		}

		retrieved, err := suite.PostRepo.GetByID(post.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve post: %v", err)
		}
		if retrieved.Title != "Integration Test Post" {
			t.Errorf("Expected title 'Integration Test Post', got '%s'", retrieved.Title)
		}

		retrieved.Title = "Updated Integration Test Post"
		err = suite.PostRepo.Update(retrieved)
		if err != nil {
			t.Fatalf("Failed to update post: %v", err)
		}

		updated, err := suite.PostRepo.GetByID(post.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve updated post: %v", err)
		}
		if updated.Title != "Updated Integration Test Post" {
			t.Errorf("Expected updated title, got '%s'", updated.Title)
		}

		err = suite.PostRepo.Delete(post.ID)
		if err != nil {
			t.Fatalf("Failed to delete post: %v", err)
		}
	})

	t.Run("Vote_Lifecycle", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:      "voter",
			Email:         "voter@example.com",
			Password:      hashPassword("SecurePass123!"),
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		post := &database.Post{
			Title:    "Vote Test Post",
			URL:      "https://example.com/vote-test",
			Content:  "Vote test content",
			AuthorID: &user.ID,
		}
		err = suite.PostRepo.Create(post)
		if err != nil {
			t.Fatalf("Failed to create post: %v", err)
		}

		vote := &database.Vote{
			UserID: &user.ID,
			PostID: post.ID,
			Type:   database.VoteUp,
		}
		err = suite.VoteRepo.Create(vote)
		if err != nil {
			t.Fatalf("Failed to create vote: %v", err)
		}

		retrieved, err := suite.VoteRepo.GetByID(vote.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve vote: %v", err)
		}
		if retrieved.Type != database.VoteUp {
			t.Errorf("Expected vote type %v, got %v", database.VoteUp, retrieved.Type)
		}

		retrieved.Type = database.VoteDown
		err = suite.VoteRepo.Update(retrieved)
		if err != nil {
			t.Fatalf("Failed to update vote: %v", err)
		}

		updated, err := suite.VoteRepo.GetByID(vote.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve updated vote: %v", err)
		}
		if updated.Type != database.VoteDown {
			t.Errorf("Expected updated vote type, got %v", updated.Type)
		}

		err = suite.VoteRepo.Delete(vote.ID)
		if err != nil {
			t.Fatalf("Failed to delete vote: %v", err)
		}
	})

	t.Run("Security_SQL_Injection_Protection", func(t *testing.T) {
		suite.Reset()

		initialCount, err := suite.UserRepo.Count()
		if err != nil {
			t.Fatalf("Failed to get initial user count: %v", err)
		}

		maliciousUser := &database.User{
			Username:      "'; DROP TABLE users; --",
			Email:         "malicious@example.com",
			Password:      hashPassword("password"),
			EmailVerified: true,
		}
		err = suite.UserRepo.Create(maliciousUser)
		if err != nil {
			t.Fatalf("Failed to create user with malicious username: %v", err)
		}

		finalCount, err := suite.UserRepo.Count()
		if err != nil {
			t.Fatalf("Failed to get final user count: %v", err)
		}
		if finalCount != initialCount+1 {
			t.Errorf("Expected user count to increase by 1, got %d -> %d", initialCount, finalCount)
		}

		retrieved, err := suite.UserRepo.GetByUsername("'; DROP TABLE users; --")
		if err != nil {
			t.Fatalf("Failed to retrieve user with malicious username: %v", err)
		}
		if retrieved.Username != "'; DROP TABLE users; --" {
			t.Errorf("Expected malicious username to be stored as-is")
		}

		users, err := suite.UserRepo.GetAll(10, 0)
		if err != nil {
			t.Fatalf("Failed to get users after SQL injection test: %v", err)
		}
		if len(users) == 0 {
			t.Error("Users table appears to have been dropped")
		}

		var tableName string
		err = suite.DB.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name='users'").Scan(&tableName).Error
		if err != nil || tableName != "users" {
			t.Error("Users table should still exist after SQL injection attempt")
		}
	})

	t.Run("Security_Input_Validation", func(t *testing.T) {
		suite.Reset()

		longString := string(make([]byte, 10000))
		for i := range longString {
			longString = longString[:i] + "a" + longString[i+1:]
		}

		user := &database.User{
			Username:      longString,
			Email:         "long@example.com",
			Password:      hashPassword("password"),
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user)
		if err != nil {
			t.Fatalf("Failed to create user with long username: %v", err)
		}

		specialUser := &database.User{
			Username:      "user<script>alert('xss')</script>",
			Email:         "special@example.com",
			Password:      hashPassword("password"),
			EmailVerified: true,
		}
		err = suite.UserRepo.Create(specialUser)
		if err != nil {
			t.Fatalf("Failed to create user with special characters: %v", err)
		}
	})

	t.Run("Data_Consistency_Cross_Repository", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:      "consistency_user",
			Email:         "consistency@example.com",
			Password:      hashPassword("SecurePass123!"),
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		post := &database.Post{
			Title:    "Consistency Test Post",
			URL:      "https://example.com/consistency",
			Content:  "Consistency test content",
			AuthorID: &user.ID,
		}
		err = suite.PostRepo.Create(post)
		if err != nil {
			t.Fatalf("Failed to create post: %v", err)
		}

		voters := make([]*database.User, 5)
		for i := 0; i < 5; i++ {
			voter := &database.User{
				Username:      fmt.Sprintf("voter_%d", i),
				Email:         fmt.Sprintf("voter%d@example.com", i),
				Password:      hashPassword("SecurePass123!"),
				EmailVerified: true,
			}
			err := suite.UserRepo.Create(voter)
			if err != nil {
				t.Fatalf("Failed to create voter %d: %v", i, err)
			}
			voters[i] = voter
		}

		for i, voter := range voters {
			voteType := database.VoteUp
			if i%2 == 0 {
				voteType = database.VoteDown
			}

			vote := &database.Vote{
				UserID: &voter.ID,
				PostID: post.ID,
				Type:   voteType,
			}
			err := suite.VoteRepo.Create(vote)
			if err != nil {
				t.Fatalf("Failed to create vote %d: %v", i, err)
			}
		}

		votes, err := suite.VoteRepo.GetByPostID(post.ID)
		if err != nil {
			t.Fatalf("Failed to get votes: %v", err)
		}

		var upVotes, downVotes int64
		for _, vote := range votes {
			switch vote.Type {
			case database.VoteUp:
				upVotes++
			case database.VoteDown:
				downVotes++
			}
		}

		expectedScore := int(upVotes - downVotes)
		post.Score = expectedScore
		err = suite.PostRepo.Update(post)
		if err != nil {
			t.Fatalf("Failed to update post score: %v", err)
		}

		updatedPost, err := suite.PostRepo.GetByID(post.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve updated post: %v", err)
		}
		if updatedPost.Score != expectedScore {
			t.Errorf("Expected post score %d, got %d", expectedScore, updatedPost.Score)
		}
	})

	t.Run("Edge_Cases_Invalid_Data", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:      "",
			Email:         "empty@example.com",
			Password:      hashPassword("password"),
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user)
		if err == nil {
			t.Error("Expected error for empty username")
		}

		user = &database.User{
			Username:      "invalid_email",
			Email:         "not-an-email",
			Password:      hashPassword("password"),
			EmailVerified: true,
		}
		err = suite.UserRepo.Create(user)
		if err == nil {
			t.Error("Expected error for invalid email format")
		}

		user1 := &database.User{
			Username:      "duplicate",
			Email:         "duplicate1@example.com",
			Password:      hashPassword("SecurePass123!"),
			EmailVerified: true,
		}
		err = suite.UserRepo.Create(user1)
		if err != nil {
			t.Fatalf("Failed to create first user: %v", err)
		}
		user2 := &database.User{
			Username:      "duplicate",
			Email:         "duplicate2@example.com",
			Password:      hashPassword("password"),
			EmailVerified: true,
		}
		err = suite.UserRepo.Create(user2)
		if err == nil {
			t.Error("Expected error for duplicate username")
		}

		user3 := &database.User{
			Username:      "duplicate_email",
			Email:         "duplicate1@example.com",
			Password:      hashPassword("password"),
			EmailVerified: true,
		}
		err = suite.UserRepo.Create(user3)
		if err == nil {
			t.Error("Expected error for duplicate email")
		}
	})

	t.Run("Edge_Cases_Concurrent_Conflicts", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:      "conflict_user",
			Email:         "conflict@example.com",
			Password:      hashPassword("SecurePass123!"),
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		post := &database.Post{
			Title:    "Conflict Test Post",
			URL:      "https://example.com/conflict",
			Content:  "Conflict test content",
			AuthorID: &user.ID,
		}
		err = suite.PostRepo.Create(post)
		if err != nil {
			t.Fatalf("Failed to create post: %v", err)
		}

		vote1 := &database.Vote{
			UserID: &user.ID,
			PostID: post.ID,
			Type:   database.VoteUp,
		}
		err = suite.VoteRepo.Create(vote1)
		if err != nil {
			t.Fatalf("Failed to create first vote: %v", err)
		}

		vote2 := &database.Vote{
			UserID: &user.ID,
			PostID: post.ID,
			Type:   database.VoteDown,
		}
		err = suite.VoteRepo.Create(vote2)
		if err == nil {
			t.Error("Expected error for duplicate vote")
		}
	})

	t.Run("Transaction_Rollback_On_Error", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:      "transaction_user",
			Email:         "transaction@example.com",
			Password:      hashPassword("SecurePass123!"),
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		tx := suite.DB.Begin()
		defer tx.Rollback()

		post := &database.Post{
			Title:    "Transaction Test Post",
			URL:      "https://example.com/transaction",
			Content:  "This is a transaction test post",
			AuthorID: &user.ID,
		}
		err = tx.Create(post).Error
		if err != nil {
			t.Fatalf("Failed to create post in transaction: %v", err)
		}

		var postInTx database.Post
		err = tx.First(&postInTx, post.ID).Error
		if err != nil {
			t.Fatalf("Failed to retrieve post in transaction: %v", err)
		}

		tx.Rollback()

		var postAfterRollback database.Post
		err = suite.DB.First(&postAfterRollback, post.ID).Error
		if err == nil {
			t.Error("Expected post to not exist after transaction rollback")
		}
	})

	t.Run("Cascading_Delete_User_With_Posts", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:      "cascade_user",
			Email:         "cascade@example.com",
			Password:      hashPassword("SecurePass123!"),
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		post1 := &database.Post{
			Title:    "Post 1",
			URL:      "https://example.com/1",
			Content:  "Content 1",
			AuthorID: &user.ID,
		}
		post2 := &database.Post{
			Title:    "Post 2",
			URL:      "https://example.com/2",
			Content:  "Content 2",
			AuthorID: &user.ID,
		}
		err = suite.PostRepo.Create(post1)
		if err != nil {
			t.Fatalf("Failed to create post1: %v", err)
		}
		err = suite.PostRepo.Create(post2)
		if err != nil {
			t.Fatalf("Failed to create post2: %v", err)
		}

		vote := &database.Vote{
			UserID: &user.ID,
			PostID: post1.ID,
			Type:   database.VoteUp,
		}
		err = suite.VoteRepo.Create(vote)
		if err != nil {
			t.Fatalf("Failed to create vote: %v", err)
		}

		err = suite.UserRepo.Delete(user.ID)
		if err != nil {
			t.Fatalf("Failed to delete user: %v", err)
		}

		_, err = suite.UserRepo.GetByID(user.ID)
		if err == nil {
			t.Error("Expected user to be deleted")
		}

		posts, err := suite.PostRepo.GetByUserID(user.ID, 10, 0)
		if err != nil {
			t.Fatalf("Failed to get posts: %v", err)
		}
		if len(posts) > 0 {
			t.Errorf("Expected posts to be deleted or orphaned, found %d posts", len(posts))
		}
	})

	t.Run("Search_Functionality", func(t *testing.T) {
		suite.Reset()

		user := &database.User{
			Username:      "search_user",
			Email:         "search@example.com",
			Password:      hashPassword("SecurePass123!"),
			EmailVerified: true,
		}
		err := suite.UserRepo.Create(user)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		posts := []struct {
			title   string
			content string
		}{
			{"Go Programming", "This post is about Go programming language"},
			{"Python Tutorial", "Learn Python programming with this tutorial"},
			{"Database Design", "Best practices for database design"},
			{"Web Development", "Modern web development techniques"},
		}

		for i, p := range posts {
			post := &database.Post{
				Title:    p.title,
				URL:      fmt.Sprintf("https://example.com/post-%d", i),
				Content:  p.content,
				AuthorID: &user.ID,
			}
			err := suite.PostRepo.Create(post)
			if err != nil {
				t.Fatalf("Failed to create post %d: %v", i, err)
			}
		}

		results, err := suite.PostRepo.Search("Go", 10, 0)
		if err != nil {
			t.Fatalf("Failed to search posts: %v", err)
		}
		if len(results) == 0 {
			t.Error("Expected to find posts containing 'Go'")
		}

		found := false
		for _, result := range results {
			if result.Title == "Go Programming" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find 'Go Programming' post in search results")
		}
	})
}

func hashPassword(password string) string {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("Failed to hash password: %v", err))
	}
	return string(hashed)
}
