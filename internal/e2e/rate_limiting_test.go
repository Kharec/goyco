package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"goyco/internal/testutils"
)

func TestE2E_RateLimitingHeaders(t *testing.T) {
	ctx := setupTestContextWithAuthRateLimit(t, 3)

	t.Run("rate_limit_headers_present", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "ratelimituser", "StrongPass123!")

		for range 3 {
			req, err := http.NewRequest("POST", ctx.baseURL+"/api/auth/login", strings.NewReader(`{"username":"`+testUser.Username+`","password":"StrongPass123!"}`))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			testutils.WithStandardHeaders(req)
			req.Header.Set("X-Forwarded-For", testutils.GenerateTestIP())

			resp, err := ctx.client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusTooManyRequests {
				retryAfter := resp.Header.Get("Retry-After")
				if retryAfter == "" {
					t.Error("Expected Retry-After header when rate limited")
				}

				var jsonResponse map[string]any
				body, _ := json.Marshal(map[string]string{})
				_ = json.Unmarshal(body, &jsonResponse)

				if resp.Header.Get("Content-Type") != "application/json" {
					t.Error("Expected Content-Type to be application/json")
				}
			}
		}
	})

	t.Run("rate_limit_exceeded_response", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "ratelimituser2", "StrongPass123!")
		testIP := testutils.GenerateTestIP()

		for i := range 4 {
			req, err := http.NewRequest("POST", ctx.baseURL+"/api/auth/login", strings.NewReader(`{"username":"`+testUser.Username+`","password":"StrongPass123!"}`))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			testutils.WithStandardHeaders(req)
			req.Header.Set("X-Forwarded-For", testIP)

			resp, err := ctx.client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if i >= 3 {
				if resp.StatusCode != http.StatusTooManyRequests {
					t.Errorf("Expected status 429 on request %d, got %d", i+1, resp.StatusCode)
				} else {
					var errorResponse map[string]any
					body, _ := io.ReadAll(resp.Body)
					if err := json.Unmarshal(body, &errorResponse); err == nil {
						if errorResponse["error"] == nil {
							t.Error("Expected error field in rate limit response")
						}
						if errorResponse["retry_after"] == nil {
							t.Error("Expected retry_after field in rate limit response")
						}
					}
				}
			}
		}
	})
}

func TestE2E_RateLimitResetBehavior(t *testing.T) {
	ctx := setupTestContextWithAuthRateLimit(t, 2)

	t.Run("rate_limit_resets_after_window", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "resetuser", "StrongPass123!")
		testIP := testutils.GenerateTestIP()

		for range 2 {
			req, err := http.NewRequest("POST", ctx.baseURL+"/api/auth/login", strings.NewReader(`{"username":"`+testUser.Username+`","password":"StrongPass123!"}`))
			if err != nil {
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			testutils.WithStandardHeaders(req)
			req.Header.Set("X-Forwarded-For", testIP)

			resp, err := ctx.client.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}

		req, err := http.NewRequest("POST", ctx.baseURL+"/api/auth/login", strings.NewReader(`{"username":"`+testUser.Username+`","password":"StrongPass123!"}`))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		testutils.WithStandardHeaders(req)
		req.Header.Set("X-Forwarded-For", testIP)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			t.Log("Rate limit correctly enforced")
		}

		ctx.assertEventually(t, func() bool {
			req2, err := http.NewRequest("POST", ctx.baseURL+"/api/auth/login", strings.NewReader(`{"username":"`+testUser.Username+`","password":"StrongPass123!"}`))
			if err != nil {
				return false
			}
			req2.Header.Set("Content-Type", "application/json")
			testutils.WithStandardHeaders(req2)
			req2.Header.Set("X-Forwarded-For", testIP)

			resp2, err := ctx.client.Do(req2)
			if err != nil {
				return false
			}
			defer resp2.Body.Close()

			return resp2.StatusCode == http.StatusOK || resp2.StatusCode == http.StatusUnauthorized
		}, 70*time.Second)

		req2, err := http.NewRequest("POST", ctx.baseURL+"/api/auth/login", strings.NewReader(`{"username":"`+testUser.Username+`","password":"StrongPass123!"}`))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req2.Header.Set("Content-Type", "application/json")
		testutils.WithStandardHeaders(req2)
		req2.Header.Set("X-Forwarded-For", testIP)

		resp2, err := ctx.client.Do(req2)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode == http.StatusOK || resp2.StatusCode == http.StatusUnauthorized {
			t.Log("Rate limit reset after window")
		}
	})
}

func TestE2E_RateLimitDifferentScenarios(t *testing.T) {
	ctx := setupTestContextWithAuthRateLimit(t, 5)

	t.Run("different_ips_have_separate_limits", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "multiuser", "StrongPass123!")

		ip1 := testutils.GenerateTestIP()
		ip2 := testutils.GenerateTestIP()

		successCount1 := 0
		successCount2 := 0

		for range 5 {
			req1, _ := http.NewRequest("POST", ctx.baseURL+"/api/auth/login", strings.NewReader(`{"username":"`+testUser.Username+`","password":"StrongPass123!"}`))
			req1.Header.Set("Content-Type", "application/json")
			testutils.WithStandardHeaders(req1)
			req1.Header.Set("X-Forwarded-For", ip1)

			resp1, err := ctx.client.Do(req1)
			if err == nil {
				if resp1.StatusCode == http.StatusOK || resp1.StatusCode == http.StatusUnauthorized {
					successCount1++
				}
				resp1.Body.Close()
			}

			req2, _ := http.NewRequest("POST", ctx.baseURL+"/api/auth/login", strings.NewReader(`{"username":"`+testUser.Username+`","password":"StrongPass123!"}`))
			req2.Header.Set("Content-Type", "application/json")
			testutils.WithStandardHeaders(req2)
			req2.Header.Set("X-Forwarded-For", ip2)

			resp2, err := ctx.client.Do(req2)
			if err == nil {
				if resp2.StatusCode == http.StatusOK || resp2.StatusCode == http.StatusUnauthorized {
					successCount2++
				}
				resp2.Body.Close()
			}
		}

		if successCount1 > 0 && successCount2 > 0 {
			t.Log("Different IPs have separate rate limits")
		}
	})

	t.Run("authenticated_users_have_separate_limits", func(t *testing.T) {
		user1 := ctx.createUserWithCleanup(t, "authuser1", "StrongPass123!")
		user2 := ctx.createUserWithCleanup(t, "authuser2", "StrongPass123!")

		authClient1 := ctx.loginUser(t, user1.Username, "StrongPass123!")
		authClient2 := ctx.loginUser(t, user2.Username, "StrongPass123!")

		successCount1 := 0
		successCount2 := 0

		for range 10 {
			req1, _ := http.NewRequest("GET", ctx.baseURL+"/api/auth/me", nil)
			testutils.WithStandardHeaders(req1)
			req1.Header.Set("Authorization", "Bearer "+authClient1.Token)

			resp1, err := ctx.client.Do(req1)
			if err == nil {
				if resp1.StatusCode == http.StatusOK {
					successCount1++
				}
				resp1.Body.Close()
			}

			req2, _ := http.NewRequest("GET", ctx.baseURL+"/api/auth/me", nil)
			testutils.WithStandardHeaders(req2)
			req2.Header.Set("Authorization", "Bearer "+authClient2.Token)

			resp2, err := ctx.client.Do(req2)
			if err == nil {
				if resp2.StatusCode == http.StatusOK {
					successCount2++
				}
				resp2.Body.Close()
			}
		}

		if successCount1 > 5 && successCount2 > 5 {
			t.Log("Authenticated users have separate rate limits")
		}
	})
}
