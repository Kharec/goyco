package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/handlers"
	"goyco/internal/middleware"
	"goyco/internal/services"
	"goyco/internal/testutils"

	"gorm.io/gorm"
)

type mockTokenVerifier struct{}

func (m *mockTokenVerifier) VerifyToken(token string) (uint, error) {
	return 0, nil
}

type mockRefreshTokenRepository struct{}

func (m *mockRefreshTokenRepository) Create(token *database.RefreshToken) error {
	return nil
}

func (m *mockRefreshTokenRepository) GetByTokenHash(tokenHash string) (*database.RefreshToken, error) {
	return nil, gorm.ErrRecordNotFound
}

func (m *mockRefreshTokenRepository) DeleteByUserID(userID uint) error {
	return nil
}

func (m *mockRefreshTokenRepository) DeleteExpired() error {
	return nil
}

func (m *mockRefreshTokenRepository) DeleteByID(id uint) error {
	return nil
}

func (m *mockRefreshTokenRepository) GetByUserID(userID uint) ([]database.RefreshToken, error) {
	return []database.RefreshToken{}, nil
}

func (m *mockRefreshTokenRepository) CountByUserID(userID uint) (int64, error) {
	return 0, nil
}

type mockAccountDeletionRepository struct{}

func (m *mockAccountDeletionRepository) Create(req *database.AccountDeletionRequest) error {
	return nil
}

func (m *mockAccountDeletionRepository) GetByTokenHash(hash string) (*database.AccountDeletionRequest, error) {
	return nil, gorm.ErrRecordNotFound
}

func (m *mockAccountDeletionRepository) DeleteByID(id uint) error {
	return nil
}

func (m *mockAccountDeletionRepository) DeleteByUserID(userID uint) error {
	return nil
}

func setupTestHandlers() (*handlers.AuthHandler, *handlers.PostHandler, *handlers.VoteHandler, *handlers.UserHandler, *handlers.APIHandler, middleware.TokenVerifier) {
	userRepo := testutils.NewMockUserRepository()
	postRepo := testutils.NewMockPostRepository()
	voteRepo := testutils.NewMockVoteRepository()
	emailSender := &testutils.MockEmailSender{}

	voteService := services.NewVoteService(voteRepo, postRepo, nil)
	metadataService := services.NewURLMetadataService()

	mockRefreshRepo := &mockRefreshTokenRepository{}
	mockDeletionRepo := &mockAccountDeletionRepository{}
	authFacade, err := services.NewAuthFacadeForTest(
		testutils.AppTestConfig,
		userRepo,
		postRepo,
		mockDeletionRepo,
		mockRefreshRepo,
		emailSender,
	)
	if err != nil {
		panic("Failed to create auth facade: " + err.Error())
	}

	authHandler := handlers.NewAuthHandler(authFacade, userRepo)
	postHandler := handlers.NewPostHandler(postRepo, metadataService, voteService)
	voteHandler := handlers.NewVoteHandler(voteService)
	userHandler := handlers.NewUserHandler(userRepo, authFacade)
	apiHandler := handlers.NewAPIHandler(testutils.AppTestConfig, postRepo, userRepo, voteService)

	return authHandler, postHandler, voteHandler, userHandler, apiHandler, &mockTokenVerifier{}
}

func defaultRateLimitConfig() config.RateLimitConfig {
	return testutils.AppTestConfig.RateLimit
}

func TestAPIRootRouting(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()
	router := NewRouter(RouterConfig{
		APIHandler:      apiHandler,
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		AuthService:     authService,
		RateLimitConfig: defaultRateLimitConfig(),
	})

	testCases := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "without trailing slash", path: "/api", wantStatus: http.StatusOK},
		{name: "with trailing slash", path: "/api/", wantStatus: http.StatusNotFound},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, recorder.Code)
			}
		})
	}
}

func TestProtectedRoutesRequireAuth(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()
	router := NewRouter(RouterConfig{
		APIHandler:      apiHandler,
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		AuthService:     authService,
		RateLimitConfig: defaultRateLimitConfig(),
	})

	protectedRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/auth/me"},
		{http.MethodPost, "/api/posts"},
		{http.MethodPost, "/api/posts/1/vote"},
		{http.MethodDelete, "/api/posts/1/vote"},
		{http.MethodGet, "/api/posts/1/vote"},
		{http.MethodGet, "/api/posts/1/votes"},
		{http.MethodGet, "/api/users"},
		{http.MethodPost, "/api/users"},
		{http.MethodGet, "/api/users/1"},
		{http.MethodGet, "/api/users/1/posts"},
	}

	for _, route := range protectedRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, nil)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("expected status 401 for protected route %s %s, got %d", route.method, route.path, recorder.Code)
			}
		})
	}
}

func TestRouterWithDebugMode(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()
	router := NewRouter(RouterConfig{
		Debug:           true,
		APIHandler:      apiHandler,
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		AuthService:     authService,
		RateLimitConfig: defaultRateLimitConfig(),
	})

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestRouterWithCacheDisabled(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()
	router := NewRouter(RouterConfig{
		DisableCache: true,
		APIHandler:   apiHandler,
		AuthHandler:  authHandler,
		PostHandler:  postHandler,
		VoteHandler:  voteHandler,
		UserHandler:  userHandler,
		AuthService:  authService,
	})

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestRouterWithCompressionDisabled(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()
	router := NewRouter(RouterConfig{
		DisableCompression: true,
		APIHandler:         apiHandler,
		AuthHandler:        authHandler,
		PostHandler:        postHandler,
		VoteHandler:        voteHandler,
		UserHandler:        userHandler,
		AuthService:        authService,
		RateLimitConfig:    defaultRateLimitConfig(),
	})

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestRouterWithCustomDBMonitor(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()
	customDBMonitor := middleware.NewInMemoryDBMonitor()

	router := NewRouter(RouterConfig{
		DBMonitor:       customDBMonitor,
		APIHandler:      apiHandler,
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		AuthService:     authService,
		RateLimitConfig: defaultRateLimitConfig(),
	})

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestRouterWithPageHandler(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()
	pageHandler := &handlers.PageHandler{}

	router := NewRouter(RouterConfig{
		PageHandler:     pageHandler,
		APIHandler:      apiHandler,
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		AuthService:     authService,
		RateLimitConfig: defaultRateLimitConfig(),
	})

	if router == nil {
		t.Error("Router should not be nil")
	}
}

func TestRouterWithStaticDir(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()

	router := NewRouter(RouterConfig{
		StaticDir:       "/custom/static/path",
		APIHandler:      apiHandler,
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		AuthService:     authService,
		RateLimitConfig: defaultRateLimitConfig(),
	})

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestRouterWithEmptyStaticDir(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()

	router := NewRouter(RouterConfig{
		StaticDir:       "",
		APIHandler:      apiHandler,
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		AuthService:     authService,
		RateLimitConfig: defaultRateLimitConfig(),
	})

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestRouterWithAllFeaturesDisabled(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()

	router := NewRouter(RouterConfig{
		Debug:              true,
		DisableCache:       true,
		DisableCompression: true,
		APIHandler:         apiHandler,
		AuthHandler:        authHandler,
		PostHandler:        postHandler,
		VoteHandler:        voteHandler,
		UserHandler:        userHandler,
		AuthService:        authService,
		RateLimitConfig:    defaultRateLimitConfig(),
	})

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestRouterWithoutAPIHandler(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, _, authService := setupTestHandlers()
	router := NewRouter(RouterConfig{
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		AuthService:     authService,
		RateLimitConfig: defaultRateLimitConfig(),
	})

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", recorder.Code)
	}
}

func TestRouterWithoutPageHandler(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()

	router := NewRouter(RouterConfig{
		APIHandler:      apiHandler,
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		AuthService:     authService,
		RateLimitConfig: defaultRateLimitConfig(),
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", recorder.Code)
	}
}

func TestSwaggerRoute(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()

	router := NewRouter(RouterConfig{
		APIHandler:      apiHandler,
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		AuthService:     authService,
		RateLimitConfig: defaultRateLimitConfig(),
	})

	request := httptest.NewRequest(http.MethodGet, "/swagger/", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK && recorder.Code != http.StatusMovedPermanently {
		t.Errorf("Expected status 200 or 301 for swagger, got %d", recorder.Code)
	}
}

func TestStaticFileRoute(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()

	router := NewRouter(RouterConfig{
		StaticDir:       "../../internal/static/",
		APIHandler:      apiHandler,
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		AuthService:     authService,
		RateLimitConfig: defaultRateLimitConfig(),
	})

	request := httptest.NewRequest(http.MethodGet, "/static/css/main.css", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound && recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200 or 404 for static files, got %d", recorder.Code)
	}
}

func TestRouterConfiguration(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()

	router := NewRouter(RouterConfig{
		APIHandler:      apiHandler,
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		AuthService:     authService,
		RateLimitConfig: defaultRateLimitConfig(),
	})

	if router == nil {
		t.Error("Router should not be nil")
	}

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code == 0 {
		t.Error("Router should return a status code")
	}
}

func TestRouterMiddlewareIntegration(t *testing.T) {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()

	router := NewRouter(RouterConfig{
		APIHandler:      apiHandler,
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		AuthService:     authService,
		RateLimitConfig: defaultRateLimitConfig(),
	})

	if router == nil {
		t.Error("Router should not be nil")
	}

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code == 0 {
		t.Error("Router should return a status code")
	}
}
