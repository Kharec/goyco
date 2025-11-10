package services

import (
	"errors"
	"testing"
	"time"

	"goyco/internal/database"
	"goyco/internal/testutils"

	"gorm.io/gorm"
)

func TestNewPostQueries(t *testing.T) {
	repo := testutils.NewMockPostRepository()
	voteService := NewVoteService(testutils.NewMockVoteRepository(), repo, nil)
	postQueries := NewPostQueries(repo, voteService)
	if postQueries == nil {
		t.Fatal("expected PostQueries to be created")
	}
	if postQueries.postRepo != repo {
		t.Error("expected postRepo to be set")
	}
	if postQueries.voteService != voteService {
		t.Error("expected voteService to be set")
	}
}

func TestPostQueries_GetAll(t *testing.T) {
	tests := []struct {
		name          string
		setupRepo     func() *testutils.MockPostRepository
		opts          QueryOptions
		ctx           VoteContext
		expectedCount int
		expectedError bool
	}{
		{
			name: "success with pagination",
			setupRepo: func() *testutils.MockPostRepository {
				repo := testutils.NewMockPostRepository()
				repo.Create(&database.Post{ID: 1, Title: "Post 1", Score: 10})
				repo.Create(&database.Post{ID: 2, Title: "Post 2", Score: 5})
				repo.Create(&database.Post{ID: 3, Title: "Post 3", Score: 15})
				return repo
			},
			opts: QueryOptions{
				Limit:  2,
				Offset: 0,
			},
			ctx:           VoteContext{UserID: 1},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name: "success with offset",
			setupRepo: func() *testutils.MockPostRepository {
				repo := testutils.NewMockPostRepository()
				repo.Create(&database.Post{ID: 1, Title: "Post 1", Score: 10})
				repo.Create(&database.Post{ID: 2, Title: "Post 2", Score: 5})
				repo.Create(&database.Post{ID: 3, Title: "Post 3", Score: 15})
				return repo
			},
			opts: QueryOptions{
				Limit:  2,
				Offset: 1,
			},
			ctx:           VoteContext{UserID: 1},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name: "repository error",
			setupRepo: func() *testutils.MockPostRepository {
				repo := testutils.NewMockPostRepository()
				repo.GetErr = errors.New("database error")
				return repo
			},
			opts: QueryOptions{
				Limit:  10,
				Offset: 0,
			},
			ctx:           VoteContext{UserID: 1},
			expectedCount: 0,
			expectedError: true,
		},
		{
			name: "empty result",
			setupRepo: func() *testutils.MockPostRepository {
				return testutils.NewMockPostRepository()
			},
			opts: QueryOptions{
				Limit:  10,
				Offset: 0,
			},
			ctx:           VoteContext{UserID: 1},
			expectedCount: 0,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			voteRepo := testutils.NewMockVoteRepository()
			voteService := NewVoteService(voteRepo, repo, nil)
			postQueries := NewPostQueries(repo, voteService)
			posts, err := postQueries.GetAll(tt.opts, tt.ctx)
			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(posts) != tt.expectedCount {
					t.Errorf("expected %d posts, got %d", tt.expectedCount, len(posts))
				}
			}
		})
	}
}

func TestPostQueries_GetAll_WithVoteEnrichment(t *testing.T) {
	repo := testutils.NewMockPostRepository()
	post1 := &database.Post{ID: 1, Title: "Post 1", Score: 10}
	post2 := &database.Post{ID: 2, Title: "Post 2", Score: 5}
	repo.Create(post1)
	repo.Create(post2)
	voteRepo := testutils.NewMockVoteRepository()
	voteService := NewVoteService(voteRepo, repo, nil)
	userID := uint(1)
	voteRepo.Create(&database.Vote{
		UserID: &userID,
		PostID: 1,
		Type:   database.VoteUp,
	})
	postQueries := NewPostQueries(repo, voteService)
	ctx := VoteContext{
		UserID:    1,
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}
	posts, err := postQueries.GetAll(QueryOptions{Limit: 10, Offset: 0}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}
	if posts[0].CurrentVote != database.VoteUp && posts[0].ID == 1 {
		if posts[1].ID == 1 && posts[1].CurrentVote != database.VoteUp {
			t.Error("expected post 1 to have CurrentVote set to VoteUp")
		}
	}
	for _, post := range posts {
		if post.ID == 2 && post.CurrentVote != "" {
			t.Errorf("expected post 2 to have no vote, got %s", post.CurrentVote)
		}
	}
}

func TestPostQueries_GetTop(t *testing.T) {
	repo := testutils.NewMockPostRepository()
	post1 := &database.Post{ID: 1, Title: "Post 1", Score: 10}
	post2 := &database.Post{ID: 2, Title: "Post 2", Score: 15}
	post3 := &database.Post{ID: 3, Title: "Post 3", Score: 5}
	repo.Create(post1)
	repo.Create(post2)
	repo.Create(post3)
	voteRepo := testutils.NewMockVoteRepository()
	voteService := NewVoteService(voteRepo, repo, nil)
	postQueries := NewPostQueries(repo, voteService)
	posts, err := postQueries.GetTop(2, VoteContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) != 2 {
		t.Errorf("expected 2 posts, got %d", len(posts))
	}
	if len(posts) == 0 {
		t.Error("expected at least one post")
	}
}

func TestPostQueries_GetNewest(t *testing.T) {
	repo := testutils.NewMockPostRepository()
	now := time.Now()
	post1 := &database.Post{ID: 1, Title: "Post 1", CreatedAt: now.Add(-2 * time.Hour)}
	post2 := &database.Post{ID: 2, Title: "Post 2", CreatedAt: now.Add(-1 * time.Hour)}
	post3 := &database.Post{ID: 3, Title: "Post 3", CreatedAt: now}
	repo.Create(post1)
	repo.Create(post2)
	repo.Create(post3)
	voteRepo := testutils.NewMockVoteRepository()
	voteService := NewVoteService(voteRepo, repo, nil)
	postQueries := NewPostQueries(repo, voteService)
	posts, err := postQueries.GetNewest(2, VoteContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) != 2 {
		t.Errorf("expected 2 posts, got %d", len(posts))
	}
	if len(posts) == 0 {
		t.Error("expected at least one post")
	}
}

func TestPostQueries_GetBySort(t *testing.T) {
	repo := testutils.NewMockPostRepository()
	post1 := &database.Post{ID: 1, Title: "Post 1", Score: 10}
	post2 := &database.Post{ID: 2, Title: "Post 2", Score: 15}
	repo.Create(post1)
	repo.Create(post2)
	voteRepo := testutils.NewMockVoteRepository()
	voteService := NewVoteService(voteRepo, repo, nil)
	postQueries := NewPostQueries(repo, voteService)
	tests := []struct {
		name      string
		sort      string
		expectTop bool
	}{
		{"new sort", "new", false},
		{"newest sort", "newest", false},
		{"latest sort", "latest", false},
		{"default sort", "", true},
		{"invalid sort", "invalid", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			posts, err := postQueries.GetBySort(tt.sort, 10, VoteContext{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(posts) == 0 {
				t.Error("expected at least one post")
			}
		})
	}
}

func TestPostQueries_GetSearch(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		setupRepo     func() *testutils.MockPostRepository
		opts          QueryOptions
		ctx           VoteContext
		expectedCount int
		expectedError bool
	}{
		{
			name:  "successful search",
			query: "test",
			setupRepo: func() *testutils.MockPostRepository {
				repo := testutils.NewMockPostRepository()
				repo.Create(&database.Post{ID: 1, Title: "Test Post", Score: 10})
				repo.Create(&database.Post{ID: 2, Title: "Another Post", Score: 5})
				return repo
			},
			opts: QueryOptions{
				Limit:  10,
				Offset: 0,
			},
			ctx:           VoteContext{},
			expectedCount: 1,
			expectedError: false,
		},
		{
			name:  "search with pagination",
			query: "post",
			setupRepo: func() *testutils.MockPostRepository {
				repo := testutils.NewMockPostRepository()
				repo.Create(&database.Post{ID: 1, Title: "Post 1", Score: 10})
				repo.Create(&database.Post{ID: 2, Title: "Post 2", Score: 5})
				repo.Create(&database.Post{ID: 3, Title: "Post 3", Score: 15})
				return repo
			},
			opts: QueryOptions{
				Limit:  2,
				Offset: 0,
			},
			ctx:           VoteContext{},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:  "search error",
			query: "test",
			setupRepo: func() *testutils.MockPostRepository {
				repo := testutils.NewMockPostRepository()
				repo.SearchErr = errors.New("search error")
				return repo
			},
			opts: QueryOptions{
				Limit:  10,
				Offset: 0,
			},
			ctx:           VoteContext{},
			expectedCount: 0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			voteRepo := testutils.NewMockVoteRepository()
			voteService := NewVoteService(voteRepo, repo, nil)
			postQueries := NewPostQueries(repo, voteService)
			posts, err := postQueries.GetSearch(tt.query, tt.opts, tt.ctx)
			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(posts) < tt.expectedCount {
					t.Errorf("expected at least %d posts, got %d", tt.expectedCount, len(posts))
				}
			}
		})
	}
}

func TestPostQueries_GetByID(t *testing.T) {
	tests := []struct {
		name          string
		postID        uint
		setupRepo     func() *testutils.MockPostRepository
		ctx           VoteContext
		expectedError bool
		expectedID    uint
	}{
		{
			name:   "successful retrieval",
			postID: 1,
			setupRepo: func() *testutils.MockPostRepository {
				repo := testutils.NewMockPostRepository()
				repo.Create(&database.Post{ID: 1, Title: "Test Post", Score: 10})
				return repo
			},
			ctx:           VoteContext{UserID: 1},
			expectedError: false,
			expectedID:    1,
		},
		{
			name:   "post not found",
			postID: 999,
			setupRepo: func() *testutils.MockPostRepository {
				return testutils.NewMockPostRepository()
			},
			ctx:           VoteContext{UserID: 1},
			expectedError: true,
			expectedID:    0,
		},
		{
			name:   "repository error",
			postID: 1,
			setupRepo: func() *testutils.MockPostRepository {
				repo := testutils.NewMockPostRepository()
				repo.GetErr = errors.New("database error")
				return repo
			},
			ctx:           VoteContext{UserID: 1},
			expectedError: true,
			expectedID:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			voteRepo := testutils.NewMockVoteRepository()
			voteService := NewVoteService(voteRepo, repo, nil)
			postQueries := NewPostQueries(repo, voteService)
			post, err := postQueries.GetByID(tt.postID, tt.ctx)
			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if post == nil {
					t.Fatal("expected post to be returned")
				}
				if post.ID != tt.expectedID {
					t.Errorf("expected post ID %d, got %d", tt.expectedID, post.ID)
				}
			}
		})
	}
}

func TestPostQueries_GetByID_WithVoteEnrichment(t *testing.T) {
	repo := testutils.NewMockPostRepository()
	post := &database.Post{ID: 1, Title: "Test Post", Score: 10}
	repo.Create(post)
	voteRepo := testutils.NewMockVoteRepository()
	voteService := NewVoteService(voteRepo, repo, nil)
	userID := uint(1)
	voteRepo.Create(&database.Vote{
		UserID: &userID,
		PostID: 1,
		Type:   database.VoteDown,
	})
	postQueries := NewPostQueries(repo, voteService)
	ctx := VoteContext{
		UserID:    1,
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}
	retrievedPost, err := postQueries.GetByID(1, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retrievedPost.CurrentVote != database.VoteDown {
		t.Errorf("expected CurrentVote to be VoteDown, got %s", retrievedPost.CurrentVote)
	}
}

func TestPostQueries_GetByUserID(t *testing.T) {
	tests := []struct {
		name          string
		userID        uint
		setupRepo     func() *testutils.MockPostRepository
		opts          QueryOptions
		ctx           VoteContext
		expectedCount int
		expectedError bool
	}{
		{
			name:   "successful retrieval",
			userID: 1,
			setupRepo: func() *testutils.MockPostRepository {
				repo := testutils.NewMockPostRepository()
				authorID1 := uint(1)
				authorID2 := uint(2)
				repo.Create(&database.Post{ID: 1, Title: "User 1 Post", AuthorID: &authorID1, Score: 10})
				repo.Create(&database.Post{ID: 2, Title: "User 2 Post", AuthorID: &authorID2, Score: 5})
				repo.Create(&database.Post{ID: 3, Title: "User 1 Post 2", AuthorID: &authorID1, Score: 15})
				return repo
			},
			opts: QueryOptions{
				Limit:  10,
				Offset: 0,
			},
			ctx:           VoteContext{UserID: 1},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:   "with pagination",
			userID: 1,
			setupRepo: func() *testutils.MockPostRepository {
				repo := testutils.NewMockPostRepository()
				authorID := uint(1)
				repo.Create(&database.Post{ID: 1, Title: "Post 1", AuthorID: &authorID, Score: 10})
				repo.Create(&database.Post{ID: 2, Title: "Post 2", AuthorID: &authorID, Score: 5})
				repo.Create(&database.Post{ID: 3, Title: "Post 3", AuthorID: &authorID, Score: 15})
				return repo
			},
			opts: QueryOptions{
				Limit:  2,
				Offset: 0,
			},
			ctx:           VoteContext{UserID: 1},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:   "repository error",
			userID: 1,
			setupRepo: func() *testutils.MockPostRepository {
				repo := testutils.NewMockPostRepository()
				repo.GetErr = errors.New("database error")
				return repo
			},
			opts: QueryOptions{
				Limit:  10,
				Offset: 0,
			},
			ctx:           VoteContext{UserID: 1},
			expectedCount: 0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo()
			voteRepo := testutils.NewMockVoteRepository()
			voteService := NewVoteService(voteRepo, repo, nil)
			postQueries := NewPostQueries(repo, voteService)
			posts, err := postQueries.GetByUserID(tt.userID, tt.opts, tt.ctx)
			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(posts) < tt.expectedCount {
					t.Errorf("expected at least %d posts, got %d", tt.expectedCount, len(posts))
				}
			}
		})
	}
}

func TestPostQueries_WithoutVoteService(t *testing.T) {
	repo := testutils.NewMockPostRepository()
	repo.Create(&database.Post{ID: 1, Title: "Test Post", Score: 10})
	postQueries := NewPostQueries(repo, nil)
	ctx := VoteContext{
		UserID:    1,
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}
	post, err := postQueries.GetByID(1, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if post == nil {
		t.Fatal("expected post to be returned")
	}
	if post.CurrentVote != "" {
		t.Errorf("expected CurrentVote to be empty when voteService is nil, got %s", post.CurrentVote)
	}
}

func TestPostQueries_WithIPBasedVote(t *testing.T) {
	repo := testutils.NewMockPostRepository()
	post := &database.Post{ID: 1, Title: "Test Post", Score: 10}
	repo.Create(post)
	voteRepo := testutils.NewMockVoteRepository()
	voteService := NewVoteService(voteRepo, repo, nil)
	voteHash := voteService.GenerateVoteHash("127.0.0.1", "test-agent", 1)
	voteRepo.Create(&database.Vote{
		PostID:   1,
		Type:     database.VoteUp,
		VoteHash: &voteHash,
	})
	postQueries := NewPostQueries(repo, voteService)
	ctx := VoteContext{
		UserID:    0,
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}
	retrievedPost, err := postQueries.GetByID(1, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retrievedPost.CurrentVote != database.VoteUp {
		t.Errorf("expected CurrentVote to be VoteUp for IP-based vote, got %s", retrievedPost.CurrentVote)
	}
}

func TestPostQueries_EnrichPostsWithVotes_NoVotes(t *testing.T) {
	repo := testutils.NewMockPostRepository()
	post1 := &database.Post{ID: 1, Title: "Post 1", Score: 10}
	post2 := &database.Post{ID: 2, Title: "Post 2", Score: 5}
	repo.Create(post1)
	repo.Create(post2)
	voteRepo := testutils.NewMockVoteRepository()
	voteService := NewVoteService(voteRepo, repo, nil)
	postQueries := NewPostQueries(repo, voteService)
	ctx := VoteContext{
		UserID:    1,
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}
	posts, err := postQueries.GetAll(QueryOptions{Limit: 10, Offset: 0}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) != 2 {
		t.Errorf("expected 2 posts, got %d", len(posts))
	}
	for _, post := range posts {
		if post.CurrentVote != "" {
			t.Errorf("expected CurrentVote to be empty when no votes exist, got %s", post.CurrentVote)
		}
	}
}

func TestPostQueries_GetByID_NotFound(t *testing.T) {
	repo := testutils.NewMockPostRepository()
	voteRepo := testutils.NewMockVoteRepository()
	voteService := NewVoteService(voteRepo, repo, nil)
	postQueries := NewPostQueries(repo, voteService)
	post, err := postQueries.GetByID(999, VoteContext{})
	if err == nil {
		t.Fatal("expected error for non-existent post")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
	}
	if post != nil {
		t.Error("expected nil post when not found")
	}
}
