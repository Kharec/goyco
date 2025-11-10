package testutils

import (
	"fmt"
	"testing"

	"goyco/internal/database"
	"goyco/internal/repositories"
)

func CreatePostWithRepo(t *testing.T, repo repositories.PostRepository, authorID uint, title, url string) *database.Post {
	t.Helper()

	post := &database.Post{
		Title:    title,
		URL:      url,
		Content:  fmt.Sprintf("Content for %s", title),
		AuthorID: &authorID,
	}

	if err := repo.Create(post); err != nil {
		t.Fatalf("Failed to create test post: %v", err)
	}

	return post
}
