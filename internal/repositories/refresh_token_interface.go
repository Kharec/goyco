package repositories

import "goyco/internal/database"

type RefreshTokenRepositoryInterface interface {
	Create(token *database.RefreshToken) error
	GetByTokenHash(tokenHash string) (*database.RefreshToken, error)
	DeleteByUserID(userID uint) error
	DeleteExpired() error
	DeleteByID(id uint) error
	GetByUserID(userID uint) ([]database.RefreshToken, error)
	CountByUserID(userID uint) (int64, error)
}
