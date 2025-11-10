package e2e

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"goyco/internal/testutils"
)

func TestE2E_CompleteUserJourney(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("complete_user_journey", func(t *testing.T) {
		_, authClient := ctx.createUserAndLogin(t, "testuser", "StrongPass123!")

		createdPost := authClient.CreatePost(t, "Test Post", "https://example.com/test", "This is a test post content")

		voteResp := authClient.VoteOnPost(t, createdPost.ID, "up")
		if !voteResp.Success {
			t.Errorf("Expected vote to be successful, got failure: %s", voteResp.Message)
		}

		postsResp := authClient.GetPosts(t)
		assertPostInList(t, postsResp, createdPost)

		searchResp := authClient.SearchPosts(t, "test")
		assertPostInList(t, searchResp, createdPost)

		authClient.Logout(t)
	})
}

func TestE2E_ErrorHandlingWorkflows(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("unauthenticated_user_workflow", func(t *testing.T) {
		request, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", bytes.NewReader([]byte(`{"title":"Test","url":"https://example.com"}`)))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		request.Header.Set("Content-Type", "application/json")
		testutils.WithStandardHeaders(request)

		resp, err := ctx.client.Do(request)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401 for unauthenticated post creation, got %d", resp.StatusCode)
		}

		request, err = testutils.NewRequestBuilder("GET", ctx.baseURL+"/api/auth/me").Build()
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err = ctx.client.Do(request)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401 for unauthenticated profile access, got %d", resp.StatusCode)
		}
	})

	t.Run("invalid_registration_workflow", func(t *testing.T) {
		invalidData := []struct {
			name string
			body []byte
		}{
			{
				name: "empty_username",
				body: []byte(`{"username":"","email":"test@example.com","password":"ValidPass123!"}`),
			},
			{
				name: "invalid_email",
				body: []byte(`{"username":"testuser","email":"invalid-email","password":"ValidPass123!"}`),
			},
			{
				name: "weak_password",
				body: []byte(`{"username":"testuser","email":"test@example.com","password":"123"}`),
			},
			{
				name: "malformed_json",
				body: []byte(`{"username": "test", "password": }`),
			},
		}

		for _, test := range invalidData {
			t.Run(test.name, func(t *testing.T) {
				request, err := testutils.NewRequestBuilder("POST", ctx.baseURL+"/api/auth/register").
					WithBody(bytes.NewReader(test.body)).
					Build()
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp, err := ctx.client.Do(request)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
					t.Errorf("Expected invalid registration to fail, got success status %d", resp.StatusCode)
				}
			})
		}
	})
}

func TestE2E_ConcurrentUserWorkflows(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("concurrent_user_workflows", func(t *testing.T) {
		users := ctx.createMultipleUsersWithCleanup(t, 3, "concurrent", "StrongPass123!")

		type result struct {
			userID uint
			err    error
		}

		results := make(chan result, len(users))
		var wg sync.WaitGroup
		done := make(chan struct{})

		for _, user := range users {
			u := user
			wg.Add(1)
			go func() {
				defer wg.Done()
				var err error
				authClient, loginErr := ctx.loginUserSafe(t, u.Username, u.Password)
				if loginErr != nil || authClient == nil || authClient.Token == "" {
					err = fmt.Errorf("User %s failed to login", u.Username)
				} else {
					postURL := fmt.Sprintf("https://example.com/concurrent/%d", u.ID)
					post, postErr := authClient.CreatePostSafe("Concurrent Post", postURL, "Content")
					if postErr != nil || post == nil || post.ID == 0 {
						err = fmt.Errorf("User %s failed to create post: %v", u.Username, postErr)
					} else {
						voteResp, voteErr := authClient.VoteOnPostSafe(post.ID, "up")
						if voteErr != nil || voteResp == nil || !voteResp.Success {
							err = fmt.Errorf("User %s failed to vote: %v", u.Username, voteErr)
						}
					}
				}
				select {
				case results <- result{userID: u.ID, err: err}:
				case <-done:
				}
			}()
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		timeout := time.After(10 * time.Second)
		successCount := 0
		receivedCount := 0

		for {
			select {
			case res, ok := <-results:
				if !ok {
					return
				}
				receivedCount++
				if res.err != nil {
					t.Errorf("Concurrent operation error for user %d: %v", res.userID, res.err)
				} else {
					successCount++
				}
				if receivedCount >= len(users) {
					return
				}
			case <-timeout:
				close(done)
				t.Errorf("Timeout waiting for concurrent operations to complete")
				return
			}
		}
	})
}

func TestE2E_SystemMonitoringWorkflows(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("system_monitoring_workflows", func(t *testing.T) {
		t.Run("health_endpoint", func(t *testing.T) {
			health := getHealth(t, ctx.client, ctx.baseURL)
			if !health.Success {
				t.Errorf("Expected health check to succeed, got failure: %s", health.Message)
			}
		})

		t.Run("metrics_endpoint", func(t *testing.T) {
			metrics := getMetrics(t, ctx.client, ctx.baseURL)
			if metrics == nil {
				t.Errorf("Expected metrics to be returned")
			}
		})
	})
}

func TestE2E_AccountDeletion(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("account_deletion_flow", func(t *testing.T) {
		_, authClient := ctx.createUserAndLogin(t, "testuser", "StrongPass123!")

		_ = authClient.CreatePost(t, "Test Post", "https://example.com/test", "Test content")

		statusCode, deletionResp := ctx.requestAccountDeletionExpectStatus(t, authClient.Token, http.StatusOK)
		if statusCode == http.StatusTooManyRequests {
			statusCode = retryOnRateLimit(t, 3, func() int {
				code, _ := ctx.requestAccountDeletionExpectStatus(t, authClient.Token, http.StatusOK)
				return code
			})
			if statusCode == http.StatusTooManyRequests {
				t.Skip("Skipping account deletion flow test: rate limited after retries")
				return
			}
		}

		if deletionResp == nil {
			t.Fatalf("Expected account deletion response, got nil")
		}
		if !deletionResp.Success {
			t.Errorf("Expected account deletion request to be successful, got %v", deletionResp.Success)
		}
		if deletionResp.Message == "" {
			t.Errorf("Expected deletion message to be present, got empty string")
		}
	})
}
