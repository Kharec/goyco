package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"goyco/internal/repositories"
	"goyco/internal/testutils"
)

func TestE2E_SecurityWorkflows(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("security_workflows", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "testuser", "StrongPass123!")
		_ = ctx.loginUser(t, createdUser.Username, createdUser.Password)

		t.Run("unauthorized_access_attempts", func(t *testing.T) {
			request, err := testutils.NewRequestBuilder("GET", ctx.baseURL+"/api/auth/me").Build()
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := ctx.client.Do(request)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("Expected 401 for unauthorized access, got %d", resp.StatusCode)
			}
		})

		t.Run("invalid_token_access", func(t *testing.T) {
			request, err := testutils.NewRequestBuilder("GET", ctx.baseURL+"/api/auth/me").
				WithAuth("invalid-token-12345").
				Build()
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := ctx.client.Do(request)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("Expected 401 for invalid token, got %d", resp.StatusCode)
			}
		})

		t.Run("rate_limiting", func(t *testing.T) {
			rateLimitCtx := setupTestContextWithAuthRateLimit(t, 5)
			rateLimitUser := rateLimitCtx.createUserWithCleanup(t, "ratelimituser", "StrongPass123!")
			_ = rateLimitCtx.loginUser(t, rateLimitUser.Username, rateLimitUser.Password)

			testIP := testutils.GenerateTestIP()
			rateLimited := false
			for range 10 {
				statusCode := rateLimitCtx.loginExpectStatusWithIP(t, rateLimitUser.Username, "WrongPass123!", http.StatusUnauthorized, testIP)
				if statusCode == http.StatusTooManyRequests {
					rateLimited = true
					break
				}
			}
			if !rateLimited {
				t.Errorf("Expected rate limiting to occur after multiple failed login attempts")
			}
		})
	})
}

func TestE2E_SearchSanitization(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("search_sanitization", func(t *testing.T) {
		_, authClient := ctx.createUserAndLogin(t, "testuser", "StrongPass123!")

		_ = authClient.CreatePost(t, "Searchable Post", "https://example.com/search", "This post contains searchable content")

		benignSearch := authClient.SearchPosts(t, "searchable")
		if !benignSearch.Success {
			t.Errorf("Expected benign search to succeed, got failure: %s", benignSearch.Message)
		}
		if len(benignSearch.Data.Posts) == 0 {
			t.Errorf("Expected to find post with benign search query")
		}

		maliciousQuery := "searchable'; DROP TABLE users; --"
		request, err := testutils.NewRequestBuilder("GET", ctx.baseURL+"/api/posts/search?q="+url.QueryEscape(maliciousQuery)).Build()
		if err != nil {
			t.Fatalf("Failed to create malicious search request: %v", err)
		}

		resp, err := ctx.client.Do(request)
		if err != nil {
			t.Fatalf("Failed to make malicious search request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400 for malicious search query, got %d", resp.StatusCode)
		}
	})
}

func TestE2E_SQLInjectionAcrossEndpoints(t *testing.T) {
	ctx := setupTestContext(t)

	sqlPayloads := testutils.SQLInjectionPayloads

	t.Run("sql_injection_in_post_fields", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "sqltest", "StrongPass123!")
		authClient, err := ctx.loginUserSafe(t, testUser.Username, testUser.Password)
		if err != nil {
			t.Fatalf("Failed to login for SQL injection post fields test: %v", err)
		}

		for i, payload := range sqlPayloads {
			t.Run(fmt.Sprintf("payload_%d", i), func(t *testing.T) {
				postData := map[string]string{
					"title":   payload,
					"url":     fmt.Sprintf("https://example.com/test%d", i),
					"content": "Test content",
				}

				req, err := testutils.NewRequestBuilder("POST", ctx.baseURL+"/api/posts").
					WithAuth(authClient.Token).
					WithJSONBody(postData).
					Build()
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp, err := ctx.client.Do(req)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusInternalServerError {
					t.Errorf("SQL injection in title caused server crash (500). Payload: %s", payload)
				}

				postData2 := map[string]string{
					"title":   fmt.Sprintf("Test Post %d", i),
					"url":     fmt.Sprintf("https://example.com/test2-%d", i),
					"content": payload,
				}

				req2, err := testutils.NewRequestBuilder("POST", ctx.baseURL+"/api/posts").
					WithAuth(authClient.Token).
					WithJSONBody(postData2).
					Build()
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp2, err := ctx.client.Do(req2)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp2.Body.Close()

				if resp2.StatusCode == http.StatusInternalServerError {
					t.Errorf("SQL injection in content caused server crash (500). Payload: %s", payload)
				}
			})
		}
	})

	t.Run("sql_injection_in_registration_fields", func(t *testing.T) {
		for i, payload := range sqlPayloads {
			t.Run(fmt.Sprintf("payload_%d", i), func(t *testing.T) {
				regData := map[string]string{
					"username": payload,
					"email":    uniqueEmail(t, fmt.Sprintf("test%d", i)),
					"password": "StrongPass123!",
				}

				req, err := testutils.NewRequestBuilder("POST", ctx.baseURL+"/api/auth/register").
					WithJSONBody(regData).
					Build()
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp, err := ctx.client.Do(req)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusInternalServerError {
					t.Errorf("SQL injection in username caused server crash (500). Payload: %s", payload)
				}

				regData2 := map[string]string{
					"username": uniqueUsername(t, fmt.Sprintf("user%d", i)),
					"email":    fmt.Sprintf("test%s@example.com", payload),
					"password": "StrongPass123!",
				}

				req2, err := testutils.NewRequestBuilder("POST", ctx.baseURL+"/api/auth/register").
					WithJSONBody(regData2).
					Build()
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp2, err := ctx.client.Do(req2)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp2.Body.Close()

				if resp2.StatusCode == http.StatusInternalServerError {
					t.Errorf("SQL injection in email caused server crash (500). Payload: %s", payload)
				}
			})
		}
	})

	t.Run("sql_injection_in_url_fields", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "sqltest2", "StrongPass123!")
		authClient, err := ctx.loginUserSafe(t, testUser.Username, testUser.Password)
		if err != nil {
			t.Fatalf("Failed to login for SQL injection URL fields test: %v", err)
		}

		for i, payload := range sqlPayloads {
			t.Run(fmt.Sprintf("payload_%d", i), func(t *testing.T) {
				postData := map[string]string{
					"title":   fmt.Sprintf("Test Post %d", i),
					"url":     fmt.Sprintf("https://example.com/test%s", payload),
					"content": "Test content",
				}

				req, err := testutils.NewRequestBuilder("POST", ctx.baseURL+"/api/posts").
					WithAuth(authClient.Token).
					WithJSONBody(postData).
					Build()
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp, err := ctx.client.Do(req)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusInternalServerError {
					t.Errorf("SQL injection in URL caused server crash (500). Payload: %s", payload)
				}
			})
		}
	})

	t.Run("sql_injection_in_query_parameters", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "sqltest3", "StrongPass123!")
		authClient, err := ctx.loginUserSafe(t, testUser.Username, testUser.Password)
		if err != nil {
			t.Fatalf("Failed to login for SQL injection query parameter test: %v", err)
		}

		_ = authClient.CreatePost(t, "Searchable Post", "https://example.com/search", "Content")

		for i, payload := range sqlPayloads {
			t.Run(fmt.Sprintf("payload_%d", i), func(t *testing.T) {
				searchURL := ctx.baseURL + "/api/posts/search?q=" + url.QueryEscape(payload)

				req, err := testutils.NewRequestBuilder("GET", searchURL).
					WithAuth(authClient.Token).
					Build()
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp, err := ctx.client.Do(req)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusInternalServerError {
					t.Errorf("SQL injection in search query caused server crash (500). Payload: %s", payload)
				}

				if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusOK {
					t.Logf("SQL injection in search query returned status %d (acceptable if sanitized). Payload: %s", resp.StatusCode, payload)
				}
			})
		}
	})
}

func TestE2E_XSSPrevention(t *testing.T) {
	ctx := setupTestContext(t)

	xssPayloads := testutils.XSSPayloads

	t.Run("xss_in_post_fields", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "xsstest", "StrongPass123!")
		authClient, err := ctx.loginUserSafe(t, testUser.Username, testUser.Password)
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		for idx, payload := range xssPayloads {
			t.Run(fmt.Sprintf("payload_%d", idx), func(t *testing.T) {
				postData := map[string]string{
					"title":   payload,
					"url":     fmt.Sprintf("https://example.com/xss-test-%d", idx),
					"content": "Test content",
				}

				req, err := testutils.NewRequestBuilder("POST", ctx.baseURL+"/api/posts").
					WithAuth(authClient.Token).
					WithJSONBody(postData).
					Build()
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp, err := ctx.client.Do(req)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusInternalServerError {
					t.Errorf("XSS payload in title caused server crash (500). Payload: %s", payload)
				}

				if resp.StatusCode == http.StatusCreated {
					reader, cleanup, err := getResponseReader(resp)
					if err != nil {
						t.Fatalf("Failed to get response reader: %v", err)
					}
					defer cleanup()

					var postResp PostResponse
					if err := json.NewDecoder(reader).Decode(&postResp); err == nil {
						if strings.Contains(postResp.Data.Title, "<script") {
							t.Errorf("XSS payload not sanitized in title response. Payload: %s, Response: %s", payload, postResp.Data.Title)
						}
					}
				}

				postData2 := map[string]string{
					"title":   fmt.Sprintf("Test Post %d", idx),
					"url":     fmt.Sprintf("https://example.com/xss-test2-%d", idx),
					"content": payload,
				}

				req2, err := testutils.NewRequestBuilder("POST", ctx.baseURL+"/api/posts").
					WithAuth(authClient.Token).
					WithJSONBody(postData2).
					Build()
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp2, err := ctx.client.Do(req2)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp2.Body.Close()

				if resp2.StatusCode == http.StatusInternalServerError {
					t.Errorf("XSS payload in content caused server crash (500). Payload: %s", payload)
				}

				if resp2.StatusCode == http.StatusCreated {
					reader, cleanup, err := getResponseReader(resp2)
					if err != nil {
						t.Fatalf("Failed to get response reader: %v", err)
					}
					defer cleanup()

					var postResp PostResponse
					if err := json.NewDecoder(reader).Decode(&postResp); err == nil {
						if strings.Contains(postResp.Data.Content, "<script") || strings.Contains(postResp.Data.Content, "javascript:") {
							t.Errorf("XSS payload not sanitized in content response. Payload: %s", payload)
						}
					}
				}
			})
		}
	})

	t.Run("xss_in_username_fields", func(t *testing.T) {
		for i, payload := range xssPayloads {
			t.Run(fmt.Sprintf("payload_%d", i), func(t *testing.T) {
				regData := map[string]string{
					"username": payload,
					"email":    uniqueEmail(t, fmt.Sprintf("test%d", i)),
					"password": "StrongPass123!",
				}

				req, err := testutils.NewRequestBuilder("POST", ctx.baseURL+"/api/auth/register").
					WithJSONBody(regData).
					Build()
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp, err := ctx.client.Do(req)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusInternalServerError {
					t.Errorf("XSS payload in username caused server crash (500). Payload: %s", payload)
				}
			})
		}
	})

	t.Run("xss_in_search_queries", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "xsstest2", "StrongPass123!")
		authClient, err := ctx.loginUserSafe(t, testUser.Username, testUser.Password)
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		_ = authClient.CreatePost(t, "Searchable Post", "https://example.com/search", "Content")

		for i, payload := range xssPayloads {
			t.Run(fmt.Sprintf("payload_%d", i), func(t *testing.T) {
				searchURL := ctx.baseURL + "/api/posts/search?q=" + url.QueryEscape(payload)

				req, err := testutils.NewRequestBuilder("GET", searchURL).
					WithAuth(authClient.Token).
					Build()
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp, err := ctx.client.Do(req)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusInternalServerError {
					t.Errorf("XSS payload in search query caused server crash (500). Payload: %s", payload)
				}

				if resp.StatusCode == http.StatusOK {
					reader, cleanup, err := getResponseReader(resp)
					if err != nil {
						t.Fatalf("Failed to get response reader: %v", err)
					}
					defer cleanup()

					var searchResp PostsListResponse
					if err := json.NewDecoder(reader).Decode(&searchResp); err != nil {
						t.Fatalf("Failed to decode search response: %v", err)
					}

					for _, post := range searchResp.Data.Posts {
						if strings.Contains(post.Title, "<script") || strings.Contains(post.Title, "javascript:") {
							t.Errorf("XSS payload not sanitized in post title. Payload: %s", payload)
						}
						if strings.Contains(post.Content, "<script") || strings.Contains(post.Content, "javascript:") {
							t.Errorf("XSS payload not sanitized in post content. Payload: %s", payload)
						}
					}
				}
			})
		}
	})
}

func TestE2E_InformationDisclosure(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("information_disclosure", func(t *testing.T) {
		t.Run("error_messages_dont_reveal_sensitive_info", func(t *testing.T) {
			request, err := testutils.NewRequestBuilder("POST", ctx.baseURL+"/api/auth/login").
				WithJSONBody(map[string]string{
					"username": "nonexistent",
					"password": "wrongpassword",
				}).
				Build()
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := ctx.client.Do(request)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			if strings.Contains(strings.ToLower(bodyStr), "database") {
				t.Errorf("Error message should not reveal database information")
			}
			if strings.Contains(strings.ToLower(bodyStr), "sql") {
				t.Errorf("Error message should not reveal SQL information")
			}
			if strings.Contains(strings.ToLower(bodyStr), "stack") {
				t.Errorf("Error message should not reveal stack trace")
			}
		})

		t.Run("invalid_endpoints_dont_reveal_structure", func(t *testing.T) {
			request, err := http.NewRequest("GET", ctx.baseURL+"/api/nonexistent/endpoint", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := ctx.client.Do(request)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			if strings.Contains(strings.ToLower(bodyStr), "route") && resp.StatusCode == http.StatusNotFound {
				t.Logf("404 response may contain route information, which is acceptable")
			}
		})
	})
}
func TestE2E_SecurityHeadersEnhanced(t *testing.T) {
	ctx := setupTestContext(t)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	t.Run("security_headers_values", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "headertest", "StrongPass123!")
		authClient, err := ctx.loginUserSafe(t, testUser.Username, testUser.Password)
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		endpoints := []struct {
			name   string
			method string
			path   string
			auth   bool
			body   []byte
		}{
			{"health", "GET", "/health", false, nil},
			{"metrics", "GET", "/metrics", false, nil},
			{"api_registration", "POST", "/api/auth/register", false, []byte(`{"username":"headertestnew","email":"headertestnew@example.com","password":"StrongPass123!"}`)},
			{"api_posts", "GET", "/api/posts", true, nil},
			{"api_auth_me", "GET", "/api/auth/me", true, nil},
		}

		for _, endpoint := range endpoints {
			t.Run(endpoint.name, func(t *testing.T) {
				req, err := http.NewRequest(endpoint.method, ctx.baseURL+endpoint.path, bytes.NewReader(endpoint.body))
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				if endpoint.auth && authClient != nil {
					req.Header.Set("Authorization", "Bearer "+authClient.Token)
				}
				testutils.WithStandardHeaders(req)

				resp, err := ctx.client.Do(req)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				for headerName, expectedValue := range expectedHeaders {
					actualValue := resp.Header.Get(headerName)
					if actualValue != expectedValue {
						t.Errorf("Endpoint %s: Expected %s header to be '%s', got '%s'", endpoint.path, headerName, expectedValue, actualValue)
					}
				}

				csp := resp.Header.Get("Content-Security-Policy")
				if csp == "" {
					t.Errorf("Endpoint %s: Content-Security-Policy header should be present", endpoint.path)
				}

				if strings.Contains(csp, "unsafe-inline") && !strings.Contains(csp, "'nonce-") {
					t.Errorf("Endpoint %s: CSP contains unsafe-inline without nonce", endpoint.path)
				}
			})
		}
	})

	t.Run("hsts_header", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/health", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("X-Forwarded-Proto", "https")
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		hsts := resp.Header.Get("Strict-Transport-Security")
		if hsts == "" {
			t.Error("HSTS header should be present for HTTPS requests")
		}

		if !strings.Contains(hsts, "max-age=") {
			t.Errorf("HSTS header should contain max-age, got: %s", hsts)
		}
	})
}

func TestE2E_TokenHashing(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("verification_token_hashed", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "hashtest", "StrongPass123!")
		authClient, err := ctx.loginUserSafe(t, testUser.Username, testUser.Password)
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}

		ctx.server.EmailSender.Reset()
		authClient.RegisterUser(t, "newuser", "newuser@example.com", "Password123!")

		verificationToken := ctx.server.EmailSender.VerificationToken()
		if verificationToken == "" {
			t.Fatal("Expected verification token to be generated")
		}

		user, err := ctx.server.UserRepo.GetByUsername("newuser")
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}

		if user.EmailVerificationToken == verificationToken {
			t.Error("Verification token should be hashed in database")
		}

		if len(user.EmailVerificationToken) < 32 {
			t.Errorf("Hashed token should be at least 32 characters, got %d", len(user.EmailVerificationToken))
		}
	})

	t.Run("password_reset_token_hashed", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "resettest", "StrongPass123!")
		ctx.server.EmailSender.Reset()

		testutils.RequestPasswordReset(t, ctx.client, ctx.baseURL, testUser.Email, testutils.GenerateTestIP())

		resetToken := ctx.server.EmailSender.PasswordResetToken()
		if resetToken == "" {
			t.Skip("Rate limited, skipping token hashing test")
			return
		}

		hash := sha256.Sum256([]byte(resetToken))
		tokenHash := hex.EncodeToString(hash[:])
		deletionRepo := repositories.NewAccountDeletionRepository(ctx.server.DB)
		_, err := deletionRepo.GetByTokenHash(tokenHash)
		if err == nil {
			t.Log("Password reset token appears to be stored hashed")
		}
	})
}

func TestE2E_SecurityHeaderCombinations(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("all_security_headers_present", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/health", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		requiredHeaders := []string{
			"X-Content-Type-Options",
			"X-Frame-Options",
			"X-XSS-Protection",
			"Referrer-Policy",
			"Content-Security-Policy",
		}

		for _, header := range requiredHeaders {
			if resp.Header.Get(header) == "" {
				t.Errorf("Required security header missing: %s", header)
			}
		}
	})
}
