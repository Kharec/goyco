package fuzz

import (
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

	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			email_verified INTEGER DEFAULT 0 NOT NULL,
			email_verified_at DATETIME,
			email_verification_token TEXT,
			email_verification_sent_at DATETIME,
			password_reset_token TEXT,
			password_reset_sent_at DATETIME,
			password_reset_expires_at DATETIME,
			locked INTEGER DEFAULT 0,
			session_version INTEGER DEFAULT 1 NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		);
		CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			url TEXT UNIQUE,
			content TEXT,
			author_id INTEGER,
			author_name TEXT,
			up_votes INTEGER DEFAULT 0,
			down_votes INTEGER DEFAULT 0,
			score INTEGER DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			FOREIGN KEY(author_id) REFERENCES users(id)
		);
		CREATE TABLE IF NOT EXISTS votes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			post_id INTEGER NOT NULL,
			type TEXT NOT NULL,
			vote_hash TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			FOREIGN KEY(user_id) REFERENCES users(id),
			FOREIGN KEY(post_id) REFERENCES posts(id)
		);
		CREATE TABLE IF NOT EXISTS account_deletion_requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			token_hash TEXT UNIQUE NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME,
			FOREIGN KEY(user_id) REFERENCES users(id)
		);
		CREATE TABLE IF NOT EXISTS refresh_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			token_hash TEXT UNIQUE NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME,
			FOREIGN KEY(user_id) REFERENCES users(id)
		);
	`).Error

	return db, err
}
