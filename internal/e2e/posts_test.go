package e2e

import (
	"net/http"
	"testing"
)

func TestE2E_PostManagement(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("post_crud_operations", func(t *testing.T) {
		_, authClient := ctx.createUserAndLogin(t, "testuser", "StrongPass123!")

		createdPost := authClient.CreatePost(t, "Original Post", "https://example.com/original", "Original content")
		updatedPost := authClient.UpdatePost(t, createdPost.ID, "Updated Post", "https://example.com/updated", "Updated content")

		if updatedPost.Title != "Updated Post" {
			t.Errorf("Expected updated title 'Updated Post', got '%s'", updatedPost.Title)
		}
		if updatedPost.Content != "Updated content" {
			t.Errorf("Expected updated content 'Updated content', got '%s'", updatedPost.Content)
		}

		postsResp := authClient.GetPosts(t)
		assertPostInList(t, postsResp, updatedPost)

		authClient.DeletePost(t, createdPost.ID)

		finalPostsResp := authClient.GetPosts(t)
		if len(finalPostsResp.Data.Posts) > 0 {
			for _, post := range finalPostsResp.Data.Posts {
				if post.ID == createdPost.ID {
					t.Errorf("Expected post to be deleted, but it still appears in posts list")
					break
				}
			}
		}
	})
}

func TestE2E_PostOwnershipAuthorization(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("post_ownership_authorization", func(t *testing.T) {
		createdUsers := ctx.createMultipleUsersWithCleanup(t, 2, "user", "StrongPass123!")
		user1 := createdUsers[0]
		user2 := createdUsers[1]

		authClient1 := ctx.loginUser(t, user1.Username, user1.Password)
		createdPost := authClient1.CreatePost(t, "User1's Post", "https://example.com/user1", "This is user1's post content")

		authClient2 := ctx.loginUser(t, user2.Username, user2.Password)

		t.Run("user2_cannot_update_user1_post", func(t *testing.T) {
			statusCode := authClient2.UpdatePostExpectStatus(t, createdPost.ID, "Hacked Title", "https://evil.com", "Hacked content")
			if statusCode != http.StatusForbidden {
				t.Errorf("Expected 403 Forbidden when User2 tries to update User1's post, got %d", statusCode)
			}
		})

		t.Run("user2_cannot_delete_user1_post", func(t *testing.T) {
			statusCode := authClient2.DeletePostExpectStatus(t, createdPost.ID)
			if statusCode != http.StatusForbidden {
				t.Errorf("Expected 403 Forbidden when User2 tries to delete User1's post, got %d", statusCode)
			}
		})

		t.Run("user1_post_unchanged", func(t *testing.T) {
			postsResp := authClient1.GetPosts(t)
			found := false
			for _, post := range postsResp.Data.Posts {
				if post.ID == createdPost.ID {
					found = true
					if post.Title != createdPost.Title {
						t.Errorf("Expected post title to remain '%s', but it was modified to '%s'", createdPost.Title, post.Title)
					}
					if post.Content != createdPost.Content {
						t.Errorf("Expected post content to remain unchanged, but it was modified")
					}
					break
				}
			}
			if !found {
				t.Errorf("Expected User1's post to still exist, but it was not found in the posts list")
			}
		})

		t.Run("user1_can_update_own_post", func(t *testing.T) {
			updatedPost := authClient1.UpdatePost(t, createdPost.ID, "Updated by User1", "https://example.com/updated", "Updated content by User1")
			if updatedPost.Title != "Updated by User1" {
				t.Errorf("Expected post title to be 'Updated by User1', got '%s'", updatedPost.Title)
			}
		})

		t.Run("user1_can_delete_own_post", func(t *testing.T) {
			deletablePost := authClient1.CreatePost(t, "Deletable Post", "https://example.com/deletable", "This post will be deleted")
			authClient1.DeletePost(t, deletablePost.ID)

			postsResp := authClient1.GetPosts(t)
			for _, post := range postsResp.Data.Posts {
				if post.ID == deletablePost.ID {
					t.Errorf("Expected post %d to be deleted, but it still exists", deletablePost.ID)
					break
				}
			}
		})
	})
}
