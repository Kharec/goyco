package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/handlers"
	"goyco/internal/middleware"
	"goyco/internal/repositories"
	"goyco/internal/server"
	"goyco/internal/services"
	"goyco/internal/testutils"
)

type testContext struct {
	Router      http.Handler
	Suite       *testutils.ServiceSuite
	AuthService *services.AuthFacade
}

type commonHandlers struct {
	AuthService     *services.AuthFacade
	VoteService     *services.VoteService
	MetadataService services.TitleFetcher
	AuthHandler     *handlers.AuthHandler
	PostHandler     *handlers.PostHandler
	VoteHandler     *handlers.VoteHandler
	UserHandler     *handlers.UserHandler
	APIHandler      *handlers.APIHandler
}

func setupCommonHandlers(t *testing.T, suite *testutils.ServiceSuite, useMonitoring bool) *commonHandlers {
	t.Helper()

	authService, err := services.NewAuthFacadeForTest(testutils.AppTestConfig, suite.UserRepo, suite.PostRepo, suite.DeletionRepo, suite.RefreshTokenRepo, suite.EmailSender)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	voteService := services.NewVoteService(suite.VoteRepo, suite.PostRepo, suite.DB)
	metadataService := suite.TitleFetcher

	authHandler := handlers.NewAuthHandler(authService, suite.UserRepo)
	postHandler := handlers.NewPostHandler(suite.PostRepo, metadataService, voteService)
	voteHandler := handlers.NewVoteHandler(voteService)
	userHandler := handlers.NewUserHandler(suite.UserRepo, authService)

	var apiHandler *handlers.APIHandler
	if useMonitoring {
		apiHandler = handlers.NewAPIHandlerWithMonitoring(testutils.AppTestConfig, suite.PostRepo, suite.UserRepo, voteService, suite.DB, middleware.NewInMemoryDBMonitor())
	} else {
		apiHandler = handlers.NewAPIHandler(testutils.AppTestConfig, suite.PostRepo, suite.UserRepo, voteService)
	}

	return &commonHandlers{
		AuthService:     authService,
		VoteService:     voteService,
		MetadataService: metadataService,
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		APIHandler:      apiHandler,
	}
}

type routerConfigBuilder struct {
	authHandler        *handlers.AuthHandler
	postHandler        *handlers.PostHandler
	voteHandler        *handlers.VoteHandler
	userHandler        *handlers.UserHandler
	apiHandler         *handlers.APIHandler
	authService        middleware.TokenVerifier
	pageHandler        *handlers.PageHandler
	staticDir          string
	debug              bool
	disableCache       bool
	disableCompression bool
	dbMonitor          middleware.DBMonitor
	rateLimitConfig    config.RateLimitConfig
}

func newRouterConfigBuilder() *routerConfigBuilder {
	return &routerConfigBuilder{
		debug:              false,
		disableCache:       false,
		disableCompression: false,
		dbMonitor:          middleware.NewInMemoryDBMonitor(),
		rateLimitConfig:    testutils.AppTestConfig.RateLimit,
	}
}

func (b *routerConfigBuilder) withHandlers(h *commonHandlers) *routerConfigBuilder {
	b.authHandler = h.AuthHandler
	b.postHandler = h.PostHandler
	b.voteHandler = h.VoteHandler
	b.userHandler = h.UserHandler
	b.apiHandler = h.APIHandler
	b.authService = h.AuthService
	return b
}

func (b *routerConfigBuilder) withIndividualHandlers(
	authHandler *handlers.AuthHandler,
	postHandler *handlers.PostHandler,
	voteHandler *handlers.VoteHandler,
	userHandler *handlers.UserHandler,
	apiHandler *handlers.APIHandler,
	authService middleware.TokenVerifier,
) *routerConfigBuilder {
	b.authHandler = authHandler
	b.postHandler = postHandler
	b.voteHandler = voteHandler
	b.userHandler = userHandler
	b.apiHandler = apiHandler
	b.authService = authService
	return b
}

func (b *routerConfigBuilder) withPageHandler(pageHandler *handlers.PageHandler) *routerConfigBuilder {
	b.pageHandler = pageHandler
	return b
}

func (b *routerConfigBuilder) withStaticDir(staticDir string) *routerConfigBuilder {
	b.staticDir = staticDir
	return b
}

func (b *routerConfigBuilder) withRateLimitConfig(rateLimitConfig config.RateLimitConfig) *routerConfigBuilder {
	b.rateLimitConfig = rateLimitConfig
	return b
}

func (b *routerConfigBuilder) build() server.RouterConfig {
	return server.RouterConfig{
		AuthHandler:        b.authHandler,
		PostHandler:        b.postHandler,
		VoteHandler:        b.voteHandler,
		UserHandler:        b.userHandler,
		APIHandler:         b.apiHandler,
		AuthService:        b.authService,
		PageHandler:        b.pageHandler,
		StaticDir:          b.staticDir,
		Debug:              b.debug,
		DisableCache:       b.disableCache,
		DisableCompression: b.disableCompression,
		DBMonitor:          b.dbMonitor,
		RateLimitConfig:    b.rateLimitConfig,
	}
}

func buildRouterConfig(h *commonHandlers, staticDir string, pageHandler *handlers.PageHandler) server.RouterConfig {
	return newRouterConfigBuilder().
		withHandlers(h).
		withPageHandler(pageHandler).
		withStaticDir(staticDir).
		build()
}

func setupTestContext(t *testing.T) *testContext {
	t.Helper()
	middleware.StopAllRateLimiters()
	suite := testutils.NewServiceSuite(t)

	h := setupCommonHandlers(t, suite, true)

	staticDir := t.TempDir()
	robotsFile := filepath.Join(staticDir, "robots.txt")
	os.WriteFile(robotsFile, []byte("User-agent: *\nDisallow: /"), 0644)

	router := server.NewRouter(buildRouterConfig(h, staticDir, nil))

	return &testContext{
		Router:      router,
		Suite:       suite,
		AuthService: h.AuthService,
	}
}

func setupPageHandlerTestContext(t *testing.T) *testContext {
	t.Helper()
	middleware.StopAllRateLimiters()
	suite := testutils.NewServiceSuite(t)

	h := setupCommonHandlers(t, suite, false)

	staticDir := t.TempDir()
	templatesDir := t.TempDir()

	baseTemplate := `{{define "layout"}}<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
</head>
<body>
    {{block "content" .}}{{end}}
</body>
</html>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "base.gohtml"), []byte(baseTemplate), 0644)

	os.MkdirAll(filepath.Join(templatesDir, "partials"), 0755)

	homeTemplate := `{{define "content"}}<h1>Home</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "home.gohtml"), []byte(homeTemplate), 0644)

	loginTemplate := `{{define "content"}}<h1>Login</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "login.gohtml"), []byte(loginTemplate), 0644)

	registerTemplate := `{{define "content"}}<h1>Register</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "register.gohtml"), []byte(registerTemplate), 0644)

	settingsTemplate := `{{define "content"}}<h1>Settings</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "settings.gohtml"), []byte(settingsTemplate), 0644)

	postTemplate := `{{define "content"}}<h1>{{.Post.Title}}</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "post.gohtml"), []byte(postTemplate), 0644)

	errorTemplate := `{{define "content"}}<h1>Error</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "error.gohtml"), []byte(errorTemplate), 0644)

	confirmTemplate := `{{define "content"}}<h1>Confirm</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "confirm.gohtml"), []byte(confirmTemplate), 0644)

	confirmEmailTemplate := `{{define "content"}}<h1>Confirm Email</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "confirm_email.gohtml"), []byte(confirmEmailTemplate), 0644)

	resendTemplate := `{{define "content"}}<h1>Resend</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "resend-verification.gohtml"), []byte(resendTemplate), 0644)

	resendVerificationTemplate := `{{define "content"}}<h1>Resend Verification</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "resend_verification.gohtml"), []byte(resendVerificationTemplate), 0644)

	forgotTemplate := `{{define "content"}}<h1>Forgot Password</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "forgot-password.gohtml"), []byte(forgotTemplate), 0644)

	forgotPasswordTemplate := `{{define "content"}}<h1>Forgot Password</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "forgot_password.gohtml"), []byte(forgotPasswordTemplate), 0644)

	resetTemplate := `{{define "content"}}<h1>Reset Password</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "reset-password.gohtml"), []byte(resetTemplate), 0644)

	resetPasswordTemplate := `{{define "content"}}<h1>Reset Password</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "reset_password.gohtml"), []byte(resetPasswordTemplate), 0644)

	searchTemplate := `{{define "content"}}<h1>Search</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "search.gohtml"), []byte(searchTemplate), 0644)

	newPostTemplate := `{{define "content"}}<h1>New Post</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "new-post.gohtml"), []byte(newPostTemplate), 0644)

	newPostTemplate2 := `{{define "content"}}<h1>New Post</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "new_post.gohtml"), []byte(newPostTemplate2), 0644)

	confirmDeleteTemplate := `{{define "content"}}<h1>Confirm Delete</h1>{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "confirm_delete.gohtml"), []byte(confirmDeleteTemplate), 0644)

	pageHandler, err := handlers.NewPageHandler(templatesDir, h.AuthService, suite.PostRepo, h.VoteService, suite.UserRepo, h.MetadataService, testutils.AppTestConfig)
	if err != nil {
		t.Fatalf("Failed to create page handler: %v", err)
	}

	router := server.NewRouter(buildRouterConfig(h, staticDir, pageHandler))

	return &testContext{
		Router:      router,
		Suite:       suite,
		AuthService: h.AuthService,
	}
}

func getCSRFToken(t *testing.T, router http.Handler, path string, cookies ...*http.Cookie) string {
	t.Helper()
	request := httptest.NewRequest("GET", path, nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	cookieList := recorder.Result().Cookies()
	for _, cookie := range cookieList {
		if cookie.Name == "csrf_token" {
			return cookie.Value
		}
	}
	t.Fatal("CSRF token not found")
	return ""
}

func assertJSONResponse(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int) map[string]any {
	t.Helper()
	if recorder.Code != expectedStatus {
		t.Errorf("Expected status %d, got %d. Body: %s", expectedStatus, recorder.Code, recorder.Body.String())
		return nil
	}

	var response map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v. Body: %s", err, recorder.Body.String())
		return nil
	}

	return response
}

func assertErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int) {
	t.Helper()
	if recorder.Code != expectedStatus {
		t.Errorf("Expected status %d, got %d. Body: %s", expectedStatus, recorder.Code, recorder.Body.String())
		return
	}

	var response map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode error response: %v. Body: %s", err, recorder.Body.String())
		return
	}

	if _, ok := response["error"]; !ok {
		if _, ok := response["message"]; !ok {
			t.Error("Expected error or message field in error response")
		}
	}
}

func assertStatus(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int) {
	t.Helper()
	if recorder.Code != expectedStatus {
		t.Errorf("Expected status %d, got %d. Body: %s", expectedStatus, recorder.Code, recorder.Body.String())
	}
}

func assertStatusRange(t *testing.T, recorder *httptest.ResponseRecorder, minStatus, maxStatus int) {
	t.Helper()
	if recorder.Code < minStatus || recorder.Code > maxStatus {
		t.Errorf("Expected status between %d and %d, got %d. Body: %s", minStatus, maxStatus, recorder.Code, recorder.Body.String())
	}
}

func assertCookie(t *testing.T, recorder *httptest.ResponseRecorder, name, expectedValue string) {
	t.Helper()
	cookies := recorder.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == name {
			if expectedValue != "" && cookie.Value != expectedValue {
				t.Errorf("Expected cookie %s value %s, got %s", name, expectedValue, cookie.Value)
			}
			return
		}
	}
	t.Errorf("Expected cookie %s not found", name)
}

func assertCookieCleared(t *testing.T, recorder *httptest.ResponseRecorder, name string) {
	t.Helper()
	cookies := recorder.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == name {
			if cookie.Value != "" {
				t.Errorf("Expected cookie %s to be cleared, got value %s", name, cookie.Value)
			}
			return
		}
	}
}

func assertHeader(t *testing.T, recorder *httptest.ResponseRecorder, name string) {
	t.Helper()
	actualValue := recorder.Header().Get(name)
	if actualValue == "" {
		t.Errorf("Expected header %s to be present", name)
	}
}

func assertHeaderContains(t *testing.T, recorder *httptest.ResponseRecorder, name, substring string) {
	t.Helper()
	actualValue := recorder.Header().Get(name)
	if !strings.Contains(actualValue, substring) {
		t.Errorf("Expected header %s to contain %s, got %s", name, substring, actualValue)
	}
}

type authenticatedUser struct {
	User  *database.User
	Token string
}

type authServiceInterface interface {
	Login(username, password string) (*services.AuthResult, error)
}

func createAuthenticatedUser(t *testing.T, authService authServiceInterface, userRepo repositories.UserRepository, username, email string) *authenticatedUser {
	t.Helper()

	password := "SecurePass123!"
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

	if err := userRepo.Create(user); err != nil {
		t.Fatalf("Failed to create authenticated user: %v", err)
	}

	loginResult, err := authService.Login(username, password)
	if err != nil {
		t.Fatalf("Failed to login authenticated user: %v", err)
	}

	return &authenticatedUser{
		User:  loginResult.User,
		Token: loginResult.AccessToken,
	}
}

func createAuthenticatedUserOnly(t *testing.T, authService authServiceInterface, userRepo repositories.UserRepository, username, email string) *database.User {
	t.Helper()
	return createAuthenticatedUser(t, authService, userRepo, username, email).User
}

func uniqueTestUsername(t *testing.T, prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), len(t.Name()))
}

func uniqueTestEmail(t *testing.T, prefix string) string {
	return fmt.Sprintf("%s_%d_%d@example.com", prefix, time.Now().UnixNano(), len(t.Name()))
}

func createUserWithCleanup(t *testing.T, ctx *testContext, username, email string) *authenticatedUser {
	t.Helper()
	user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, username, email)
	t.Cleanup(func() {
		if err := ctx.Suite.UserRepo.Delete(user.User.ID); err != nil {
			t.Logf("Failed to cleanup user %d: %v", user.User.ID, err)
		}
	})
	return user
}

func makeRequest(t *testing.T, router http.Handler, method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var requestBody *bytes.Buffer
	if body != nil {
		requestBody = bytes.NewBuffer(body)
	} else {
		requestBody = bytes.NewBuffer(nil)
	}
	request := httptest.NewRequest(method, path, requestBody)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func makeAuthenticatedRequest(t *testing.T, router http.Handler, method, path string, body []byte, user *authenticatedUser, urlParams map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var requestBody *bytes.Buffer
	if body != nil {
		requestBody = bytes.NewBuffer(body)
	} else {
		requestBody = bytes.NewBuffer(nil)
	}
	request := httptest.NewRequest(method, path, requestBody)
	request.Header.Set("Authorization", "Bearer "+user.Token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request = testutils.WithUserContext(request, middleware.UserIDKey, user.User.ID)
	if urlParams != nil {
		request = testutils.WithURLParams(request, urlParams)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func makeGetRequest(t *testing.T, router http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	return makeRequest(t, router, "GET", path, nil, nil)
}

func makeAuthenticatedGetRequest(t *testing.T, router http.Handler, path string, user *authenticatedUser, urlParams map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return makeAuthenticatedRequest(t, router, "GET", path, nil, user, urlParams)
}

func makePostRequest(t *testing.T, router http.Handler, path string, body map[string]any, user *authenticatedUser, urlParams map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	bodyBytes, _ := json.Marshal(body)
	return makeAuthenticatedRequest(t, router, "POST", path, bodyBytes, user, urlParams)
}

func makePutRequest(t *testing.T, router http.Handler, path string, body map[string]any, user *authenticatedUser, urlParams map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	bodyBytes, _ := json.Marshal(body)
	return makeAuthenticatedRequest(t, router, "PUT", path, bodyBytes, user, urlParams)
}

func makeDeleteRequest(t *testing.T, router http.Handler, path string, user *authenticatedUser, urlParams map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return makeAuthenticatedRequest(t, router, "DELETE", path, nil, user, urlParams)
}

func makePostRequestWithJSON(t *testing.T, router http.Handler, path string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	bodyBytes, _ := json.Marshal(body)
	return makeRequest(t, router, "POST", path, bodyBytes, map[string]string{"Content-Type": "application/json"})
}

func getDataFromResponse(response map[string]any) (map[string]any, bool) {
	if response == nil {
		return nil, false
	}
	data, ok := response["data"].(map[string]any)
	return data, ok
}
