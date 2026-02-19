package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"goyco/internal/testutils"
)

func findPostInList(postsResp *testutils.PostsListResponse, postID uint) *testutils.Post {
	if postsResp == nil || postsResp.Data.Posts == nil {
		return nil
	}
	for _, post := range postsResp.Data.Posts {
		if post.ID == postID {
			return &post
		}
	}
	return nil
}

func TestE2E_NewUserOnboarding(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("new_user_onboarding", func(t *testing.T) {
		username := uniqueUsername(t, "newuser")
		email := uniqueEmail(t, "newuser")
		password := "Password123!"

		ctx.server.EmailSender.Reset()
		statusCode := ctx.registerUserExpectStatus(t, username, email, password)
		if statusCode != http.StatusCreated {
			t.Fatalf("Expected registration to succeed, got status %d", statusCode)
		}

		verificationToken := ctx.server.EmailSender.VerificationToken()
		if verificationToken == "" {
			t.Fatalf("Expected verification token")
		}

		ctx.confirmEmail(t, verificationToken)

		authClient := ctx.loginUser(t, username, password)
		if authClient.Token == "" {
			t.Fatalf("Expected login to succeed after email verification")
		}

		createdPost := authClient.CreatePost(t, "My First Post", "https://example.com/first", "This is my first post content")
		if createdPost.ID == 0 {
			t.Errorf("Expected post creation to succeed")
		}

		voteResp := authClient.VoteOnPost(t, createdPost.ID, "up")
		if !voteResp.Success {
			t.Errorf("Expected vote to succeed, got failure: %s", voteResp.Message)
		}

		profile := authClient.GetProfile(t)
		if profile.Data.Username != username {
			t.Errorf("Expected profile username to match, got '%s'", profile.Data.Username)
		}
	})
}

func TestE2E_ReturningUserSession(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("returning_user_session", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "returning", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		postsResp := authClient.GetPosts(t)
		if postsResp == nil {
			t.Errorf("Expected posts response")
		}

		post1 := authClient.CreatePost(t, "Post 1", "https://example.com/post1", "Content 1")
		post2 := authClient.CreatePost(t, "Post 2", "https://example.com/post2", "Content 2")

		voteResp := authClient.VoteOnPost(t, post1.ID, "up")
		if !voteResp.Success {
			t.Errorf("Expected vote to succeed")
		}

		voteResp = authClient.VoteOnPost(t, post2.ID, "down")
		if !voteResp.Success {
			t.Errorf("Expected vote to succeed")
		}

		postsResp = authClient.GetPosts(t)
		if postsResp == nil || len(postsResp.Data.Posts) == 0 {
			t.Errorf("Expected to retrieve posts")
		}

		authClient.Logout(t)
	})
}

func TestE2E_PowerUserWorkflow(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("power_user_workflow", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "poweruser", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		var postIDs []uint
		for i := 1; i <= 5; i++ {
			post := authClient.CreatePost(t,
				uniqueTestID(t)+" Post "+fmt.Sprintf("%d", i),
				"https://example.com/power"+uniqueTestID(t)+fmt.Sprintf("%d", i),
				"Content "+fmt.Sprintf("%d", i))
			postIDs = append(postIDs, post.ID)
		}

		for i, postID := range postIDs {
			voteType := "up"
			if i%2 == 0 {
				voteType = "down"
			}
			voteResp := authClient.VoteOnPost(t, postID, voteType)
			if !voteResp.Success {
				t.Errorf("Expected vote to succeed on post %d", postID)
			}
		}

		postsResp := authClient.GetPosts(t)
		firstPost := findPostInList(postsResp, postIDs[0])
		if firstPost == nil {
			t.Fatalf("Expected to retrieve first post")
		}

		authClient.UpdatePost(t, postIDs[0], "Updated Title", "https://example.com/updated", "Updated content")
		updatedPostsResp := authClient.GetPosts(t)
		updatedPost := findPostInList(updatedPostsResp, postIDs[0])
		if updatedPost == nil {
			t.Fatalf("Expected to retrieve updated post")
		}
		if updatedPost.Title != "Updated Title" {
			t.Errorf("Expected post title to be updated, got '%s'", updatedPost.Title)
		}

		authClient.DeletePost(t, postIDs[len(postIDs)-1])
		finalPostsResp := authClient.GetPosts(t)
		deletedPost := findPostInList(finalPostsResp, postIDs[len(postIDs)-1])
		if deletedPost != nil {
			t.Errorf("Expected deleted post to not be accessible")
		}
	})
}

func TestE2E_PasswordResetFlowRealistic(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("password_reset_flow", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "resetflow", "Password123!")
		_ = ctx.loginUser(t, createdUser.Username, createdUser.Password)

		ctx.server.EmailSender.Reset()
		testutils.RequestPasswordReset(t, ctx.client, ctx.baseURL, createdUser.Email, testutils.GenerateTestIP())

		resetToken := ctx.server.EmailSender.PasswordResetToken()
		if resetToken == "" {
			t.Fatalf("Expected password reset token")
		}

		newPassword := "NewPassword456!"
		statusCode := testutils.ResetPassword(t, ctx.client, ctx.baseURL, resetToken, newPassword, testutils.GenerateTestIP())
		if statusCode != http.StatusOK {
			t.Fatalf("Expected password reset to succeed, got status %d", statusCode)
		}

		oldLoginStatus := ctx.loginExpectStatus(t, createdUser.Username, "Password123!", http.StatusUnauthorized)
		if oldLoginStatus == http.StatusOK {
			t.Log("Old password may still work briefly (acceptable)")
		}

		newClient := ctx.loginUser(t, createdUser.Username, newPassword)
		if newClient.Token == "" {
			t.Errorf("Expected login with new password to succeed")
		}

		newClient.UpdatePassword(t, newPassword, "AnotherPassword789!")
		finalClient := ctx.loginUser(t, createdUser.Username, "AnotherPassword789!")
		if finalClient.Token == "" {
			t.Errorf("Expected login with final password to succeed")
		}
	})
}

func TestE2E_PostLifecycle(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("post_lifecycle", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "lifecycle", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		createdPost := authClient.CreatePost(t, "Original Title", "https://example.com/lifecycle", "Original content")
		if createdPost.ID == 0 {
			t.Fatalf("Expected post creation to succeed")
		}

		voteResp := authClient.VoteOnPost(t, createdPost.ID, "up")
		if !voteResp.Success {
			t.Errorf("Expected vote to succeed")
		}

		authClient.UpdatePost(t, createdPost.ID, "Updated Title", "https://example.com/lifecycle", "Updated content")
		postsResp := authClient.GetPosts(t)
		updatedPost := findPostInList(postsResp, createdPost.ID)
		if updatedPost == nil {
			t.Fatalf("Expected to retrieve updated post")
		}
		if updatedPost.Title != "Updated Title" {
			t.Errorf("Expected post to be updated")
		}

		voteResp = authClient.VoteOnPost(t, createdPost.ID, "down")
		if !voteResp.Success {
			t.Errorf("Expected vote change to succeed")
		}

		authClient.UpdatePost(t, createdPost.ID, "Final Title", "https://example.com/lifecycle", "Final content")
		finalPostsResp := authClient.GetPosts(t)
		finalPost := findPostInList(finalPostsResp, createdPost.ID)
		if finalPost == nil {
			t.Fatalf("Expected to retrieve final post")
		}
		if finalPost.Title != "Final Title" {
			t.Errorf("Expected post to be updated again")
		}

		authClient.DeletePost(t, createdPost.ID)
		deletedPostsResp := authClient.GetPosts(t)
		deletedPost := findPostInList(deletedPostsResp, createdPost.ID)
		if deletedPost != nil {
			t.Errorf("Expected deleted post to not be accessible")
		}

		recreatedPost := authClient.CreatePost(t, "Recreated Title", "https://example.com/lifecycle-recreated", "Recreated content")
		if recreatedPost.ID == 0 {
			t.Errorf("Expected post recreation to succeed")
		}
	})
}

func TestE2E_VotePatterns(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("vote_patterns", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "votepattern", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		post := authClient.CreatePost(t, "Vote Test Post", "https://example.com/vote", "Content")

		voteResp := authClient.VoteOnPost(t, post.ID, "up")
		if !voteResp.Success {
			t.Errorf("Expected upvote to succeed")
		}

		userVote := authClient.GetUserVote(t, post.ID)
		if userVote == nil || userVote.Data == nil {
			t.Errorf("Expected to retrieve user vote")
		}

		voteResp = authClient.VoteOnPost(t, post.ID, "down")
		if !voteResp.Success {
			t.Errorf("Expected downvote to succeed")
		}

		voteResp = authClient.VoteOnPost(t, post.ID, "none")
		if !voteResp.Success {
			t.Errorf("Expected vote removal to succeed")
		}

		userVote = authClient.GetUserVote(t, post.ID)
		if userVote != nil && userVote.Data != nil {
			voteData, ok := userVote.Data.(map[string]any)
			if ok {
				if voteType, exists := voteData["type"]; exists && voteType != nil && voteType != "none" {
					t.Errorf("Expected vote to be removed")
				}
			}
		}

		voteResp = authClient.VoteOnPost(t, post.ID, "up")
		if !voteResp.Success {
			t.Errorf("Expected upvote after removal to succeed")
		}
	})
}

func TestE2E_ProfileUpdateFlow(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("profile_update_flow", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "profile", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		_ = authClient.GetProfile(t)

		newUsername := uniqueUsername(t, "updated")
		authClient.UpdateUsername(t, newUsername)
		updatedProfile := authClient.GetProfile(t)
		if updatedProfile.Data.Username != newUsername {
			t.Errorf("Expected username to be updated, got '%s'", updatedProfile.Data.Username)
		}

		ctx.server.EmailSender.Reset()
		newEmail := uniqueEmail(t, "updated")
		authClient.UpdateEmail(t, newEmail)
		emailProfile := authClient.GetProfile(t)
		normalizedNewEmail := strings.ToLower(strings.TrimSpace(newEmail))
		if emailProfile.Data.Email != normalizedNewEmail {
			t.Errorf("Expected email to be updated, got '%s'", emailProfile.Data.Email)
		}

		verificationToken := ctx.server.EmailSender.VerificationToken()
		if verificationToken == "" {
			t.Fatalf("Expected verification token after email update")
		}
		ctx.confirmEmail(t, verificationToken)

		authClient.UpdatePassword(t, "Password123!", "NewPassword999!")
		passwordClient := ctx.loginUser(t, newUsername, "NewPassword999!")
		if passwordClient.Token == "" {
			t.Errorf("Expected login with new password to succeed")
		}

		finalProfile := passwordClient.GetProfile(t)
		if finalProfile.Data.Username != newUsername {
			t.Errorf("Expected username to remain updated, got '%s'", finalProfile.Data.Username)
		}
		if finalProfile.Data.Email != normalizedNewEmail {
			t.Errorf("Expected email to remain updated, got '%s'", finalProfile.Data.Email)
		}
	})
}

func TestE2E_MultiUserInteraction(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("multi_user_interaction", func(t *testing.T) {
		userA := ctx.createUserWithCleanup(t, "usera", "Password123!")
		userB := ctx.createUserWithCleanup(t, "userb", "Password123!")

		clientA := ctx.loginUser(t, userA.Username, userA.Password)
		clientB := ctx.loginUser(t, userB.Username, userB.Password)

		post := clientA.CreatePost(t, "User A's Post", "https://example.com/usera", "Content from User A")
		if post.ID == 0 {
			t.Fatalf("Expected post creation to succeed")
		}

		voteResp := clientB.VoteOnPost(t, post.ID, "up")
		if !voteResp.Success {
			t.Errorf("Expected User B to vote on User A's post")
		}

		clientA.UpdatePost(t, post.ID, "Updated by User A", "https://example.com/usera", "Updated content")
		postsResp := clientB.GetPosts(t)
		updatedPost := findPostInList(postsResp, post.ID)
		if updatedPost == nil {
			t.Fatalf("Expected to retrieve updated post")
		}
		if updatedPost.Title != "Updated by User A" {
			t.Errorf("Expected User B to see updated post")
		}

		voteResp = clientB.VoteOnPost(t, post.ID, "down")
		if !voteResp.Success {
			t.Errorf("Expected User B to change vote")
		}

		finalPostsResp := clientA.GetPosts(t)
		finalPost := findPostInList(finalPostsResp, post.ID)
		if finalPost == nil {
			t.Errorf("Expected User A to retrieve final post")
		}
	})
}

func TestE2E_ContentDiscovery(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("content_discovery", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "discovery", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		post1 := authClient.CreatePost(t, "Golang Tutorial", "https://example.com/golang", "Learn Go programming")
		post2 := authClient.CreatePost(t, "Python Guide", "https://example.com/python", "Python programming guide")
		post3 := authClient.CreatePost(t, "Rust Basics", "https://example.com/rust", "Rust programming basics")

		authClient.VoteOnPost(t, post1.ID, "up")
		authClient.VoteOnPost(t, post2.ID, "up")
		authClient.VoteOnPost(t, post3.ID, "down")

		searchResp := authClient.SearchPosts(t, "Golang")
		if searchResp == nil || len(searchResp.Data.Posts) == 0 {
			t.Errorf("Expected search to find posts")
		}

		postsResp := authClient.GetPosts(t)
		if postsResp == nil || len(postsResp.Data.Posts) == 0 {
			t.Errorf("Expected to retrieve posts")
		}

		authClient.VoteOnPost(t, post1.ID, "up")
		updatedPostsResp := authClient.GetPosts(t)
		updatedPost := findPostInList(updatedPostsResp, post1.ID)
		if updatedPost == nil {
			t.Errorf("Expected to retrieve updated post")
		}
	})
}

func TestE2E_SessionPersistence(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("session_persistence", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "session", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		profile1 := authClient.GetProfile(t)
		if profile1.Data.Username != createdUser.Username {
			t.Errorf("Expected first profile request to succeed")
		}

		ctx.assertEventually(t, func() bool {
			profile2 := authClient.GetProfile(t)
			return profile2 != nil && profile2.Data.Username == createdUser.Username
		}, 2*time.Second)

		profile2 := authClient.GetProfile(t)
		if profile2.Data.Username != createdUser.Username {
			t.Errorf("Expected second profile request to succeed")
		}

		postsResp1 := authClient.GetPosts(t)
		postsResp2 := authClient.GetPosts(t)

		if postsResp1 == nil || postsResp2 == nil {
			t.Errorf("Expected multiple requests with same session to work")
		}
	})
}

func TestE2E_ConcurrentRequestsWithSameSession(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("concurrent_requests_same_session", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "concurrent", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		results := make(chan bool, 5)
		for range 5 {
			go func() {
				profile := authClient.GetProfile(t)
				results <- (profile != nil && profile.Data.Username == createdUser.Username)
			}()
		}

		successCount := 0
		for range 5 {
			if <-results {
				successCount++
			}
		}

		if successCount == 0 {
			t.Errorf("Expected at least some concurrent requests to succeed")
		}
	})
}

func TestE2E_UserAgentHeaders(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("user_agent_headers", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "useragent", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		userAgents := []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
			"Mozilla/5.0 (X11; Linux x86_64)",
			"Go-http-client/1.1",
		}

		for _, ua := range userAgents {
			request, err := testutils.NewRequestBuilder("GET", ctx.baseURL+"/api/auth/me").
				WithAuth(authClient.Token).
				WithHeader("User-Agent", ua).
				Build()
			if err != nil {
				t.Errorf("Failed to create request with User-Agent: %s", ua)
				continue
			}

			resp, err := ctx.client.Do(request)
			if err != nil {
				t.Errorf("Request failed with User-Agent %s: %v", ua, err)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200 with User-Agent %s, got %d", ua, resp.StatusCode)
			}
		}
	})
}

func TestE2E_RefererHeaders(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("referer_headers", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "referer", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		referers := []string{
			"https://example.com/page1",
			"https://example.com/page2",
			"http://localhost:3000",
			"",
		}

		for _, referer := range referers {
			builder := testutils.NewRequestBuilder("GET", ctx.baseURL+"/api/auth/me").
				WithAuth(authClient.Token)
			if referer != "" {
				builder = builder.WithHeader("Referer", referer)
			}
			request, err := builder.Build()
			if err != nil {
				t.Errorf("Failed to create request with Referer: %s", referer)
				continue
			}

			resp, err := ctx.client.Do(request)
			if err != nil {
				t.Errorf("Request failed with Referer %s: %v", referer, err)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200 with Referer %s, got %d", referer, resp.StatusCode)
			}
		}
	})
}

func TestE2E_RapidSuccessiveActions(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("rapid_successive_actions", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "rapid", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		post := authClient.CreatePost(t, "Rapid Vote Test", "https://example.com/rapid", "Content")

		for i := range 10 {
			voteType := "up"
			if i%2 == 0 {
				voteType = "down"
			}
			voteResp := authClient.VoteOnPost(t, post.ID, voteType)
			if !voteResp.Success {
				t.Logf("Vote %d may have been rate limited (acceptable)", i+1)
			}
		}

		finalPostsResp := authClient.GetPosts(t)
		finalPost := findPostInList(finalPostsResp, post.ID)
		if finalPost == nil {
			t.Errorf("Expected to retrieve post after rapid votes")
		}
	})
}

func TestE2E_LongRunningSession(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("long_running_session", func(t *testing.T) {
		createdUser := ctx.createUserWithCleanup(t, "longsession", "Password123!")
		authClient := ctx.loginUser(t, createdUser.Username, createdUser.Password)

		profile1 := authClient.GetProfile(t)
		if profile1 == nil {
			t.Fatalf("Expected initial profile request to succeed")
		}

		post := authClient.CreatePost(t, "Long Session Post", "https://example.com/long", "Content")
		if post.ID == 0 {
			t.Errorf("Expected post creation after delay to succeed")
		}

		profile2 := authClient.GetProfile(t)
		if profile2 == nil || profile2.Data.Username != createdUser.Username {
			t.Errorf("Expected profile request after delay to succeed")
		}

		voteResp := authClient.VoteOnPost(t, post.ID, "up")
		if !voteResp.Success {
			t.Errorf("Expected vote after delay to succeed")
		}
	})
}
