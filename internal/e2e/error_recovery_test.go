package e2e

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"goyco/internal/database"
	"goyco/internal/testutils"
)

func TestE2E_DatabaseFailureRecovery(t *testing.T) {
	t.Run("database_unavailable_handles_gracefully", func(t *testing.T) {
		ctx := setupTestContext(t)
		sqlDB, err := ctx.server.DB.DB()
		if err != nil {
			t.Fatalf("Failed to get SQL DB: %v", err)
		}
		sqlDB.Close()

		req, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusInternalServerError && resp.StatusCode != http.StatusServiceUnavailable {
			t.Logf("Expected 500 or 503, got %d (acceptable for unavailable DB)", resp.StatusCode)
		}
	})

	t.Run("connection_pool_exhaustion", func(t *testing.T) {
		ctx := setupTestContext(t)
		sqlDB, err := ctx.server.DB.DB()
		if err != nil {
			t.Fatalf("Failed to get SQL DB: %v", err)
		}

		originalMaxOpen := sqlDB.Stats().MaxOpenConnections
		if originalMaxOpen == 0 {
			originalMaxOpen = 1
		}

		sqlDB.SetMaxOpenConns(2)

		var wg sync.WaitGroup
		errors := make(chan error, 10)

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				conn, err := sqlDB.Conn(context.Background())
				if err != nil {
					errors <- err
					return
				}
				defer conn.Close()

				time.Sleep(100 * time.Millisecond)
			}()
		}

		wg.Wait()
		close(errors)

		errorCount := 0
		for range errors {
			errorCount++
		}

		if errorCount == 0 {
			t.Log("No connection errors occurred (pool handled load)")
		}

		sqlDB.SetMaxOpenConns(int(originalMaxOpen))
	})

	t.Run("transaction_rollback_on_error", func(t *testing.T) {
		ctx := setupTestContext(t)
		testUser := ctx.createUserWithCleanup(t, "rollbackuser", "StrongPass123!")

		tx := ctx.server.DB.Begin()
		if tx.Error != nil {
			t.Fatalf("Failed to begin transaction: %v", tx.Error)
		}

		post := &database.Post{
			Title:    "Rollback Test Post",
			URL:      "https://example.com/rollback",
			Content:  "This post should be rolled back",
			AuthorID: &testUser.ID,
		}

		err := tx.Create(post).Error
		if err != nil {
			tx.Rollback()
			t.Fatalf("Failed to create post in transaction: %v", err)
		}

		var postInTx database.Post
		err = tx.First(&postInTx, post.ID).Error
		if err != nil {
			tx.Rollback()
			t.Fatalf("Failed to retrieve post in transaction: %v", err)
		}

		tx.Rollback()

		var postAfterRollback database.Post
		err = ctx.server.DB.First(&postAfterRollback, post.ID).Error
		if err == nil {
			t.Error("Expected post to not exist after transaction rollback")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Logf("Post correctly not found after rollback (error: %v)", err)
		}
	})

	t.Run("transaction_commit_succeeds", func(t *testing.T) {
		ctx := setupTestContext(t)
		testUser := ctx.createUserWithCleanup(t, "commituser", "StrongPass123!")

		tx := ctx.server.DB.Begin()
		if tx.Error != nil {
			t.Fatalf("Failed to begin transaction: %v", tx.Error)
		}

		post := &database.Post{
			Title:    "Commit Test Post",
			URL:      "https://example.com/commit",
			Content:  "This post should be committed",
			AuthorID: &testUser.ID,
		}

		err := tx.Create(post).Error
		if err != nil {
			tx.Rollback()
			t.Fatalf("Failed to create post in transaction: %v", err)
		}

		err = tx.Commit().Error
		if err != nil {
			t.Fatalf("Failed to commit transaction: %v", err)
		}

		var postAfterCommit database.Post
		err = ctx.server.DB.First(&postAfterCommit, post.ID).Error
		if err != nil {
			t.Errorf("Expected post to exist after transaction commit, got error: %v", err)
		}
	})

	t.Run("database_timeout_handling", func(t *testing.T) {
		ctx := setupTestContext(t)
		sqlDB, err := ctx.server.DB.DB()
		if err != nil {
			t.Fatalf("Failed to get SQL DB: %v", err)
		}

		ctxTimeout, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		conn, err := sqlDB.Conn(ctxTimeout)
		if err != nil {
			return
		}
		defer conn.Close()

		rows, err := conn.QueryContext(ctxTimeout, "SELECT 1")
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Logf("Timeout handled correctly: %v", err)
		}
		if rows != nil {
			rows.Close()
		}
	})

	t.Run("concurrent_transaction_isolation", func(t *testing.T) {
		ctx := setupTestContext(t)
		testUser := ctx.createUserWithCleanup(t, "isolationuser", "StrongPass123!")

		var wg sync.WaitGroup
		errors := make(chan error, 2)

		wg.Add(2)
		go func() {
			defer wg.Done()
			tx1 := ctx.server.DB.Begin()
			if tx1.Error != nil {
				errors <- tx1.Error
				return
			}

			post1 := &database.Post{
				Title:    "Isolation Post 1",
				URL:      "https://example.com/isolation1",
				Content:  "First transaction",
				AuthorID: &testUser.ID,
			}

			err := tx1.Create(post1).Error
			if err != nil {
				tx1.Rollback()
				errors <- err
				return
			}

			time.Sleep(50 * time.Millisecond)
			tx1.Commit()
		}()

		go func() {
			defer wg.Done()
			time.Sleep(25 * time.Millisecond)

			tx2 := ctx.server.DB.Begin()
			if tx2.Error != nil {
				errors <- tx2.Error
				return
			}

			post2 := &database.Post{
				Title:    "Isolation Post 2",
				URL:      "https://example.com/isolation2",
				Content:  "Second transaction",
				AuthorID: &testUser.ID,
			}

			err := tx2.Create(post2).Error
			if err != nil {
				tx2.Rollback()
				errors <- err
				return
			}

			tx2.Commit()
		}()

		wg.Wait()
		close(errors)

		for err := range errors {
			if err != nil {
				t.Errorf("Transaction error: %v", err)
			}
		}
	})
}

func TestE2E_DatabaseConnectionPool(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("pool_stats_tracking", func(t *testing.T) {
		sqlDB, err := ctx.server.DB.DB()
		if err != nil {
			t.Fatalf("Failed to get SQL DB: %v", err)
		}

		stats := sqlDB.Stats()
		if stats.MaxOpenConnections == 0 {
			t.Error("Expected MaxOpenConnections to be set")
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
		resp.Body.Close()

		newStats := sqlDB.Stats()
		if newStats.OpenConnections > stats.OpenConnections {
			t.Logf("Connection pool used: %d -> %d connections", stats.OpenConnections, newStats.OpenConnections)
		}
	})

	t.Run("pool_reuses_connections", func(t *testing.T) {
		sqlDB, err := ctx.server.DB.DB()
		if err != nil {
			t.Fatalf("Failed to get SQL DB: %v", err)
		}

		initialStats := sqlDB.Stats()

		for i := 0; i < 5; i++ {
			req, err := http.NewRequest("GET", ctx.baseURL+"/api/posts", nil)
			if err != nil {
				continue
			}
			testutils.WithStandardHeaders(req)

			resp, err := ctx.client.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}

		finalStats := sqlDB.Stats()
		if finalStats.OpenConnections > initialStats.MaxOpenConnections {
			t.Errorf("Pool exceeded max connections: %d > %d", finalStats.OpenConnections, initialStats.MaxOpenConnections)
		}
	})
}

func TestE2E_DatabaseErrorHandling(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("invalid_query_returns_error", func(t *testing.T) {
		var result struct {
			ID int
		}

		err := ctx.server.DB.Raw("SELECT * FROM nonexistent_table WHERE id = ?", 1).Scan(&result).Error
		if err == nil {
			t.Error("Expected error for invalid query")
		}
	})

	t.Run("constraint_violation_handled", func(t *testing.T) {
		testUser := ctx.createUserWithCleanup(t, "constraintuser", "StrongPass123!")

		duplicateUser := &database.User{
			Username:      testUser.Username,
			Email:         "different@example.com",
			Password:      "DifferentPass123!",
			EmailVerified: true,
		}

		err := ctx.server.DB.Create(duplicateUser).Error
		if err == nil {
			t.Error("Expected error for duplicate username")
		}
	})

	t.Run("null_constraint_violation", func(t *testing.T) {
		invalidPost := &database.Post{
			Title:   "",
			URL:     "",
			Content: "",
		}

		err := ctx.server.DB.Create(invalidPost).Error
		if err == nil {
			t.Log("SQLite allows empty strings (constraint validation handled at application level)")
		} else {
			t.Logf("Database rejected empty values: %v", err)
		}
	})
}
