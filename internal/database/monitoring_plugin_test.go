package database

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"goyco/internal/middleware"
)

func TestNewGormDBMonitor(t *testing.T) {
	monitor := middleware.NewInMemoryDBMonitor()
	gormMonitor := NewGormDBMonitor(monitor)

	if gormMonitor == nil {
		t.Fatal("Expected non-nil GormDBMonitor")
	}

	if gormMonitor.monitor != monitor {
		t.Error("Expected monitor to be set correctly")
	}
}

func TestGormDBMonitor_Name(t *testing.T) {
	monitor := middleware.NewInMemoryDBMonitor()
	gormMonitor := NewGormDBMonitor(monitor)

	if gormMonitor.Name() != "db_monitor" {
		t.Errorf("Expected name 'db_monitor', got '%s'", gormMonitor.Name())
	}
}

func TestGormDBMonitor_Initialize(t *testing.T) {
	monitor := middleware.NewInMemoryDBMonitor()
	gormMonitor := NewGormDBMonitor(monitor)

	db := newTestDB(t)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	err := gormMonitor.Initialize(db)
	if err != nil {
		t.Fatalf("Expected Initialize to succeed, got error: %v", err)
	}
}

func TestGormDBMonitor_InitializeWithNilMonitor(t *testing.T) {
	gormMonitor := NewGormDBMonitor(nil)

	db := newTestDB(t)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	err := gormMonitor.Initialize(db)
	if err != nil {
		t.Fatalf("Expected Initialize to succeed with nil monitor, got error: %v", err)
	}
}

func TestGormDBMonitor_Callbacks(t *testing.T) {
	monitor := middleware.NewInMemoryDBMonitor()
	gormMonitor := NewGormDBMonitor(monitor)

	db := newTestDB(t)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	err := gormMonitor.Initialize(db)
	if err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}

	user := &User{
		Username:      "testuser",
		Email:         "test@example.com",
		Password:      "hashedpassword",
		EmailVerified: true,
	}

	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	var foundUser User
	if err := db.First(&foundUser, user.ID).Error; err != nil {
		t.Fatalf("Failed to find user: %v", err)
	}

	foundUser.Username = "updateduser"
	if err := db.Save(&foundUser).Error; err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	if err := db.Delete(&foundUser).Error; err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}
}

func TestGormDBMonitor_CallbacksWithNilMonitor(t *testing.T) {
	gormMonitor := NewGormDBMonitor(nil)

	db := newTestDB(t)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	err := gormMonitor.Initialize(db)
	if err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}

	user := &User{
		Username:      "testuser",
		Email:         "test@example.com",
		Password:      "hashedpassword",
		EmailVerified: true,
	}

	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
}

func TestGormDBMonitor_BuildQueryString(t *testing.T) {
	monitor := middleware.NewInMemoryDBMonitor()
	gormMonitor := NewGormDBMonitor(monitor)

	db := newTestDB(t)
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	err := gormMonitor.Initialize(db)
	if err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}

	tests := []struct {
		name      string
		operation string
		table     string
		expected  string
	}{
		{
			name:      "create_operation",
			operation: "CREATE",
			table:     "users",
			expected:  "CREATE FROM users",
		},
		{
			name:      "select_operation",
			operation: "SELECT",
			table:     "posts",
			expected:  "SELECT FROM posts",
		},
		{
			name:      "update_operation",
			operation: "UPDATE",
			table:     "votes",
			expected:  "UPDATE FROM votes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			stmt := &gorm.Statement{
				Table: tt.table,
			}

			mockDB := &gorm.DB{
				Statement: stmt,
			}

			result := gormMonitor.buildQueryString(mockDB, tt.operation)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestGormDBMonitor_LogOperation(t *testing.T) {
	monitor := middleware.NewInMemoryDBMonitor()
	gormMonitor := NewGormDBMonitor(monitor)

	startTime := time.Now()
	ctx := context.WithValue(context.Background(), gormOperationStartKey, startTime)

	stmt := &gorm.Statement{
		Context: ctx,
		Table:   "users",
	}

	mockDB := &gorm.DB{
		Statement: stmt,
	}

	gormMonitor.logOperation(mockDB, "CREATE")

	gormMonitor.monitor = nil
	gormMonitor.logOperation(mockDB, "CREATE")
}

func TestGormDBMonitor_LogOperationWithoutStartTime(t *testing.T) {
	monitor := middleware.NewInMemoryDBMonitor()
	gormMonitor := NewGormDBMonitor(monitor)

	ctx := context.Background()

	stmt := &gorm.Statement{
		Context: ctx,
		Table:   "users",
	}

	mockDB := &gorm.DB{
		Statement: stmt,
	}

	gormMonitor.logOperation(mockDB, "CREATE")
}

func TestGormDBMonitor_AllCallbackMethods(t *testing.T) {
	monitor := middleware.NewInMemoryDBMonitor()
	gormMonitor := NewGormDBMonitor(monitor)

	gormMonitor.monitor = nil

	ctx := context.Background()
	stmt := &gorm.Statement{
		Context: ctx,
		Table:   "users",
	}

	mockDB := &gorm.DB{
		Statement: stmt,
	}

	gormMonitor.beforeCreate(mockDB)
	gormMonitor.beforeQuery(mockDB)
	gormMonitor.beforeUpdate(mockDB)
	gormMonitor.beforeDelete(mockDB)
	gormMonitor.beforeRow(mockDB)
	gormMonitor.beforeRaw(mockDB)

	gormMonitor.afterCreate(mockDB)
	gormMonitor.afterQuery(mockDB)
	gormMonitor.afterUpdate(mockDB)
	gormMonitor.afterDelete(mockDB)
	gormMonitor.afterRow(mockDB)
	gormMonitor.afterRaw(mockDB)
}

func TestGormDBMonitor_WithRealDatabase(t *testing.T) {
	monitor := middleware.NewInMemoryDBMonitor()
	gormMonitor := NewGormDBMonitor(monitor)

	dbName := "file:memdb_" + t.Name() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	err = gormMonitor.Initialize(db)
	if err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}

	user := &User{
		Username:      "testuser",
		Email:         "test@example.com",
		Password:      "hashedpassword",
		EmailVerified: true,
	}

	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	var foundUser User
	if err := db.First(&foundUser, user.ID).Error; err != nil {
		t.Fatalf("Failed to find user: %v", err)
	}

	foundUser.Username = "updateduser"
	if err := db.Save(&foundUser).Error; err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	if err := db.Delete(&foundUser).Error; err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	stats := monitor.GetStats()
	if stats.TotalQueries == 0 {
		t.Error("Expected monitor to have recorded some queries")
	}
}
