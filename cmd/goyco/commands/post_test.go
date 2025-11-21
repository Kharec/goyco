package commands

import (
	"errors"
	"strings"
	"testing"
	"time"

	"goyco/internal/database"
	"goyco/internal/repositories"
	"goyco/internal/services"
	"goyco/internal/testutils"

	"gorm.io/gorm"
)

func createPostQueries(repo repositories.PostRepository) *services.PostQueries {
	voteRepo := testutils.NewMockVoteRepository()
	voteService := services.NewVoteService(voteRepo, repo, nil)
	return services.NewPostQueries(repo, voteService)
}

func TestHandlePostCommand(t *testing.T) {
	cfg := testutils.NewTestConfig()

	t.Run("help requested", func(t *testing.T) {
		err := HandlePostCommand(cfg, "post", []string{"--help"})

		if err != nil {
			t.Errorf("unexpected error for help: %v", err)
		}
	})
}

func TestRunPostCommand(t *testing.T) {
	mockRepo := testutils.NewMockPostRepository()
	postQueries := createPostQueries(mockRepo)

	t.Run("missing subcommand", func(t *testing.T) {
		err := runPostCommand(postQueries, mockRepo, []string{})

		if err == nil {
			t.Error("expected error for missing subcommand")
		}

		if err.Error() != "missing post subcommand" {
			t.Errorf("expected specific error, got: %v", err)
		}
	})

	t.Run("unknown subcommand", func(t *testing.T) {
		err := runPostCommand(postQueries, mockRepo, []string{"unknown"})

		if err == nil {
			t.Error("expected error for unknown subcommand")
		}

		expectedErr := "unknown post subcommand: unknown"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("help subcommand", func(t *testing.T) {
		err := runPostCommand(postQueries, mockRepo, []string{"help"})

		if err != nil {
			t.Errorf("unexpected error for help: %v", err)
		}
	})
}

func TestPostDelete(t *testing.T) {
	mockRepo := testutils.NewMockPostRepository()

	testPost := &database.Post{
		Title:    "Test Post",
		Content:  "Test Content",
		AuthorID: &[]uint{1}[0],
		Score:    0,
	}
	_ = mockRepo.Create(testPost)

	t.Run("successful delete", func(t *testing.T) {
		err := postDelete(mockRepo, []string{"1"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("successful delete with JSON output", func(t *testing.T) {
		SetJSONOutput(true)
		defer SetJSONOutput(false)

		freshMockRepo := testutils.NewMockPostRepository()
		testPost := &database.Post{
			Title:    "Test Post",
			Content:  "Test Content",
			AuthorID: &[]uint{1}[0],
			Score:    0,
		}
		_ = freshMockRepo.Create(testPost)

		err := postDelete(freshMockRepo, []string{"1"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		err := postDelete(mockRepo, []string{})

		if err == nil {
			t.Error("expected error for missing id")
		}

		if err.Error() != "post ID is required" {
			t.Errorf("expected specific error, got: %v", err)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		err := postDelete(mockRepo, []string{"0"})

		if err == nil {
			t.Error("expected error for invalid id")
		}

		if err.Error() != "post ID must be greater than 0" {
			t.Errorf("expected specific error, got: %v", err)
		}
	})

	t.Run("non-existent post", func(t *testing.T) {
		err := postDelete(mockRepo, []string{"999"})

		if err == nil {
			t.Error("expected error for non-existent post")
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("expected record not found error, got: %v", err)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		mockRepo.DeleteErr = errors.New("database error")
		err := postDelete(mockRepo, []string{"1"})

		if err == nil {
			t.Error("expected error from repository")
		}

		expectedErr := "delete post: database error"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})
}

func TestPostList(t *testing.T) {
	mockRepo := testutils.NewMockPostRepository()

	testPosts := []*database.Post{
		{
			Title:     "First Post",
			Content:   "First Content",
			AuthorID:  &[]uint{1}[0],
			Score:     10,
			CreatedAt: time.Now().Add(-2 * time.Hour),
		},
		{
			Title:     "Second Post",
			Content:   "Second Content",
			AuthorID:  &[]uint{2}[0],
			Score:     5,
			CreatedAt: time.Now().Add(-1 * time.Hour),
		},
	}

	for _, post := range testPosts {
		_ = mockRepo.Create(post)
	}

	postQueries := createPostQueries(mockRepo)

	t.Run("list all posts", func(t *testing.T) {
		err := postList(postQueries, []string{})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("list all posts with JSON output", func(t *testing.T) {
		SetJSONOutput(true)
		defer SetJSONOutput(false)

		err := postList(postQueries, []string{})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("list with limit", func(t *testing.T) {
		err := postList(postQueries, []string{"--limit", "1"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("list with offset", func(t *testing.T) {
		err := postList(postQueries, []string{"--offset", "1"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("list with user filter", func(t *testing.T) {
		err := postList(postQueries, []string{"--user-id", "1"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("list with all filters", func(t *testing.T) {
		err := postList(postQueries, []string{"--limit", "1", "--offset", "0", "--user-id", "1"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		emptyRepo := testutils.NewMockPostRepository()
		emptyPostQueries := createPostQueries(emptyRepo)
		err := postList(emptyPostQueries, []string{})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		mockRepo.GetErr = errors.New("database error")
		err := postList(postQueries, []string{})

		if err == nil {
			t.Error("expected error from repository")
		}

		expectedErr := "list posts: database error"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})
}

func TestPostSearch(t *testing.T) {
	mockRepo := testutils.NewMockPostRepository()
	postQueries := createPostQueries(mockRepo)

	testPosts := []*database.Post{
		{
			Title:     "Golang Tutorial",
			Content:   "Learn Go programming language",
			AuthorID:  &[]uint{1}[0],
			Score:     10,
			CreatedAt: time.Now().Add(-2 * time.Hour),
		},
		{
			Title:     "Python Guide",
			Content:   "Learn Python programming",
			AuthorID:  &[]uint{2}[0],
			Score:     5,
			CreatedAt: time.Now().Add(-1 * time.Hour),
		},
		{
			Title:     "Go Best Practices",
			Content:   "Advanced Go techniques and patterns",
			AuthorID:  &[]uint{1}[0],
			Score:     15,
			CreatedAt: time.Now().Add(-30 * time.Minute),
		},
	}

	for _, post := range testPosts {
		_ = mockRepo.Create(post)
	}

	t.Run("search with results", func(t *testing.T) {
		err := postSearch(postQueries, []string{"Go"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("search with results and JSON output", func(t *testing.T) {
		SetJSONOutput(true)
		defer SetJSONOutput(false)

		err := postSearch(postQueries, []string{"Go"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("case insensitive search", func(t *testing.T) {
		mockRepo.SearchCalls = nil

		err := postSearch(postQueries, []string{"golang"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(mockRepo.SearchCalls) != 1 {
			t.Errorf("expected 1 search call, got %d", len(mockRepo.SearchCalls))
		} else {
			call := mockRepo.SearchCalls[0]
			if call.Query != "golang" {
				t.Errorf("expected query 'golang', got %q", call.Query)
			}
		}
	})

	t.Run("search with no results", func(t *testing.T) {
		err := postSearch(postQueries, []string{"nonexistent"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("search with limit", func(t *testing.T) {
		mockRepo.SearchCalls = nil

		err := postSearch(postQueries, []string{"--limit", "1", "Go"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(mockRepo.SearchCalls) != 1 {
			t.Errorf("expected 1 search call, got %d", len(mockRepo.SearchCalls))
		} else {
			call := mockRepo.SearchCalls[0]
			if call.Query != "Go" {
				t.Errorf("expected query 'Go', got %q", call.Query)
			}
			if call.Limit != 1 {
				t.Errorf("expected limit 1, got %d", call.Limit)
			}
			if call.Offset != 0 {
				t.Errorf("expected offset 0, got %d", call.Offset)
			}
		}
	})

	t.Run("search with offset", func(t *testing.T) {
		mockRepo.SearchCalls = nil

		err := postSearch(postQueries, []string{"--offset", "1", "Go"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(mockRepo.SearchCalls) != 1 {
			t.Errorf("expected 1 search call, got %d", len(mockRepo.SearchCalls))
		} else {
			call := mockRepo.SearchCalls[0]
			if call.Query != "Go" {
				t.Errorf("expected query 'Go', got %q", call.Query)
			}
			if call.Limit != 10 {
				t.Errorf("expected limit 10, got %d", call.Limit)
			}
			if call.Offset != 1 {
				t.Errorf("expected offset 1, got %d", call.Offset)
			}
		}
	})

	t.Run("search with limit and offset", func(t *testing.T) {
		mockRepo.SearchCalls = nil

		err := postSearch(postQueries, []string{"--limit", "1", "--offset", "1", "Go"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(mockRepo.SearchCalls) != 1 {
			t.Errorf("expected 1 search call, got %d", len(mockRepo.SearchCalls))
		} else {
			call := mockRepo.SearchCalls[0]
			if call.Query != "Go" {
				t.Errorf("expected query 'Go', got %q", call.Query)
			}
			if call.Limit != 1 {
				t.Errorf("expected limit 1, got %d", call.Limit)
			}
			if call.Offset != 1 {
				t.Errorf("expected offset 1, got %d", call.Offset)
			}
		}
	})

	t.Run("missing search term", func(t *testing.T) {
		err := postSearch(postQueries, []string{})

		if err == nil {
			t.Error("expected error for missing search term")
		}

		expectedErr := "search term is required"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("invalid limit flag", func(t *testing.T) {
		err := postSearch(postQueries, []string{"--limit", "invalid", "Go"})

		if err == nil {
			t.Error("expected error for invalid limit")
		}
	})

	t.Run("invalid offset flag", func(t *testing.T) {
		err := postSearch(postQueries, []string{"--offset", "invalid", "Go"})

		if err == nil {
			t.Error("expected error for invalid offset")
		}
	})

	t.Run("negative limit", func(t *testing.T) {
		err := postSearch(postQueries, []string{"--limit", "-1", "Go"})

		if err == nil {
			t.Error("expected error for negative limit")
		}

		expectedErr := "limit must be non-negative"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("negative offset", func(t *testing.T) {
		err := postSearch(postQueries, []string{"--offset", "-1", "Go"})

		if err == nil {
			t.Error("expected error for negative offset")
		}

		expectedErr := "offset must be non-negative"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("repository error", func(t *testing.T) {
		mockRepo.SearchErr = errors.New("database error")
		err := postSearch(postQueries, []string{"Go"})

		if err == nil {
			t.Error("expected error from repository")
		}

		expectedErr := "search posts: database error"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("unknown flag", func(t *testing.T) {
		err := postSearch(postQueries, []string{"--unknown-flag", "Go"})

		if err == nil {
			t.Error("expected error for unknown flag")
		}
	})

	t.Run("missing limit value", func(t *testing.T) {
		err := postSearch(postQueries, []string{"--limit"})

		if err == nil {
			t.Error("expected error for missing limit value")
		}
	})

	t.Run("missing offset value", func(t *testing.T) {
		err := postSearch(postQueries, []string{"--offset"})

		if err == nil {
			t.Error("expected error for missing offset value")
		}
	})
}

func TestPostListFlagParsing(t *testing.T) {
	mockRepo := testutils.NewMockPostRepository()
	postQueries := createPostQueries(mockRepo)

	testPosts := []*database.Post{
		{
			Title:     "First Post",
			Content:   "First Content",
			AuthorID:  &[]uint{1}[0],
			Score:     10,
			CreatedAt: time.Now().Add(-2 * time.Hour),
		},
	}

	for _, post := range testPosts {
		_ = mockRepo.Create(post)
	}

	t.Run("invalid limit type", func(t *testing.T) {
		err := postList(postQueries, []string{"--limit", "abc"})

		if err == nil {
			t.Error("expected error for invalid limit type")
		}
	})

	t.Run("invalid offset type", func(t *testing.T) {
		err := postList(postQueries, []string{"--offset", "xyz"})

		if err == nil {
			t.Error("expected error for invalid offset type")
		}
	})

	t.Run("invalid user-id type", func(t *testing.T) {
		err := postList(postQueries, []string{"--user-id", "invalid"})

		if err == nil {
			t.Error("expected error for invalid user-id type")
		}
	})

	t.Run("unknown flag", func(t *testing.T) {
		err := postList(postQueries, []string{"--unknown-flag"})

		if err == nil {
			t.Error("expected error for unknown flag")
		}
	})

	t.Run("missing limit value", func(t *testing.T) {
		err := postList(postQueries, []string{"--limit"})

		if err == nil {
			t.Error("expected error for missing limit value")
		}
	})

	t.Run("missing offset value", func(t *testing.T) {
		err := postList(postQueries, []string{"--offset"})

		if err == nil {
			t.Error("expected error for missing offset value")
		}
	})

	t.Run("missing user-id value", func(t *testing.T) {
		err := postList(postQueries, []string{"--user-id"})

		if err == nil {
			t.Error("expected error for missing user-id value")
		}
	})
}

func TestPostDeleteFlagParsing(t *testing.T) {
	mockRepo := testutils.NewMockPostRepository()

	t.Run("invalid id type", func(t *testing.T) {
		err := postDelete(mockRepo, []string{"abc"})

		if err == nil {
			t.Error("expected error for invalid id type")
		}

		if !strings.Contains(err.Error(), "invalid post ID") {
			t.Errorf("expected invalid post ID error, got: %v", err)
		}
	})

	t.Run("non-numeric id", func(t *testing.T) {
		err := postDelete(mockRepo, []string{"not-a-number"})

		if err == nil {
			t.Error("expected error for non-numeric id")
		}
	})
}
