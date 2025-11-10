package repositories

import (
	"time"

	"gorm.io/gorm"
	"goyco/internal/database"
)

type RefreshTokenRepository struct {
	db *gorm.DB
}

var _ RefreshTokenRepositoryInterface = (*RefreshTokenRepository)(nil)

func NewRefreshTokenRepository(db *gorm.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(token *database.RefreshToken) error {
	return r.db.Create(token).Error
}

func (r *RefreshTokenRepository) GetByTokenHash(tokenHash string) (*database.RefreshToken, error) {
	var token database.RefreshToken
	err := r.db.Where("token_hash = ?", tokenHash).First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *RefreshTokenRepository) DeleteByUserID(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&database.RefreshToken{}).Error
}

func (r *RefreshTokenRepository) DeleteExpired() error {
	return r.db.Where("expires_at < ?", time.Now()).Delete(&database.RefreshToken{}).Error
}

func (r *RefreshTokenRepository) DeleteByID(id uint) error {
	return r.db.Delete(&database.RefreshToken{}, id).Error
}

func (r *RefreshTokenRepository) GetByUserID(userID uint) ([]database.RefreshToken, error) {
	var tokens []database.RefreshToken
	err := r.db.Where("user_id = ?", userID).Find(&tokens).Error
	return tokens, err
}

func (r *RefreshTokenRepository) CountByUserID(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&database.RefreshToken{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}
