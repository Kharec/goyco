package commands

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"goyco/internal/database"
	"goyco/internal/testutils"
)

func TestHandlePruneCommand(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "help requested",
			args:    []string{"help"},
			wantErr: false,
		},
		{
			name:    "missing subcommand",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "unknown subcommand",
			args:    []string{"unknown"},
			wantErr: true,
		},
		{
			name:    "posts subcommand",
			args:    []string{"posts", "--dry-run"},
			wantErr: false,
		},
		{
			name:    "all subcommand",
			args:    []string{"all", "--dry-run"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testutils.NewTestConfig()
			userRepo := testutils.NewMockUserRepository()
			postRepo := testutils.NewMockPostRepository()
			err := runPruneCommand(cfg, userRepo, postRepo, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("runPruneCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunPruneCommand(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "help requested",
			args:    []string{"help"},
			wantErr: false,
		},
		{
			name:    "missing subcommand",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "unknown subcommand",
			args:    []string{"unknown"},
			wantErr: true,
		},
		{
			name:    "posts subcommand",
			args:    []string{"posts", "--dry-run"},
			wantErr: false,
		},
		{
			name:    "all subcommand",
			args:    []string{"all", "--dry-run"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testutils.NewTestConfig()
			userRepo := testutils.NewMockUserRepository()
			postRepo := testutils.NewMockPostRepository()
			err := runPruneCommand(cfg, userRepo, postRepo, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("runPruneCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPrunePosts(t *testing.T) {
	postRepo := testutils.NewMockPostRepository()

	err := prunePosts(postRepo, []string{"--dry-run"})
	if err != nil {
		t.Errorf("prunePosts() with dry-run error = %v", err)
	}

	t.Run("prunePosts with JSON output", func(t *testing.T) {
		SetJSONOutput(true)
		defer SetJSONOutput(false)

		err := prunePosts(postRepo, []string{"--dry-run"})
		if err != nil {
			t.Errorf("prunePosts() with dry-run and JSON output error = %v", err)
		}
	})

	post1 := database.Post{
		ID:       1,
		Title:    "Post by deleted user 1",
		URL:      "http://example.com/1",
		AuthorID: nil,
	}
	post2 := database.Post{
		ID:       2,
		Title:    "Post by deleted user 2",
		URL:      "http://example.com/2",
		AuthorID: nil,
	}
	postRepo.Posts[post1.ID] = &post1
	postRepo.Posts[post2.ID] = &post2

	postRepo.GetPostsByDeletedUsersFunc = func() ([]database.Post, error) {
		return []database.Post{post1, post2}, nil
	}
	postRepo.HardDeletePostsByDeletedUsersFunc = func() (int64, error) {
		delete(postRepo.Posts, post1.ID)
		delete(postRepo.Posts, post2.ID)
		return 2, nil
	}

	err = prunePosts(postRepo, []string{"--dry-run"})
	if err != nil {
		t.Errorf("prunePosts() with dry-run error = %v", err)
	}

	t.Run("prunePosts with JSON output and mock data", func(t *testing.T) {
		SetJSONOutput(true)
		defer SetJSONOutput(false)

		err := prunePosts(postRepo, []string{"--dry-run"})
		if err != nil {
			t.Errorf("prunePosts() with dry-run and JSON output error = %v", err)
		}
	})
}

func TestPruneAll(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	postRepo := testutils.NewMockPostRepository()

	err := pruneAll(userRepo, postRepo, []string{"--dry-run"})
	if err != nil {
		t.Errorf("pruneAll() with dry-run error = %v", err)
	}

	user1 := database.User{ID: 1, Username: "user1", Email: "user1@example.com"}
	user2 := database.User{ID: 2, Username: "user2", Email: "user2@example.com"}
	post1 := database.Post{ID: 1, Title: "Post 1", URL: "http://example.com/1", AuthorID: &user1.ID}
	post2 := database.Post{ID: 2, Title: "Post 2", URL: "http://example.com/2", AuthorID: &user2.ID}

	userRepo.Users[user1.ID] = &user1
	userRepo.Users[user2.ID] = &user2
	postRepo.Posts[post1.ID] = &post1
	postRepo.Posts[post2.ID] = &post2

	userRepo.HardDeleteAllFunc = func() (int64, error) {
		count := int64(len(userRepo.Users) + len(userRepo.DeletedUsers))
		userRepo.Users = make(map[uint]*database.User)
		userRepo.DeletedUsers = make(map[uint]*database.User)
		return count, nil
	}
	postRepo.HardDeleteAllFunc = func() (int64, error) {
		count := int64(len(postRepo.Posts))
		postRepo.Posts = make(map[uint]*database.Post)
		return count, nil
	}

	err = pruneAll(userRepo, postRepo, []string{"--dry-run"})
	if err != nil {
		t.Errorf("pruneAll() with dry-run error = %v", err)
	}

	t.Run("pruneAll with JSON output", func(t *testing.T) {
		SetJSONOutput(true)
		defer SetJSONOutput(false)

		err := pruneAll(userRepo, postRepo, []string{"--dry-run"})
		if err != nil {
			t.Errorf("pruneAll() with dry-run and JSON output error = %v", err)
		}
	})
}

func TestPrunePostsWithError(t *testing.T) {
	postRepo := testutils.NewMockPostRepository()

	postRepo.GetPostsByDeletedUsersFunc = func() ([]database.Post, error) {
		return nil, fmt.Errorf("database error")
	}

	err := prunePosts(postRepo, []string{"--dry-run"})
	if err == nil {
		t.Errorf("Expected error from GetPostsByDeletedUsers, got nil")
	}
	if !strings.Contains(err.Error(), "get posts by deleted users") {
		t.Errorf("Expected error message to contain 'get posts by deleted users', got: %v", err)
	}
}

func TestPruneAllWithUserError(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	postRepo := testutils.NewMockPostRepository()

	userRepo.GetAllFunc = func(limit, offset int) ([]database.User, error) {
		return nil, fmt.Errorf("user get error")
	}

	err := pruneAll(userRepo, postRepo, []string{"--dry-run"})
	if err == nil {
		t.Errorf("Expected error from GetAll, got nil")
	}
	if !strings.Contains(err.Error(), "get user count") {
		t.Errorf("Expected error message to contain 'get user count', got: %v", err)
	}
}

func TestPruneAllWithPostError(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	postRepo := testutils.NewMockPostRepository()

	postRepo.CountFunc = func() (int64, error) {
		return 0, fmt.Errorf("post count error")
	}

	err := pruneAll(userRepo, postRepo, []string{"--dry-run"})
	if err == nil {
		t.Errorf("Expected error from Count, got nil")
	}
	if !strings.Contains(err.Error(), "get post count") {
		t.Errorf("Expected error message to contain 'get post count', got: %v", err)
	}
}

func TestPrintPruneUsage(t *testing.T) {
	printPruneUsage()
}

func TestPruneFlagParsing(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	postRepo := testutils.NewMockPostRepository()

	t.Run("prunePosts unknown flag", func(t *testing.T) {
		err := prunePosts(postRepo, []string{"--unknown-flag"})

		if err == nil {
			t.Error("expected error for unknown flag in prunePosts")
		}
	})

	t.Run("prunePosts missing dry-run value (bool)", func(t *testing.T) {
		err := prunePosts(postRepo, []string{"--dry-run"})

		if err != nil {
			t.Errorf("unexpected error for dry-run: %v", err)
		}
	})

	t.Run("pruneUsers unknown flag", func(t *testing.T) {
		err := pruneUsers(userRepo, postRepo, []string{"--unknown-flag"})

		if err == nil {
			t.Error("expected error for unknown flag in pruneUsers")
		}
	})

	t.Run("pruneUsers with-posts as non-bool", func(t *testing.T) {
		err := pruneUsers(userRepo, postRepo, []string{"--with-posts", "true"})
		if err != nil {
			t.Errorf("unexpected error for with-posts: %v", err)
		}
	})

	t.Run("pruneAll unknown flag", func(t *testing.T) {
		err := pruneAll(userRepo, postRepo, []string{"--unknown-flag"})

		if err == nil {
			t.Error("expected error for unknown flag in pruneAll")
		}
	})
}

func TestPrunePostsWithMockData(t *testing.T) {
	postRepo := testutils.NewMockPostRepository()

	post1 := database.Post{
		ID:       1,
		Title:    "Test Post 1",
		URL:      "http://example.com/1",
		AuthorID: nil,
	}
	post2 := database.Post{
		ID:       2,
		Title:    "Test Post 2",
		URL:      "http://example.com/2",
		AuthorID: nil,
	}

	postRepo.GetPostsByDeletedUsersFunc = func() ([]database.Post, error) {
		return []database.Post{post1, post2}, nil
	}

	err := prunePosts(postRepo, []string{"--dry-run"})
	if err != nil {
		t.Errorf("prunePosts() with mock data error = %v", err)
	}
}

func TestPruneAllWithMockData(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	postRepo := testutils.NewMockPostRepository()

	userRepo.HardDeleteAllFunc = func() (int64, error) {
		return 5, nil
	}
	postRepo.HardDeleteAllFunc = func() (int64, error) {
		return 10, nil
	}

	err := pruneAll(userRepo, postRepo, []string{"--dry-run"})
	if err != nil {
		t.Errorf("pruneAll() with mock data error = %v", err)
	}
}

func TestPrunePostsActualDeletion(t *testing.T) {
	postRepo := testutils.NewMockPostRepository()

	post1 := database.Post{
		ID:       1,
		Title:    "Test Post 1",
		URL:      "http://example.com/1",
		AuthorID: nil,
	}
	post2 := database.Post{
		ID:       2,
		Title:    "Test Post 2",
		URL:      "http://example.com/2",
		AuthorID: nil,
	}

	postRepo.GetPostsByDeletedUsersFunc = func() ([]database.Post, error) {
		return []database.Post{post1, post2}, nil
	}

	var deletedCount int64
	postRepo.HardDeletePostsByDeletedUsersFunc = func() (int64, error) {
		deletedCount = 2
		return 2, nil
	}

	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	os.Stdin = r

	go func() {
		_, _ = w.WriteString("yes\n")
		_ = w.Close()
	}()

	err = prunePosts(postRepo, []string{})
	if err != nil {
		t.Errorf("prunePosts() actual deletion error = %v", err)
	}

	if deletedCount != 2 {
		t.Errorf("Expected 2 posts to be deleted, got %d", deletedCount)
	}
}

func TestPruneAllActualDeletion(t *testing.T) {
	userRepo := testutils.NewMockUserRepository()
	postRepo := testutils.NewMockPostRepository()

	user1 := database.User{ID: 1, Username: "user1", Email: "user1@example.com"}
	user2 := database.User{ID: 2, Username: "user2", Email: "user2@example.com"}
	post1 := database.Post{ID: 1, Title: "Post 1", URL: "http://example.com/1", AuthorID: &user1.ID}
	post2 := database.Post{ID: 2, Title: "Post 2", URL: "http://example.com/2", AuthorID: &user2.ID}

	userRepo.Users[user1.ID] = &user1
	userRepo.Users[user2.ID] = &user2
	postRepo.Posts[post1.ID] = &post1
	postRepo.Posts[post2.ID] = &post2

	var totalDeleted int64
	userRepo.HardDeleteAllFunc = func() (int64, error) {
		totalDeleted = 2
		return 2, nil
	}

	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()

	os.Stdin = reader

	go func() {
		_, _ = writer.WriteString("yes\n")
		_ = writer.Close()
	}()

	err = pruneAll(userRepo, postRepo, []string{})
	if err != nil {
		t.Errorf("pruneAll() actual deletion error = %v", err)
	}

	if totalDeleted != 2 {
		t.Errorf("Expected 2 users to be deleted, got %d", totalDeleted)
	}
}
