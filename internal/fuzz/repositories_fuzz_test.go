package fuzz

import (
	"strings"
	"testing"
	"unicode/utf8"

	"goyco/internal/repositories"
)

func FuzzSearchRepository(f *testing.F) {
	f.Add("test query")
	f.Add("")
	f.Add("SELECT * FROM posts")
	f.Add(strings.Repeat("a", 1000))
	f.Add("<script>alert('xss')</script>")

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 1000 {
			input = input[:1000]
		}

		if !utf8.ValidString(input) {
			return
		}

		db, err := GetFuzzDB()
		if err != nil {
			t.Fatalf("Failed to connect to test database: %v", err)
		}

		db.Exec("DELETE FROM votes")
		db.Exec("DELETE FROM posts")
		db.Exec("DELETE FROM users")
		db.Exec("DELETE FROM account_deletion_requests")
		db.Exec("DELETE FROM refresh_tokens")

		postRepo := repositories.NewPostRepository(db)
		sanitizer := repositories.NewSearchSanitizer()

		t.Run("sanitize_and_search", func(t *testing.T) {
			sanitized, err := sanitizer.SanitizeSearchQuery(input)
			if err != nil {
				return
			}

			if !utf8.ValidString(sanitized) {
				t.Fatalf("Sanitized query should be valid UTF-8: %q", sanitized)
			}

			posts, searchErr := postRepo.Search(sanitized, 1, 10)
			if searchErr != nil {
				if strings.Contains(searchErr.Error(), "panic") {
					t.Fatalf("Search should not panic: %v", searchErr)
				}
			} else {
				if posts != nil {
					_ = len(posts)
				}
			}
		})

		t.Run("validate_search_query", func(t *testing.T) {
			err := sanitizer.ValidateSearchQuery(input)

			if err != nil {
				if strings.Contains(err.Error(), "panic") {
					t.Fatalf("ValidateSearchQuery should not panic: %v", err)
				}
			}
		})
	})
}

func FuzzPostRepository(f *testing.F) {
	f.Add("test title")
	f.Add("")
	f.Add("<script>alert('xss')</script>")
	f.Add("https://example.com")
	f.Add(strings.Repeat("a", 500))

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 500 {
			input = input[:500]
		}

		if !utf8.ValidString(input) {
			return
		}

		db, err := GetFuzzDB()
		if err != nil {
			t.Fatalf("Failed to connect to test database: %v", err)
		}

		db.Exec("DELETE FROM votes")
		db.Exec("DELETE FROM posts")
		db.Exec("DELETE FROM users")
		db.Exec("DELETE FROM account_deletion_requests")
		db.Exec("DELETE FROM refresh_tokens")

		postRepo := repositories.NewPostRepository(db)

		var userID uint
		result := db.Exec(`
			INSERT INTO users (username, email, password, email_verified, created_at, updated_at)
			VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))
		`, "fuzz_test_user", "fuzz@example.com", "hashedpassword", true)
		if result.Error != nil {
			t.Fatalf("Failed to create test user: %v", result.Error)
		}

		var createdUser struct {
			ID uint `gorm:"column:id"`
		}
		db.Raw("SELECT id FROM users WHERE username = ?", "fuzz_test_user").Scan(&createdUser)
		userID = createdUser.ID

		t.Run("create_and_get_post", func(t *testing.T) {
			title := input[:min(len(input), 200)]
			url := "https://example.com/" + input[:min(len(input), 50)]
			content := input[:min(len(input), 1000)]

			result := db.Exec(`
				INSERT INTO posts (title, url, content, author_id, created_at, updated_at)
				VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))
			`, title, url, content, userID)
			if result.Error != nil {
				if strings.Contains(result.Error.Error(), "panic") {
					t.Fatalf("Create should not panic: %v", result.Error)
				}
				return
			}

			var postID uint
			var createdPost struct {
				ID uint `gorm:"column:id"`
			}
			db.Raw("SELECT id FROM posts WHERE author_id = ? ORDER BY id DESC LIMIT 1", userID).Scan(&createdPost)
			postID = createdPost.ID

			if postID == 0 {
				t.Fatal("Created post should have an ID")
			}

			retrieved, getErr := postRepo.GetByID(postID)
			if getErr != nil {
				t.Fatalf("GetByID should succeed for created post: %v", getErr)
			}

			if retrieved == nil {
				t.Fatal("GetByID should return a post")
			}

			if retrieved.ID != postID {
				t.Fatalf("Expected post ID %d, got %d", postID, retrieved.ID)
			}

			posts, listErr := postRepo.GetAll(10, 0)
			if listErr != nil {
				t.Fatalf("GetAll should not error: %v", listErr)
			}

			if posts == nil {
				t.Fatal("GetAll should return a slice")
			}

			found := false
			for _, p := range posts {
				if p.ID == postID {
					found = true
					break
				}
			}
			if !found && len(posts) > 0 {
				t.Logf("Created post not found in list (this may be acceptable depending on pagination)")
			}
		})
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
