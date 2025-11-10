package repositories

import (
	"gorm.io/gorm"
	"goyco/internal/database"
)

type AccountDeletionRepository interface {
	Create(req *database.AccountDeletionRequest) error
	GetByTokenHash(hash string) (*database.AccountDeletionRequest, error)
	DeleteByID(id uint) error
	DeleteByUserID(userID uint) error
}

type accountDeletionRepository struct {
	db *gorm.DB
}

func NewAccountDeletionRepository(db *gorm.DB) AccountDeletionRepository {
	return &accountDeletionRepository{db: db}
}

func (r *accountDeletionRepository) Create(req *database.AccountDeletionRequest) error {
	return r.db.Create(req).Error
}

func (r *accountDeletionRepository) GetByTokenHash(hash string) (*database.AccountDeletionRequest, error) {
	var request database.AccountDeletionRequest
	if err := r.db.Where("token_hash = ?", hash).First(&request).Error; err != nil {
		return nil, err
	}
	return &request, nil
}

func (r *accountDeletionRepository) DeleteByID(id uint) error {
	return r.db.Delete(&database.AccountDeletionRequest{}, id).Error
}

func (r *accountDeletionRepository) DeleteByUserID(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&database.AccountDeletionRequest{}).Error
}
