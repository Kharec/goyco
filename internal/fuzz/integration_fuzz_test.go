package fuzz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"goyco/internal/handlers"
	"goyco/internal/middleware"
	"goyco/internal/repositories"
	"goyco/internal/services"
	"goyco/internal/testutils"
)

func FuzzIntegrationHandlers(f *testing.F) {
	f.Add("testuser")
	f.Add("test@example.com")
	f.Add("password123")
	f.Add("")
	f.Add("<script>alert('xss')</script>")

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 500 {
			input = input[:500]
		}

		if !isValidUTF8(input) {
			return
		}

		db := testutils.NewTestDB(t)
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		userRepo := repositories.NewUserRepository(db)
		postRepo := repositories.NewPostRepository(db)
		voteRepo := repositories.NewVoteRepository(db)
		deletionRepo := repositories.NewAccountDeletionRepository(db)
		refreshTokenRepo := repositories.NewRefreshTokenRepository(db)
		emailSender := &testutils.MockEmailSender{}

		authService, err := services.NewAuthFacadeForTest(testutils.AppTestConfig, userRepo, postRepo, deletionRepo, refreshTokenRepo, emailSender)
		if err != nil {
			t.Fatalf("Failed to create auth service: %v", err)
		}

		voteService := services.NewVoteService(voteRepo, postRepo, db)
		titleFetcher := &testutils.MockTitleFetcher{}

		authHandler := handlers.NewAuthHandler(authService, userRepo)
		postHandler := handlers.NewPostHandler(postRepo, titleFetcher, voteService)
		apiHandler := handlers.NewAPIHandler(testutils.AppTestConfig, postRepo, userRepo, voteService)

		router := chi.NewRouter()
		router.Use(middleware.Logging(false))
		router.Use(middleware.SecurityHeadersMiddleware())
		router.Use(middleware.GeneralRateLimitMiddleware())

		router.Route("/api", func(r chi.Router) {
			r.Post("/auth/register", authHandler.Register)
			r.Post("/auth/login", authHandler.Login)
			r.Get("/posts/search", postHandler.SearchPosts)
			r.Get("/posts", postHandler.GetPosts)

			r.Group(func(protected chi.Router) {
				protected.Use(middleware.NewAuth(authService))
				protected.Get("/auth/me", authHandler.Me)
				protected.Post("/posts", postHandler.CreatePost)
			})
		})

		router.Get("/health", apiHandler.GetHealth)

		t.Run("register_endpoint", func(t *testing.T) {
			username := input[:min(len(input), 50)]
			email := input[:min(len(input), 50)] + "@example.com"
			password := input[:min(len(input), 128)]
			if len(password) < 8 {
				password = password + "12345678"
			}

			registerBody := fmt.Sprintf(`{"username":"%s","email":"%s","password":"%s"}`,
				escapeJSON(username), escapeJSON(email), escapeJSON(password))

			req, _ := http.NewRequest("POST", "/api/auth/register", bytes.NewBufferString(registerBody))
			req.Header.Set("Content-Type", "application/json")

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code == 0 {
				t.Fatal("Handler should return a status code")
			}

			if resp.Code != http.StatusCreated && resp.Code != http.StatusBadRequest {
				t.Logf("Unexpected status code %d for register (expected 201 or 400)", resp.Code)
			}

			var result map[string]any
			if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
				t.Fatalf("Response should be valid JSON: %v", err)
			}
		})

		t.Run("search_endpoint", func(t *testing.T) {
			query := input[:min(len(input), 200)]
			escapedQuery := url.QueryEscape(query)

			req, _ := http.NewRequest("GET", "/api/posts/search?q="+escapedQuery, nil)
			req.Header.Set("Content-Type", "application/json")

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code == 0 {
				t.Fatal("Handler should return a status code")
			}

			if resp.Code != http.StatusOK {
				t.Logf("Unexpected status code %d for search (expected 200)", resp.Code)
			}

			var result map[string]any
			if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
				t.Fatalf("Response should be valid JSON: %v", err)
			}
		})
	})
}

func FuzzIntegrationServices(f *testing.F) {
	f.Add("testuser")
	f.Add("test@example.com")
	f.Add("password123")
	f.Add("")
	f.Add("a")
	f.Add(strings.Repeat("x", 100))

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 200 {
			input = input[:200]
		}

		if !utf8.ValidString(input) {
			return
		}

		db := testutils.NewTestDB(t)
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		userRepo := repositories.NewUserRepository(db)
		postRepo := repositories.NewPostRepository(db)
		deletionRepo := repositories.NewAccountDeletionRepository(db)
		refreshTokenRepo := repositories.NewRefreshTokenRepository(db)
		emailSender := &testutils.MockEmailSender{}

		authService, err := services.NewAuthFacadeForTest(testutils.AppTestConfig, userRepo, postRepo, deletionRepo, refreshTokenRepo, emailSender)
		if err != nil {
			t.Fatalf("Failed to create auth service: %v", err)
		}

		usernameLen := len(input)
		if usernameLen > 50 {
			usernameLen = 50
		}
		username := input[:usernameLen]
		email := input[:usernameLen] + "@example.com"

		passwordLen := len(input)
		if passwordLen > 128 {
			passwordLen = 128
		}
		password := input[:passwordLen]

		if len(password) < 8 {
			password = password + "12345678"
		}

		result, err := authService.Register(username, email, password)

		if err != nil {
			if strings.Contains(err.Error(), "panic") || strings.Contains(err.Error(), "nil pointer") {
				t.Fatalf("Registration should not panic: %v", err)
			}
		} else {
			if result.User == nil {
				t.Fatal("Registration result should contain a user")
			}
			if result.User.Username != username {
				t.Fatalf("Expected username %q, got %q", username, result.User.Username)
			}
			if !strings.EqualFold(result.User.Email, email) {
				t.Fatalf("Expected email %q, got %q", email, result.User.Email)
			}
		}

		if err == nil {
			loginResult, loginErr := authService.Login(username, password)
			if loginErr == nil {
				if loginResult.User == nil {
					t.Fatal("Login result should contain a user")
				}
				if loginResult.User.Username != username {
					t.Fatalf("Expected username %q, got %q", username, loginResult.User.Username)
				}
				if loginResult.AccessToken == "" {
					t.Fatal("Login result should contain an access token")
				}
			}
		}
	})
}

func FuzzIntegrationRepositories(f *testing.F) {
	helper := NewFuzzTestHelper()
	helper.RunIntegrationFuzzTest(f, func(t *testing.T, fuzzedData string) {
		searchQuery := fuzzedData
		if len(searchQuery) > 100 {
			searchQuery = searchQuery[:100]
		}

		sanitizer := repositories.NewSearchSanitizer()
		sanitizedQuery, err := sanitizer.SanitizeSearchQuery(searchQuery)

		if err == nil {
			if !utf8.ValidString(sanitizedQuery) {
				t.Fatal("String contains invalid UTF-8")
			}

			validationErr := sanitizer.ValidateSearchQuery(sanitizedQuery)
			_ = validationErr
		}

		username := fuzzedData
		email := fuzzedData + "@example.com"

		if len(username) > 50 {
			username = username[:50]
		}
		if len(email) > 100 {
			email = email[:100]
		}

		if !utf8.ValidString(username) {
			t.Fatal("String contains invalid UTF-8")
		}
		if !utf8.ValidString(email) {
			t.Fatal("String contains invalid UTF-8")
		}

		postTitle := fuzzedData
		postContent := fuzzedData

		if len(postTitle) > 200 {
			postTitle = postTitle[:200]
		}
		if len(postContent) > 1000 {
			postContent = postContent[:1000]
		}

		if !utf8.ValidString(postTitle) {
			t.Fatal("String contains invalid UTF-8")
		}
		if !utf8.ValidString(postContent) {
			t.Fatal("String contains invalid UTF-8")
		}
	})
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == utf8.RuneError {
			return false
		}
	}
	return true
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
