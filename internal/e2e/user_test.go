package e2e

import (
	"net/http"
	"testing"
)

func TestE2E_UserDirectory(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("user_directory", func(t *testing.T) {
		users := ctx.createMultipleUsersWithCleanup(t, 3, "user", "StrongPass123!")

		authClient := ctx.loginUser(t, users[0].Username, users[0].Password)

		usersResp := authClient.GetUsers(t)
		if len(usersResp.Data.Users) < 3 {
			t.Errorf("Expected at least 3 users, got %d", len(usersResp.Data.Users))
		}

		for _, user := range usersResp.Data.Users {
			if user.Username == "" {
				t.Errorf("Expected username to be present, got empty string")
			}
		}
	})
}

func TestE2E_ProfileManagement(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("profile_management", func(t *testing.T) {
		createdUser, authClient := ctx.createUserAndLogin(t, "testuser", "StrongPass123!")

		profile := authClient.GetProfile(t)
		assertUserResponse(t, profile, createdUser)
	})
}

func TestE2E_ProfileAccessAuthorization(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("profile_access_authorization", func(t *testing.T) {
		createdUsers := ctx.createMultipleUsersWithCleanup(t, 2, "profileuser", "StrongPass123!")
		user1 := createdUsers[0]
		user2 := createdUsers[1]

		authClient1 := ctx.loginUser(t, user1.Username, user1.Password)
		authClient2 := ctx.loginUser(t, user2.Username, user2.Password)

		user2CurrentUsername := user2.Username

		t.Run("users_only_see_own_profile_via_me_endpoint", func(t *testing.T) {
			profile1 := authClient1.GetProfile(t)
			if profile1.Data.ID != user1.ID {
				t.Errorf("User1's /api/auth/me shows wrong ID: expected %d, got %d", user1.ID, profile1.Data.ID)
			}
			if profile1.Data.Username != user1.Username {
				t.Errorf("User1's /api/auth/me shows wrong username: expected '%s', got '%s'", user1.Username, profile1.Data.Username)
			}
			if profile1.Data.Email != user1.Email {
				t.Errorf("User1's /api/auth/me shows wrong email: expected '%s', got '%s'", user1.Email, profile1.Data.Email)
			}

			profile2 := authClient2.GetProfile(t)
			if profile2.Data.ID != user2.ID {
				t.Errorf("User2's /api/auth/me shows wrong ID: expected %d, got %d", user2.ID, profile2.Data.ID)
			}
			if profile2.Data.Username != user2.Username {
				t.Errorf("User2's /api/auth/me shows wrong username: expected '%s', got '%s'", user2.Username, profile2.Data.Username)
			}
			if profile2.Data.Email != user2.Email {
				t.Errorf("User2's /api/auth/me shows wrong email: expected '%s', got '%s'", user2.Email, profile2.Data.Email)
			}

			if profile1.Data.ID == profile2.Data.ID {
				t.Errorf("User1 and User2 profiles should have different IDs via /api/auth/me, but both show %d", profile1.Data.ID)
			}
			if profile1.Data.Username == profile2.Data.Username {
				t.Errorf("User1 and User2 profiles should have different usernames via /api/auth/me, but both show '%s'", profile1.Data.Username)
			}
			if profile1.Data.Email == profile2.Data.Email {
				t.Errorf("User1 and User2 profiles should have different emails via /api/auth/me, but both show '%s'", profile1.Data.Email)
			}
		})

		t.Run("users_cannot_modify_other_users_email", func(t *testing.T) {
			originalProfile1 := authClient1.GetProfile(t)
			originalEmail1 := originalProfile1.Data.Email

			ctx.server.EmailSender.Reset()
			statusCode := authClient2.UpdateEmailExpectStatus(t, uniqueEmail(t, "newemail2"))
			if statusCode != http.StatusOK {
				t.Errorf("Expected User2 to be able to update their own email with status 200, got %d", statusCode)
			}

			verificationToken := ctx.server.EmailSender.VerificationToken()
			if verificationToken != "" {
				ctx.confirmEmail(t, verificationToken)
			}

			updatedProfile1 := authClient1.GetProfile(t)
			if updatedProfile1.Data.Email != originalEmail1 {
				t.Errorf("User2 updating their own email should not affect User1's email. Expected '%s', got '%s'", originalEmail1, updatedProfile1.Data.Email)
			}
		})

		t.Run("users_cannot_modify_other_users_username", func(t *testing.T) {
			originalProfile1 := authClient1.GetProfile(t)
			originalUsername1 := originalProfile1.Data.Username

			user2CurrentUsername = uniqueUsername(t, "newusername2")
			authClient2.UpdateUsername(t, user2CurrentUsername)

			updatedProfile1 := authClient1.GetProfile(t)
			if updatedProfile1.Data.Username != originalUsername1 {
				t.Errorf("User2 updating their own username should not affect User1's username. Expected '%s', got '%s'", originalUsername1, updatedProfile1.Data.Username)
			}

			updatedProfile2 := authClient2.GetProfile(t)
			if updatedProfile2.Data.Username == originalUsername1 {
				t.Errorf("Expected User2's username to be updated, but it's still '%s'", originalUsername1)
			}
		})

		t.Run("users_cannot_modify_other_users_password", func(t *testing.T) {
			baselineAuthClient1 := ctx.loginUser(t, user1.Username, "StrongPass123!")
			if baselineAuthClient1.Token == "" {
				t.Fatalf("User1 should be able to login with original password before User2's update")
			}

			authClient2.UpdatePassword(t, "StrongPass123!", "NewPass456!")

			newAuthClient1 := ctx.loginUser(t, user1.Username, "StrongPass123!")
			if newAuthClient1.Token == "" {
				t.Errorf("User1 should still be able to login with original password after User2 updates their own password")
			}

			profile1After := newAuthClient1.GetProfile(t)
			if profile1After.Data.Username != user1.Username {
				t.Errorf("User1's username should remain unchanged after User2's password update. Expected '%s', got '%s'", user1.Username, profile1After.Data.Username)
			}
		})

		t.Run("user1_updates_dont_affect_user2", func(t *testing.T) {
			authClient2 = ctx.loginUser(t, user2CurrentUsername, "NewPass456!")
			originalProfile2 := authClient2.GetProfile(t)
			originalUsername2 := originalProfile2.Data.Username

			authClient1.UpdateUsername(t, uniqueUsername(t, "newusername1"))

			updatedProfile2 := authClient2.GetProfile(t)
			if updatedProfile2.Data.Username != originalUsername2 {
				t.Errorf("User1 updating their own username should not affect User2's username. Expected '%s', got '%s'", originalUsername2, updatedProfile2.Data.Username)
			}

			updatedProfile1 := authClient1.GetProfile(t)
			if updatedProfile1.Data.Username == originalUsername2 {
				t.Errorf("Expected User1's username to be updated, but it's still '%s'", originalUsername2)
			}
		})

		t.Run("profiles_remain_isolated_after_updates", func(t *testing.T) {
			authClient2 = ctx.loginUser(t, user2CurrentUsername, "NewPass456!")
			finalProfile1 := authClient1.GetProfile(t)
			finalProfile2 := authClient2.GetProfile(t)

			if finalProfile1.Data.ID == finalProfile2.Data.ID {
				t.Errorf("After all updates, User1 and User2 should still have different IDs, but both show %d", finalProfile1.Data.ID)
			}
			if finalProfile1.Data.Username == finalProfile2.Data.Username {
				t.Errorf("After all updates, User1 and User2 should still have different usernames, but both show '%s'", finalProfile1.Data.Username)
			}
			if finalProfile1.Data.Email == finalProfile2.Data.Email {
				t.Errorf("After all updates, User1 and User2 should still have different emails, but both show '%s'", finalProfile1.Data.Email)
			}
		})
	})
}
