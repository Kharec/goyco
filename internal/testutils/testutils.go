package testutils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/middleware"
	"goyco/internal/repositories"
)

var AppTestConfig = &config.Config{
	JWT: config.JWTConfig{
		Secret:            "test-secret-key-for-testing-purposes-only",
		Expiration:        24,
		RefreshExpiration: 168,
		Issuer:            "goyco",
		Audience:          "goyco-users",
	},
	App: config.AppConfig{
		BaseURL:    "http://localhost:8080",
		BcryptCost: 10,
	},
	RateLimit: config.RateLimitConfig{
		AuthLimit:         5,
		GeneralLimit:      100,
		HealthLimit:       60,
		MetricsLimit:      10,
		TrustProxyHeaders: false,
	},
}

func NewTestConfig() *config.Config {
	return &config.Config{
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "test",
			Password: "test",
			Name:     "test_db",
			SSLMode:  "disable",
		},
		Server: config.ServerConfig{
			Host: "localhost",
			Port: "8080",
		},
		JWT: config.JWTConfig{
			Secret:            "test-jwt-secret-key-that-is-long-enough",
			Expiration:        24,
			RefreshExpiration: 168,
			Issuer:            "goyco",
			Audience:          "goyco-users",
		},
		SMTP: config.SMTPConfig{
			Host:     "localhost",
			Port:     587,
			Username: "test",
			Password: "test",
			From:     "test@example.com",
		},
		App: config.AppConfig{
			Debug:   true,
			BaseURL: "http://localhost:8080",
		},
		RateLimit: config.RateLimitConfig{
			AuthLimit:         5,
			GeneralLimit:      100,
			HealthLimit:       60,
			MetricsLimit:      10,
			TrustProxyHeaders: false,
		},
		LogDir: "/tmp/goyco-test-logs",
		PIDDir: "/tmp/goyco-test-pids",
	}
}

func sanitizeTestName(name string) string {

	replacer := strings.NewReplacer(
		"/", "_",
		"#", "_",
		"\\", "_",
		"?", "_",
		"&", "_",
		"=", "_",
		" ", "_",
	)
	return replacer.Replace(name)
}

func NewTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	sanitizedName := sanitizeTestName(t.Name())
	dbName := "file:memdb_" + sanitizedName + "?mode=memory&cache=shared&_journal_mode=WAL&_synchronous=NORMAL"
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
		t.Fatalf("Failed to migrate database: %v", err)
	}

	if execErr := db.Exec("PRAGMA busy_timeout = 5000").Error; execErr != nil {
		t.Fatalf("Failed to configure busy timeout: %v", execErr)
	}
	if execErr := db.Exec("PRAGMA foreign_keys = ON").Error; execErr != nil {
		t.Fatalf("Failed to enable foreign keys: %v", execErr)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("Failed to access SQL DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	return db
}

type HTTPTestHelpers struct {
	t *testing.T
}

func NewHTTPTestHelpers(t *testing.T) *HTTPTestHelpers {
	return &HTTPTestHelpers{t: t}
}

func (h *HTTPTestHelpers) POST(url string, body any) *http.Request {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		h.t.Fatalf("Failed to marshal request body: %v", err)
	}
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func (h *HTTPTestHelpers) GET(url string) *http.Request {
	return httptest.NewRequest("GET", url, nil)
}

func (h *HTTPTestHelpers) PUT(url string, body any) *http.Request {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		h.t.Fatalf("Failed to marshal request body: %v", err)
	}
	req := httptest.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func (h *HTTPTestHelpers) DELETE(url string) *http.Request {
	return httptest.NewRequest("DELETE", url, nil)
}

func WithURLParams(req *http.Request, params map[string]string) *http.Request {
	routeContext := chi.NewRouteContext()
	for key, value := range params {
		routeContext.URLParams.Add(key, value)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeContext)
	return req.WithContext(ctx)
}

func WithUserContext(req *http.Request, key any, userID uint) *http.Request {
	ctx := context.WithValue(req.Context(), key, userID)
	return req.WithContext(ctx)
}

func CreateTestUser(t *testing.T, db *gorm.DB) *database.User {
	t.Helper()
	user := &database.User{
		Username:      "testuser",
		Email:         "test@example.com",
		Password:      "hashedpassword123",
		EmailVerified: true,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	return user
}

func CreateTestPost(t *testing.T, db *gorm.DB, authorID uint) *database.Post {
	t.Helper()
	post := &database.Post{
		Title:    "Test Post",
		URL:      "https://example.com/test",
		Content:  "Test content",
		AuthorID: &authorID,
	}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("Failed to create test post: %v", err)
	}
	return post
}

func CreateTestPIDFile(t *testing.T, pid int) string {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "goyco.pid")

	err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
	if err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	return pidFile
}

type HandlerTestHelper struct {
	t *testing.T
}

func NewHandlerTestHelper(t *testing.T) *HandlerTestHelper {
	return &HandlerTestHelper{t: t}
}

func (h *HandlerTestHelper) AssertResponseSuccess(t *testing.T, response map[string]any) {
	if success, ok := response["success"].(bool); !ok || !success {
		t.Fatalf("Expected success=true, got %v", response["success"])
	}
}

func (h *HandlerTestHelper) AssertResponseError(t *testing.T, response map[string]any) {
	if success, ok := response["success"].(bool); !ok || success {
		t.Fatalf("Expected success=false, got %v", response["success"])
	}
}

func (h *HandlerTestHelper) AssertStatusCode(t *testing.T, recorder *httptest.ResponseRecorder, expected int) {
	if recorder.Result().StatusCode != expected {
		t.Fatalf("Expected status %d, got %d", expected, recorder.Result().StatusCode)
	}
}

func (h *HandlerTestHelper) CreateTestRequestWithUser(method, url string, body any, userID uint) *http.Request {
	var req *http.Request
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("Failed to marshal request body: %v", err)
		}
		req = httptest.NewRequest(method, url, bytes.NewBuffer(jsonBody))
	} else {
		req = httptest.NewRequest(method, url, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	return WithUserContext(req, middleware.UserIDKey, userID)
}

func (h *HandlerTestHelper) CreateTestRequest(method, url string, body any) *http.Request {
	var req *http.Request
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("Failed to marshal request body: %v", err)
		}
		req = httptest.NewRequest(method, url, bytes.NewBuffer(jsonBody))
	} else {
		req = httptest.NewRequest(method, url, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}

func (h *HandlerTestHelper) DecodeResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	return response
}

type ServiceSuite struct {
	DB               *gorm.DB
	UserRepo         repositories.UserRepository
	PostRepo         repositories.PostRepository
	VoteRepo         repositories.VoteRepository
	DeletionRepo     repositories.AccountDeletionRepository
	RefreshTokenRepo *repositories.RefreshTokenRepository
	EmailSender      *MockEmailSender
	TitleFetcher     *MockTitleFetcher
}

func NewServiceSuite(t *testing.T) *ServiceSuite {
	t.Helper()
	db := NewTestDB(t)

	userRepo := repositories.NewUserRepository(db)
	postRepo := repositories.NewPostRepository(db)
	voteRepo := repositories.NewVoteRepository(db)
	deletionRepo := repositories.NewAccountDeletionRepository(db)
	refreshTokenRepo := repositories.NewRefreshTokenRepository(db)

	emailSender := &MockEmailSender{}
	titleFetcher := &MockTitleFetcher{}

	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	return &ServiceSuite{
		DB:               db,
		UserRepo:         userRepo,
		PostRepo:         postRepo,
		VoteRepo:         voteRepo,
		DeletionRepo:     deletionRepo,
		RefreshTokenRepo: refreshTokenRepo,
		EmailSender:      emailSender,
		TitleFetcher:     titleFetcher,
	}
}

func (s *ServiceSuite) Cleanup() {
}

func HashPassword(password string) string {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("Failed to hash password: %v", err))
	}
	return string(hashed)
}
