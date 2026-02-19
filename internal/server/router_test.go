package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
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
	titleFetcher := &testutils.MockTitleFetcher{}
	titleFetcher.SetTitle("Example Domain")

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
	postHandler := handlers.NewPostHandler(postRepo, titleFetcher, voteService)
	voteHandler := handlers.NewVoteHandler(voteService)
	userHandler := handlers.NewUserHandler(userRepo, authFacade)
	apiHandler := handlers.NewAPIHandler(testutils.AppTestConfig, postRepo, userRepo, voteService)

	return authHandler, postHandler, voteHandler, userHandler, apiHandler, &mockTokenVerifier{}
}

func defaultRateLimitConfig() config.RateLimitConfig {
	return testutils.AppTestConfig.RateLimit
}

func createDefaultRouterConfig() RouterConfig {
	authHandler, postHandler, voteHandler, userHandler, apiHandler, authService := setupTestHandlers()
	return RouterConfig{
		APIHandler:      apiHandler,
		AuthHandler:     authHandler,
		PostHandler:     postHandler,
		VoteHandler:     voteHandler,
		UserHandler:     userHandler,
		AuthService:     authService,
		RateLimitConfig: defaultRateLimitConfig(),
	}
}

func createTestRouter(cfg RouterConfig) http.Handler {
	return NewRouter(cfg)
}

func TestAPIRootRouting(t *testing.T) {
	router := createTestRouter(createDefaultRouterConfig())

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
	router := createTestRouter(createDefaultRouterConfig())

	protectedRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/auth/me"},
		{http.MethodPost, "/api/auth/logout"},
		{http.MethodPost, "/api/auth/revoke"},
		{http.MethodPost, "/api/auth/revoke-all"},
		{http.MethodPut, "/api/auth/email"},
		{http.MethodPut, "/api/auth/username"},
		{http.MethodPut, "/api/auth/password"},
		{http.MethodDelete, "/api/auth/account"},
		{http.MethodPost, "/api/posts"},
		{http.MethodPut, "/api/posts/1"},
		{http.MethodDelete, "/api/posts/1"},
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
	cfg := createDefaultRouterConfig()
	cfg.Debug = true
	router := createTestRouter(cfg)

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestRouterWithCacheDisabled(t *testing.T) {
	cfg := createDefaultRouterConfig()
	cfg.DisableCache = true
	router := createTestRouter(cfg)

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestRouterWithCompressionDisabled(t *testing.T) {
	cfg := createDefaultRouterConfig()
	cfg.DisableCompression = true
	router := createTestRouter(cfg)

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestRouterWithCustomDBMonitor(t *testing.T) {
	cfg := createDefaultRouterConfig()
	cfg.DBMonitor = middleware.NewInMemoryDBMonitor()
	router := createTestRouter(cfg)

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
	cfg := createDefaultRouterConfig()
	cfg.StaticDir = "/custom/static/path"
	router := createTestRouter(cfg)

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestRouterWithEmptyStaticDir(t *testing.T) {
	cfg := createDefaultRouterConfig()
	cfg.StaticDir = ""
	router := createTestRouter(cfg)

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestRouterWithAllFeaturesDisabled(t *testing.T) {
	cfg := createDefaultRouterConfig()
	cfg.Debug = true
	cfg.DisableCache = true
	cfg.DisableCompression = true
	router := createTestRouter(cfg)

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestRouterWithoutAPIHandler(t *testing.T) {
	cfg := createDefaultRouterConfig()
	cfg.APIHandler = nil
	router := createTestRouter(cfg)

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", recorder.Code)
	}
}

func TestRouterWithoutPageHandler(t *testing.T) {
	router := createTestRouter(createDefaultRouterConfig())

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", recorder.Code)
	}
}

func TestSwaggerRoute(t *testing.T) {
	router := createTestRouter(createDefaultRouterConfig())

	request := httptest.NewRequest(http.MethodGet, "/swagger/", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK && recorder.Code != http.StatusMovedPermanently {
		t.Errorf("Expected status 200 or 301 for swagger, got %d", recorder.Code)
	}
}

func TestStaticFileRoute(t *testing.T) {
	cfg := createDefaultRouterConfig()
	cfg.StaticDir = "../../internal/static/"
	router := createTestRouter(cfg)

	t.Run("existing file should return 200", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/static/css/main.css", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200 for existing static file, got %d", recorder.Code)
		}
	})

	t.Run("non-existent file should return 404", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/static/css/nonexistent.css", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for non-existent static file, got %d", recorder.Code)
		}
	})
}

func TestRouterConfiguration(t *testing.T) {
	router := createTestRouter(createDefaultRouterConfig())

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

func TestAllRoutesExist(t *testing.T) {
	router := createTestRouter(createDefaultRouterConfig())

	publicRoutes := []struct {
		method      string
		path        string
		description string
	}{
		{http.MethodGet, "/api", "API info"},
		{http.MethodGet, "/health", "Health check"},
		{http.MethodGet, "/metrics", "Metrics"},
		{http.MethodGet, "/robots.txt", "Robots.txt"},
		{http.MethodGet, "/api/posts", "Get posts"},
		{http.MethodGet, "/api/posts/search", "Search posts"},
		{http.MethodGet, "/api/posts/title", "Fetch title from URL"},
		{http.MethodGet, "/api/posts/1", "Get post by ID"},
		{http.MethodPost, "/api/auth/register", "Register"},
		{http.MethodPost, "/api/auth/login", "Login"},
		{http.MethodPost, "/api/auth/refresh", "Refresh token"},
		{http.MethodGet, "/api/auth/confirm", "Confirm email"},
		{http.MethodPost, "/api/auth/resend-verification", "Resend verification"},
		{http.MethodPost, "/api/auth/forgot-password", "Forgot password"},
		{http.MethodPost, "/api/auth/reset-password", "Reset password"},
		{http.MethodPost, "/api/auth/account/confirm", "Confirm account deletion"},
	}

	protectedRoutes := []struct {
		method      string
		path        string
		description string
	}{
		{http.MethodGet, "/api/auth/me", "Get current user"},
		{http.MethodPost, "/api/auth/logout", "Logout"},
		{http.MethodPost, "/api/auth/revoke", "Revoke token"},
		{http.MethodPost, "/api/auth/revoke-all", "Revoke all tokens"},
		{http.MethodPut, "/api/auth/email", "Update email"},
		{http.MethodPut, "/api/auth/username", "Update username"},
		{http.MethodPut, "/api/auth/password", "Update password"},
		{http.MethodDelete, "/api/auth/account", "Delete account"},
		{http.MethodPost, "/api/posts", "Create post"},
		{http.MethodPut, "/api/posts/1", "Update post"},
		{http.MethodDelete, "/api/posts/1", "Delete post"},
		{http.MethodPost, "/api/posts/1/vote", "Cast vote"},
		{http.MethodDelete, "/api/posts/1/vote", "Remove vote"},
		{http.MethodGet, "/api/posts/1/vote", "Get user vote"},
		{http.MethodGet, "/api/posts/1/votes", "Get post votes"},
		{http.MethodGet, "/api/users", "Get users"},
		{http.MethodPost, "/api/users", "Create user"},
		{http.MethodGet, "/api/users/1", "Get user by ID"},
		{http.MethodGet, "/api/users/1/posts", "Get user posts"},
	}

	for _, route := range publicRoutes {
		t.Run(route.description+" "+route.method+" "+route.path, func(t *testing.T) {
			invalidMethod := http.MethodPatch
			switch route.method {
			case http.MethodGet:
				invalidMethod = http.MethodDelete
			case http.MethodPost:
				invalidMethod = http.MethodGet
			}
			request := httptest.NewRequest(invalidMethod, route.path, nil)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			routeExists := recorder.Code == http.StatusMethodNotAllowed

			if !routeExists {
				request = httptest.NewRequest(route.method, route.path, nil)
				recorder = httptest.NewRecorder()
				router.ServeHTTP(recorder, request)

				if recorder.Code == http.StatusNotFound && route.path != "/api/posts/1" && route.path != "/robots.txt" {
					t.Errorf("Route %s %s should exist, got 404", route.method, route.path)
				}
			}
		})
	}

	for _, route := range protectedRoutes {
		t.Run(route.description+" "+route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, nil)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code == http.StatusNotFound {
				t.Errorf("Route %s %s should exist, got 404", route.method, route.path)
			}
			if recorder.Code != http.StatusUnauthorized {
				t.Errorf("Protected route %s %s should return 401 without auth, got %d", route.method, route.path, recorder.Code)
			}
		})
	}
}

func TestRouteParameters(t *testing.T) {
	router := createTestRouter(createDefaultRouterConfig())

	testCases := []struct {
		name        string
		method      string
		pathPattern string
		testIDs     []string
		isProtected bool
	}{
		{
			name:        "Get post by ID",
			method:      http.MethodGet,
			pathPattern: "/api/posts/{id}",
			testIDs:     []string{"1", "42", "999", "12345"},
			isProtected: false,
		},
		{
			name:        "Update post by ID",
			method:      http.MethodPut,
			pathPattern: "/api/posts/{id}",
			testIDs:     []string{"1", "42", "999"},
			isProtected: true,
		},
		{
			name:        "Delete post by ID",
			method:      http.MethodDelete,
			pathPattern: "/api/posts/{id}",
			testIDs:     []string{"1", "42", "999"},
			isProtected: true,
		},
		{
			name:        "Get user by ID",
			method:      http.MethodGet,
			pathPattern: "/api/users/{id}",
			testIDs:     []string{"1", "42", "999", "12345"},
			isProtected: true,
		},
		{
			name:        "Get user posts by user ID",
			method:      http.MethodGet,
			pathPattern: "/api/users/{id}/posts",
			testIDs:     []string{"1", "42", "999", "12345"},
			isProtected: true,
		},
		{
			name:        "Cast vote for post ID",
			method:      http.MethodPost,
			pathPattern: "/api/posts/{id}/vote",
			testIDs:     []string{"1", "42", "999"},
			isProtected: true,
		},
		{
			name:        "Remove vote for post ID",
			method:      http.MethodDelete,
			pathPattern: "/api/posts/{id}/vote",
			testIDs:     []string{"1", "42", "999"},
			isProtected: true,
		},
		{
			name:        "Get user vote for post ID",
			method:      http.MethodGet,
			pathPattern: "/api/posts/{id}/vote",
			testIDs:     []string{"1", "42", "999", "12345"},
			isProtected: true,
		},
		{
			name:        "Get post votes by post ID",
			method:      http.MethodGet,
			pathPattern: "/api/posts/{id}/votes",
			testIDs:     []string{"1", "42", "999", "12345"},
			isProtected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, id := range tc.testIDs {
				path := replaceID(tc.pathPattern, id)
				t.Run("ID_"+id, func(t *testing.T) {
					request := httptest.NewRequest(http.MethodPatch, path, nil)
					recorder := httptest.NewRecorder()
					router.ServeHTTP(recorder, request)

					routeExists := recorder.Code == http.StatusMethodNotAllowed

					request = httptest.NewRequest(tc.method, path, nil)
					recorder = httptest.NewRecorder()
					router.ServeHTTP(recorder, request)

					if !routeExists {
						if recorder.Code == http.StatusNotFound {
							t.Errorf("Route %s %s should exist with ID %s, got 404", tc.method, path, id)
							return
						}
					}

					if tc.isProtected {
						if recorder.Code != http.StatusUnauthorized {
							t.Errorf("Protected route %s %s should return 401 without auth, got %d", tc.method, path, recorder.Code)
						}
					}
				})
			}
		})
	}
}

func replaceID(pattern, id string) string {
	return strings.Replace(pattern, "{id}", id, 1)
}

func TestInvalidRouteParameters(t *testing.T) {
	router := createTestRouter(createDefaultRouterConfig())

	testCases := []struct {
		name        string
		method      string
		path        string
		expectedMin int
		expectedMax int
		isProtected bool
		allow401    bool
	}{
		{
			name:        "Non-numeric post ID",
			method:      http.MethodGet,
			path:        "/api/posts/abc",
			expectedMin: http.StatusBadRequest,
			expectedMax: http.StatusBadRequest,
			isProtected: false,
		},
		{
			name:        "Negative post ID",
			method:      http.MethodGet,
			path:        "/api/posts/-1",
			expectedMin: http.StatusBadRequest,
			expectedMax: http.StatusBadRequest,
			isProtected: false,
		},
		{
			name:        "Zero post ID",
			method:      http.MethodGet,
			path:        "/api/posts/0",
			expectedMin: http.StatusBadRequest,
			expectedMax: http.StatusNotFound,
			isProtected: false,
		},
		{
			name:        "Post ID with special characters",
			method:      http.MethodGet,
			path:        "/api/posts/123@456",
			expectedMin: http.StatusBadRequest,
			expectedMax: http.StatusBadRequest,
			isProtected: false,
		},
		{
			name:        "Post ID with encoded spaces",
			method:      http.MethodGet,
			path:        "/api/posts/12%2034",
			expectedMin: http.StatusBadRequest,
			expectedMax: http.StatusBadRequest,
			isProtected: false,
		},
		{
			name:        "Non-numeric user ID",
			method:      http.MethodGet,
			path:        "/api/users/xyz",
			expectedMin: http.StatusBadRequest,
			expectedMax: http.StatusUnauthorized,
			isProtected: true,
			allow401:    true,
		},
		{
			name:        "Negative user ID",
			method:      http.MethodGet,
			path:        "/api/users/-5",
			expectedMin: http.StatusBadRequest,
			expectedMax: http.StatusUnauthorized,
			isProtected: true,
			allow401:    true,
		},
		{
			name:        "Non-numeric post ID in vote route",
			method:      http.MethodGet,
			path:        "/api/posts/invalid/vote",
			expectedMin: http.StatusBadRequest,
			expectedMax: http.StatusUnauthorized,
			isProtected: true,
			allow401:    true,
		},
		{
			name:        "Very large post ID",
			method:      http.MethodGet,
			path:        "/api/posts/999999999999",
			expectedMin: http.StatusBadRequest,
			expectedMax: http.StatusNotFound,
			isProtected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if tc.isProtected && tc.allow401 {
				if recorder.Code != http.StatusUnauthorized && (recorder.Code < tc.expectedMin || recorder.Code > tc.expectedMax) {
					t.Errorf("Protected route %s %s with invalid parameter should return 401 or status between %d and %d, got %d", tc.method, tc.path, tc.expectedMin, tc.expectedMax, recorder.Code)
				}
			} else {
				if recorder.Code < tc.expectedMin || recorder.Code > tc.expectedMax {
					t.Errorf("Route %s %s should return status between %d and %d, got %d", tc.method, tc.path, tc.expectedMin, tc.expectedMax, recorder.Code)
				}

				if recorder.Code != http.StatusNotFound && recorder.Code < 400 {
					t.Errorf("Route %s %s with invalid parameter should return error status (4xx), got %d", tc.method, tc.path, recorder.Code)
				}
			}
		})
	}
}

func TestQueryParameters(t *testing.T) {
	router := createTestRouter(createDefaultRouterConfig())

	testCases := []struct {
		name        string
		method      string
		path        string
		queryParams string
		expectRoute bool
	}{
		{
			name:        "Get posts with limit and offset",
			method:      http.MethodGet,
			path:        "/api/posts",
			queryParams: "limit=10&offset=5",
			expectRoute: true,
		},
		{
			name:        "Get posts with only limit",
			method:      http.MethodGet,
			path:        "/api/posts",
			queryParams: "limit=20",
			expectRoute: true,
		},
		{
			name:        "Get posts with only offset",
			method:      http.MethodGet,
			path:        "/api/posts",
			queryParams: "offset=10",
			expectRoute: true,
		},
		{
			name:        "Search posts with query parameter",
			method:      http.MethodGet,
			path:        "/api/posts/search",
			queryParams: "q=test",
			expectRoute: true,
		},
		{
			name:        "Search posts with query, limit, and offset",
			method:      http.MethodGet,
			path:        "/api/posts/search",
			queryParams: "q=test&limit=15&offset=3",
			expectRoute: true,
		},
		{
			name:        "Fetch title with URL parameter",
			method:      http.MethodGet,
			path:        "/api/posts/title",
			queryParams: "url=https://example.com",
			expectRoute: true,
		},
		{
			name:        "Confirm email with token parameter",
			method:      http.MethodGet,
			path:        "/api/auth/confirm",
			queryParams: "token=abc123",
			expectRoute: true,
		},
		{
			name:        "Get posts with invalid limit",
			method:      http.MethodGet,
			path:        "/api/posts",
			queryParams: "limit=abc",
			expectRoute: true,
		},
		{
			name:        "Get posts with negative limit",
			method:      http.MethodGet,
			path:        "/api/posts",
			queryParams: "limit=-5",
			expectRoute: true,
		},
		{
			name:        "Get posts with negative offset",
			method:      http.MethodGet,
			path:        "/api/posts",
			queryParams: "offset=-10",
			expectRoute: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fullPath := tc.path
			if tc.queryParams != "" {
				fullPath += "?" + tc.queryParams
			}

			request := httptest.NewRequest(tc.method, fullPath, nil)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if tc.expectRoute {
				if recorder.Code == http.StatusNotFound {
					t.Errorf("Route %s %s should exist with query parameters, got 404", tc.method, fullPath)
				}
				if recorder.Code >= http.StatusInternalServerError {
					t.Errorf("Route %s %s should not return server error (5xx), got %d", tc.method, fullPath, recorder.Code)
				}
			}
		})
	}
}

func TestRouteConflicts(t *testing.T) {
	router := createTestRouter(createDefaultRouterConfig())

	testCases := []struct {
		name        string
		method      string
		path        string
		description string
	}{
		{
			name:        "posts/search should not match posts/{id}",
			method:      http.MethodGet,
			path:        "/api/posts/search",
			description: "search route should be matched, not treated as ID",
		},
		{
			name:        "posts/title should not match posts/{id}",
			method:      http.MethodGet,
			path:        "/api/posts/title",
			description: "title route should be matched, not treated as ID",
		},
		{
			name:        "posts/{id} should work with numeric ID",
			method:      http.MethodGet,
			path:        "/api/posts/123",
			description: "numeric ID should match {id} route",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			switch tc.path {
			case "/api/posts/search":
				if recorder.Code == http.StatusNotFound {
					t.Errorf("%s: Route %s %s should exist (not 404), got %d", tc.description, tc.method, tc.path, recorder.Code)
				}
			case "/api/posts/title":
				if recorder.Code == http.StatusNotFound {
					t.Errorf("%s: Route %s %s should exist (not 404), got %d", tc.description, tc.method, tc.path, recorder.Code)
				}
			case "/api/posts/123":
				if recorder.Code == http.StatusNotFound {
					return
				}
				if recorder.Code < 400 {
					t.Errorf("%s: Route %s %s should return 4xx or 5xx, got %d", tc.description, tc.method, tc.path, recorder.Code)
				}
			}
		})
	}
}
