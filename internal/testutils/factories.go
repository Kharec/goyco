package testutils

import (
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"goyco/internal/database"
	"goyco/internal/repositories"
)

type TestDataFactory struct{}

type AuthResult struct {
	User        *database.User `json:"user"`
	AccessToken string         `json:"access_token"`
}

type RegistrationResult struct {
	User *database.User `json:"user"`
}

type VoteRequest struct {
	Type      database.VoteType `json:"type"`
	UserID    uint              `json:"user_id"`
	PostID    uint              `json:"post_id"`
	IPAddress string            `json:"ip_address"`
	UserAgent string            `json:"user_agent"`
}

func NewTestDataFactory() *TestDataFactory {
	return &TestDataFactory{}
}

type UserBuilder struct {
	user *database.User
}

func (f *TestDataFactory) NewUserBuilder() *UserBuilder {
	return &UserBuilder{
		user: &database.User{
			ID:            1,
			Username:      "testuser",
			Email:         "test@example.com",
			EmailVerified: true,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}
}

func (b *UserBuilder) WithID(id uint) *UserBuilder {
	b.user.ID = id
	return b
}

func (b *UserBuilder) WithUsername(username string) *UserBuilder {
	b.user.Username = username
	return b
}

func (b *UserBuilder) WithEmail(email string) *UserBuilder {
	b.user.Email = email
	return b
}

func (b *UserBuilder) WithPassword(password string) *UserBuilder {
	hashed, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	b.user.Password = string(hashed)
	return b
}

func (b *UserBuilder) WithEmailVerified(verified bool) *UserBuilder {
	b.user.EmailVerified = verified
	return b
}

func (b *UserBuilder) WithCreatedAt(t time.Time) *UserBuilder {
	b.user.CreatedAt = t
	return b
}

func (b *UserBuilder) Build() *database.User {
	return b.user
}

type PostBuilder struct {
	post *database.Post
}

func (f *TestDataFactory) NewPostBuilder() *PostBuilder {
	return &PostBuilder{
		post: &database.Post{
			ID:        1,
			Title:     "Test Post",
			URL:       "https://example.com",
			Content:   "Test content",
			AuthorID:  uintPtr(1),
			UpVotes:   0,
			DownVotes: 0,
			Score:     0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

func (b *PostBuilder) WithID(id uint) *PostBuilder {
	b.post.ID = id
	return b
}

func (b *PostBuilder) WithTitle(title string) *PostBuilder {
	b.post.Title = title
	return b
}

func (b *PostBuilder) WithURL(url string) *PostBuilder {
	b.post.URL = url
	return b
}

func (b *PostBuilder) WithContent(content string) *PostBuilder {
	b.post.Content = content
	return b
}

func (b *PostBuilder) WithAuthorID(authorID uint) *PostBuilder {
	b.post.AuthorID = &authorID
	return b
}

func (b *PostBuilder) WithVotes(upVotes, downVotes int) *PostBuilder {
	b.post.UpVotes = upVotes
	b.post.DownVotes = downVotes
	b.post.Score = upVotes - downVotes
	return b
}

func (b *PostBuilder) WithCreatedAt(t time.Time) *PostBuilder {
	b.post.CreatedAt = t
	return b
}

func (b *PostBuilder) Build() *database.Post {
	return b.post
}

type VoteBuilder struct {
	vote *database.Vote
}

func (f *TestDataFactory) NewVoteBuilder() *VoteBuilder {
	return &VoteBuilder{
		vote: &database.Vote{
			ID:     1,
			Type:   database.VoteUp,
			UserID: uintPtr(1),
			PostID: 1,
		},
	}
}

func (b *VoteBuilder) WithID(id uint) *VoteBuilder {
	b.vote.ID = id
	return b
}

func (b *VoteBuilder) WithType(voteType database.VoteType) *VoteBuilder {
	b.vote.Type = voteType
	return b
}

func (b *VoteBuilder) WithUserID(userID uint) *VoteBuilder {
	b.vote.UserID = &userID
	return b
}

func (b *VoteBuilder) WithPostID(postID uint) *VoteBuilder {
	b.vote.PostID = postID
	return b
}

func (b *VoteBuilder) Build() *database.Vote {
	return b.vote
}

type AuthResultBuilder struct {
	result *AuthResult
}

func (f *TestDataFactory) NewAuthResultBuilder() *AuthResultBuilder {
	return &AuthResultBuilder{
		result: &AuthResult{
			User:        &database.User{ID: 1, Username: "testuser"},
			AccessToken: "access_token",
		},
	}
}

func (b *AuthResultBuilder) WithUser(user *database.User) *AuthResultBuilder {
	b.result.User = user
	return b
}

func (b *AuthResultBuilder) WithAccessToken(token string) *AuthResultBuilder {
	b.result.AccessToken = token
	return b
}

func (b *AuthResultBuilder) Build() *AuthResult {
	return b.result
}

type RegistrationResultBuilder struct {
	result *RegistrationResult
}

func (f *TestDataFactory) NewRegistrationResultBuilder() *RegistrationResultBuilder {
	return &RegistrationResultBuilder{
		result: &RegistrationResult{
			User: &database.User{ID: 1, Username: "testuser"},
		},
	}
}

func (b *RegistrationResultBuilder) WithUser(user *database.User) *RegistrationResultBuilder {
	b.result.User = user
	return b
}

func (b *RegistrationResultBuilder) WithMessage(message string) *RegistrationResultBuilder {
	return b
}

func (b *RegistrationResultBuilder) Build() *RegistrationResult {
	return b.result
}

type VoteRequestBuilder struct {
	request VoteRequest
}

func (f *TestDataFactory) NewVoteRequestBuilder() *VoteRequestBuilder {
	return &VoteRequestBuilder{
		request: VoteRequest{
			Type:      database.VoteUp,
			UserID:    1,
			PostID:    1,
			IPAddress: "127.0.0.1",
			UserAgent: "test-agent",
		},
	}
}

func (b *VoteRequestBuilder) WithType(voteType database.VoteType) *VoteRequestBuilder {
	b.request.Type = voteType
	return b
}

func (b *VoteRequestBuilder) WithUserID(userID uint) *VoteRequestBuilder {
	b.request.UserID = userID
	return b
}

func (b *VoteRequestBuilder) WithPostID(postID uint) *VoteRequestBuilder {
	b.request.PostID = postID
	return b
}

func (b *VoteRequestBuilder) WithIPAddress(ip string) *VoteRequestBuilder {
	b.request.IPAddress = ip
	return b
}

func (b *VoteRequestBuilder) WithUserAgent(agent string) *VoteRequestBuilder {
	b.request.UserAgent = agent
	return b
}

func (b *VoteRequestBuilder) Build() VoteRequest {
	return b.request
}

func (f *TestDataFactory) CreateTestUsers(count int) []*database.User {
	users := make([]*database.User, count)
	for i := range count {
		users[i] = f.NewUserBuilder().
			WithID(uint(i + 1)).
			WithUsername(fmt.Sprintf("user%d", i+1)).
			WithEmail(fmt.Sprintf("user%d@example.com", i+1)).
			Build()
	}
	return users
}

func (f *TestDataFactory) CreateTestPosts(count int) []*database.Post {
	posts := make([]*database.Post, count)
	for i := range count {
		posts[i] = f.NewPostBuilder().
			WithID(uint(i+1)).
			WithTitle(fmt.Sprintf("Post %d", i+1)).
			WithURL(fmt.Sprintf("https://example.com/post%d", i+1)).
			WithContent(fmt.Sprintf("Content for post %d", i+1)).
			WithAuthorID(uint((i%10)+1)).
			WithVotes(i%10, i%5).
			Build()
	}
	return posts
}

func (f *TestDataFactory) CreateTestVotes(count int) []*database.Vote {
	votes := make([]*database.Vote, count)
	for i := range count {
		voteType := database.VoteUp
		if i%3 == 0 {
			voteType = database.VoteDown
		} else if i%5 == 0 {
			voteType = database.VoteNone
		}

		votes[i] = f.NewVoteBuilder().
			WithID(uint(i + 1)).
			WithType(voteType).
			WithUserID(uint((i % 20) + 1)).
			WithPostID(uint((i % 100) + 1)).
			Build()
	}
	return votes
}

func (f *TestDataFactory) CreateTestAuthResults(count int) []*AuthResult {
	results := make([]*AuthResult, count)
	for i := range count {
		results[i] = f.NewAuthResultBuilder().
			WithUser(f.NewUserBuilder().
				WithID(uint(i + 1)).
				WithUsername(fmt.Sprintf("user%d", i+1)).
				Build()).
			WithAccessToken(fmt.Sprintf("token_%d", i+1)).
			Build()
	}
	return results
}

func (f *TestDataFactory) CreateTestVoteRequests(count int) []VoteRequest {
	requests := make([]VoteRequest, count)
	for i := range count {
		voteType := database.VoteUp
		if i%3 == 0 {
			voteType = database.VoteDown
		} else if i%5 == 0 {
			voteType = database.VoteNone
		}

		requests[i] = f.NewVoteRequestBuilder().
			WithType(voteType).
			WithUserID(uint((i % 20) + 1)).
			WithPostID(uint((i % 100) + 1)).
			WithIPAddress(fmt.Sprintf("192.168.1.%d", (i%254)+1)).
			WithUserAgent(fmt.Sprintf("test-agent-%d", i+1)).
			Build()
	}
	return requests
}

//go:fix inline
func uintPtr(u uint) *uint {
	return new(u)
}

type E2ETestDataFactory struct {
	UserRepo repositories.UserRepository
	PostRepo repositories.PostRepository
}

func NewE2ETestDataFactory(userRepo repositories.UserRepository, postRepo repositories.PostRepository) *E2ETestDataFactory {
	return &E2ETestDataFactory{
		UserRepo: userRepo,
		PostRepo: postRepo,
	}
}

type PostData struct {
	Title   string
	URL     string
	Content string
}

func (f *E2ETestDataFactory) CreateUserWithPosts(t *testing.T, username, email, password string, posts []PostData) (*TestUser, []*TestPost) {
	t.Helper()

	user := CreateE2ETestUser(t, f.UserRepo, username, email, password)

	var createdPosts []*TestPost
	for i, postData := range posts {
		url := postData.URL
		if url == "" {
			url = fmt.Sprintf("https://example.com/post-%d-%d", user.ID, i+1)
		}

		title := postData.Title
		if title == "" {
			title = fmt.Sprintf("Test Post %d", i+1)
		}

		content := postData.Content
		if content == "" {
			content = fmt.Sprintf("Test content for post %d", i+1)
		}

		post := &database.Post{
			Title:     title,
			URL:       url,
			Content:   content,
			AuthorID:  &user.ID,
			UpVotes:   0,
			DownVotes: 0,
			Score:     0,
		}

		if err := f.PostRepo.Create(post); err != nil {
			t.Fatalf("Failed to create test post: %v", err)
		}

		createdPost, err := f.PostRepo.GetByID(post.ID)
		if err != nil {
			t.Fatalf("Failed to fetch created post: %v", err)
		}

		createdPosts = append(createdPosts, &TestPost{
			ID:       createdPost.ID,
			Title:    createdPost.Title,
			URL:      createdPost.URL,
			Content:  createdPost.Content,
			AuthorID: *createdPost.AuthorID,
		})
	}

	return user, createdPosts
}

func (f *E2ETestDataFactory) CreateMultipleUsers(t *testing.T, count int, usernamePrefix, emailPrefix, password string) []*TestUser {
	t.Helper()

	if count <= 0 {
		t.Fatalf("count must be greater than 0, got %d", count)
	}

	var users []*TestUser
	timestamp := time.Now().UnixNano()

	for i := range count {
		uniqueID := timestamp + int64(i)
		username := fmt.Sprintf("%s%d", usernamePrefix, uniqueID)
		email := fmt.Sprintf("%s%d@example.com", emailPrefix, uniqueID)

		userPassword := password
		if userPassword == "" {
			userPassword = "StrongPass123!"
		}

		user := CreateE2ETestUser(t, f.UserRepo, username, email, userPassword)
		users = append(users, user)
	}

	return users
}

func (f *E2ETestDataFactory) CreateUserWithDefaultPosts(t *testing.T, username, email, password string, count int) (*TestUser, []*TestPost) {
	t.Helper()

	if count <= 0 {
		count = 3
	}

	posts := make([]PostData, count)
	for i := 0; i < count; i++ {
		posts[i] = PostData{
			Title:   fmt.Sprintf("Default Post %d", i+1),
			URL:     fmt.Sprintf("https://example.com/default-post-%d", i+1),
			Content: fmt.Sprintf("This is default post content number %d", i+1),
		}
	}

	return f.CreateUserWithPosts(t, username, email, password, posts)
}

func (f *E2ETestDataFactory) CreateUsersWithPosts(t *testing.T, count int, usernamePrefix, emailPrefix, password string, postsPerUser []PostData) map[uint]*UserWithPosts {
	t.Helper()

	if count <= 0 {
		t.Fatalf("count must be greater than 0, got %d", count)
	}

	users := f.CreateMultipleUsers(t, count, usernamePrefix, emailPrefix, password)

	result := make(map[uint]*UserWithPosts)
	for _, user := range users {
		var createdPosts []*TestPost
		for i, postData := range postsPerUser {
			url := postData.URL
			if url == "" {
				url = fmt.Sprintf("https://example.com/post-%d-%d", user.ID, i+1)
			}

			title := postData.Title
			if title == "" {
				title = fmt.Sprintf("Test Post %d for User %d", i+1, user.ID)
			}

			content := postData.Content
			if content == "" {
				content = fmt.Sprintf("Test content for post %d", i+1)
			}

			post := &database.Post{
				Title:     title,
				URL:       url,
				Content:   content,
				AuthorID:  &user.ID,
				UpVotes:   0,
				DownVotes: 0,
				Score:     0,
			}

			if err := f.PostRepo.Create(post); err != nil {
				t.Fatalf("Failed to create test post: %v", err)
			}

			createdPost, err := f.PostRepo.GetByID(post.ID)
			if err != nil {
				t.Fatalf("Failed to fetch created post: %v", err)
			}

			createdPosts = append(createdPosts, &TestPost{
				ID:       createdPost.ID,
				Title:    createdPost.Title,
				URL:      createdPost.URL,
				Content:  createdPost.Content,
				AuthorID: *createdPost.AuthorID,
			})
		}

		result[user.ID] = &UserWithPosts{
			User:  user,
			Posts: createdPosts,
		}
	}

	return result
}

type UserWithPosts struct {
	User  *TestUser
	Posts []*TestPost
}

func (f *E2ETestDataFactory) CreatePostForUser(t *testing.T, userID uint, postData PostData) *TestPost {
	t.Helper()

	url := postData.URL
	if url == "" {
		url = fmt.Sprintf("https://example.com/post-%d-%d", userID, time.Now().UnixNano())
	}

	title := postData.Title
	if title == "" {
		title = "Test Post"
	}

	content := postData.Content
	if content == "" {
		content = "Test post content"
	}

	post := &database.Post{
		Title:     title,
		URL:       url,
		Content:   content,
		AuthorID:  &userID,
		UpVotes:   0,
		DownVotes: 0,
		Score:     0,
	}

	if err := f.PostRepo.Create(post); err != nil {
		t.Fatalf("Failed to create test post: %v", err)
	}

	createdPost, err := f.PostRepo.GetByID(post.ID)
	if err != nil {
		t.Fatalf("Failed to fetch created post: %v", err)
	}

	return &TestPost{
		ID:       createdPost.ID,
		Title:    createdPost.Title,
		URL:      createdPost.URL,
		Content:  createdPost.Content,
		AuthorID: *createdPost.AuthorID,
	}
}
