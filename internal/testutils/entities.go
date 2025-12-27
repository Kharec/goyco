package testutils

import (
	"fmt"
	"testing"
	"time"

	"goyco/internal/database"
	"goyco/internal/repositories"
)

func CreatePostWithRepo(t *testing.T, repo repositories.PostRepository, authorID uint, title, url string) *database.Post {
	t.Helper()

	if title == "" {
		title = fmt.Sprintf("Post %d", time.Now().UnixNano())
	}
	if url == "" {
		url = fmt.Sprintf("https://example.com/%d", time.Now().UnixNano())
	}

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

func CreateUserWithRepo(t *testing.T, repo repositories.UserRepository, username, email, password string) *database.User {
	t.Helper()

	if username == "" {
		username = fmt.Sprintf("user_%d", time.Now().UnixNano())
	}
	if email == "" {
		email = fmt.Sprintf("%s@example.com", username)
	}
	if password == "" {
		password = "hashed-password"
	}

	user := &database.User{
		Username: username,
		Email:    email,
		Password: password,
	}

	if err := repo.Create(user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return user
}

func CreateVoteWithRepo(t *testing.T, repo repositories.VoteRepository, userID, postID uint, voteType database.VoteType) *database.Vote {
	t.Helper()

	vote := &database.Vote{
		UserID: &userID,
		PostID: postID,
		Type:   voteType,
	}

	if err := repo.Create(vote); err != nil {
		t.Fatalf("Failed to create test vote: %v", err)
	}

	return vote
}

func CreateRefreshTokenWithRepo(t *testing.T, repo repositories.RefreshTokenRepositoryInterface, userID uint, tokenHash string, expiresAt time.Time) *database.RefreshToken {
	t.Helper()

	if tokenHash == "" {
		tokenHash = fmt.Sprintf("token_%d", time.Now().UnixNano())
	}
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(24 * time.Hour)
	}

	token := &database.RefreshToken{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}

	if err := repo.Create(token); err != nil {
		t.Fatalf("Failed to create test refresh token: %v", err)
	}

	return token
}

func CreateAccountDeletionRequestWithRepo(t *testing.T, repo repositories.AccountDeletionRepository, userID uint, tokenHash string, expiresAt time.Time) *database.AccountDeletionRequest {
	t.Helper()

	if tokenHash == "" {
		tokenHash = fmt.Sprintf("delete_%d", time.Now().UnixNano())
	}
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(24 * time.Hour)
	}

	request := &database.AccountDeletionRequest{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}

	if err := repo.Create(request); err != nil {
		t.Fatalf("Failed to create test account deletion request: %v", err)
	}

	return request
}
