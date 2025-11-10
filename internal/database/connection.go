package database

import (
	"fmt"

	"goyco/internal/config"
	"goyco/internal/middleware"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func connectDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := cfg.GetConnectionString()
	gormLogger := CreateSecureLogger(!cfg.App.Debug)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

func Connect(cfg *config.Config) (*gorm.DB, error) {
	return connectDB(cfg)
}

func ConnectWithMonitoring(cfg *config.Config, monitor middleware.DBMonitor) (*gorm.DB, error) {
	db, err := connectDB(cfg)
	if err != nil {
		return nil, err
	}

	if monitor != nil {
		monitoringPlugin := NewGormDBMonitor(monitor)
		if err := db.Use(monitoringPlugin); err != nil {
			return nil, fmt.Errorf("failed to add monitoring plugin: %w", err)
		}
	}

	return db, nil
}

func Migrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	err := db.AutoMigrate(
		&User{},
		&Post{},
		&Vote{},
		&AccountDeletionRequest{},
		&RefreshToken{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	return nil
}

func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	return sqlDB.Close()
}
