package testutils

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"goyco/internal/database"
	"goyco/internal/repositories"

	"gorm.io/gorm"
)

type MockEmailSender struct {
	sendFunc               func(to, subject, body string) error
	lastVerificationToken  string
	lastDeletionToken      string
	lastPasswordResetToken string
	mu                     sync.Mutex
}

func (m *MockEmailSender) Send(to, subject, body string) error {
	if m.sendFunc != nil {
		return m.sendFunc(to, subject, body)
	}

	if len(body) == 0 {
		return nil
	}

	normalized := strings.ToLower(strings.TrimSpace(subject))

	token := extractTokenFromBody(body)

	switch {
	case strings.Contains(normalized, "resend") && strings.Contains(normalized, "confirm"):
		m.SetVerificationToken(defaultIfEmpty(token, "test-verification-token"))
	case strings.Contains(normalized, "confirm your goyco account") || strings.Contains(normalized, "confirm your account"):
		m.SetVerificationToken(defaultIfEmpty(token, "test-verification-token"))
	case strings.Contains(normalized, "confirm") && strings.Contains(normalized, "email"):
		m.SetVerificationToken(defaultIfEmpty(token, "test-verification-token"))
	case strings.Contains(normalized, "confirm your new email"):
		m.SetVerificationToken(defaultIfEmpty(token, "test-verification-token"))
	case strings.Contains(normalized, "account deletion"):
		m.SetDeletionToken(defaultIfEmpty(token, "test-deletion-token"))
	case strings.Contains(normalized, "password reset") || strings.Contains(normalized, "reset your") || strings.Contains(normalized, "reset password"):
		m.SetPasswordResetToken(defaultIfEmpty(token, "test-password-reset-token"))
	}

	return nil
}

func (m *MockEmailSender) GetLastVerificationToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastVerificationToken
}

func (m *MockEmailSender) GetLastDeletionToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastDeletionToken
}

func (m *MockEmailSender) GetLastPasswordResetToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastPasswordResetToken
}

func (m *MockEmailSender) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastVerificationToken = ""
	m.lastDeletionToken = ""
	m.lastPasswordResetToken = ""
}

func (m *MockEmailSender) VerificationToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastVerificationToken
}

func (m *MockEmailSender) SetVerificationToken(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastVerificationToken = token
}

func (m *MockEmailSender) DeletionToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastDeletionToken
}

func (m *MockEmailSender) PasswordResetToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastPasswordResetToken
}

func (m *MockEmailSender) SetDeletionToken(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastDeletionToken = token
}

func (m *MockEmailSender) SetPasswordResetToken(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastPasswordResetToken = token
}

func defaultIfEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func extractTokenFromBody(body string) string {
	_, after, ok := strings.Cut(body, "token=")
	if !ok {
		return ""
	}

	tokenPart := after

	if delimIdx := strings.IndexAny(tokenPart, "&\"'\\\r\n <>"); delimIdx != -1 {
		tokenPart = tokenPart[:delimIdx]
	}

	trimmed := strings.Trim(tokenPart, "\"' ")
	if trimmed == "" {
		return ""
	}

	unescaped, err := url.QueryUnescape(trimmed)
	if err != nil {
		return trimmed
	}

	return unescaped
}

type MockTitleFetcher struct {
	fetchFunc func(ctx context.Context, url string) (string, error)
	title     string
	err       error
}

func (m *MockTitleFetcher) FetchTitle(ctx context.Context, url string) (string, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, url)
	}
	if m.err != nil {
		return "", m.err
	}
	return m.title, nil
}

func (m *MockTitleFetcher) SetTitle(title string) {
	m.title = title
	m.err = nil
}

func (m *MockTitleFetcher) SetError(err error) {
	m.err = err
	m.title = ""
}

type MockUserRepository struct {
	users                      map[uint]*database.User
	usersByUsername            map[string]*database.User
	usersByEmail               map[string]*database.User
	usersByVerificationToken   map[string]*database.User
	usersByPasswordResetToken  map[string]*database.User
	deletedUsers               map[uint]*database.User
	nextID                     uint
	createErr                  error
	getByIDErr                 error
	getByUsernameErr           error
	getByEmailErr              error
	getByVerificationTokenErr  error
	getByPasswordResetTokenErr error
	updateErr                  error
	deleteErr                  error
	mu                         sync.RWMutex

	GetAllFunc          func(limit, offset int) ([]database.User, error)
	GetDeletedUsersFunc func() ([]database.User, error)
	HardDeleteAllFunc   func() (int64, error)

	GetErr    error
	DeleteErr error

	Users        map[uint]*database.User
	DeletedUsers map[uint]*database.User
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users:                     make(map[uint]*database.User),
		usersByUsername:           make(map[string]*database.User),
		usersByEmail:              make(map[string]*database.User),
		usersByVerificationToken:  make(map[string]*database.User),
		usersByPasswordResetToken: make(map[string]*database.User),
		deletedUsers:              make(map[uint]*database.User),
		nextID:                    1,
		Users:                     make(map[uint]*database.User),
		DeletedUsers:              make(map[uint]*database.User),
	}
}

func (m *MockUserRepository) Create(user *database.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.createErr != nil {
		return m.createErr
	}

	user.ID = m.nextID
	m.nextID++

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	userCopy := *user
	m.users[user.ID] = &userCopy
	m.usersByUsername[user.Username] = &userCopy
	m.usersByEmail[user.Email] = &userCopy
	m.Users[user.ID] = &userCopy

	if user.EmailVerificationToken != "" {
		m.usersByVerificationToken[user.EmailVerificationToken] = &userCopy
	}
	if user.PasswordResetToken != "" {
		m.usersByPasswordResetToken[user.PasswordResetToken] = &userCopy
	}

	return nil
}

func (m *MockUserRepository) GetByID(id uint) (*database.User, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}

	if user, ok := m.users[id]; ok {
		userCopy := *user
		return &userCopy, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockUserRepository) GetByUsername(username string) (*database.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getByUsernameErr != nil {
		return nil, m.getByUsernameErr
	}

	if user, ok := m.usersByUsername[username]; ok {
		userCopy := *user
		return &userCopy, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockUserRepository) GetByUsernameIncludingDeleted(username string) (*database.User, error) {
	return m.GetByUsername(username)
}

func (m *MockUserRepository) GetByIDIncludingDeleted(id uint) (*database.User, error) {
	return m.GetByID(id)
}

func (m *MockUserRepository) GetByEmail(email string) (*database.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getByEmailErr != nil {
		return nil, m.getByEmailErr
	}

	if user, ok := m.usersByEmail[email]; ok {
		userCopy := *user
		return &userCopy, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockUserRepository) GetByVerificationToken(token string) (*database.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getByVerificationTokenErr != nil {
		return nil, m.getByVerificationTokenErr
	}

	if user, ok := m.usersByVerificationToken[token]; ok {
		userCopy := *user
		return &userCopy, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockUserRepository) GetByPasswordResetToken(token string) (*database.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getByPasswordResetTokenErr != nil {
		return nil, m.getByPasswordResetTokenErr
	}

	if user, ok := m.usersByPasswordResetToken[token]; ok {
		userCopy := *user
		return &userCopy, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockUserRepository) GetAll(limit, offset int) ([]database.User, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	if m.GetAllFunc != nil {
		return m.GetAllFunc(limit, offset)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var users []database.User
	count := 0
	for _, user := range m.users {
		if count >= offset && count < offset+limit {
			users = append(users, *user)
		}
		count++
	}
	return users, nil
}

func (m *MockUserRepository) Update(user *database.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.updateErr != nil {
		return m.updateErr
	}

	if _, ok := m.users[user.ID]; !ok {
		return gorm.ErrRecordNotFound
	}

	user.UpdatedAt = time.Now()

	userCopy := *user
	m.users[user.ID] = &userCopy
	m.usersByUsername[user.Username] = &userCopy
	m.usersByEmail[user.Email] = &userCopy

	return nil
}

func (m *MockUserRepository) Delete(id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.DeleteErr != nil {
		return m.DeleteErr
	}

	if user, ok := m.users[id]; ok {
		delete(m.users, id)
		delete(m.usersByUsername, user.Username)
		delete(m.usersByEmail, user.Email)
		return nil
	}
	return gorm.ErrRecordNotFound
}

func (m *MockUserRepository) HardDelete(id uint) error {
	return m.Delete(id)
}

func (m *MockUserRepository) SoftDeleteWithPosts(id uint) error {
	return m.Delete(id)
}

func (m *MockUserRepository) GetPosts(userID uint, limit, offset int) ([]database.Post, error) {
	return []database.Post{}, nil
}

func (m *MockUserRepository) Lock(id uint) error {
	return nil
}

func (m *MockUserRepository) Unlock(id uint) error {
	return nil
}

func (m *MockUserRepository) GetDeletedUsers() ([]database.User, error) {
	if m.GetDeletedUsersFunc != nil {
		return m.GetDeletedUsersFunc()
	}
	return []database.User{}, nil
}

func (m *MockUserRepository) GetByUsernamePrefix(prefix string) (*database.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, user := range m.users {
		if len(user.Username) >= len(prefix) && user.Username[:len(prefix)] == prefix {
			if len(user.Email) >= 13 && strings.HasSuffix(user.Email, "@goyco.local") {
				emailPrefix := user.Email[:len(user.Email)-13]
				if len(emailPrefix) >= len(prefix) && emailPrefix[:len(prefix)] == prefix {
					return user, nil
				}
			}
		}
	}

	return nil, gorm.ErrRecordNotFound
}

func (m *MockUserRepository) HardDeleteAll() (int64, error) {
	if m.HardDeleteAllFunc != nil {
		return m.HardDeleteAllFunc()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	count := int64(len(m.users))
	m.users = make(map[uint]*database.User)
	m.usersByUsername = make(map[string]*database.User)
	m.usersByEmail = make(map[string]*database.User)
	m.usersByVerificationToken = make(map[string]*database.User)
	m.usersByPasswordResetToken = make(map[string]*database.User)

	return count, nil
}

func (m *MockUserRepository) Count() (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.users)), nil
}

func (m *MockUserRepository) WithTx(tx *gorm.DB) repositories.UserRepository {
	return m
}

func (m *MockUserRepository) SetCreateError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createErr = err
}

func (m *MockUserRepository) SetGetByIDError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getByIDErr = err
}

func (m *MockUserRepository) SetGetByUsernameError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getByUsernameErr = err
}

func (m *MockUserRepository) SetGetByEmailError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getByEmailErr = err
}

func (m *MockUserRepository) SetUpdateError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateErr = err
}

func (m *MockUserRepository) SetDeleteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteErr = err
}

type MockPostRepository struct {
	createFunc                        func(*database.Post) error
	getByIDFunc                       func(uint) (*database.Post, error)
	getAllFunc                        func(int, int) ([]database.Post, error)
	getByUserIDFunc                   func(uint, int, int) ([]database.Post, error)
	updateFunc                        func(*database.Post) error
	deleteFunc                        func(uint) error
	countFunc                         func() (int64, error)
	countByUserIDFunc                 func(uint) (int64, error)
	getTopPostsFunc                   func(int) ([]database.Post, error)
	getNewestPostsFunc                func(int) ([]database.Post, error)
	searchFunc                        func(string, int, int) ([]database.Post, error)
	getPostsByDeletedUsersFunc        func() ([]database.Post, error)
	hardDeletePostsByDeletedUsersFunc func() (int64, error)
	hardDeleteAllFunc                 func() (int64, error)
	withTxFunc                        func(*gorm.DB) repositories.PostRepository

	GetPostsByDeletedUsersFunc        func() ([]database.Post, error)
	HardDeletePostsByDeletedUsersFunc func() (int64, error)
	HardDeleteAllFunc                 func() (int64, error)
	CountFunc                         func() (int64, error)

	posts  map[uint]*database.Post
	nextID uint
	mu     sync.RWMutex

	SearchCalls []SearchCall

	GetErr    error
	DeleteErr error
	SearchErr error

	Posts map[uint]*database.Post
}

type SearchCall struct {
	Query  string
	Limit  int
	Offset int
}

func NewMockPostRepository() *MockPostRepository {
	return &MockPostRepository{
		posts:  make(map[uint]*database.Post),
		nextID: 1,
		Posts:  make(map[uint]*database.Post),
	}
}

func (m *MockPostRepository) Create(post *database.Post) error {
	if m.createFunc != nil {
		return m.createFunc(post)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	post.ID = m.nextID
	m.nextID++

	postCopy := *post
	m.posts[post.ID] = &postCopy
	m.Posts[post.ID] = &postCopy
	return nil
}

func (m *MockPostRepository) GetByID(id uint) (*database.Post, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	if m.getByIDFunc != nil {
		return m.getByIDFunc(id)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if post, ok := m.posts[id]; ok {
		postCopy := *post
		return &postCopy, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockPostRepository) GetAll(limit, offset int) ([]database.Post, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	if m.getAllFunc != nil {
		return m.getAllFunc(limit, offset)
	}

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

func (m *MockPostRepository) GetByUserID(userID uint, limit, offset int) ([]database.Post, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	if m.getByUserIDFunc != nil {
		return m.getByUserIDFunc(userID, limit, offset)
	}

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

func (m *MockPostRepository) Update(post *database.Post) error {
	if m.updateFunc != nil {
		return m.updateFunc(post)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.posts[post.ID]; !ok {
		return gorm.ErrRecordNotFound
	}

	postCopy := *post
	m.posts[post.ID] = &postCopy
	m.Posts[post.ID] = &postCopy
	return nil
}

func (m *MockPostRepository) Delete(id uint) error {
	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	if m.deleteFunc != nil {
		return m.deleteFunc(id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.posts[id]; !ok {
		return gorm.ErrRecordNotFound
	}

	delete(m.posts, id)
	return nil
}

func (m *MockPostRepository) Count() (int64, error) {
	if m.CountFunc != nil {
		return m.CountFunc()
	}
	if m.countFunc != nil {
		return m.countFunc()
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.posts)), nil
}

func (m *MockPostRepository) CountByUserID(userID uint) (int64, error) {
	if m.countByUserIDFunc != nil {
		return m.countByUserIDFunc(userID)
	}

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

func (m *MockPostRepository) GetTopPosts(limit int) ([]database.Post, error) {
	if m.getTopPostsFunc != nil {
		return m.getTopPostsFunc(limit)
	}
	return m.GetAll(limit, 0)
}

func (m *MockPostRepository) GetNewestPosts(limit int) ([]database.Post, error) {
	if m.getNewestPostsFunc != nil {
		return m.getNewestPostsFunc(limit)
	}
	return m.GetAll(limit, 0)
}

func (m *MockPostRepository) Search(query string, limit, offset int) ([]database.Post, error) {
	if m.SearchErr != nil {
		return nil, m.SearchErr
	}

	m.mu.Lock()
	m.SearchCalls = append(m.SearchCalls, SearchCall{
		Query:  query,
		Limit:  limit,
		Offset: offset,
	})
	m.mu.Unlock()

	if m.searchFunc != nil {
		return m.searchFunc(query, limit, offset)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var posts []database.Post
	count := 0
	for _, post := range m.posts {
		if containsIgnoreCase(post.Title, query) || containsIgnoreCase(post.Content, query) {
			if count >= offset && count < offset+limit {
				posts = append(posts, *post)
			}
			count++
		}
	}
	return posts, nil
}

func (m *MockPostRepository) WithTx(tx *gorm.DB) repositories.PostRepository {
	if m.withTxFunc != nil {
		return m.withTxFunc(tx)
	}
	return m
}

func (m *MockPostRepository) GetPostsByDeletedUsers() ([]database.Post, error) {
	if m.GetPostsByDeletedUsersFunc != nil {
		return m.GetPostsByDeletedUsersFunc()
	}
	if m.getPostsByDeletedUsersFunc != nil {
		return m.getPostsByDeletedUsersFunc()
	}
	return []database.Post{}, nil
}

func (m *MockPostRepository) HardDeletePostsByDeletedUsers() (int64, error) {
	if m.HardDeletePostsByDeletedUsersFunc != nil {
		return m.HardDeletePostsByDeletedUsersFunc()
	}
	if m.hardDeletePostsByDeletedUsersFunc != nil {
		return m.hardDeletePostsByDeletedUsersFunc()
	}
	return 0, nil
}

func (m *MockPostRepository) HardDeleteAll() (int64, error) {
	if m.HardDeleteAllFunc != nil {
		return m.HardDeleteAllFunc()
	}
	if m.hardDeleteAllFunc != nil {
		return m.hardDeleteAllFunc()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	count := int64(len(m.posts))
	m.posts = make(map[uint]*database.Post)
	return count, nil
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr)
}

type MockVoteRepository struct {
	votes      map[uint]*database.Vote
	byUserPost map[string]*database.Vote
	nextID     uint
	createErr  error
	updateErr  error
	deleteErr  error
	mu         sync.RWMutex

	DeleteErr error
}

func NewMockVoteRepository() *MockVoteRepository {
	return &MockVoteRepository{
		votes:      make(map[uint]*database.Vote),
		byUserPost: make(map[string]*database.Vote),
		nextID:     1,
	}
}

func (m *MockVoteRepository) Create(vote *database.Vote) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.createErr != nil {
		return m.createErr
	}

	var key string
	if vote.UserID != nil {
		key = m.key(*vote.UserID, vote.PostID)
	} else {
		key = fmt.Sprintf("anon-%d", vote.PostID)
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
	return nil
}

func (m *MockVoteRepository) CreateOrUpdate(vote *database.Vote) error {
	return m.Create(vote)
}

func (m *MockVoteRepository) GetByID(id uint) (*database.Vote, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if vote, ok := m.votes[id]; ok {
		voteCopy := *vote
		return &voteCopy, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockVoteRepository) GetByUserAndPost(userID, postID uint) (*database.Vote, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := m.key(userID, postID)
	if vote, ok := m.byUserPost[key]; ok {
		voteCopy := *vote
		return &voteCopy, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockVoteRepository) GetByVoteHash(voteHash string) (*database.Vote, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, vote := range m.votes {
		if vote.VoteHash != nil && *vote.VoteHash == voteHash {
			voteCopy := *vote
			return &voteCopy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockVoteRepository) GetByPostID(postID uint) ([]database.Vote, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var votes []database.Vote
	for _, vote := range m.votes {
		if vote.PostID == postID {
			votes = append(votes, *vote)
		}
	}
	return votes, nil
}

func (m *MockVoteRepository) GetByUserID(userID uint) ([]database.Vote, error) {
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

func (m *MockVoteRepository) Update(vote *database.Vote) error {
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
	} else {
		key = fmt.Sprintf("anon-%d", vote.PostID)
	}
	m.byUserPost[key] = &voteCopy
	return nil
}

func (m *MockVoteRepository) Delete(id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.DeleteErr != nil {
		return m.DeleteErr
	}

	if vote, ok := m.votes[id]; ok {
		delete(m.votes, id)
		var key string
		if vote.UserID != nil {
			key = m.key(*vote.UserID, vote.PostID)
		} else {
			key = fmt.Sprintf("anon-%d", vote.PostID)
		}
		delete(m.byUserPost, key)
		return nil
	}
	return gorm.ErrRecordNotFound
}

func (m *MockVoteRepository) CountByPostID(postID uint) (int64, error) {
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

func (m *MockVoteRepository) CountByUserID(userID uint) (int64, error) {
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

func (m *MockVoteRepository) GetVoteCountsByPostID(postID uint) (int, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	upVotes := 0
	downVotes := 0
	for _, vote := range m.votes {
		if vote.PostID == postID {
			switch vote.Type {
			case database.VoteUp:
				upVotes++
			case database.VoteDown:
				downVotes++
			}
		}
	}
	return upVotes, downVotes, nil
}

func (m *MockVoteRepository) Count() (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.votes)), nil
}

func (m *MockVoteRepository) WithTx(tx *gorm.DB) repositories.VoteRepository {
	return m
}

func (m *MockVoteRepository) key(userID, postID uint) string {
	return fmt.Sprintf("%d-%d", userID, postID)
}

func (m *MockVoteRepository) SetCreateError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createErr = err
}

func (m *MockVoteRepository) SetUpdateError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateErr = err
}

func (m *MockVoteRepository) SetDeleteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteErr = err
}
