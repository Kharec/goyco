package e2e

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"goyco/internal/testutils"
)

func TestE2E_Performance(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("response_times", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "perfuser", "StrongPass123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		endpoints := []struct {
			name string
			req  func() (*http.Request, error)
		}{
			{
				name: "health",
				req: func() (*http.Request, error) {
					return http.NewRequest("GET", ctx.baseURL+"/health", nil)
				},
			},
			{
				name: "posts_list",
				req: func() (*http.Request, error) {
					req, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
					if err == nil {
						testutils.WithStandardHeaders(req)
					}
					return req, err
				},
			},
			{
				name: "profile",
				req: func() (*http.Request, error) {
					req, err := http.NewRequest("GET", ctx.baseURL+"/api/auth/me", nil)
					if err == nil {
						req.Header.Set("Authorization", "Bearer "+authClient.Token)
						testutils.WithStandardHeaders(req)
					}
					return req, err
				},
			},
		}

		for _, endpoint := range endpoints {
			t.Run(endpoint.name, func(t *testing.T) {
				var totalTime time.Duration
				iterations := 10

				for range iterations {
					req, err := endpoint.req()
					if err != nil {
						t.Fatalf("Failed to create request: %v", err)
					}

					start := time.Now()
					resp, err := ctx.client.Do(req)
					duration := time.Since(start)

					if err != nil {
						t.Fatalf("Request failed: %v", err)
					}
					resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						t.Errorf("Expected 200, got %d", resp.StatusCode)
					}

					totalTime += duration
				}

				avgTime := totalTime / time.Duration(iterations)
				if avgTime > 500*time.Millisecond {
					t.Errorf("Average response time %v exceeds 500ms", avgTime)
				}
			})
		}
	})

	t.Run("concurrent_requests", func(t *testing.T) {
		ctx.createUserWithCleanup(t, "concurrentperf", "StrongPass123!")

		concurrency := 20
		requestsPerGoroutine := 5
		var successCount int64
		var errorCount int64

		var wg sync.WaitGroup
		for range concurrency {
			wg.Go(func() {
				for range requestsPerGoroutine {
					req, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						continue
					}
					testutils.WithStandardHeaders(req)

					resp, err := ctx.client.Do(req)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						continue
					}
					resp.Body.Close()

					if resp.StatusCode == http.StatusOK {
						atomic.AddInt64(&successCount, 1)
					} else {
						atomic.AddInt64(&errorCount, 1)
					}
				}
			})
		}

		wg.Wait()

		totalRequests := int64(concurrency * requestsPerGoroutine)
		if successCount < totalRequests*8/10 {
			t.Errorf("Expected at least 80%% success rate, got %d/%d successful", successCount, totalRequests)
		}
	})

	t.Run("database_query_performance", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "dbperf", "StrongPass123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		for i := range 10 {
			authClient.CreatePost(t, fmt.Sprintf("Post %d", i), fmt.Sprintf("https://example.com/%d", i), "Content")
		}

		start := time.Now()
		postsResp := authClient.GetPosts(t)
		duration := time.Since(start)

		if len(postsResp.Data.Posts) < 10 {
			t.Errorf("Expected at least 10 posts, got %d", len(postsResp.Data.Posts))
		}

		if duration > 1*time.Second {
			t.Errorf("Posts query took %v, expected under 1s", duration)
		}
	})

	t.Run("memory_usage", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "memuser", "StrongPass123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		initialPosts := 50
		for i := range initialPosts {
			authClient.CreatePost(t, fmt.Sprintf("Memory Test Post %d", i), fmt.Sprintf("https://example.com/mem%d", i), "Content")
		}

		req, err := http.NewRequest("GET", ctx.baseURL+"/api/posts?limit=100", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200, got %d", resp.StatusCode)
		}

		var postsResp testutils.PostsListResponse
		reader := resp.Body
		if resp.Header.Get("Content-Encoding") == "gzip" {
			gzReader, err := gzip.NewReader(resp.Body)
			if err != nil {
				t.Fatalf("Failed to create gzip reader: %v", err)
			}
			defer gzReader.Close()
			reader = gzReader
		}
		if err := json.NewDecoder(reader).Decode(&postsResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(postsResp.Data.Posts) < initialPosts {
			t.Errorf("Expected at least %d posts, got %d", initialPosts, len(postsResp.Data.Posts))
		}
	})
}

func TestE2E_LoadTest(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("sustained_load", func(t *testing.T) {
		ctx.createUserWithCleanup(t, "loaduser", "StrongPass123!")

		duration := 5 * time.Second
		requestsPerSecond := 10
		ticker := time.NewTicker(time.Second / time.Duration(requestsPerSecond))
		defer ticker.Stop()

		var successCount int64
		var errorCount int64
		done := make(chan bool)

		go func() {
			time.Sleep(duration)
			done <- true
		}()

		for {
			select {
			case <-done:
				totalRequests := successCount + errorCount
				if totalRequests == 0 {
					t.Error("No requests were made")
					return
				}
				successRate := float64(successCount) / float64(totalRequests)
				if successRate < 0.9 {
					t.Errorf("Success rate %.2f%% below 90%% threshold", successRate*100)
				}
				return
			case <-ticker.C:
				go func() {
					req, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						return
					}
					testutils.WithStandardHeaders(req)

					resp, err := ctx.client.Do(req)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						return
					}
					resp.Body.Close()

					if resp.StatusCode == http.StatusOK {
						atomic.AddInt64(&successCount, 1)
					} else {
						atomic.AddInt64(&errorCount, 1)
					}
				}()
			}
		}
	})
}

func TestE2E_ConcurrentWrites(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("concurrent_post_creation", func(t *testing.T) {
		users := ctx.createMultipleUsersWithCleanup(t, 5, "writeuser", "StrongPass123!")
		var wg sync.WaitGroup
		var successCount int64
		var errorCount int64

		for _, user := range users {
			u := user
			wg.Go(func() {
				authClient, err := ctx.loginUserSafe(t, u.Username, u.Password)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					return
				}

				for i := range 5 {
					post, err := authClient.CreatePostSafe(
						fmt.Sprintf("Concurrent Post %d", i),
						fmt.Sprintf("https://example.com/concurrent%d-%d", u.ID, i),
						"Content",
					)
					if err == nil && post != nil {
						atomic.AddInt64(&successCount, 1)
					} else {
						atomic.AddInt64(&errorCount, 1)
					}
				}
			})
		}

		wg.Wait()

		expectedPosts := int64(len(users) * 5)
		if successCount < expectedPosts*7/10 {
			t.Errorf("Expected at least 70%% success rate, got %d/%d successful (errors: %d)", successCount, expectedPosts, errorCount)
		}
	})
}

func TestE2E_ResponseSize(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("large_response", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "sizetest", "StrongPass123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		for i := range 100 {
			authClient.CreatePost(t, fmt.Sprintf("Post %d", i), fmt.Sprintf("https://example.com/%d", i), "Content")
		}

		req, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		responseSize := buf.Len()

		if responseSize > 10*1024*1024 {
			t.Errorf("Response size %d bytes exceeds 10MB limit", responseSize)
		}
	})
}

func TestE2E_Throughput(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("requests_per_second", func(t *testing.T) {
		ctx.createUserWithCleanup(t, "throughput", "StrongPass123!")

		duration := 3 * time.Second
		start := time.Now()
		var requestCount int64

		for time.Since(start) < duration {
			req, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
			if err != nil {
				continue
			}
			testutils.WithStandardHeaders(req)

			resp, err := ctx.client.Do(req)
			if err == nil {
				resp.Body.Close()
				atomic.AddInt64(&requestCount, 1)
			}
		}

		elapsed := time.Since(start)
		rps := float64(requestCount) / elapsed.Seconds()

		if rps < 10 {
			t.Errorf("Throughput %.2f req/s below 10 req/s threshold", rps)
		}
	})
}
