package repositories

import (
	"fmt"
	"testing"

	"goyco/internal/database"

	"gorm.io/gorm"
)

func TestApplyPagination(t *testing.T) {
	suite := NewTestSuite(t)

	tests := []struct {
		name             string
		limit            int
		offset           int
		setupQuery       func(*gorm.DB) *gorm.DB
		verifyPagination func(*testing.T, *gorm.DB, int, int)
	}{
		{
			name:   "limit > 0 and offset > 0",
			limit:  10,
			offset: 5,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 20; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				if len(users) != limit {
					t.Errorf("Expected %d users, got %d", limit, len(users))
				}
			},
		},
		{
			name:   "limit > 0 and offset = 0",
			limit:  5,
			offset: 0,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 10; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				if len(users) != limit {
					t.Errorf("Expected %d users, got %d", limit, len(users))
				}
			},
		},
		{
			name:   "limit = 0 (should not apply limit)",
			limit:  0,
			offset: 5,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 10; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				expected := 5
				if len(users) != expected {
					t.Errorf("Expected %d users with offset %d, got %d", expected, offset, len(users))
				}
			},
		},
		{
			name:   "offset = 0 (should not apply offset)",
			limit:  10,
			offset: 0,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 15; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				if len(users) != limit {
					t.Errorf("Expected %d users, got %d", limit, len(users))
				}
			},
		},
		{
			name:   "limit = 0 and offset = 0 (should not apply pagination)",
			limit:  0,
			offset: 0,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 10; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				if len(users) != 10 {
					t.Errorf("Expected all 10 users, got %d", len(users))
				}
			},
		},
		{
			name:   "negative limit (should not apply limit)",
			limit:  -5,
			offset: 10,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 20; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				expected := 10
				if len(users) != expected {
					t.Errorf("Expected %d users with offset %d, got %d", expected, offset, len(users))
				}
			},
		},
		{
			name:   "negative offset (should not apply offset)",
			limit:  10,
			offset: -5,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 15; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				if len(users) != limit {
					t.Errorf("Expected %d users, got %d", limit, len(users))
				}
			},
		},
		{
			name:   "large limit and offset values",
			limit:  1000,
			offset: 500,
			setupQuery: func(db *gorm.DB) *gorm.DB {
				return db.Model(&database.User{})
			},
			verifyPagination: func(t *testing.T, query *gorm.DB, limit, offset int) {
				for i := 0; i < 2000; i++ {
					suite.CreateTestUser(
						fmt.Sprintf("testuser_%d", i),
						fmt.Sprintf("user%d@example.com", i),
						"password123",
					)
				}

				var users []database.User
				result := query.Find(&users)
				if result.Error != nil {
					t.Fatalf("Query failed: %v", result.Error)
				}

				if len(users) != limit {
					t.Errorf("Expected %d users, got %d", limit, len(users))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suite.Reset()

			baseQuery := tt.setupQuery(suite.DB)
			paginatedQuery := ApplyPagination(baseQuery, tt.limit, tt.offset)

			tt.verifyPagination(t, paginatedQuery, tt.limit, tt.offset)
		})
	}
}
