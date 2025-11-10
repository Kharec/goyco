package e2e

import (
	"net/http"
	"testing"
)

func TestE2E_VoteManagement(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("vote_operations", func(t *testing.T) {
		_, authClient := ctx.createUserAndLogin(t, "testuser", "StrongPass123!")

		createdPost := authClient.CreatePost(t, "Vote Test Post", "https://example.com/vote", "Content for voting")

		var voteResp *VoteResponse
		statusCode := retryOnRateLimit(t, 3, func() int {
			resp, code := authClient.VoteOnPostRaw(t, createdPost.ID, "up")
			if code == http.StatusOK {
				voteResp = resp
			}
			return code
		})
		if statusCode == http.StatusTooManyRequests {
			t.Skip("Skipping vote operations test: rate limited after retries")
			return
		}
		if statusCode != http.StatusOK {
			t.Fatalf("Vote failed with status %d", statusCode)
		}
		assertVoteResponse(t, voteResp, "up")

		userVote := authClient.GetUserVote(t, createdPost.ID)
		if !userVote.Success {
			t.Errorf("Expected to get user vote, got failure: %s", userVote.Message)
		}
		userVoteData := assertVoteData(t, userVote)
		if hasVote, ok := userVoteData["has_vote"].(bool); !ok || !hasVote {
			t.Errorf("Expected has_vote true after casting vote, got %#v", userVoteData["has_vote"])
		}

		postVotes := authClient.GetPostVotes(t, createdPost.ID)
		if !postVotes.Success {
			t.Errorf("Expected to get post votes, got failure: %s", postVotes.Message)
		}
		postVotesData := assertVoteData(t, postVotes)
		if count, ok := postVotesData["count"].(float64); !ok || count < 1 {
			t.Errorf("Expected post votes count to be >= 1, got %#v", postVotesData["count"])
		}

		authClient.RemoveVote(t, createdPost.ID)

		removedVote := authClient.GetUserVote(t, createdPost.ID)
		if !removedVote.Success {
			t.Errorf("Expected to get vote removal state, got failure: %s", removedVote.Message)
		}
		removedVoteData := assertVoteData(t, removedVote)
		if hasVote, ok := removedVoteData["has_vote"].(bool); ok && hasVote {
			t.Errorf("Expected has_vote false after removal, got true")
		}
		if voteVal, present := removedVoteData["vote"]; present && voteVal != nil {
			t.Errorf("Expected vote data to be nil after removal, got %#v", voteVal)
		}

		postVotesAfter := authClient.GetPostVotes(t, createdPost.ID)
		if !postVotesAfter.Success {
			t.Errorf("Expected to get post votes after removal, got failure: %s", postVotesAfter.Message)
		}
		postVotesAfterData := assertVoteData(t, postVotesAfter)
		if count, ok := postVotesAfterData["count"].(float64); ok && count != 0 {
			t.Errorf("Expected post votes count to be 0 after removal, got %v", count)
		}
	})
}

func TestE2E_VoteAuthorization(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("vote_authorization", func(t *testing.T) {
		createdUsers := ctx.createMultipleUsersWithCleanup(t, 2, "voteuser", "StrongPass123!")
		user1 := createdUsers[0]
		user2 := createdUsers[1]

		authClient1 := ctx.loginUser(t, user1.Username, user1.Password)
		authClient2 := ctx.loginUser(t, user2.Username, user2.Password)

		createdPost := authClient1.CreatePost(t, "Vote Test Post", "https://example.com/vote", "Content for voting tests")

		t.Run("users_can_only_vote_with_own_token", func(t *testing.T) {
			voteResp1 := authClient1.VoteOnPost(t, createdPost.ID, "up")
			if !voteResp1.Success {
				t.Errorf("Expected User1 to be able to vote with their own token, got failure: %s", voteResp1.Message)
			}

			userVote1 := authClient1.GetUserVote(t, createdPost.ID)
			if !userVote1.Success {
				t.Errorf("Expected to get User1's vote, got failure: %s", userVote1.Message)
			}
			userVote1Data := assertVoteData(t, userVote1)
			if hasVote, ok := userVote1Data["has_vote"].(bool); !ok || !hasVote {
				t.Errorf("Expected User1 to have a vote after voting, got has_vote=%v", userVote1Data["has_vote"])
			}

			voteResp2 := authClient2.VoteOnPost(t, createdPost.ID, "up")
			if !voteResp2.Success {
				t.Errorf("Expected User2 to be able to vote with their own token, got failure: %s", voteResp2.Message)
			}

			userVote2 := authClient2.GetUserVote(t, createdPost.ID)
			if !userVote2.Success {
				t.Errorf("Expected to get User2's vote, got failure: %s", userVote2.Message)
			}
			userVote2Data := assertVoteData(t, userVote2)
			if hasVote, ok := userVote2Data["has_vote"].(bool); !ok || !hasVote {
				t.Errorf("Expected User2 to have a vote after voting, got has_vote=%v", userVote2Data["has_vote"])
			}

			userVote1After := authClient1.GetUserVote(t, createdPost.ID)
			if !userVote1After.Success {
				t.Errorf("Expected to still get User1's vote after User2 votes, got failure: %s", userVote1After.Message)
			}
			userVote1AfterData := assertVoteData(t, userVote1After)
			if hasVote, ok := userVote1AfterData["has_vote"].(bool); !ok || !hasVote {
				t.Errorf("Expected User1's vote to still exist after User2 votes, got has_vote=%v", userVote1AfterData["has_vote"])
			}
		})

		t.Run("vote_counts_reflect_authenticated_votes", func(t *testing.T) {
			postVotes := authClient1.GetPostVotes(t, createdPost.ID)
			if !postVotes.Success {
				t.Errorf("Expected to get post votes, got failure: %s", postVotes.Message)
			}

			postVotesData := assertVoteData(t, postVotes)

			count, ok := postVotesData["count"].(float64)
			if !ok {
				t.Fatalf("Expected count to be a number, got %T", postVotesData["count"])
			}
			if count < 2 {
				t.Errorf("Expected vote count to be at least 2 (User1 and User2 both voted), got %v", count)
			}

			votesArray, ok := postVotesData["votes"].([]any)
			if !ok {
				t.Fatalf("Expected votes to be an array, got %T", postVotesData["votes"])
			}
			if len(votesArray) < 2 {
				t.Errorf("Expected at least 2 votes in the votes array, got %d", len(votesArray))
			}
		})

		t.Run("users_can_only_modify_own_votes", func(t *testing.T) {
			authClient1.RemoveVote(t, createdPost.ID)

			userVote1After := authClient1.GetUserVote(t, createdPost.ID)
			if !userVote1After.Success {
				t.Errorf("Expected to get vote state after removal, got failure: %s", userVote1After.Message)
			}
			userVote1AfterData := assertVoteData(t, userVote1After)
			if hasVote, ok := userVote1AfterData["has_vote"].(bool); ok && hasVote {
				t.Errorf("Expected User1's vote to be removed, but has_vote is still true")
			}

			userVote2After := authClient2.GetUserVote(t, createdPost.ID)
			if !userVote2After.Success {
				t.Errorf("Expected to get User2's vote, got failure: %s", userVote2After.Message)
			}
			userVote2AfterData := assertVoteData(t, userVote2After)
			if hasVote, ok := userVote2AfterData["has_vote"].(bool); !ok || !hasVote {
				t.Errorf("Expected User2's vote to still exist after User1 removes their vote, got has_vote=%v", userVote2AfterData["has_vote"])
			}

			postVotesAfter := authClient1.GetPostVotes(t, createdPost.ID)
			if !postVotesAfter.Success {
				t.Errorf("Expected to get post votes after removal, got failure: %s", postVotesAfter.Message)
			}
			postVotesAfterData := assertVoteData(t, postVotesAfter)
			countAfter, ok := postVotesAfterData["count"].(float64)
			if !ok {
				t.Fatalf("Expected count to be a number, got %T", postVotesAfterData["count"])
			}

			if countAfter < 1 {
				t.Errorf("Expected vote count to be at least 1 after User1 removes vote, got %v", countAfter)
			}
		})

		t.Run("vote_counts_accurate_with_different_types", func(t *testing.T) {
			voteResp1Down := authClient1.VoteOnPost(t, createdPost.ID, "down")
			if !voteResp1Down.Success {
				t.Errorf("Expected User1 to be able to vote down, got failure: %s", voteResp1Down.Message)
			}

			postVotes := authClient2.GetPostVotes(t, createdPost.ID)
			if !postVotes.Success {
				t.Errorf("Expected to get post votes, got failure: %s", postVotes.Message)
			}

			postVotesData := assertVoteData(t, postVotes)

			count := postVotesData["count"].(float64)
			if count < 2 {
				t.Errorf("Expected vote count to be at least 2 (User1 downvote, User2 upvote), got %v", count)
			}

			userVote1 := authClient1.GetUserVote(t, createdPost.ID)
			userVote1Data := assertVoteData(t, userVote1)
			if voteData, exists := userVote1Data["vote"].(map[string]any); exists {
				if voteType, exists := voteData["type"].(string); exists {
					if voteType != "down" {
						t.Errorf("Expected User1's vote type to be 'down', got '%s'", voteType)
					}
				}
			}

			userVote2 := authClient2.GetUserVote(t, createdPost.ID)
			userVote2Data := assertVoteData(t, userVote2)
			if voteData, exists := userVote2Data["vote"].(map[string]any); exists {
				if voteType, exists := voteData["type"].(string); exists {
					if voteType != "up" {
						t.Errorf("Expected User2's vote type to be 'up', got '%s'", voteType)
					}
				}
			}
		})

		t.Run("multiple_users_vote_independently", func(t *testing.T) {
			user3 := ctx.createUserWithCleanup(t, "voteuser3", "StrongPass123!")
			authClient3 := ctx.loginUser(t, user3.Username, user3.Password)

			voteResp3 := authClient3.VoteOnPost(t, createdPost.ID, "up")
			if !voteResp3.Success {
				t.Errorf("Expected User3 to be able to vote, got failure: %s", voteResp3.Message)
			}

			userVote1 := authClient1.GetUserVote(t, createdPost.ID)
			userVote1Data := assertVoteData(t, userVote1)
			if hasVote, ok := userVote1Data["has_vote"].(bool); !ok || !hasVote {
				t.Errorf("Expected User1 to still have a vote")
			}

			userVote2 := authClient2.GetUserVote(t, createdPost.ID)
			userVote2Data := assertVoteData(t, userVote2)
			if hasVote, ok := userVote2Data["has_vote"].(bool); !ok || !hasVote {
				t.Errorf("Expected User2 to still have a vote")
			}

			userVote3 := authClient3.GetUserVote(t, createdPost.ID)
			userVote3Data := assertVoteData(t, userVote3)
			if hasVote, ok := userVote3Data["has_vote"].(bool); !ok || !hasVote {
				t.Errorf("Expected User3 to have a vote after voting")
			}

			postVotes := authClient3.GetPostVotes(t, createdPost.ID)
			postVotesData := assertVoteData(t, postVotes)
			count, ok := postVotesData["count"].(float64)
			if !ok {
				t.Fatalf("Expected count to be a number, got %T", postVotesData["count"])
			}
			if count < 3 {
				t.Errorf("Expected vote count to be at least 3 (three users voted), got %v", count)
			}
		})
	})
}
