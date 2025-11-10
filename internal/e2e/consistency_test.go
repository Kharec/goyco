package e2e

import (
	"testing"

	"goyco/internal/database"
)

func TestE2E_VoteCountConsistency(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("vote_count_consistency", func(t *testing.T) {
		user1 := ctx.createUserWithCleanup(t, "voteuser1", "Password123!")
		user2 := ctx.createUserWithCleanup(t, "voteuser2", "Password123!")
		user3 := ctx.createUserWithCleanup(t, "voteuser3", "Password123!")

		client1 := ctx.loginUser(t, user1.Username, user1.Password)
		post := client1.CreatePost(t, "Vote Count Test", "https://example.com/votecount", "Content")

		client1.VoteOnPost(t, post.ID, "up")
		client2 := ctx.loginUser(t, user2.Username, user2.Password)
		client2.VoteOnPost(t, post.ID, "up")
		client3 := ctx.loginUser(t, user3.Username, user3.Password)
		client3.VoteOnPost(t, post.ID, "down")

		var dbPost database.Post
		if err := ctx.server.DB.First(&dbPost, post.ID).Error; err != nil {
			t.Fatalf("Failed to find post in database: %v", err)
		}

		var voteCount int64
		ctx.server.DB.Model(&database.Vote{}).Where("post_id = ? AND type = ?", post.ID, database.VoteUp).Count(&voteCount)
		if voteCount != int64(dbPost.UpVotes) {
			t.Errorf("Expected upvote count %d to match database count %d", dbPost.UpVotes, voteCount)
		}

		ctx.server.DB.Model(&database.Vote{}).Where("post_id = ? AND type = ?", post.ID, database.VoteDown).Count(&voteCount)
		if voteCount != int64(dbPost.DownVotes) {
			t.Errorf("Expected downvote count %d to match database count %d", dbPost.DownVotes, voteCount)
		}

		postsResp := client1.GetPosts(t)
		apiPost := findPostInList(postsResp, post.ID)
		if apiPost == nil {
			t.Fatalf("Expected to find post in API response")
		}
		if apiPost.UpVotes != dbPost.UpVotes {
			t.Errorf("Expected API upvote count %d to match database %d", apiPost.UpVotes, dbPost.UpVotes)
		}
		if apiPost.DownVotes != dbPost.DownVotes {
			t.Errorf("Expected API downvote count %d to match database %d", apiPost.DownVotes, dbPost.DownVotes)
		}
	})
}

func TestE2E_PostScoreCalculation(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("post_score_calculation", func(t *testing.T) {
		user1 := ctx.createUserWithCleanup(t, "scoreuser1", "Password123!")
		user2 := ctx.createUserWithCleanup(t, "scoreuser2", "Password123!")
		user3 := ctx.createUserWithCleanup(t, "scoreuser3", "Password123!")

		client1 := ctx.loginUser(t, user1.Username, user1.Password)
		post := client1.CreatePost(t, "Score Test", "https://example.com/score", "Content")

		client1.VoteOnPost(t, post.ID, "up")
		client2 := ctx.loginUser(t, user2.Username, user2.Password)
		client2.VoteOnPost(t, post.ID, "up")
		client3 := ctx.loginUser(t, user3.Username, user3.Password)
		client3.VoteOnPost(t, post.ID, "down")

		var dbPost database.Post
		if err := ctx.server.DB.First(&dbPost, post.ID).Error; err != nil {
			t.Fatalf("Failed to find post in database: %v", err)
		}

		expectedScore := dbPost.UpVotes - dbPost.DownVotes
		if dbPost.Score != expectedScore {
			t.Errorf("Expected score %d (upvotes %d - downvotes %d), got %d", expectedScore, dbPost.UpVotes, dbPost.DownVotes, dbPost.Score)
		}

		postsResp := client1.GetPosts(t)
		apiPost := findPostInList(postsResp, post.ID)
		if apiPost == nil {
			t.Fatalf("Expected to find post in API response")
		}
		if apiPost.Score != expectedScore {
			t.Errorf("Expected API score %d to match calculated score %d", apiPost.Score, expectedScore)
		}
	})
}

func TestE2E_PostDeletionCascades(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("post_deletion_cascades", func(t *testing.T) {
		user1 := ctx.createUserWithCleanup(t, "cascadeuser1", "Password123!")
		user2 := ctx.createUserWithCleanup(t, "cascadeuser2", "Password123!")

		client1 := ctx.loginUser(t, user1.Username, user1.Password)
		post := client1.CreatePost(t, "Cascade Test", "https://example.com/cascade", "Content")

		client1.VoteOnPost(t, post.ID, "up")
		client2 := ctx.loginUser(t, user2.Username, user2.Password)
		client2.VoteOnPost(t, post.ID, "down")

		var voteCountBefore int64
		ctx.server.DB.Model(&database.Vote{}).Where("post_id = ?", post.ID).Count(&voteCountBefore)
		if voteCountBefore == 0 {
			t.Fatalf("Expected votes to exist before deletion")
		}

		client1.DeletePost(t, post.ID)

		var voteCountAfter int64
		ctx.server.DB.Model(&database.Vote{}).Where("post_id = ?", post.ID).Count(&voteCountAfter)
		if voteCountAfter != 0 {
			t.Errorf("Expected votes to be deleted after post deletion, found %d votes", voteCountAfter)
		}

		var dbPost database.Post
		if err := ctx.server.DB.First(&dbPost, post.ID).Error; err == nil {
			t.Errorf("Expected post to be deleted from database")
		}
	})
}

func TestE2E_UserDeletionCascades(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("user_deletion_cascades", func(t *testing.T) {
		user1 := ctx.createUserWithCleanup(t, "deleteuser1", "Password123!")
		user2 := ctx.createUserWithCleanup(t, "deleteuser2", "Password123!")

		client1 := ctx.loginUser(t, user1.Username, user1.Password)
		post1 := client1.CreatePost(t, "Post 1", "https://example.com/post1", "Content 1")
		post2 := client1.CreatePost(t, "Post 2", "https://example.com/post2", "Content 2")

		client2 := ctx.loginUser(t, user2.Username, user2.Password)
		client2.VoteOnPost(t, post1.ID, "up")

		var postCountBefore int64
		ctx.server.DB.Model(&database.Post{}).Where("author_id = ?", user1.ID).Count(&postCountBefore)
		if postCountBefore == 0 {
			t.Fatalf("Expected posts to exist before deletion")
		}

		var voteCountBefore int64
		ctx.server.DB.Model(&database.Vote{}).Where("post_id IN (?)", []uint{post1.ID, post2.ID}).Count(&voteCountBefore)
		if voteCountBefore == 0 {
			t.Fatalf("Expected votes to exist before deletion")
		}

		ctx.server.EmailSender.Reset()
		client1.RequestAccountDeletion(t)
		deletionToken := ctx.server.EmailSender.DeletionToken()
		if deletionToken == "" {
			t.Fatalf("Expected deletion token")
		}

		client1.ConfirmAccountDeletion(t, deletionToken, false)

		var postCountAfter int64
		ctx.server.DB.Model(&database.Post{}).Where("author_id = ?", user1.ID).Count(&postCountAfter)
		if postCountAfter != 0 {
			t.Errorf("Expected posts to be deleted after user deletion, found %d posts", postCountAfter)
		}

		var voteCountAfter int64
		ctx.server.DB.Model(&database.Vote{}).Where("post_id IN (?)", []uint{post1.ID, post2.ID}).Count(&voteCountAfter)
		if voteCountAfter != 0 {
			t.Errorf("Expected votes to be deleted after post deletion, found %d votes", voteCountAfter)
		}

		var dbUser database.User
		if err := ctx.server.DB.First(&dbUser, user1.ID).Error; err == nil {
			t.Errorf("Expected user to be deleted from database")
		}
	})
}

func TestE2E_ReferentialIntegrity(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("referential_integrity", func(t *testing.T) {
		user1 := ctx.createUserWithCleanup(t, "refuser1", "Password123!")
		user2 := ctx.createUserWithCleanup(t, "refuser2", "Password123!")

		client1 := ctx.loginUser(t, user1.Username, user1.Password)
		post := client1.CreatePost(t, "Ref Integrity Test", "https://example.com/ref", "Content")

		client2 := ctx.loginUser(t, user2.Username, user2.Password)
		client2.VoteOnPost(t, post.ID, "up")

		var voteCount int64
		ctx.server.DB.Model(&database.Vote{}).Where("post_id = ? AND user_id = ?", post.ID, user2.ID).Count(&voteCount)
		if voteCount != 1 {
			t.Errorf("Expected vote to exist with correct foreign keys")
		}

		var postCount int64
		ctx.server.DB.Model(&database.Post{}).Where("author_id = ?", user1.ID).Count(&postCount)
		if postCount == 0 {
			t.Errorf("Expected post to exist with correct author foreign key")
		}
	})
}

func TestE2E_OrphanedRecordsPrevention(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("orphaned_records_prevention", func(t *testing.T) {
		user1 := ctx.createUserWithCleanup(t, "orphanuser1", "Password123!")
		user2 := ctx.createUserWithCleanup(t, "orphanuser2", "Password123!")

		client1 := ctx.loginUser(t, user1.Username, user1.Password)
		post := client1.CreatePost(t, "Orphan Test", "https://example.com/orphan", "Content")

		client2 := ctx.loginUser(t, user2.Username, user2.Password)
		client2.VoteOnPost(t, post.ID, "up")

		var voteCountBefore int64
		ctx.server.DB.Model(&database.Vote{}).Where("post_id = ?", post.ID).Count(&voteCountBefore)

		client1.DeletePost(t, post.ID)

		var orphanedVotes int64
		ctx.server.DB.Unscoped().Model(&database.Vote{}).Where("post_id = ?", post.ID).Count(&orphanedVotes)
		if orphanedVotes != 0 {
			t.Errorf("Expected no orphaned votes after post deletion, found %d", orphanedVotes)
		}

		post2 := client1.CreatePost(t, "Orphan Test 2", "https://example.com/orphan2", "Content")
		client2.VoteOnPost(t, post2.ID, "up")

		ctx.server.EmailSender.Reset()
		client1.RequestAccountDeletion(t)
		deletionToken := ctx.server.EmailSender.DeletionToken()
		if deletionToken == "" {
			t.Fatalf("Expected deletion token")
		}

		client1.ConfirmAccountDeletion(t, deletionToken, false)

		var orphanedPosts int64
		ctx.server.DB.Unscoped().Model(&database.Post{}).Where("author_id = ?", user1.ID).Count(&orphanedPosts)
		if orphanedPosts != 0 {
			t.Errorf("Expected no posts with author_id = %d after user deletion, found %d", user1.ID, orphanedPosts)
		}

		var orphanedVotesAfter int64
		ctx.server.DB.Unscoped().Model(&database.Vote{}).Where("post_id = ?", post2.ID).Count(&orphanedVotesAfter)
		if orphanedVotesAfter != 0 {
			t.Errorf("Expected no orphaned votes after post deletion via user deletion, found %d", orphanedVotesAfter)
		}
	})
}
