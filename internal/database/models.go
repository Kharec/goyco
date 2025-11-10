package database

import (
	"time"

	"gorm.io/gorm"
)

type Post struct {
	ID          uint   `gorm:"primaryKey"`
	Title       string `gorm:"not null"`
	URL         string `gorm:"uniqueIndex"`
	Content     string
	AuthorID    *uint
	AuthorName  string
	Author      User     `gorm:"foreignKey:AuthorID;constraint:OnDelete:CASCADE"`
	UpVotes     int      `gorm:"default:0"`
	DownVotes   int      `gorm:"default:0"`
	Score       int      `gorm:"default:0"`
	Votes       []Vote   `gorm:"foreignKey:PostID;constraint:OnDelete:CASCADE"`
	CurrentVote VoteType `gorm:"-"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

type User struct {
	ID                      uint   `gorm:"primaryKey"`
	Username                string `gorm:"uniqueIndex;not null"`
	Email                   string `gorm:"uniqueIndex;not null"`
	Password                string `gorm:"not null"`
	EmailVerified           bool   `gorm:"default:false;not null"`
	EmailVerifiedAt         *time.Time
	EmailVerificationToken  string `gorm:"index"`
	EmailVerificationSentAt *time.Time
	PasswordResetToken      string `gorm:"index"`
	PasswordResetSentAt     *time.Time
	PasswordResetExpiresAt  *time.Time
	Locked                  bool           `gorm:"default:false"`
	SessionVersion          uint           `gorm:"default:1;not null"`
	RefreshTokens           []RefreshToken `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Posts                   []Post         `gorm:"foreignKey:AuthorID"`
	Votes                   []Vote         `gorm:"foreignKey:UserID"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
	DeletedAt               gorm.DeletedAt `gorm:"index"`
}

type RefreshToken struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"not null;index"`
	User      User      `gorm:"constraint:OnDelete:CASCADE"`
	TokenHash string    `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"not null;index"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type AccountDeletionRequest struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"uniqueIndex"`
	User      User      `gorm:"constraint:OnDelete:CASCADE"`
	TokenHash string    `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
}

type Vote struct {
	ID        uint     `gorm:"primaryKey"`
	UserID    *uint    `gorm:"uniqueIndex:idx_user_post_vote,where:deleted_at IS NULL AND user_id IS NOT NULL"`
	User      *User    `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	PostID    uint     `gorm:"not null;uniqueIndex:idx_user_post_vote,where:deleted_at IS NULL AND user_id IS NOT NULL;uniqueIndex:idx_hash_post_vote,where:deleted_at IS NULL AND vote_hash IS NOT NULL"`
	Post      Post     `gorm:"foreignKey:PostID;constraint:OnDelete:CASCADE"`
	Type      VoteType `gorm:"not null"`
	VoteHash  *string  `gorm:"uniqueIndex:idx_hash_post_vote,where:deleted_at IS NULL AND vote_hash IS NOT NULL"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type VoteType string

const (
	VoteUp   VoteType = "up"
	VoteDown VoteType = "down"
	VoteNone VoteType = "none"
)
