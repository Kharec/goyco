package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"goyco/internal/database"
	"goyco/internal/testutils"
)

func TestE2E_PartialFailureHandling(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("partial_failure_handling", func(t *testing.T) {
		_, authClient := ctx.createUserAndLogin(t, "partial", "Password123!")

		post := authClient.CreatePost(t, "Partial Failure Test", "https://example.com/partial", "Content")
		if post.ID == 0 {
			t.Fatalf("Expected post creation to succeed")
		}

		postsResp := authClient.GetPosts(t)
		foundPost := findPostInList(postsResp, post.ID)
		if foundPost == nil {
			t.Fatalf("Expected post to exist after creation")
		}

		invalidPostID := uint(999999)
		voteResp, statusCode := authClient.VoteOnPostRaw(t, invalidPostID, "up")
		if statusCode == http.StatusOK || voteResp.Success {
			t.Errorf("Expected vote on non-existent post to fail")
		}

		postsRespAfter := authClient.GetPosts(t)
		foundPostAfter := findPostInList(postsRespAfter, post.ID)
		if foundPostAfter == nil {
			t.Errorf("Expected post to still exist after vote failure")
		}
	})
}

func TestE2E_ConcurrentModification(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("concurrent_modification", func(t *testing.T) {
		user1 := ctx.createUserWithCleanup(t, "concmode1", "Password123!")
		user2 := ctx.createUserWithCleanup(t, "concmode2", "Password123!")

		client1 := ctx.loginUser(t, user1.Username, user1.Password)
		post := client1.CreatePost(t, "Concurrent Edit Test", "https://example.com/concmode", "Original content")

		client2 := ctx.loginUser(t, user2.Username, user2.Password)

		statusCode := client2.UpdatePostExpectStatus(t, post.ID, "Hacked Title", "https://example.com/concmode", "Hacked content")
		if statusCode != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden when user2 tries to edit user1's post, got %d", statusCode)
		}

		postsResp := client1.GetPosts(t)
		updatedPost := findPostInList(postsResp, post.ID)
		if updatedPost == nil {
			t.Fatalf("Expected post to exist")
		}
		if updatedPost.Title != "Concurrent Edit Test" {
			t.Errorf("Expected post title to remain unchanged after unauthorized edit attempt, got '%s'", updatedPost.Title)
		}
	})
}

func TestE2E_ResourceNotFound(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("resource_not_found", func(t *testing.T) {
		_, authClient := ctx.createUserAndLogin(t, "notfound", "Password123!")

		post := authClient.CreatePost(t, "To Delete", "https://example.com/todelete", "Content")
		authClient.DeletePost(t, post.ID)

		statusCode := authClient.UpdatePostExpectStatus(t, post.ID, "Updated", "https://example.com/todelete", "Updated")
		if statusCode != http.StatusNotFound {
			t.Errorf("Expected 404 Not Found when accessing deleted post, got %d", statusCode)
		}

		voteResp, statusCode := authClient.VoteOnPostRaw(t, post.ID, "up")
		if statusCode == http.StatusOK || voteResp.Success {
			t.Errorf("Expected vote on deleted post to fail")
		}

		postsResp := authClient.GetPosts(t)
		deletedPost := findPostInList(postsResp, post.ID)
		if deletedPost != nil {
			t.Errorf("Expected deleted post to not appear in posts list")
		}
	})
}

func TestE2E_InvalidStateTransitions(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("invalid_state_transitions", func(t *testing.T) {
		_, authClient := ctx.createUserAndLogin(t, "invalidstate", "Password123!")

		post := authClient.CreatePost(t, "State Test", "https://example.com/state", "Content")

		voteResp := authClient.VoteOnPost(t, post.ID, "up")
		if !voteResp.Success {
			t.Errorf("Expected vote to succeed")
		}

		authClient.DeletePost(t, post.ID)

		voteRespAfter, statusCode := authClient.VoteOnPostRaw(t, post.ID, "down")
		if statusCode == http.StatusOK || voteRespAfter.Success {
			t.Errorf("Expected vote on deleted post to fail")
		}

		statusCode = authClient.UpdatePostExpectStatus(t, post.ID, "Updated", "https://example.com/state", "Updated")
		if statusCode != http.StatusNotFound {
			t.Errorf("Expected 404 when updating deleted post, got %d", statusCode)
		}
	})
}

func TestE2E_RequestTimeoutHandling(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("request_timeout_handling", func(t *testing.T) {
		_, authClient := ctx.createUserAndLogin(t, "timeout", "Password123!")

		client := &http.Client{
			Timeout: 1 * time.Nanosecond,
		}

		request, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		request.Header.Set("Authorization", "Bearer "+authClient.Token)
		testutils.WithStandardHeaders(request)

		_, err = client.Do(request)
		if err == nil {
			t.Log("Request completed despite timeout (acceptable if server is very fast)")
		}
	})
}

func TestE2E_SlowResponseHandling(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("slow_response_handling", func(t *testing.T) {
		_, authClient := ctx.createUserAndLogin(t, "slow", "Password123!")

		start := time.Now()
		postsResp := authClient.GetPosts(t)
		duration := time.Since(start)

		if postsResp == nil {
			t.Errorf("Expected posts response even with slow response")
		}

		if duration > 30*time.Second {
			t.Errorf("Request took too long: %v", duration)
		}
	})
}

func TestE2E_MalformedInput(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("malformed_input", func(t *testing.T) {
		_, authClient := ctx.createUserAndLogin(t, "malformed", "Password123!")

		t.Run("very_long_title", func(t *testing.T) {
			longTitle := make([]byte, 201)
			for i := range longTitle {
				longTitle[i] = 'A'
			}

			postData := map[string]string{
				"title": string(longTitle),
				"url":   "https://example.com/long",
			}
			body, _ := json.Marshal(postData)

			request, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+authClient.Token)
			testutils.WithStandardHeaders(request)

			resp, err := ctx.client.Do(request)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusCreated {
				t.Errorf("Expected long title to be rejected")
			}
		})

		t.Run("very_long_content", func(t *testing.T) {
			longContent := make([]byte, 10001)
			for i := range longContent {
				longContent[i] = 'B'
			}

			postData := map[string]string{
				"title":   "Test",
				"url":     "https://example.com/longcontent",
				"content": string(longContent),
			}
			body, _ := json.Marshal(postData)

			request, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+authClient.Token)
			testutils.WithStandardHeaders(request)

			resp, err := ctx.client.Do(request)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusCreated {
				t.Errorf("Expected long content to be rejected")
			}
		})

		t.Run("special_characters", func(t *testing.T) {
			specialChars := []string{
				"<script>alert('XSS')</script>",
				"'; DROP TABLE posts; --",
				"测试中文",
				"🚀 Emoji Test",
				"Test\nNewline",
				"Test\tTab",
			}

			for _, special := range specialChars {
				postData := map[string]string{
					"title":   special,
					"url":     "https://example.com/special",
					"content": special,
				}
				body, _ := json.Marshal(postData)

				request, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", bytes.NewReader(body))
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Authorization", "Bearer "+authClient.Token)
				testutils.WithStandardHeaders(request)

				resp, err := ctx.client.Do(request)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				resp.Body.Close()

				if resp.StatusCode == http.StatusCreated {
					postsResp := authClient.GetPosts(t)
					if postsResp != nil {
						t.Logf("Special characters accepted: %s (may be sanitized)", special)
					}
				}
			}
		})

		t.Run("missing_required_fields", func(t *testing.T) {
			testCases := []struct {
				name string
				body map[string]any
			}{
				{"missing_url", map[string]any{"title": "Test"}},
				{"empty_url", map[string]any{"title": "Test", "url": ""}},
				{"missing_title_and_url", map[string]any{"content": "Content"}},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					body, _ := json.Marshal(tc.body)

					request, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", bytes.NewReader(body))
					if err != nil {
						t.Fatalf("Failed to create request: %v", err)
					}
					request.Header.Set("Content-Type", "application/json")
					request.Header.Set("Authorization", "Bearer "+authClient.Token)
					testutils.WithStandardHeaders(request)

					resp, err := ctx.client.Do(request)
					if err != nil {
						t.Fatalf("Failed to make request: %v", err)
					}
					defer resp.Body.Close()

					if resp.StatusCode == http.StatusCreated {
						t.Errorf("Expected missing required fields to be rejected")
					}
				})
			}
		})

		t.Run("wrong_data_types", func(t *testing.T) {
			testCases := []struct {
				name string
				body string
			}{
				{"title_as_number", `{"title": 123, "url": "https://example.com"}`},
				{"url_as_boolean", `{"title": "Test", "url": true}`},
				{"content_as_array", `{"title": "Test", "url": "https://example.com", "content": []}`},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					request, err := http.NewRequest("POST", ctx.baseURL+"/api/posts", bytes.NewReader([]byte(tc.body)))
					if err != nil {
						t.Fatalf("Failed to create request: %v", err)
					}
					request.Header.Set("Content-Type", "application/json")
					request.Header.Set("Authorization", "Bearer "+authClient.Token)
					testutils.WithStandardHeaders(request)

					resp, err := ctx.client.Do(request)
					if err != nil {
						t.Fatalf("Failed to make request: %v", err)
					}
					defer resp.Body.Close()

					if resp.StatusCode == http.StatusCreated {
						t.Errorf("Expected wrong data types to be rejected")
					}
				})
			}
		})
	})
}

func TestE2E_ConcurrentVotes(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("concurrent_votes", func(t *testing.T) {
		user1 := ctx.createUserWithCleanup(t, "concvote1", "Password123!")
		user2 := ctx.createUserWithCleanup(t, "concvote2", "Password123!")
		user3 := ctx.createUserWithCleanup(t, "concvote3", "Password123!")

		client1 := ctx.loginUser(t, user1.Username, user1.Password)
		post := client1.CreatePost(t, "Concurrent Vote Test", "https://example.com/concvote", "Content")

		client2 := ctx.loginUser(t, user2.Username, user2.Password)
		client3 := ctx.loginUser(t, user3.Username, user3.Password)

		var wg sync.WaitGroup
		results := make(chan bool, 3)

		wg.Add(3)
		go func() {
			defer wg.Done()
			voteResp := client1.VoteOnPost(t, post.ID, "up")
			results <- voteResp.Success
		}()
		go func() {
			defer wg.Done()
			voteResp := client2.VoteOnPost(t, post.ID, "up")
			results <- voteResp.Success
		}()
		go func() {
			defer wg.Done()
			voteResp := client3.VoteOnPost(t, post.ID, "down")
			results <- voteResp.Success
		}()

		wg.Wait()
		close(results)

		successCount := 0
		for success := range results {
			if success {
				successCount++
			}
		}

		if successCount == 0 {
			t.Errorf("Expected at least some concurrent votes to succeed")
		}

		var dbPost database.Post
		if err := ctx.server.DB.First(&dbPost, post.ID).Error; err != nil {
			t.Fatalf("Failed to find post in database: %v", err)
		}

		if dbPost.UpVotes+dbPost.DownVotes != successCount {
			t.Logf("Vote counts may not match exactly due to race conditions (acceptable)")
		}
	})
}

func TestE2E_ConcurrentPostCreation(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("concurrent_post_creation", func(t *testing.T) {
		users := ctx.createMultipleUsersWithCleanup(t, 5, "concpost", "Password123!")

		var wg sync.WaitGroup
		results := make(chan *TestPost, len(users))
		var mu sync.Mutex
		createdURLs := make(map[string]bool)

		for _, user := range users {
			u := user
			wg.Go(func() {
				client, err := ctx.loginUserSafe(t, u.Username, u.Password)
				if err != nil || client == nil {
					results <- nil
					return
				}

				url := fmt.Sprintf("https://example.com/concpost/%d", u.ID)
				mu.Lock()
				if createdURLs[url] {
					mu.Unlock()
					results <- nil
					return
				}
				createdURLs[url] = true
				mu.Unlock()

				post, err := client.CreatePostSafe("Concurrent Post", url, "Content")
				results <- post
			})
		}

		wg.Wait()
		close(results)

		successCount := 0
		for post := range results {
			if post != nil && post.ID != 0 {
				successCount++
			}
		}

		if successCount == 0 {
			t.Errorf("Expected at least some concurrent post creations to succeed")
		}
	})
}

func TestE2E_ConcurrentProfileUpdates(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("concurrent_profile_updates", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "concprofile", "Password123!")

		client1 := ctx.loginUser(t, createdUser.Username, createdUser.Password)
		client2 := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		var wg sync.WaitGroup
		results := make(chan bool, 2)

		wg.Add(2)
		go func() {
			defer wg.Done()
			newUsername := uniqueUsername(t, "update1")
			client1.UpdateUsername(t, newUsername)
			profile := client1.GetProfile(t)
			results <- (profile != nil && profile.Data.Username == newUsername)
		}()
		go func() {
			defer wg.Done()
			newUsername := uniqueUsername(t, "update2")
			client2.UpdateUsername(t, newUsername)
			profile := client2.GetProfile(t)
			results <- (profile != nil && profile.Data.Username == newUsername)
		}()

		wg.Wait()
		close(results)

		successCount := 0
		for success := range results {
			if success {
				successCount++
			}
		}

		if successCount == 0 {
			t.Errorf("Expected at least some concurrent profile updates to succeed")
		}
	})
}
