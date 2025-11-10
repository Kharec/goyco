package dto

import (
	"testing"
	"time"

	"goyco/internal/database"
)

func TestToPostDTO(t *testing.T) {
	t.Run("nil post", func(t *testing.T) {
		dto := ToPostDTO(nil)
		if dto.ID != 0 {
			t.Errorf("Expected zero value for nil post, got ID %d", dto.ID)
		}
	})

	t.Run("valid post without author", func(t *testing.T) {
		post := &database.Post{
			ID:          1,
			Title:       "Test Post",
			URL:         "https://example.com",
			Content:     "Test content",
			AuthorID:    nil,
			AuthorName:  "",
			UpVotes:     5,
			DownVotes:   2,
			Score:       3,
			CurrentVote: database.VoteUp,
			CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		}

		dto := ToPostDTO(post)

		if dto.ID != post.ID {
			t.Errorf("Expected ID %d, got %d", post.ID, dto.ID)
		}
		if dto.Title != post.Title {
			t.Errorf("Expected Title %q, got %q", post.Title, dto.Title)
		}
		if dto.URL != post.URL {
			t.Errorf("Expected URL %q, got %q", post.URL, dto.URL)
		}
		if dto.Content != post.Content {
			t.Errorf("Expected Content %q, got %q", post.Content, dto.Content)
		}
		if dto.UpVotes != post.UpVotes {
			t.Errorf("Expected UpVotes %d, got %d", post.UpVotes, dto.UpVotes)
		}
		if dto.DownVotes != post.DownVotes {
			t.Errorf("Expected DownVotes %d, got %d", post.DownVotes, dto.DownVotes)
		}
		if dto.Score != post.Score {
			t.Errorf("Expected Score %d, got %d", post.Score, dto.Score)
		}
		if dto.CurrentVote != string(post.CurrentVote) {
			t.Errorf("Expected CurrentVote %q, got %q", post.CurrentVote, dto.CurrentVote)
		}
		if !dto.CreatedAt.Equal(post.CreatedAt) {
			t.Errorf("Expected CreatedAt %v, got %v", post.CreatedAt, dto.CreatedAt)
		}
		if !dto.UpdatedAt.Equal(post.UpdatedAt) {
			t.Errorf("Expected UpdatedAt %v, got %v", post.UpdatedAt, dto.UpdatedAt)
		}
		if dto.Author != nil {
			t.Error("Expected Author to be nil when post.Author.ID is 0")
		}
	})

	t.Run("post with author", func(t *testing.T) {
		authorID := uint(42)
		post := &database.Post{
			ID:         1,
			Title:      "Test Post",
			URL:        "https://example.com",
			AuthorID:   &authorID,
			AuthorName: "Test Author",
			Author: database.User{
				ID:       authorID,
				Username: "testuser",
				Email:    "test@example.com",
			},
			CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		}

		dto := ToPostDTO(post)

		if dto.AuthorID == nil || *dto.AuthorID != authorID {
			t.Errorf("Expected AuthorID %d, got %v", authorID, dto.AuthorID)
		}
		if dto.AuthorName != post.AuthorName {
			t.Errorf("Expected AuthorName %q, got %q", post.AuthorName, dto.AuthorName)
		}
		if dto.Author == nil {
			t.Fatal("Expected Author to be set")
		}
		if dto.Author.ID != authorID {
			t.Errorf("Expected Author.ID %d, got %d", authorID, dto.Author.ID)
		}
		if dto.Author.Username != post.Author.Username {
			t.Errorf("Expected Author.Username %q, got %q", post.Author.Username, dto.Author.Username)
		}
	})

	t.Run("post with VoteNone", func(t *testing.T) {
		post := &database.Post{
			ID:          1,
			Title:       "Test Post",
			CurrentVote: database.VoteNone,
			CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		}

		dto := ToPostDTO(post)

		if dto.CurrentVote != "none" {
			t.Errorf("Expected CurrentVote %q, got %q", "none", dto.CurrentVote)
		}
	})

	t.Run("post without CurrentVote set", func(t *testing.T) {
		post := &database.Post{
			ID:          1,
			Title:       "Test Post",
			CurrentVote: "",
			CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		}

		dto := ToPostDTO(post)

		if dto.CurrentVote != "" {
			t.Errorf("Expected empty CurrentVote, got %q", dto.CurrentVote)
		}
	})
}

func TestToPostDTOs(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		posts := []database.Post{}
		dtos := ToPostDTOs(posts)
		if len(dtos) != 0 {
			t.Errorf("Expected empty slice, got %d items", len(dtos))
		}
	})

	t.Run("multiple posts", func(t *testing.T) {
		posts := []database.Post{
			{
				ID:    1,
				Title: "Post 1",
				URL:   "https://example.com/1",
			},
			{
				ID:    2,
				Title: "Post 2",
				URL:   "https://example.com/2",
			},
			{
				ID:    3,
				Title: "Post 3",
				URL:   "https://example.com/3",
			},
		}

		dtos := ToPostDTOs(posts)

		if len(dtos) != len(posts) {
			t.Fatalf("Expected %d DTOs, got %d", len(posts), len(dtos))
		}

		for i := range posts {
			if dtos[i].ID != posts[i].ID {
				t.Errorf("Post %d: Expected ID %d, got %d", i, posts[i].ID, dtos[i].ID)
			}
			if dtos[i].Title != posts[i].Title {
				t.Errorf("Post %d: Expected Title %q, got %q", i, posts[i].Title, dtos[i].Title)
			}
		}
	})
}
