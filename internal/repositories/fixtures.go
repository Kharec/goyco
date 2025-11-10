package repositories

import (
	"fmt"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"goyco/internal/database"
)

type TestDatabase struct {
	DB               *gorm.DB
	UserRepo         UserRepository
	PostRepo         PostRepository
	VoteRepo         VoteRepository
	DeletionRepo     AccountDeletionRepository
	RefreshTokenRepo *RefreshTokenRepository
}

func NewTestDatabase(t *testing.T) *TestDatabase {
	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	err = db.AutoMigrate(
		&database.User{},
		&database.Post{},
		&database.Vote{},
		&database.AccountDeletionRequest{},
		&database.RefreshToken{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	userRepo := NewUserRepository(db)
	postRepo := NewPostRepository(db)
	voteRepo := NewVoteRepository(db)
	deletionRepo := NewAccountDeletionRepository(db)
	refreshTokenRepo := NewRefreshTokenRepository(db)

	return &TestDatabase{
		DB:               db,
		UserRepo:         userRepo,
		PostRepo:         postRepo,
		VoteRepo:         voteRepo,
		DeletionRepo:     deletionRepo,
		RefreshTokenRepo: refreshTokenRepo,
	}
}

func (td *TestDatabase) Cleanup() {
	if td.DB != nil {
		sqlDB, err := td.DB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

func (td *TestDatabase) Reset() {
	td.DB.Exec("DELETE FROM votes")
	td.DB.Exec("DELETE FROM posts")
	td.DB.Exec("DELETE FROM account_deletion_requests")
	td.DB.Exec("DELETE FROM refresh_tokens")
	td.DB.Exec("DELETE FROM users")
}

func (td *TestDatabase) CreateTestUser(username, email, password string) *database.User {
	user := &database.User{
		Username:      username,
		Email:         email,
		Password:      password,
		EmailVerified: true,
	}
	err := td.UserRepo.Create(user)
	if err != nil {
		panic(fmt.Sprintf("Failed to create test user: %v", err))
	}
	return user
}

func (td *TestDatabase) CreateTestPost(authorID uint, title, url, content string) *database.Post {
	post := &database.Post{
		Title:    title,
		URL:      url,
		Content:  content,
		AuthorID: &authorID,
	}
	err := td.PostRepo.Create(post)
	if err != nil {
		panic(fmt.Sprintf("Failed to create test post: %v", err))
	}
	return post
}

func (td *TestDatabase) CreateTestVote(userID, postID uint, voteType database.VoteType) *database.Vote {
	vote := &database.Vote{
		UserID: &userID,
		PostID: postID,
		Type:   voteType,
	}
	err := td.VoteRepo.Create(vote)
	if err != nil {
		panic(fmt.Sprintf("Failed to create test vote: %v", err))
	}
	return vote
}

func (td *TestDatabase) CreateTestAccountDeletionRequest(userID uint, tokenHash string) *database.AccountDeletionRequest {
	request := &database.AccountDeletionRequest{
		UserID:    userID,
		TokenHash: tokenHash,
	}
	err := td.DeletionRepo.Create(request)
	if err != nil {
		panic(fmt.Sprintf("Failed to create test account deletion request: %v", err))
	}
	return request
}

type TestSuite struct {
	*TestDatabase
	t *testing.T
}

func NewTestSuite(t *testing.T) *TestSuite {
	testDB := NewTestDatabase(t)

	t.Cleanup(func() {
		testDB.Cleanup()
	})

	return &TestSuite{
		TestDatabase: testDB,
		t:            t,
	}
}

func (ts *TestSuite) Reset() {
	ts.TestDatabase.Reset()
}

func (ts *TestSuite) AssertUserExists(userID uint) *database.User {
	user, err := ts.UserRepo.GetByID(userID)
	if err != nil {
		ts.t.Fatalf("Expected user %d to exist, got error: %v", userID, err)
	}
	return user
}

func (ts *TestSuite) AssertUserNotExists(userID uint) {
	_, err := ts.UserRepo.GetByID(userID)
	if err == nil {
		ts.t.Fatalf("Expected user %d to not exist", userID)
	}
}

func (ts *TestSuite) AssertPostExists(postID uint) *database.Post {
	post, err := ts.PostRepo.GetByID(postID)
	if err != nil {
		ts.t.Fatalf("Expected post %d to exist, got error: %v", postID, err)
	}
	return post
}

func (ts *TestSuite) AssertPostNotExists(postID uint) {
	_, err := ts.PostRepo.GetByID(postID)
	if err == nil {
		ts.t.Fatalf("Expected post %d to not exist", postID)
	}
}

func (ts *TestSuite) AssertVoteExists(voteID uint) *database.Vote {
	vote, err := ts.VoteRepo.GetByID(voteID)
	if err != nil {
		ts.t.Fatalf("Expected vote %d to exist, got error: %v", voteID, err)
	}
	return vote
}

func (ts *TestSuite) AssertVoteNotExists(voteID uint) {
	_, err := ts.VoteRepo.GetByID(voteID)
	if err == nil {
		ts.t.Fatalf("Expected vote %d to not exist", voteID)
	}
}

func (ts *TestSuite) AssertAccountDeletionRequestExists(tokenHash string) *database.AccountDeletionRequest {
	request, err := ts.DeletionRepo.GetByTokenHash(tokenHash)
	if err != nil {
		ts.t.Fatalf("Expected account deletion request with token %s to exist, got error: %v", tokenHash, err)
	}
	return request
}

func (ts *TestSuite) AssertAccountDeletionRequestNotExists(tokenHash string) {
	_, err := ts.DeletionRepo.GetByTokenHash(tokenHash)
	if err == nil {
		ts.t.Fatalf("Expected account deletion request with token %s to not exist", tokenHash)
	}
}

func (ts *TestSuite) GetUserCount() int64 {
	var count int64
	ts.DB.Model(&database.User{}).Count(&count)
	return count
}

func (ts *TestSuite) GetPostCount() int64 {
	var count int64
	ts.DB.Model(&database.Post{}).Count(&count)
	return count
}

func (ts *TestSuite) GetVoteCount() int64 {
	var count int64
	ts.DB.Model(&database.Vote{}).Count(&count)
	return count
}

func (ts *TestSuite) GetAccountDeletionRequestCount() int64 {
	var count int64
	ts.DB.Model(&database.AccountDeletionRequest{}).Count(&count)
	return count
}

func (ts *TestSuite) CreateInvalidUserID() *uint {
	id := uint(999)
	return &id
}
