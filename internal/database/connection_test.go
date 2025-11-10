package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"goyco/internal/config"
	"goyco/internal/middleware"
)

func TestConnectReturnsErrorWhenUnableToReachDatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "127.0.0.1",
				Port:     "1",
				User:     "postgres",
				Password: "password",
				Name:     "goyco_test",
				SSLMode:  "disable",
			},
		}
		_, err := Connect(cfg)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected connection error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to connect to database") {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("connection test timed out after 5 seconds")
	}
}

func TestMigrateFailsWhenDBNil(t *testing.T) {
	err := Migrate(nil)
	if err == nil {
		t.Fatalf("expected error when DB is nil")
	}
}

func TestMigrateCreatesTables(t *testing.T) {
	dbName := "file:memdb_" + t.Name() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory database: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("expected migrations to succeed, got error: %v", err)
	}

	migrator := db.Migrator()

	models := []any{&User{}, &Post{}, &Vote{}}
	for _, model := range models {
		if !migrator.HasTable(model) {
			t.Fatalf("expected table for %T to exist after migration", model)
		}
	}
}

func TestCloseReturnsNilWhenDBNil(t *testing.T) {
	if err := Close(nil); err != nil {
		t.Fatalf("expected nil error when DB is nil, got %v", err)
	}
}

func TestCloseClosesUnderlyingConnection(t *testing.T) {
	dbName := "file:memdb_" + t.Name() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}

	if err := Close(db); err != nil {
		t.Fatalf("expected close to succeed, got %v", err)
	}

	if err := sqlDB.Ping(); err == nil {
		t.Fatalf("expected ping on closed connection to fail")
	}
}

func TestConnectWithMonitoring(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "test",
			Password: "test",
			Name:     "test_db",
			SSLMode:  "disable",
		},
		App: config.AppConfig{
			Debug: true,
		},
	}

	_, err := ConnectWithMonitoring(cfg, nil)
	if err == nil {
		t.Fatalf("expected connection error with invalid database config")
	}
	if !strings.Contains(err.Error(), "failed to connect to database") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConnectWithMonitoringWithValidMonitor(t *testing.T) {
	mockMonitor := middleware.NewInMemoryDBMonitor()

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "test",
			Password: "test",
			Name:     "test_db",
			SSLMode:  "disable",
		},
		App: config.AppConfig{
			Debug: true,
		},
	}

	_, err := ConnectWithMonitoring(cfg, mockMonitor)
	if err == nil {
		t.Fatalf("expected connection error with invalid database config")
	}
	if !strings.Contains(err.Error(), "failed to connect to database") {
		t.Fatalf("unexpected error: %v", err)
	}
}
