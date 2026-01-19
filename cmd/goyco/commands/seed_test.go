package commands

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"goyco/internal/database"
	"goyco/internal/repositories"
	"goyco/internal/testutils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	seedRandSource *rand.Rand
	seedRandOnce   sync.Once
)

const testPasswordHash = "test_password_hash"

func initSeedRand() {
	seedRandOnce.Do(func() {
		seed := time.Now().UnixNano()
		seedRandSource = rand.New(rand.NewSource(seed))
	})
}

func TestSeedCommand(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	err = db.AutoMigrate(&database.User{}, &database.Post{}, &database.Vote{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		userRepo := repositories.NewUserRepository(db).WithTx(tx)
		postRepo := repositories.NewPostRepository(db).WithTx(tx)
		voteRepo := repositories.NewVoteRepository(db).WithTx(tx)
		return seedDatabase(userRepo, postRepo, voteRepo, []string{"--users", "2", "--posts", "5", "--votes-per-post", "3"})
	})
	if err != nil {
		t.Fatalf("Failed to seed database: %v", err)
	}

	userRepo := repositories.NewUserRepository(db)
	postRepo := repositories.NewPostRepository(db)
	voteRepo := repositories.NewVoteRepository(db)

	users, err := userRepo.GetAll(100, 0)
	if err != nil {
		t.Fatalf("Failed to get users: %v", err)
	}

	seedUserCount := 0
	var seedUser *database.User
	regularUserCount := 0
	for idx := range users {
		if users[idx].Username == seedUsername {
			seedUserCount++
			seedUser = &users[idx]
		} else if strings.HasPrefix(users[idx].Username, "user_") {
			regularUserCount++
		}
	}

	if seedUserCount != 1 {
		t.Errorf("Expected 1 seed user, got %d", seedUserCount)
	}

	if seedUser == nil {
		t.Fatal("Expected seed user to be created")
	}

	if seedUser.Username != seedUsername {
		t.Errorf("Expected username to be %q, got '%s'", seedUsername, seedUser.Username)
	}

	if seedUser.Email != seedEmail {
		t.Errorf("Expected email to be %q, got '%s'", seedEmail, seedUser.Email)
	}

	if !seedUser.EmailVerified {
		t.Error("Expected seed user to be email verified")
	}

	if regularUserCount != 2 {
		t.Errorf("Expected 2 regular users, got %d", regularUserCount)
	}

	posts, err := postRepo.GetAll(100, 0)
	if err != nil {
		t.Fatalf("Failed to get posts: %v", err)
	}

	if len(posts) != 5 {
		t.Errorf("Expected 5 posts, got %d", len(posts))
	}

	for idx, post := range posts {
		if post.Title == "" {
			t.Errorf("Post %d has empty title", idx)
		}
		if post.URL == "" {
			t.Errorf("Post %d has empty URL", idx)
		}
		if post.AuthorID == nil || *post.AuthorID != seedUser.ID {
			t.Errorf("Post %d has wrong author ID: expected %d, got %v", idx, seedUser.ID, post.AuthorID)
		}

		expectedScore := post.UpVotes - post.DownVotes
		if post.Score != expectedScore {
			t.Errorf("Post %d has incorrect score: expected %d, got %d", idx, expectedScore, post.Score)
		}
	}

	voteCount, err := voteRepo.Count()
	if err != nil {
		t.Fatalf("Failed to count votes: %v", err)
	}

	if voteCount == 0 {
		t.Error("Expected some votes to be created")
	}

	for _, post := range posts {
		postVotes, err := voteRepo.GetByPostID(post.ID)
		if err != nil {
			t.Errorf("Failed to get votes for post %d: %v", post.ID, err)
			continue
		}

		for _, vote := range postVotes {
			if vote.PostID != post.ID {
				t.Errorf("Vote has wrong post ID: expected %d, got %d", post.ID, vote.PostID)
			}
			if vote.UserID == nil {
				t.Error("Vote has nil user ID")
			}
		}
	}
}

func TestGenerateRandomPath(t *testing.T) {
	const articlePathPrefix = "/article/"
	initSeedRand()
	pathLength := seedRandSource.Intn(20)
	path := articlePathPrefix

	for idx := 0; idx < pathLength+5; idx++ {
		randomChar := seedRandSource.Intn(26)
		path += string(rune('a' + randomChar))
	}

	if path == "" {
		t.Error("Generated path should not be empty")
	}

	if len(path) < 8 {
		t.Errorf("Generated path too short: %s", path)
	}

	initSeedRand()
	secondPathLength := seedRandSource.Intn(20)
	var secondPath strings.Builder
	secondPath.WriteString(articlePathPrefix)
	for idx := 0; idx < secondPathLength+5; idx++ {
		randomChar := seedRandSource.Intn(26)
		secondPath.WriteString(string(rune('a' + randomChar)))
	}

	if path == secondPath.String() {
		t.Error("Generated paths should be different")
	}
}

func TestSeedDatabaseFlagParsing(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	postRepo := testutils.NewMockPostRepository()
	voteRepo := testutils.NewMockVoteRepository()

	t.Run("invalid posts type", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--posts", "abc"})

		if err == nil {
			t.Error("expected error for invalid posts type")
		}
	})

	t.Run("invalid users type", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--users", "xyz"})

		if err == nil {
			t.Error("expected error for invalid users type")
		}
	})

	t.Run("invalid votes-per-post type", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--votes-per-post", "invalid"})

		if err == nil {
			t.Error("expected error for invalid votes-per-post type")
		}
	})

	t.Run("unknown flag", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--unknown-flag"})

		if err == nil {
			t.Error("expected error for unknown flag")
		}
	})

	t.Run("missing posts value", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--posts"})

		if err == nil {
			t.Error("expected error for missing posts value")
		}
	})

	t.Run("missing users value", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--users"})

		if err == nil {
			t.Error("expected error for missing users value")
		}
	})

	t.Run("missing votes-per-post value", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--votes-per-post"})

		if err == nil {
			t.Error("expected error for missing votes-per-post value")
		}
	})

	t.Run("negative users value is clamped", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--users", "-1", "--posts", "1"})

		if err != nil {
			t.Errorf("negative users should be clamped, not rejected. Got error: %v", err)
		}
	})

	t.Run("negative posts value is clamped", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--posts", "-5"})

		if err != nil {
			t.Errorf("negative posts should be clamped, not rejected. Got error: %v", err)
		}
	})

	t.Run("zero posts value is clamped", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--posts", "0"})

		if err != nil {
			t.Errorf("zero posts should be clamped, not rejected. Got error: %v", err)
		}
	})

	t.Run("negative votes-per-post value is clamped", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--votes-per-post", "-10", "--posts", "1"})

		if err != nil {
			t.Errorf("negative votes-per-post should be clamped, not rejected. Got error: %v", err)
		}
	})

	t.Run("zero users value is valid", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--users", "0", "--posts", "1"})

		if err != nil {
			t.Errorf("zero users should be valid, got error: %v", err)
		}
	})

	t.Run("zero votes-per-post value is valid", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--votes-per-post", "0", "--posts", "1"})

		if err != nil {
			t.Errorf("zero votes-per-post should be valid, got error: %v", err)
		}
	})

	t.Run("help flag returns no error", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--help"})

		if err != nil {
			t.Errorf("help flag should return no error, got: %v", err)
		}
	})

	t.Run("short help flag returns no error", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"-h"})

		if err != nil {
			t.Errorf("short help flag should return no error, got: %v", err)
		}
	})
}

func TestSeedCommandIdempotency(t *testing.T) {
	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	err = db.AutoMigrate(&database.User{}, &database.Post{}, &database.Vote{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	userRepo := repositories.NewUserRepository(db)
	postRepo := repositories.NewPostRepository(db)
	voteRepo := repositories.NewVoteRepository(db)

	t.Run("first run creates seed user", func(t *testing.T) {
		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--users", "1", "--posts", "2"})
		if err != nil {
			t.Fatalf("First seed run failed: %v", err)
		}

		users, err := userRepo.GetAll(100, 0)
		if err != nil {
			t.Fatalf("Failed to get users: %v", err)
		}

		seedUserCount := 0
		for _, user := range users {
			if user.Username == seedUsername {
				seedUserCount++
			}
		}

		if seedUserCount != 1 {
			t.Errorf("Expected exactly 1 seed user, got %d", seedUserCount)
		}
	})

	t.Run("second run reuses seed user", func(t *testing.T) {
		usersBefore, _ := userRepo.GetAll(100, 0)
		seedUserBefore := findSeedUser(usersBefore)

		if seedUserBefore == nil {
			t.Fatal("No seed user found before second run")
		}

		err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--users", "1", "--posts", "2"})
		if err != nil {
			t.Fatalf("Second seed run failed: %v", err)
		}

		usersAfter, _ := userRepo.GetAll(100, 0)
		seedUserAfter := findSeedUser(usersAfter)

		if seedUserAfter == nil {
			t.Fatal("Seed user not found after second run")
		}

		if seedUserBefore.ID != seedUserAfter.ID {
			t.Errorf("Expected seed user to be reused (ID %d), but got different user (ID %d)", seedUserBefore.ID, seedUserAfter.ID)
		}
	})

	t.Run("database remains consistent after multiple runs", func(t *testing.T) {
		for idx := range 2 {
			err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--users", "0", "--posts", "1"})
			if err != nil {
				t.Fatalf("Seed run %d failed: %v", idx+1, err)
			}
		}

		users, _ := userRepo.GetAll(100, 0)
		posts, _ := postRepo.GetAll(100, 0)

		for _, post := range posts {
			if post.AuthorID == nil {
				t.Errorf("Post %d has no author", post.ID)
				continue
			}

			authorExists := false
			for _, user := range users {
				if user.ID == *post.AuthorID {
					authorExists = true
					break
				}
			}

			if !authorExists {
				t.Errorf("Post %d has invalid author ID %d", post.ID, *post.AuthorID)
			}

			votes, _ := voteRepo.GetByPostID(post.ID)
			for _, vote := range votes {
				if vote.UserID != nil {
					userExists := false
					for _, user := range users {
						if user.ID == *vote.UserID {
							userExists = true
							break
						}
					}
					if !userExists {
						t.Errorf("Vote %d has invalid user ID %d", vote.ID, *vote.UserID)
					}
				}
			}
		}
	})
}

func findSeedUser(users []database.User) *database.User {
	for idx := range users {
		if users[idx].Username == seedUsername {
			return &users[idx]
		}
	}
	return nil
}

func TestSeedCommandTransactionRollback(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	err = db.AutoMigrate(&database.User{}, &database.Post{}, &database.Vote{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	userRepo := repositories.NewUserRepository(db)
	postRepo := repositories.NewPostRepository(db)
	voteRepo := repositories.NewVoteRepository(db)

	t.Run("transaction rolls back on failure", func(t *testing.T) {
		initialUserCount, _ := userRepo.Count()
		initialPostCount, _ := postRepo.Count()
		initialVoteCount, _ := voteRepo.Count()

		err := db.Transaction(func(tx *gorm.DB) error {
			txUserRepo := userRepo.WithTx(tx)
			txPostRepo := postRepo.WithTx(tx)
			txVoteRepo := voteRepo.WithTx(tx)

			err := seedDatabase(txUserRepo, txPostRepo, txVoteRepo, []string{"--users", "2", "--posts", "3"})
			if err != nil {
				return err
			}

			return fmt.Errorf("simulated failure")
		})

		if err == nil {
			t.Fatal("Expected transaction to fail")
		}

		finalUserCount, _ := userRepo.Count()
		finalPostCount, _ := postRepo.Count()
		finalVoteCount, _ := voteRepo.Count()

		if finalUserCount != initialUserCount {
			t.Errorf("Expected user count to remain %d after rollback, got %d", initialUserCount, finalUserCount)
		}
		if finalPostCount != initialPostCount {
			t.Errorf("Expected post count to remain %d after rollback, got %d", initialPostCount, finalPostCount)
		}
		if finalVoteCount != initialVoteCount {
			t.Errorf("Expected vote count to remain %d after rollback, got %d", initialVoteCount, finalVoteCount)
		}
	})

	t.Run("transaction commits on success", func(t *testing.T) {
		initialUserCount, _ := userRepo.Count()
		initialPostCount, _ := postRepo.Count()

		err := db.Transaction(func(tx *gorm.DB) error {
			txUserRepo := userRepo.WithTx(tx)
			txPostRepo := postRepo.WithTx(tx)
			txVoteRepo := voteRepo.WithTx(tx)

			return seedDatabase(txUserRepo, txPostRepo, txVoteRepo, []string{"--users", "1", "--posts", "1"})
		})

		if err != nil {
			t.Fatalf("Expected transaction to succeed, got error: %v", err)
		}

		finalUserCount, _ := userRepo.Count()
		finalPostCount, _ := postRepo.Count()

		expectedUsers := initialUserCount + 2
		expectedPosts := initialPostCount + 1

		if finalUserCount < expectedUsers {
			t.Errorf("Expected at least %d users after commit, got %d", expectedUsers, finalUserCount)
		}
		if finalPostCount < expectedPosts {
			t.Errorf("Expected at least %d posts after commit, got %d", expectedPosts, finalPostCount)
		}
	})
}

func TestEnsureSeedUser(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	if err := db.AutoMigrate(&database.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	userRepo := repositories.NewUserRepository(db)
	passwordHash := testPasswordHash

	firstUser, err := ensureSeedUser(userRepo, passwordHash)
	if err != nil {
		t.Fatalf("Failed to create seed user: %v", err)
	}

	if firstUser.Username != seedUsername || firstUser.Email != seedEmail || firstUser.Password != passwordHash || !firstUser.EmailVerified {
		t.Errorf("Invalid seed user: username=%s, email=%s, password matches=%v, emailVerified=%v",
			firstUser.Username, firstUser.Email, firstUser.Password == passwordHash, firstUser.EmailVerified)
	}

	secondUser, err := ensureSeedUser(userRepo, "different_password_hash")
	if err != nil {
		t.Fatalf("Failed to reuse seed user: %v", err)
	}

	if firstUser.ID != secondUser.ID {
		t.Errorf("Expected same user to be reused (ID %d), got different user (ID %d)", firstUser.ID, secondUser.ID)
	}

	for idx := range 3 {
		if _, err := ensureSeedUser(userRepo, passwordHash); err != nil {
			t.Fatalf("Call %d failed: %v", idx+1, err)
		}
	}

	users, err := userRepo.GetAll(100, 0)
	if err != nil {
		t.Fatalf("Failed to get users: %v", err)
	}

	seedUserCount := 0
	for _, user := range users {
		if user.Username == seedUsername {
			seedUserCount++
		}
	}

	if seedUserCount != 1 {
		t.Errorf("Expected exactly 1 seed user, got %d", seedUserCount)
	}
}

func TestEnsureSeedUser_HandlesDatabaseErrors(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	passwordHash := testPasswordHash

	dbError := fmt.Errorf("database connection failed")

	userRepo.SetGetByUsernameError(dbError)

	_, err := ensureSeedUser(userRepo, passwordHash)
	if err == nil {
		t.Fatal("Expected error when GetByUsername returns database error")
	}

	if !strings.Contains(err.Error(), "failed to check if seed user exists") {
		t.Errorf("Expected error message about checking seed user, got: %v", err)
	}

	if !strings.Contains(err.Error(), dbError.Error()) {
		t.Errorf("Expected error to wrap original database error, got: %v", err)
	}
}
