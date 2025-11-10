package services

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm"
	"goyco/internal/database"
	"goyco/internal/repositories"
)

type mockVoteRepo struct {
	votes               map[uint]*database.Vote
	byUserPost          map[string]*database.Vote
	byVoteHash          map[string]*database.Vote
	nextID              uint
	createErr           error
	getByUserAndPostErr error
	getByVoteHashErr    error
	getByPostIDErr      error
	updateErr           error
	deleteErr           error
	createCalls         int
	updateCalls         int
	deleteCalls         int
	mu                  sync.RWMutex
}

func newMockVoteRepo() *mockVoteRepo {
	return &mockVoteRepo{
		votes:      make(map[uint]*database.Vote),
		byUserPost: make(map[string]*database.Vote),
		byVoteHash: make(map[string]*database.Vote),
		nextID:     1,
	}
}

func (m *mockVoteRepo) Create(vote *database.Vote) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.createErr != nil {
		return m.createErr
	}

	var key string
	if vote.UserID != nil {
		key = m.key(*vote.UserID, vote.PostID)
	} else if vote.VoteHash != nil {
		key = *vote.VoteHash
	} else {
		return errors.New("vote must have either user_id or vote_hash")
	}

	if existingVote, exists := m.byUserPost[key]; exists {
		existingVote.Type = vote.Type
		existingVote.UpdatedAt = vote.UpdatedAt
		vote.ID = existingVote.ID
		return nil
	}

	vote.ID = m.nextID
	m.nextID++

	voteCopy := *vote
	m.votes[vote.ID] = &voteCopy
	m.byUserPost[key] = &voteCopy
	if vote.VoteHash != nil {
		m.byVoteHash[*vote.VoteHash] = &voteCopy
	}

	m.createCalls++
	return nil
}

func (m *mockVoteRepo) CreateOrUpdate(vote *database.Vote) error {
	return m.Create(vote)
}

func (m *mockVoteRepo) GetByID(id uint) (*database.Vote, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if vote, ok := m.votes[id]; ok {
		voteCopy := *vote
		return &voteCopy, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockVoteRepo) GetByUserAndPost(userID, postID uint) (*database.Vote, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getByUserAndPostErr != nil {
		return nil, m.getByUserAndPostErr
	}

	key := m.key(userID, postID)
	if vote, ok := m.byUserPost[key]; ok {
		voteCopy := *vote
		return &voteCopy, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockVoteRepo) GetByVoteHash(voteHash string) (*database.Vote, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getByVoteHashErr != nil {
		return nil, m.getByVoteHashErr
	}

	if vote, ok := m.byVoteHash[voteHash]; ok {
		voteCopy := *vote
		return &voteCopy, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockVoteRepo) GetByPostID(postID uint) ([]database.Vote, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getByPostIDErr != nil {
		return nil, m.getByPostIDErr
	}

	var votes []database.Vote
	for _, vote := range m.votes {
		if vote.PostID == postID {
			votes = append(votes, *vote)
		}
	}
	return votes, nil
}

func (m *mockVoteRepo) GetByUserID(userID uint) ([]database.Vote, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var votes []database.Vote
	for _, vote := range m.votes {
		if vote.UserID != nil && *vote.UserID == userID {
			votes = append(votes, *vote)
		}
	}
	return votes, nil
}

func (m *mockVoteRepo) Update(vote *database.Vote) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.updateErr != nil {
		return m.updateErr
	}

	if _, ok := m.votes[vote.ID]; !ok {
		return gorm.ErrRecordNotFound
	}

	voteCopy := *vote
	m.votes[vote.ID] = &voteCopy

	var key string
	if vote.UserID != nil {
		key = m.key(*vote.UserID, vote.PostID)
		m.byUserPost[key] = &voteCopy
	}
	if vote.VoteHash != nil {
		m.byVoteHash[*vote.VoteHash] = &voteCopy
	}

	m.updateCalls++
	return nil
}

func (m *mockVoteRepo) Delete(id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.deleteErr != nil {
		return m.deleteErr
	}

	vote, ok := m.votes[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}

	delete(m.votes, id)

	var key string
	if vote.UserID != nil {
		key = m.key(*vote.UserID, vote.PostID)
		delete(m.byUserPost, key)
	}
	if vote.VoteHash != nil {
		delete(m.byVoteHash, *vote.VoteHash)
	}

	m.deleteCalls++
	return nil
}

func (m *mockVoteRepo) Count() (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.votes)), nil
}

func (m *mockVoteRepo) CountByPostID(postID uint) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := int64(0)
	for _, vote := range m.votes {
		if vote.PostID == postID {
			count++
		}
	}
	return count, nil
}

func (m *mockVoteRepo) CountByUserID(userID uint) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := int64(0)
	for _, vote := range m.votes {
		if vote.UserID != nil && *vote.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (m *mockVoteRepo) WithTx(tx *gorm.DB) repositories.VoteRepository {
	return m
}

func (m *mockVoteRepo) key(userID, postID uint) string {
	return fmt.Sprintf("%d:%d", userID, postID)
}

type mockPostRepo struct {
	posts     map[uint]*database.Post
	nextID    uint
	getErr    error
	updateErr error
	mu        sync.RWMutex
}

func newMockPostRepo() *mockPostRepo {
	return &mockPostRepo{
		posts:  make(map[uint]*database.Post),
		nextID: 1,
	}
}

func (m *mockPostRepo) GetByID(id uint) (*database.Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getErr != nil {
		return nil, m.getErr
	}

	if post, ok := m.posts[id]; ok {
		postCopy := *post
		return &postCopy, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockPostRepo) Update(post *database.Post) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.updateErr != nil {
		return m.updateErr
	}

	if _, ok := m.posts[post.ID]; !ok {
		return gorm.ErrRecordNotFound
	}

	postCopy := *post
	m.posts[post.ID] = &postCopy
	return nil
}

func (m *mockPostRepo) Create(post *database.Post) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	post.ID = m.nextID
	m.nextID++

	postCopy := *post
	m.posts[post.ID] = &postCopy
	return nil
}

func (m *mockPostRepo) GetByURL(url string) (*database.Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, post := range m.posts {
		if post.URL == url {
			postCopy := *post
			return &postCopy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockPostRepo) GetByAuthorID(authorID uint) ([]database.Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var posts []database.Post
	for _, post := range m.posts {
		if post.AuthorID != nil && *post.AuthorID == authorID {
			posts = append(posts, *post)
		}
	}
	return posts, nil
}

func (m *mockPostRepo) GetAll(limit, offset int) ([]database.Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var posts []database.Post
	count := 0
	for _, post := range m.posts {
		if count >= offset && count < offset+limit {
			posts = append(posts, *post)
		}
		count++
	}
	return posts, nil
}

func (m *mockPostRepo) Count() (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.posts)), nil
}

func (m *mockPostRepo) Delete(id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.posts[id]; !ok {
		return gorm.ErrRecordNotFound
	}

	delete(m.posts, id)
	return nil
}

func (m *mockPostRepo) GetByUserID(userID uint, limit, offset int) ([]database.Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var posts []database.Post
	count := 0
	for _, post := range m.posts {
		if post.AuthorID != nil && *post.AuthorID == userID {
			if count >= offset && count < offset+limit {
				posts = append(posts, *post)
			}
			count++
		}
	}
	return posts, nil
}

func (m *mockPostRepo) CountByUserID(userID uint) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := int64(0)
	for _, post := range m.posts {
		if post.AuthorID != nil && *post.AuthorID == userID {
			count++
		}
	}
	return count, nil
}

func (m *mockPostRepo) GetTopPosts(limit int) ([]database.Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var posts []database.Post
	count := 0
	for _, post := range m.posts {
		if count < limit {
			posts = append(posts, *post)
			count++
		}
	}
	return posts, nil
}

func (m *mockPostRepo) GetNewestPosts(limit int) ([]database.Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var posts []database.Post
	count := 0
	for _, post := range m.posts {
		if count < limit {
			posts = append(posts, *post)
			count++
		}
	}
	return posts, nil
}

func (m *mockPostRepo) Search(query string, limit, offset int) ([]database.Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var posts []database.Post
	count := 0
	for _, post := range m.posts {
		if strings.Contains(strings.ToLower(post.Title), strings.ToLower(query)) {
			if count >= offset && count < offset+limit {
				posts = append(posts, *post)
			}
			count++
		}
	}
	return posts, nil
}

func (m *mockPostRepo) GetPostsByDeletedUsers() ([]database.Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var posts []database.Post
	for _, post := range m.posts {
		if post.AuthorID == nil {
			posts = append(posts, *post)
		}
	}
	return posts, nil
}

func (m *mockPostRepo) HardDeletePostsByDeletedUsers() (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := int64(0)
	for id, post := range m.posts {
		if post.AuthorID == nil {
			delete(m.posts, id)
			count++
		}
	}
	return count, nil
}

func (m *mockPostRepo) HardDeleteAll() (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := int64(len(m.posts))
	m.posts = make(map[uint]*database.Post)
	return count, nil
}

func (m *mockPostRepo) WithTx(tx *gorm.DB) repositories.PostRepository {
	return m
}

func TestVoteService_CastVote_Authenticated(t *testing.T) {
	voteRepo := newMockVoteRepo()
	postRepo := newMockPostRepo()
	service := NewVoteService(voteRepo, postRepo, nil)

	post := &database.Post{
		ID:        1,
		Title:     "Test Post",
		URL:       "https://example.com",
		AuthorID:  &[]uint{1}[0],
		UpVotes:   0,
		DownVotes: 0,
		Score:     0,
	}
	postRepo.posts[1] = post

	req := VoteRequest{
		UserID:    1,
		PostID:    1,
		Type:      database.VoteUp,
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}

	result, err := service.CastVote(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if result.Type != database.VoteUp {
		t.Errorf("Expected vote type 'up', got '%v'", result.Type)
	}

	if result.UpVotes != 1 {
		t.Errorf("Expected up votes to be 1, got %d", result.UpVotes)
	}

	if result.DownVotes != 0 {
		t.Errorf("Expected down votes to be 0, got %d", result.DownVotes)
	}

	if result.Score != 1 {
		t.Errorf("Expected score to be 1, got %d", result.Score)
	}

	if result.IsUnauthenticated {
		t.Error("Expected IsUnauthenticated to be false for authenticated vote")
	}
}

func TestVoteService_CastVote_Unauthenticated(t *testing.T) {
	voteRepo := newMockVoteRepo()
	postRepo := newMockPostRepo()
	service := NewVoteService(voteRepo, postRepo, nil)

	post := &database.Post{
		ID:        1,
		Title:     "Test Post",
		URL:       "https://example.com",
		AuthorID:  &[]uint{1}[0],
		UpVotes:   0,
		DownVotes: 0,
		Score:     0,
	}
	postRepo.posts[1] = post

	req := VoteRequest{
		UserID:    0,
		PostID:    1,
		Type:      database.VoteUp,
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}

	result, err := service.CastVote(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if result.Type != database.VoteUp {
		t.Errorf("Expected vote type 'up', got '%v'", result.Type)
	}

	if result.UpVotes != 1 {
		t.Errorf("Expected up votes to be 1, got %d", result.UpVotes)
	}

	if result.DownVotes != 0 {
		t.Errorf("Expected down votes to be 0, got %d", result.DownVotes)
	}

	if result.Score != 1 {
		t.Errorf("Expected score to be 1, got %d", result.Score)
	}

	if !result.IsUnauthenticated {
		t.Error("Expected IsUnauthenticated to be true for unauthenticated vote")
	}
}

func TestVoteService_CastVote_UpdateExisting(t *testing.T) {
	voteRepo := newMockVoteRepo()
	postRepo := newMockPostRepo()
	service := NewVoteService(voteRepo, postRepo, nil)

	post := &database.Post{
		ID:        1,
		Title:     "Test Post",
		URL:       "https://example.com",
		AuthorID:  &[]uint{1}[0],
		UpVotes:   0,
		DownVotes: 0,
		Score:     0,
	}
	postRepo.posts[1] = post

	req := VoteRequest{
		UserID:    1,
		PostID:    1,
		Type:      database.VoteUp,
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}

	_, err := service.CastVote(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	req.Type = database.VoteDown
	result, err := service.CastVote(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.UpVotes != 0 {
		t.Errorf("Expected up votes to be 0, got %d", result.UpVotes)
	}

	if result.DownVotes != 1 {
		t.Errorf("Expected down votes to be 1, got %d", result.DownVotes)
	}

	if result.Score != -1 {
		t.Errorf("Expected score to be -1, got %d", result.Score)
	}
}

func TestVoteService_CastVote_RemoveVote(t *testing.T) {
	voteRepo := newMockVoteRepo()
	postRepo := newMockPostRepo()
	service := NewVoteService(voteRepo, postRepo, nil)

	post := &database.Post{
		ID:        1,
		Title:     "Test Post",
		URL:       "https://example.com",
		AuthorID:  &[]uint{1}[0],
		UpVotes:   0,
		DownVotes: 0,
		Score:     0,
	}
	postRepo.posts[1] = post

	req := VoteRequest{
		UserID:    1,
		PostID:    1,
		Type:      database.VoteUp,
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}

	_, err := service.CastVote(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	req.Type = database.VoteNone
	result, err := service.CastVote(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.UpVotes != 0 {
		t.Errorf("Expected up votes to be 0, got %d", result.UpVotes)
	}

	if result.DownVotes != 0 {
		t.Errorf("Expected down votes to be 0, got %d", result.DownVotes)
	}

	if result.Score != 0 {
		t.Errorf("Expected score to be 0, got %d", result.Score)
	}
}

func TestVoteService_GetUserVote_Authenticated(t *testing.T) {
	voteRepo := newMockVoteRepo()
	postRepo := newMockPostRepo()
	service := NewVoteService(voteRepo, postRepo, nil)

	userID := uint(1)
	vote := &database.Vote{
		ID:     1,
		UserID: &userID,
		PostID: 1,
		Type:   database.VoteUp,
	}
	voteRepo.votes[1] = vote
	voteRepo.byUserPost["1:1"] = vote

	result, err := service.GetUserVote(1, 1, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("Expected vote, got nil")
	}

	if result.Type != database.VoteUp {
		t.Errorf("Expected vote type 'up', got '%v'", result.Type)
	}

	if result.UserID == nil || *result.UserID != 1 {
		t.Errorf("Expected user ID 1, got %v", result.UserID)
	}
}

func TestVoteService_GetUserVote_Unauthenticated(t *testing.T) {
	voteRepo := newMockVoteRepo()
	postRepo := newMockPostRepo()
	service := NewVoteService(voteRepo, postRepo, nil)

	voteHash := service.GenerateVoteHash("127.0.0.1", "test-agent", 1)
	vote := &database.Vote{
		ID:       1,
		UserID:   nil,
		PostID:   1,
		Type:     database.VoteUp,
		VoteHash: &voteHash,
	}
	voteRepo.votes[1] = vote
	voteRepo.byVoteHash[voteHash] = vote

	result, err := service.GetUserVote(0, 1, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("Expected vote, got nil")
	}

	if result.Type != database.VoteUp {
		t.Errorf("Expected vote type 'up', got '%v'", result.Type)
	}

	if result.UserID != nil {
		t.Error("Expected UserID to be nil for unauthenticated vote")
	}

	if result.VoteHash == nil || *result.VoteHash != voteHash {
		t.Errorf("Expected vote hash '%s', got %v", voteHash, result.VoteHash)
	}
}

func TestVoteService_GetPostVotes(t *testing.T) {
	voteRepo := newMockVoteRepo()
	postRepo := newMockPostRepo()
	service := NewVoteService(voteRepo, postRepo, nil)

	userID1 := uint(1)
	userID2 := uint(2)
	voteHash := "test-hash"

	vote1 := &database.Vote{
		ID:     1,
		UserID: &userID1,
		PostID: 1,
		Type:   database.VoteUp,
	}
	vote2 := &database.Vote{
		ID:     2,
		UserID: &userID2,
		PostID: 1,
		Type:   database.VoteDown,
	}
	vote3 := &database.Vote{
		ID:       3,
		UserID:   nil,
		PostID:   1,
		Type:     database.VoteUp,
		VoteHash: &voteHash,
	}

	voteRepo.votes[1] = vote1
	voteRepo.votes[2] = vote2
	voteRepo.votes[3] = vote3

	votes, err := service.GetPostVotes(1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(votes) != 3 {
		t.Errorf("Expected 3 votes, got %d", len(votes))
	}

	hasAuthenticated := false
	hasUnauthenticated := false
	for _, vote := range votes {
		if vote.UserID != nil {
			hasAuthenticated = true
		}
		if vote.VoteHash != nil {
			hasUnauthenticated = true
		}
	}

	if !hasAuthenticated {
		t.Error("Expected to find authenticated votes")
	}

	if !hasUnauthenticated {
		t.Error("Expected to find unauthenticated votes")
	}
}

func TestVoteService_GetVoteStatistics(t *testing.T) {
	voteRepo := newMockVoteRepo()
	postRepo := newMockPostRepo()
	service := NewVoteService(voteRepo, postRepo, nil)

	userID1 := uint(1)
	userID2 := uint(2)
	voteHash := "test-hash"

	vote1 := &database.Vote{
		ID:     1,
		UserID: &userID1,
		PostID: 1,
		Type:   database.VoteUp,
	}
	vote2 := &database.Vote{
		ID:     2,
		UserID: &userID2,
		PostID: 1,
		Type:   database.VoteDown,
	}
	vote3 := &database.Vote{
		ID:       3,
		UserID:   nil,
		PostID:   1,
		Type:     database.VoteUp,
		VoteHash: &voteHash,
	}

	voteRepo.votes[1] = vote1
	voteRepo.votes[2] = vote2
	voteRepo.votes[3] = vote3

	authenticatedCount, anonymousCount, err := service.GetVoteStatistics()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if authenticatedCount != 3 {
		t.Errorf("Expected total count to be 3, got %d", authenticatedCount)
	}

	if anonymousCount != 0 {
		t.Errorf("Expected unauthenticated count to be 0, got %d", anonymousCount)
	}
}

func TestVoteService_Validation(t *testing.T) {
	voteRepo := newMockVoteRepo()
	postRepo := newMockPostRepo()
	service := NewVoteService(voteRepo, postRepo, nil)

	req := VoteRequest{
		UserID:    1,
		PostID:    0,
		Type:      database.VoteUp,
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}

	_, err := service.CastVote(req)
	if err == nil {
		t.Error("Expected error for missing post ID")
	}

	req.PostID = 1
	req.Type = "invalid"

	_, err = service.CastVote(req)
	if err == nil {
		t.Error("Expected error for invalid vote type")
	}
}

func TestVoteService_PostNotFound(t *testing.T) {
	voteRepo := newMockVoteRepo()
	postRepo := newMockPostRepo()
	service := NewVoteService(voteRepo, postRepo, nil)

	req := VoteRequest{
		UserID:    1,
		PostID:    999,
		Type:      database.VoteUp,
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}

	_, err := service.CastVote(req)
	if err == nil {
		t.Error("Expected error for non-existent post")
	}

	if !strings.Contains(err.Error(), "post not found") {
		t.Errorf("Expected 'post not found' error, got %v", err)
	}
}
