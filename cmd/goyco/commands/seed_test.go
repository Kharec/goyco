package commands

import (
	"fmt"
	"strings"
	"testing"

	"goyco/internal/database"
	"goyco/internal/repositories"
	"goyco/internal/testutils"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSeedCommand(t *testing.T) {
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

	seedPasswordHash, err := bcrypt.GenerateFromPassword([]byte("seed-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash seed password: %v", err)
	}

	seedUser, err := ensureSeedUser(userRepo, string(seedPasswordHash))
	if err != nil {
		t.Fatalf("Failed to ensure seed user: %v", err)
	}

	if !strings.HasPrefix(seedUser.Username, "seed_admin_") {
		t.Errorf("Expected username to start with 'seed_admin_', got '%s'", seedUser.Username)
	}

	if !strings.HasPrefix(seedUser.Email, "seed_admin_") || !strings.HasSuffix(seedUser.Email, "@goyco.local") {
		t.Errorf("Expected email to start with 'seed_admin_' and end with '@goyco.local', got '%s'", seedUser.Email)
	}

	if !seedUser.EmailVerified {
		t.Error("Expected seed user to be email verified")
	}

	userPasswordHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash user password: %v", err)
	}

	users, err := createRandomUsers(userRepo, 2, string(userPasswordHash))
	if err != nil {
		t.Fatalf("Failed to create random users: %v", err)
	}

	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}

	posts, err := createRandomPosts(postRepo, seedUser.ID, 5)
	if err != nil {
		t.Fatalf("Failed to create random posts: %v", err)
	}

	if len(posts) != 5 {
		t.Errorf("Expected 5 posts, got %d", len(posts))
	}

	for i, post := range posts {
		if post.Title == "" {
			t.Errorf("Post %d has empty title", i)
		}
		if post.URL == "" {
			t.Errorf("Post %d has empty URL", i)
		}
		if post.AuthorID == nil || *post.AuthorID != seedUser.ID {
			t.Errorf("Post %d has wrong author ID: expected %d, got %v", i, seedUser.ID, post.AuthorID)
		}
	}

	allUsers := append([]database.User{*seedUser}, users...)
	votes, err := createRandomVotes(voteRepo, allUsers, posts, 3)
	if err != nil {
		t.Fatalf("Failed to create random votes: %v", err)
	}

	if votes == 0 {
		t.Error("Expected some votes to be created")
	}

	err = updatePostScores(postRepo, voteRepo, posts)
	if err != nil {
		t.Fatalf("Failed to update post scores: %v", err)
	}

	for i, post := range posts {
		updatedPost, err := postRepo.GetByID(post.ID)
		if err != nil {
			t.Errorf("Failed to get updated post %d: %v", i, err)
			continue
		}

		expectedScore := updatedPost.UpVotes - updatedPost.DownVotes
		if updatedPost.Score != expectedScore {
			t.Errorf("Post %d has incorrect score: expected %d, got %d", i, expectedScore, updatedPost.Score)
		}
	}
}

func TestGenerateRandomPath(t *testing.T) {
	path := generateRandomPath()

	if path == "" {
		t.Error("Generated path should not be empty")
	}

	if len(path) < 8 {
		t.Errorf("Generated path too short: %s", path)
	}

	secondPath := generateRandomPath()
	if path == secondPath {
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
			if strings.HasPrefix(user.Username, "seed_admin_") {
				seedUserCount++
			}
		}

		if seedUserCount < 1 {
			t.Errorf("Expected at least 1 seed user, got %d", seedUserCount)
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
		for i := range 2 {
			err := seedDatabase(userRepo, postRepo, voteRepo, []string{"--users", "0", "--posts", "1"})
			if err != nil {
				t.Fatalf("Seed run %d failed: %v", i+1, err)
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
	for i := range users {
		if strings.HasPrefix(users[i].Username, "seed_admin_") {
			return &users[i]
		}
	}
	return nil
}
