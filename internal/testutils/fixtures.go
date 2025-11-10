package testutils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"goyco/internal/database"
)

type TestConfig struct {
	Database  DatabaseConfig  `json:"database"`
	Server    ServerConfig    `json:"server"`
	JWT       JWTConfig       `json:"jwt"`
	Email     EmailConfig     `json:"email"`
	RateLimit RateLimitConfig `json:"rate_limit"`
	Cache     CacheConfig     `json:"cache"`
	Security  SecurityConfig  `json:"security"`
	Logging   LoggingConfig   `json:"logging"`
}

type DatabaseConfig struct {
	Driver   string `json:"driver"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
	SSLMode  string `json:"sslmode"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type JWTConfig struct {
	Secret     string `json:"secret"`
	Expiration int    `json:"expiration"`
}

type EmailConfig struct {
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	FromEmail    string `json:"from_email"`
	FromName     string `json:"from_name"`
}

type RateLimitConfig struct {
	RequestsPerMinute int `json:"requests_per_minute"`
	BurstSize         int `json:"burst_size"`
}

type CacheConfig struct {
	Type string `json:"type"`
}

type SecurityConfig struct {
	EnableCSRF        bool     `json:"enable_csrf"`
	CSRFSecret        string   `json:"csrf_secret"`
	EnableCORS        bool     `json:"enable_cors"`
	AllowedOrigins    []string `json:"allowed_origins"`
	EnableRateLimit   bool     `json:"enable_rate_limit"`
	EnableCompression bool     `json:"enable_compression"`
}

type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
	Output string `json:"output"`
}

type TestFixtures struct {
	Users  []*database.User
	Posts  []*database.Post
	Votes  []*database.Vote
	Config *TestConfig
}

func NewTestFixtures(t *testing.T) *TestFixtures {
	t.Helper()

	return &TestFixtures{
		Users: []*database.User{
			{
				Username:      "testuser1",
				Email:         "user1@test.local",
				Password:      "SecurePass123!",
				EmailVerified: true,
			},
			{
				Username:      "testuser2",
				Email:         "user2@test.local",
				Password:      "SecurePass456!",
				EmailVerified: true,
			},
			{
				Username:      "unverified_user",
				Email:         "unverified@test.local",
				Password:      "SecurePass789!",
				EmailVerified: false,
			},
		},
		Posts: []*database.Post{
			{
				Title:     "Test Post 1",
				URL:       "https://example.com/post1",
				Content:   "This is test content for post 1",
				UpVotes:   5,
				DownVotes: 1,
				Score:     4,
			},
			{
				Title:     "Test Post 2",
				URL:       "https://example.com/post2",
				Content:   "This is test content for post 2",
				UpVotes:   3,
				DownVotes: 0,
				Score:     3,
			},
		},
		Votes: []*database.Vote{
			{
				Type: database.VoteUp,
			},
			{
				Type: database.VoteDown,
			},
			{
				Type: database.VoteNone,
			},
		},
		Config: &TestConfig{
			Database: DatabaseConfig{
				Driver:   "sqlite",
				Host:     ":memory:",
				Port:     0,
				User:     "",
				Password: "",
				DBName:   "test",
				SSLMode:  "disable",
			},
			Server: ServerConfig{
				Host: "localhost",
				Port: 8080,
			},
			JWT: JWTConfig{
				Secret:     "test-secret-key",
				Expiration: 24,
			},
			Email: EmailConfig{
				SMTPHost:     "localhost",
				SMTPPort:     587,
				SMTPUsername: "test@example.com",
				SMTPPassword: "test-password",
				FromEmail:    "test@example.com",
				FromName:     "Test App",
			},
			RateLimit: RateLimitConfig{
				RequestsPerMinute: 60,
				BurstSize:         10,
			},
			Cache: CacheConfig{
				Type: "memory",
			},
			Security: SecurityConfig{
				EnableCSRF:        true,
				CSRFSecret:        "test-csrf-secret",
				EnableCORS:        true,
				AllowedOrigins:    []string{"http://localhost:3000"},
				EnableRateLimit:   true,
				EnableCompression: true,
			},
			Logging: LoggingConfig{
				Level:  "debug",
				Format: "json",
				Output: "stdout",
			},
		},
	}
}

func CreateSecureTestUser(t *testing.T, db *gorm.DB, username, email string) *database.User {
	t.Helper()

	if username == "" {
		username = generateSecureRandomString(8)
	}
	if email == "" {
		email = fmt.Sprintf("%s@test.local", username)
	}

	password := generateSecurePassword()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	user := &database.User{
		Username:      username,
		Email:         email,
		Password:      string(hashedPassword),
		EmailVerified: true,
	}

	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return user
}

func CreateSecureTestPost(t *testing.T, db *gorm.DB, authorID uint) *database.Post {
	t.Helper()

	title := generateSecureRandomString(12)
	url := fmt.Sprintf("https://example.com/%s", generateSecureRandomString(8))
	content := fmt.Sprintf("Test content for %s", title)

	post := &database.Post{
		Title:     title,
		URL:       url,
		Content:   content,
		AuthorID:  &authorID,
		UpVotes:   0,
		DownVotes: 0,
		Score:     0,
	}

	if err := db.Create(post).Error; err != nil {
		t.Fatalf("Failed to create test post: %v", err)
	}

	return post
}

func CreateSecureTestVote(t *testing.T, db *gorm.DB, userID, postID uint, voteType database.VoteType) *database.Vote {
	t.Helper()

	vote := &database.Vote{
		UserID: &userID,
		PostID: postID,
		Type:   voteType,
	}

	if err := db.Create(vote).Error; err != nil {
		t.Fatalf("Failed to create test vote: %v", err)
	}

	return vote
}

func (f *TestFixtures) CreateTestUsers(t *testing.T, db *gorm.DB) []*database.User {
	t.Helper()

	var users []*database.User
	for _, userData := range f.Users {
		user := *userData

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			t.Fatalf("Failed to hash password: %v", err)
		}
		user.Password = string(hashedPassword)

		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}
		users = append(users, &user)
	}

	return users
}

func (f *TestFixtures) CreateTestPosts(t *testing.T, db *gorm.DB, authorID uint) []*database.Post {
	t.Helper()

	var posts []*database.Post
	for _, postData := range f.Posts {
		post := *postData
		post.AuthorID = &authorID

		if err := db.Create(&post).Error; err != nil {
			t.Fatalf("Failed to create test post: %v", err)
		}
		posts = append(posts, &post)
	}

	return posts
}

func (f *TestFixtures) CreateTestVotes(t *testing.T, db *gorm.DB, userID, postID uint) []*database.Vote {
	t.Helper()

	var votes []*database.Vote
	for _, voteData := range f.Votes {
		vote := *voteData
		vote.UserID = &userID
		vote.PostID = postID

		if err := db.Create(&vote).Error; err != nil {
			t.Fatalf("Failed to create test vote: %v", err)
		}
		votes = append(votes, &vote)
	}

	return votes
}

func generateSecureRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)

	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}

	return string(result)
}

func generateSecurePassword() string {
	letters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	numbers := "0123456789"
	special := "!@#$%^&*"

	password := ""

	num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
	password += string(letters[num.Int64()])

	num, _ = rand.Int(rand.Reader, big.NewInt(int64(len(numbers))))
	password += string(numbers[num.Int64()])

	num, _ = rand.Int(rand.Reader, big.NewInt(int64(len(special))))
	password += string(special[num.Int64()])

	for len(password) < 12 {
		charset := letters + numbers + special
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		password += string(charset[num.Int64()])
	}

	return password
}

func CleanupTestData(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec("DELETE FROM votes").Error; err != nil {
		t.Logf("Warning: Failed to clean up votes: %v", err)
	}
	if err := db.Exec("DELETE FROM posts").Error; err != nil {
		t.Logf("Warning: Failed to clean up posts: %v", err)
	}
	if err := db.Exec("DELETE FROM account_deletion_requests").Error; err != nil {
		t.Logf("Warning: Failed to clean up account deletion requests: %v", err)
	}
	if err := db.Exec("DELETE FROM users").Error; err != nil {
		t.Logf("Warning: Failed to clean up users: %v", err)
	}
}

func AssertUserExists(t *testing.T, db *gorm.DB, userID uint) {
	t.Helper()

	var count int64
	if err := db.Model(&database.User{}).Where("id = ?", userID).Count(&count).Error; err != nil {
		t.Fatalf("Failed to check user existence: %v", err)
	}

	if count == 0 {
		t.Errorf("Expected user with ID %d to exist", userID)
	}
}

func AssertUserNotExists(t *testing.T, db *gorm.DB, userID uint) {
	t.Helper()

	var count int64
	if err := db.Model(&database.User{}).Where("id = ?", userID).Count(&count).Error; err != nil {
		t.Fatalf("Failed to check user existence: %v", err)
	}

	if count > 0 {
		t.Errorf("Expected user with ID %d to not exist", userID)
	}
}

func AssertPostExists(t *testing.T, db *gorm.DB, postID uint) {
	t.Helper()

	var count int64
	if err := db.Model(&database.Post{}).Where("id = ?", postID).Count(&count).Error; err != nil {
		t.Fatalf("Failed to check post existence: %v", err)
	}

	if count == 0 {
		t.Errorf("Expected post with ID %d to exist", postID)
	}
}

func AssertVoteExists(t *testing.T, db *gorm.DB, userID, postID uint) {
	t.Helper()

	var count int64
	if err := db.Model(&database.Vote{}).Where("user_id = ? AND post_id = ?", userID, postID).Count(&count).Error; err != nil {
		t.Fatalf("Failed to check vote existence: %v", err)
	}

	if count == 0 {
		t.Errorf("Expected vote for user %d and post %d to exist", userID, postID)
	}
}

func GetUserCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()

	var count int64
	if err := db.Model(&database.User{}).Count(&count).Error; err != nil {
		t.Fatalf("Failed to get user count: %v", err)
	}

	return count
}

func GetPostCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()

	var count int64
	if err := db.Model(&database.Post{}).Count(&count).Error; err != nil {
		t.Fatalf("Failed to get post count: %v", err)
	}

	return count
}

func GetVoteCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()

	var count int64
	if err := db.Model(&database.Vote{}).Count(&count).Error; err != nil {
		t.Fatalf("Failed to get vote count: %v", err)
	}

	return count
}
