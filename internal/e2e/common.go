package e2e

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/handlers"
	"goyco/internal/middleware"
	"goyco/internal/repositories"
	"goyco/internal/server"
	"goyco/internal/services"
	"goyco/internal/testutils"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type (
	APIResponse             = testutils.APIResponse
	LoginResponse           = testutils.LoginResponse
	PostResponse            = testutils.PostResponse
	PostsListResponse       = testutils.PostsListResponse
	Post                    = testutils.Post
	VoteResponse            = testutils.VoteResponse
	TestUser                = testutils.TestUser
	TestPost                = testutils.TestPost
	HealthResponse          = testutils.HealthResponse
	MetricsResponse         = testutils.MetricsResponse
	UserResponse            = testutils.UserResponse
	ProfileResponse         = testutils.ProfileResponse
	AccountDeletionResponse = testutils.AccountDeletionResponse
)

type AuthenticatedClient struct {
	*testutils.AuthenticatedClient
}

type testContext struct {
	server  *IntegrationTestServer
	client  *http.Client
	baseURL string
}

type IntegrationTestServer struct {
	DB               *gorm.DB
	Server           *httptest.Server
	baseURL          string
	transport        http.RoundTripper
	closeFunc        func()
	UserRepo         repositories.UserRepository
	PostRepo         repositories.PostRepository
	VoteRepo         repositories.VoteRepository
	RefreshTokenRepo *repositories.RefreshTokenRepository
	AuthService      handlers.AuthServiceInterface
	VoteService      *services.VoteService
	AuthHandler      *handlers.AuthHandler
	PostHandler      *handlers.PostHandler
	VoteHandler      *handlers.VoteHandler
	UserHandler      *handlers.UserHandler
	APIHandler       *handlers.APIHandler
	EmailSender      *testutils.MockEmailSender
}

func (server *IntegrationTestServer) BaseURL() string {
	return server.baseURL
}

func (server *IntegrationTestServer) NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: server.transport,
	}
}

func (server *IntegrationTestServer) Cleanup() {
	if server.closeFunc != nil {
		server.closeFunc()
	}
	if server.DB != nil {
		sqlDB, err := server.DB.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}
}

type inMemoryRoundTripper struct {
	handler http.Handler
}

func newInMemoryRoundTripper(handler http.Handler) http.RoundTripper {
	return &inMemoryRoundTripper{handler: handler}
}

func (rt *inMemoryRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if rt == nil || rt.handler == nil {
		return nil, fmt.Errorf("in-memory round tripper not initialized")
	}

	var bodyBytes []byte
	if request.Body != nil && request.Body != http.NoBody {
		defer request.Body.Close()
		var err error
		bodyBytes, err = io.ReadAll(request.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
	}

	if len(bodyBytes) > 0 {
		request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	} else {
		request.Body = http.NoBody
	}

	clonedRequest := request.Clone(request.Context())
	if len(bodyBytes) > 0 {
		clonedRequest.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	} else {
		clonedRequest.Body = http.NoBody
	}
	clonedRequest.RequestURI = clonedRequest.URL.RequestURI()

	recorder := httptest.NewRecorder()
	rt.handler.ServeHTTP(recorder, clonedRequest)
	resp := recorder.Result()
	return resp, nil
}

var (
	sanitizeIdentifierRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
)

func findWorkspaceRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	root := wd
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			return root
		}
		parent := filepath.Dir(root)
		if parent == root {
			return wd
		}
		root = parent
	}
}

func sanitizeIdentifier(value string) string {
	sanitized := sanitizeIdentifierRegex.ReplaceAllString(value, "_")
	sanitized = strings.Trim(sanitized, "_")
	if sanitized == "" {
		fallback := fmt.Sprintf("test_%d", time.Now().UnixNano())
		sanitized = sanitizeIdentifierRegex.ReplaceAllString(fallback, "_")
		sanitized = strings.Trim(sanitized, "_")
	}
	return sanitized
}

func uniqueTestID(t *testing.T) string {
	rawID := fmt.Sprintf("%s_%d", t.Name(), time.Now().UnixNano())
	return sanitizeIdentifier(rawID)
}

func getTestFilePrefix(t *testing.T) string {
	testName := t.Name()
	parts := strings.Split(testName, "/")
	if len(parts) > 0 {
		filePart := parts[len(parts)-1]
		if idx := strings.Index(filePart, "_"); idx > 0 {
			return filePart[:idx]
		}
		return strings.ToLower(strings.TrimPrefix(filePart, "TestE2E_"))
	}
	return "test"
}

func uniqueUsername(t *testing.T, prefix string) string {
	filePrefix := getTestFilePrefix(t)
	fullPrefix := fmt.Sprintf("%s_%s", filePrefix, prefix)
	username := fmt.Sprintf("%s_%s", fullPrefix, uniqueTestID(t))
	if len(username) > 50 {
		maxIDLength := max(50-len(fullPrefix)-1, 0)
		testID := uniqueTestID(t)
		if len(testID) > maxIDLength {
			testID = testID[:maxIDLength]
		}
		username = fmt.Sprintf("%s_%s", fullPrefix, testID)
		if len(username) > 50 {
			username = username[:50]
		}
	}
	return username
}

func uniqueEmail(t *testing.T, prefix string) string {
	return fmt.Sprintf("%s_%s@example.com", prefix, uniqueTestID(t))
}

type TestFixtures struct {
	VerifiedUser   *TestUser
	UnverifiedUser *TestUser
	LockedUser     *TestUser
	PostWithVotes  *TestPost
	PostNoVotes    *TestPost
}

func getResponseReader(resp *http.Response) (io.Reader, func(), error) {
	var reader io.Reader = resp.Body
	var cleanup func() = func() {}

	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		reader = gzReader
		cleanup = func() { _ = gzReader.Close() }
	}
	return reader, cleanup, nil
}

func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func retryOnRateLimit(t *testing.T, maxRetries int, operation func() int) int {
	t.Helper()
	for attempt := range maxRetries {
		statusCode := operation()
		if statusCode != http.StatusTooManyRequests {
			return statusCode
		}
		if attempt < maxRetries-1 {
			backoff := time.Duration(attempt+1) * 50 * time.Millisecond
			time.Sleep(backoff)
		}
	}
	return http.StatusTooManyRequests
}

func skipIfRateLimited(t *testing.T, statusCode int, reason string) {
	if statusCode == http.StatusTooManyRequests {
		t.Skipf("Skipping %s: rate limited", reason)
	}
}

func failIfRateLimited(t *testing.T, statusCode int, operation string) {
	t.Helper()
	if statusCode == http.StatusTooManyRequests {
		t.Fatalf("%s was rate limited unexpectedly (429); this test expects deterministic isolation", operation)
	}
}

func setupTestContext(t *testing.T) *testContext {
	t.Helper()
	server := setupIntegrationTestServer(t)
	t.Cleanup(func() {
		server.Cleanup()
	})
	return &testContext{
		server:  server,
		client:  server.NewHTTPClient(),
		baseURL: server.BaseURL(),
	}
}

func setupTestContextWithAuthRateLimit(t *testing.T, authLimit int) *testContext {
	t.Helper()
	server := setupIntegrationTestServerWithAuthRateLimit(t, authLimit)
	t.Cleanup(func() {
		server.Cleanup()
	})
	return &testContext{
		server:  server,
		client:  server.NewHTTPClient(),
		baseURL: server.BaseURL(),
	}
}

func (ctx *testContext) createUserWithCleanup(t *testing.T, prefix, password string) *TestUser {
	t.Helper()
	if password == "" {
		password = "Password123!"
	}
	user := testutils.CreateE2ETestUser(t, ctx.server.UserRepo, uniqueUsername(t, prefix), uniqueEmail(t, prefix), password)
	t.Cleanup(func() {
		if user != nil {
			ctx.server.UserRepo.Delete(user.ID)
		}
	})
	return user
}

func (ctx *testContext) createMultipleUsersWithCleanup(t *testing.T, count int, prefix, password string) []*TestUser {
	t.Helper()
	if password == "" {
		password = "Password123!"
	}
	var users []*TestUser
	for i := range count {
		userPrefix := fmt.Sprintf("%s%d", prefix, i+1)
		user := testutils.CreateE2ETestUser(t, ctx.server.UserRepo, uniqueUsername(t, userPrefix), uniqueEmail(t, userPrefix), password)
		users = append(users, user)
	}
	t.Cleanup(func() {
		for _, user := range users {
			if user != nil {
				ctx.server.UserRepo.Delete(user.ID)
			}
		}
	})
	return users
}

func (ctx *testContext) createUserAndLogin(t *testing.T, prefix, password string) (*TestUser, *AuthenticatedClient) {
	t.Helper()
	user := ctx.createUserWithCleanup(t, prefix, password)
	client := ctx.loginUser(t, user.Username, user.Password)
	return user, client
}

func (ctx *testContext) doLoginRequest(t *testing.T, username, password, ipAddress string) (*http.Response, error) {
	t.Helper()
	loginData := map[string]string{
		"username": username,
		"password": password,
	}
	body, err := json.Marshal(loginData)
	if err != nil {
		return nil, fmt.Errorf("marshal login data: %w", err)
	}
	req, err := http.NewRequest("POST", ctx.baseURL+"/api/auth/login", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	testutils.WithStandardHeaders(req)
	req.Header.Set("X-Forwarded-For", ipAddress)
	return ctx.client.Do(req)
}

func (ctx *testContext) doRequest(t *testing.T, method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	t.Helper()
	builder := testutils.NewRequestBuilder(method, ctx.baseURL+path)
	if body != nil {
		builder = builder.WithBody(body)
	}
	for k, v := range headers {
		builder = builder.WithHeader(k, v)
	}
	req, err := builder.Build()
	if err != nil {
		return nil, err
	}
	return ctx.client.Do(req)
}

func (ctx *testContext) doRequestExpectStatus(t *testing.T, method, path string, expectedStatus int, body io.Reader, headers map[string]string) int {
	t.Helper()
	resp, err := ctx.doRequest(t, method, path, body, headers)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != expectedStatus && resp.StatusCode != http.StatusTooManyRequests {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		t.Fatalf("expected status %d (or 429), got %d. Body: %s", expectedStatus, resp.StatusCode, string(bodyBytes[:n]))
	}
	return resp.StatusCode
}

func decodeJSONResponse(t *testing.T, resp *http.Response, target any) {
	t.Helper()
	reader, cleanup, err := getResponseReader(resp)
	if err != nil {
		t.Fatalf("failed to get response reader: %v", err)
	}
	defer cleanup()
	if err := json.NewDecoder(reader).Decode(target); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func (ctx *testContext) loginUser(t *testing.T, username, password string) *AuthenticatedClient {
	t.Helper()
	return ctx.loginUserWithIP(t, username, password, testutils.GenerateTestIP())
}

func (ctx *testContext) loginUserWithIP(t *testing.T, username, password, ipAddress string) *AuthenticatedClient {
	t.Helper()
	resp, err := ctx.doLoginRequest(t, username, password, ipAddress)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		t.Fatalf("login failed with status %d. Response: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	var loginResp testutils.LoginResponse
	decodeJSONResponse(t, resp, &loginResp)

	if !loginResp.Success {
		t.Fatalf("login response indicates failure: %s", loginResp.Message)
	}

	accessToken := loginResp.Data.AccessToken
	if accessToken == "" {
		accessToken = loginResp.Data.Token
	}
	if accessToken == "" {
		t.Fatalf("login response missing access token")
	}

	return &AuthenticatedClient{
		AuthenticatedClient: &testutils.AuthenticatedClient{
			Client:       ctx.client,
			Token:        accessToken,
			RefreshToken: loginResp.Data.RefreshToken,
			BaseURL:      ctx.baseURL,
		},
	}
}

func (ctx *testContext) loginUserSafe(t *testing.T, username, password string) (*AuthenticatedClient, error) {
	t.Helper()
	authClient, err := testutils.LoginUserSafe(ctx.client, ctx.baseURL, username, password)
	if err != nil {
		return nil, err
	}
	return &AuthenticatedClient{AuthenticatedClient: authClient}, nil
}

func (ctx *testContext) loginExpectStatus(t *testing.T, username, password string, expectedStatus int) int {
	t.Helper()
	return ctx.loginExpectStatusWithIP(t, username, password, expectedStatus, testutils.GenerateTestIP())
}

func (ctx *testContext) loginExpectStatusWithIP(t *testing.T, username, password string, expectedStatus int, ipAddress string) int {
	t.Helper()
	resp, err := ctx.doLoginRequest(t, username, password, ipAddress)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus && resp.StatusCode != http.StatusTooManyRequests {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		t.Fatalf("expected login status %d (or 429), got %d. Body: %s", expectedStatus, resp.StatusCode, string(bodyBytes[:n]))
	}

	return resp.StatusCode
}

func (ctx *testContext) confirmEmail(t *testing.T, token string) {
	t.Helper()
	if token == "" {
		t.Fatalf("confirmation token must not be empty")
	}
	confirmURL := fmt.Sprintf("%s/api/auth/confirm?token=%s", ctx.baseURL, url.QueryEscape(token))
	req, err := http.NewRequest("GET", confirmURL, nil)
	if err != nil {
		t.Fatalf("failed to build confirmation request: %v", err)
	}
	testutils.WithStandardHeaders(req)
	resp, err := ctx.client.Do(req)
	if err != nil {
		t.Fatalf("confirmation request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected confirmation to return 200, got %d. Body: %s", resp.StatusCode, string(body))
	}
}

func (ctx *testContext) registerUserExpectStatus(t *testing.T, username, email, password string) int {
	t.Helper()
	registerData := map[string]string{
		"username": username,
		"email":    email,
		"password": password,
	}
	body, err := json.Marshal(registerData)
	if err != nil {
		t.Fatalf("failed to marshal register data: %v", err)
	}
	headers := map[string]string{"Content-Type": "application/json"}
	return ctx.doRequestExpectStatus(t, "POST", "/api/auth/register", http.StatusCreated, bytes.NewReader(body), headers)
}

func (ctx *testContext) makeRequestWithToken(t *testing.T, token string) int {
	t.Helper()
	headers := make(map[string]string)
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	resp, err := ctx.doRequest(t, "GET", "/api/auth/me", nil, headers)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func (ctx *testContext) requestAccountDeletionExpectStatus(t *testing.T, token string, expectedStatus int) (int, *AccountDeletionResponse) {
	t.Helper()
	headers := map[string]string{"Authorization": "Bearer " + token}
	resp, err := ctx.doRequest(t, "DELETE", "/api/auth/account", nil, headers)
	if err != nil {
		t.Fatalf("failed to make account deletion request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus && resp.StatusCode != http.StatusTooManyRequests {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		t.Fatalf("expected account deletion status %d (or 429), got %d. Response: %s", expectedStatus, resp.StatusCode, string(bodyBytes[:n]))
	}

	if resp.StatusCode == http.StatusOK {
		var deletionResponse AccountDeletionResponse
		decodeJSONResponse(t, resp, &deletionResponse)
		return resp.StatusCode, &deletionResponse
	}

	return resp.StatusCode, nil
}

func generateTestToken(t *testing.T, user *database.User, cfg *config.JWTConfig, opts ...func(*testutils.TokenClaims, *config.JWTConfig) string) string {
	var secret string
	return testutils.GenerateTestToken(t, user, cfg, func(claims *testutils.TokenClaims, jwtCfg *config.JWTConfig) string {
		secret = jwtCfg.Secret
		for _, opt := range opts {
			if opt != nil {
				secret = opt(claims, jwtCfg)
			}
		}
		return secret
	})
}

func generateExpiredToken(t *testing.T, user *database.User, cfg *config.JWTConfig) string {
	return generateTestToken(t, user, cfg, testutils.WithExpiredToken)
}

func generateTamperedToken(t *testing.T, user *database.User, cfg *config.JWTConfig) string {
	return generateTestToken(t, user, cfg, testutils.WithTamperedSecret)
}

func generateTokenWithSessionVersion(t *testing.T, user *database.User, cfg *config.JWTConfig, sessionVersion uint) string {
	return generateTestToken(t, user, cfg, func(claims *testutils.TokenClaims, jwtCfg *config.JWTConfig) string {
		claims.SessionVersion = sessionVersion
		return jwtCfg.Secret
	})
}

func generateTokenWithType(t *testing.T, user *database.User, cfg *config.JWTConfig, tokenType string) string {
	return generateTestToken(t, user, cfg, func(claims *testutils.TokenClaims, jwtCfg *config.JWTConfig) string {
		claims.TokenType = tokenType
		return jwtCfg.Secret
	})
}

func generateTokenWithExpiration(t *testing.T, user *database.User, cfg *config.JWTConfig, expiration time.Duration) string {
	return generateTestToken(t, user, cfg, func(claims *testutils.TokenClaims, jwtCfg *config.JWTConfig) string {
		now := time.Now()
		claims.IssuedAt = jwt.NewNumericDate(now)
		claims.ExpiresAt = jwt.NewNumericDate(now.Add(expiration))
		return jwtCfg.Secret
	})
}

type serverConfig struct {
	authLimit int
}

func setupIntegrationTestServer(t *testing.T) *IntegrationTestServer {
	return setupIntegrationTestServerWithConfig(t, serverConfig{authLimit: 50000})
}

func setupIntegrationTestServerWithAuthRateLimit(t *testing.T, authLimit int) *IntegrationTestServer {
	return setupIntegrationTestServerWithConfig(t, serverConfig{authLimit: authLimit})
}

func setupDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	uniqueID := fmt.Sprintf("%s_%d", sanitizeIdentifier(t.Name()), time.Now().UnixNano())
	dbName := "file:memdb_" + uniqueID + "?mode=memory&cache=shared&_journal_mode=WAL&_synchronous=NORMAL"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to connect to in-memory database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to access underlying database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	err = db.AutoMigrate(
		&database.User{},
		&database.Post{},
		&database.Vote{},
		&database.AccountDeletionRequest{},
		&database.RefreshToken{},
	)
	if err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	if execErr := db.Exec("PRAGMA busy_timeout = 5000").Error; execErr != nil {
		t.Fatalf("failed to configure busy timeout: %v", execErr)
	}
	if execErr := db.Exec("PRAGMA foreign_keys = ON").Error; execErr != nil {
		t.Fatalf("failed to enable foreign keys: %v", execErr)
	}

	return db
}

func setupRepositories(db *gorm.DB) (repositories.UserRepository, repositories.PostRepository, repositories.VoteRepository, repositories.AccountDeletionRepository, *repositories.RefreshTokenRepository) {
	return repositories.NewUserRepository(db),
		repositories.NewPostRepository(db),
		repositories.NewVoteRepository(db),
		repositories.NewAccountDeletionRepository(db),
		repositories.NewRefreshTokenRepository(db)
}

func setupServices(cfg *config.Config, userRepo repositories.UserRepository, postRepo repositories.PostRepository, deletionRepo repositories.AccountDeletionRepository, refreshTokenRepo *repositories.RefreshTokenRepository, emailSender *testutils.MockEmailSender, voteRepo repositories.VoteRepository, db *gorm.DB) (handlers.AuthServiceInterface, *services.VoteService, error) {
	authService, err := services.NewAuthFacadeForTest(cfg, userRepo, postRepo, deletionRepo, refreshTokenRepo, emailSender)
	if err != nil {
		return nil, nil, err
	}
	voteService := services.NewVoteService(voteRepo, postRepo, db)
	return authService, voteService, nil
}

func setupHandlers(authService handlers.AuthServiceInterface, userRepo repositories.UserRepository, postRepo repositories.PostRepository, voteService *services.VoteService, cfg *config.Config) (*handlers.AuthHandler, *handlers.PostHandler, *handlers.VoteHandler, *handlers.UserHandler, *handlers.APIHandler) {
	return handlers.NewAuthHandler(authService, userRepo),
		handlers.NewPostHandler(postRepo, nil, voteService),
		handlers.NewVoteHandler(voteService),
		handlers.NewUserHandler(userRepo, authService),
		handlers.NewAPIHandler(cfg, postRepo, userRepo, voteService)
}

func setupRouter(authHandler *handlers.AuthHandler, postHandler *handlers.PostHandler, voteHandler *handlers.VoteHandler, userHandler *handlers.UserHandler, apiHandler *handlers.APIHandler, authService handlers.AuthServiceInterface, cfg *config.Config) http.Handler {
	return server.NewRouter(server.RouterConfig{
		AuthHandler:        authHandler,
		PostHandler:        postHandler,
		VoteHandler:        voteHandler,
		UserHandler:        userHandler,
		APIHandler:         apiHandler,
		AuthService:        authService,
		PageHandler:        nil,
		StaticDir:          findWorkspaceRoot() + "/internal/static/",
		Debug:              false,
		DisableCache:       true,
		DisableCompression: true,
		RateLimitConfig:    cfg.RateLimit,
	})
}

func setupIntegrationTestServerWithConfig(t *testing.T, serverCfg serverConfig) *IntegrationTestServer {
	t.Helper()

	originalTrustProxy := middleware.TrustProxyHeaders
	middleware.TrustProxyHeaders = true
	t.Cleanup(func() {
		middleware.TrustProxyHeaders = originalTrustProxy
	})

	db := setupDatabase(t)
	userRepo, postRepo, voteRepo, deletionRepo, refreshTokenRepo := setupRepositories(db)

	cfg := &config.Config{
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
			AuthLimit:         serverCfg.authLimit,
			GeneralLimit:      10000,
			HealthLimit:       10000,
			MetricsLimit:      10000,
			TrustProxyHeaders: true,
		},
	}

	mockEmailSender := &testutils.MockEmailSender{}
	authService, voteService, err := setupServices(cfg, userRepo, postRepo, deletionRepo, refreshTokenRepo, mockEmailSender, voteRepo, db)
	if err != nil {
		t.Fatalf("failed to create auth service: %v", err)
	}

	authHandler, postHandler, voteHandler, userHandler, apiHandler := setupHandlers(authService, userRepo, postRepo, voteService, cfg)
	router := setupRouter(authHandler, postHandler, voteHandler, userHandler, apiHandler, authService, cfg)

	listener, err := net.Listen("tcp", "127.0.0.1:0")

	var (
		httpServer *httptest.Server
		baseURL    string
		transport  http.RoundTripper
		closeFunc  func()
	)

	if err == nil {
		httpServer = httptest.NewUnstartedServer(router)
		httpServer.Listener = listener
		httpServer.Start()

		baseURL = httpServer.URL
		transport = nil
		closeFunc = func() {
			httpServer.Close()
		}
	} else {
		t.Logf("falling back to in-memory http server: %v", err)
		baseURL = "http://inmemory.goyco"
		transport = newInMemoryRoundTripper(router)
		closeFunc = func() {}
	}

	return &IntegrationTestServer{
		DB:               db,
		Server:           httpServer,
		baseURL:          baseURL,
		transport:        transport,
		closeFunc:        closeFunc,
		UserRepo:         userRepo,
		PostRepo:         postRepo,
		VoteRepo:         voteRepo,
		RefreshTokenRepo: refreshTokenRepo,
		AuthService:      authService,
		VoteService:      voteService,
		AuthHandler:      authHandler,
		PostHandler:      postHandler,
		VoteHandler:      voteHandler,
		UserHandler:      userHandler,
		APIHandler:       apiHandler,
		EmailSender:      mockEmailSender,
	}
}

func (ctx *testContext) resendVerification(t *testing.T, email string) int {
	t.Helper()
	return testutils.ResendVerificationEmail(t, ctx.client, ctx.baseURL, email)
}

func (ctx *testContext) resendVerificationWithIP(t *testing.T, email, ipAddress string) int {
	t.Helper()
	return testutils.ResendVerificationEmailWithIP(t, ctx.client, ctx.baseURL, email, ipAddress)
}

func (ctx *testContext) createTestFixtures(t *testing.T) *TestFixtures {
	t.Helper()
	fixtures := &TestFixtures{}

	fixtures.VerifiedUser = ctx.createUserWithCleanup(t, "verified", "Password123!")
	ctx.confirmEmail(t, ctx.server.EmailSender.VerificationToken())

	fixtures.UnverifiedUser = ctx.createUserWithCleanup(t, "unverified", "Password123!")

	lockedUser := ctx.createUserWithCleanup(t, "locked", "Password123!")
	ctx.server.UserRepo.Update(&database.User{
		ID:     lockedUser.ID,
		Locked: true,
	})
	fixtures.LockedUser = lockedUser

	client := ctx.loginUser(t, fixtures.VerifiedUser.Username, fixtures.VerifiedUser.Password)
	fixtures.PostWithVotes = client.CreatePost(t, "Post With Votes", "https://example.com/votes", "Content")
	client.VoteOnPost(t, fixtures.PostWithVotes.ID, "up")
	client2 := ctx.loginUser(t, fixtures.VerifiedUser.Username, fixtures.VerifiedUser.Password)
	client2.VoteOnPost(t, fixtures.PostWithVotes.ID, "up")

	fixtures.PostNoVotes = client.CreatePost(t, "Post No Votes", "https://example.com/novotes", "Content")

	return fixtures
}

func (ctx *testContext) waitForCondition(t *testing.T, condition func() bool, timeout time.Duration) bool {
	t.Helper()
	ctxTimeout, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctxTimeout.Done():
			return false
		case <-ticker.C:
			if condition() {
				return true
			}
		}
	}
}

func (ctx *testContext) assertEventually(t *testing.T, assertion func() bool, timeout time.Duration) {
	t.Helper()
	if !ctx.waitForCondition(t, assertion, timeout) {
		t.Errorf("Assertion failed after %v timeout", timeout)
	}
}

var (
	assertPostInList = testutils.AssertPostInList
	getHealth        = testutils.GetHealth
	getMetrics       = testutils.GetMetrics
	assertVoteData   = testutils.AssertVoteData
)

func assertUserResponse(t *testing.T, resp *ProfileResponse, expectedUser *TestUser) {
	t.Helper()
	if resp == nil {
		t.Fatalf("Expected user response, got nil")
	}
	if !resp.Success {
		t.Errorf("Expected user response success=true, got false: %s", resp.Message)
	}
	if resp.Data.ID != expectedUser.ID {
		t.Errorf("Expected user ID %d, got %d", expectedUser.ID, resp.Data.ID)
	}
	if resp.Data.Username != expectedUser.Username {
		t.Errorf("Expected username '%s', got '%s'", expectedUser.Username, resp.Data.Username)
	}
	if resp.Data.Email != expectedUser.Email {
		t.Errorf("Expected email '%s', got '%s'", expectedUser.Email, resp.Data.Email)
	}
	if resp.Data.CreatedAt == "" {
		t.Errorf("Expected user CreatedAt to be set")
	}
	if resp.Data.UpdatedAt == "" {
		t.Errorf("Expected user UpdatedAt to be set")
	}
	validateTimestamp(t, resp.Data.CreatedAt, "CreatedAt")
	validateTimestamp(t, resp.Data.UpdatedAt, "UpdatedAt")
}

func assertVoteResponse(t *testing.T, resp *VoteResponse, expectedType string) {
	t.Helper()
	if resp == nil {
		t.Fatalf("Expected vote response, got nil")
	}
	if !resp.Success {
		t.Errorf("Expected vote response success=true, got false: %s", resp.Message)
	}
	if resp.Data == nil {
		t.Errorf("Expected vote data to be present")
		return
	}
	voteData, ok := resp.Data.(map[string]any)
	if !ok {
		return
	}
	if voteType, exists := voteData["type"]; exists {
		if voteTypeStr, ok := voteType.(string); ok && voteTypeStr != expectedType {
			t.Errorf("Expected vote type '%s', got '%s'", expectedType, voteTypeStr)
		}
	}
}

func validateTimestamp(t *testing.T, timestampStr, fieldName string) {
	t.Helper()
	if timestampStr == "" {
		t.Errorf("Expected %s to be set", fieldName)
		return
	}
	_, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		_, err = time.Parse("2006-01-02T15:04:05Z07:00", timestampStr)
		if err != nil {
			t.Errorf("Invalid timestamp format for %s: '%s'", fieldName, timestampStr)
		}
	}
}
