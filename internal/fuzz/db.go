package fuzz

import (
	"goyco/internal/database"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func GetFuzzDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(
		&database.User{},
		&database.Post{},
		&database.Vote{},
		&database.AccountDeletionRequest{},
		&database.RefreshToken{},
	)
	if err != nil {
		return nil, err
	}

	if execErr := db.Exec("PRAGMA foreign_keys = ON").Error; execErr != nil {
		return nil, execErr
	}

	return db, nil
}
