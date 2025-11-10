package repositories

import (
	"fmt"
	"strings"

	"goyco/internal/database"
	"goyco/internal/validation"

	"gorm.io/gorm"
)

type UserRepository interface {
	Create(user *database.User) error
	GetByID(id uint) (*database.User, error)
	GetByIDIncludingDeleted(id uint) (*database.User, error)
	GetByUsername(username string) (*database.User, error)
	GetByUsernameIncludingDeleted(username string) (*database.User, error)
	GetByEmail(email string) (*database.User, error)
	GetByVerificationToken(token string) (*database.User, error)
	GetByPasswordResetToken(token string) (*database.User, error)
	GetAll(limit, offset int) ([]database.User, error)
	Update(user *database.User) error
	Delete(id uint) error
	HardDelete(id uint) error
	SoftDeleteWithPosts(id uint) error
	Lock(id uint) error
	Unlock(id uint) error
	GetPosts(userID uint, limit, offset int) ([]database.Post, error)
	GetDeletedUsers() ([]database.User, error)
	HardDeleteAll() (int64, error)
	Count() (int64, error)
	WithTx(tx *gorm.DB) UserRepository
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *database.User) error {
	username := strings.TrimSpace(user.Username)
	if username == "" {
		return fmt.Errorf("username is required")
	}

	email := strings.TrimSpace(user.Email)
	if email == "" {
		return fmt.Errorf("email is required")
	}

	if err := validation.ValidateEmail(email); err != nil {
		return err
	}

	normalizedEmail := strings.ToLower(email)
	user.Email = normalizedEmail
	user.Username = username

	return r.db.Create(user).Error
}

func (r *userRepository) GetByID(id uint) (*database.User, error) {
	var user database.User
	err := r.db.Preload("Posts").First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByIDIncludingDeleted(id uint) (*database.User, error) {
	var user database.User
	err := r.db.Unscoped().Preload("Posts").First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByUsername(username string) (*database.User, error) {
	var user database.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByUsernameIncludingDeleted(username string) (*database.User, error) {
	var user database.User
	err := r.db.Unscoped().Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByEmail(email string) (*database.User, error) {
	var user database.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByVerificationToken(token string) (*database.User, error) {
	var user database.User
	err := r.db.Where("email_verification_token = ?", token).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByPasswordResetToken(token string) (*database.User, error) {
	var user database.User
	err := r.db.Where("password_reset_token = ?", token).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetAll(limit, offset int) ([]database.User, error) {
	var users []database.User
	query := r.db.Order("created_at DESC")
	query = ApplyPagination(query, limit, offset)

	err := query.Find(&users).Error
	return users, err
}

func (r *userRepository) Update(user *database.User) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}

	return r.db.Model(user).Select("*").Updates(user).Error
}

func (r *userRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteUserVotes(tx, id); err != nil {
			return err
		}

		if err := tx.Model(&database.Post{}).Where("author_id = ?", id).Updates(map[string]any{
			"author_id":   nil,
			"author_name": "(deleted)",
		}).Error; err != nil {
			return fmt.Errorf("update user posts: %w", err)
		}

		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&database.AccountDeletionRequest{}).Error; err != nil {
			return fmt.Errorf("delete user deletion requests: %w", err)
		}

		if err := tx.Delete(&database.User{}, id).Error; err != nil {
			return fmt.Errorf("delete user: %w", err)
		}

		return nil
	})
}

func (r *userRepository) HardDelete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteUserVotes(tx, id); err != nil {
			return err
		}

		if err := tx.Unscoped().Where("author_id = ?", id).Delete(&database.Post{}).Error; err != nil {
			return fmt.Errorf("delete user posts: %w", err)
		}

		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&database.AccountDeletionRequest{}).Error; err != nil {
			return fmt.Errorf("delete user deletion requests: %w", err)
		}

		if err := tx.Unscoped().Delete(&database.User{}, id).Error; err != nil {
			return fmt.Errorf("delete user: %w", err)
		}

		return nil
	})
}

func (r *userRepository) SoftDeleteWithPosts(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteUserVotes(tx, id); err != nil {
			return err
		}

		if err := tx.Unscoped().Model(&database.Post{}).Where("author_id = ?", id).Updates(map[string]any{
			"author_id":   nil,
			"author_name": "(deleted)",
		}).Error; err != nil {
			return fmt.Errorf("update user posts: %w", err)
		}

		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&database.AccountDeletionRequest{}).Error; err != nil {
			return fmt.Errorf("delete user deletion requests: %w", err)
		}

		if err := tx.Unscoped().Delete(&database.User{}, id).Error; err != nil {
			return fmt.Errorf("delete user: %w", err)
		}

		return nil
	})
}

func (r *userRepository) GetPosts(userID uint, limit, offset int) ([]database.Post, error) {
	var posts []database.Post
	query := r.db.Where("author_id = ?", userID).Preload("Author").Order("created_at DESC")
	query = ApplyPagination(query, limit, offset)

	err := query.Find(&posts).Error
	return posts, err
}

func (r *userRepository) Lock(id uint) error {
	return r.db.Model(&database.User{}).Where("id = ?", id).Update("locked", true).Error
}

func (r *userRepository) Unlock(id uint) error {
	return r.db.Model(&database.User{}).Where("id = ?", id).Update("locked", false).Error
}

func (r *userRepository) GetDeletedUsers() ([]database.User, error) {
	var users []database.User
	err := r.db.Unscoped().
		Where("deleted_at IS NOT NULL").
		Find(&users).Error
	return users, err
}

func (r *userRepository) HardDeleteAll() (int64, error) {
	var totalDeleted int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Unscoped().Where("1 = 1").Delete(&database.Vote{})
		if result.Error != nil {
			return fmt.Errorf("delete all votes: %w", result.Error)
		}
		totalDeleted += result.RowsAffected

		result = tx.Unscoped().Where("1 = 1").Delete(&database.Post{})
		if result.Error != nil {
			return fmt.Errorf("delete all posts: %w", result.Error)
		}
		totalDeleted += result.RowsAffected

		result = tx.Unscoped().Where("1 = 1").Delete(&database.AccountDeletionRequest{})
		if result.Error != nil {
			return fmt.Errorf("delete all account deletion requests: %w", result.Error)
		}
		totalDeleted += result.RowsAffected

		result = tx.Unscoped().Where("1 = 1").Delete(&database.User{})
		if result.Error != nil {
			return fmt.Errorf("delete all users: %w", result.Error)
		}
		totalDeleted += result.RowsAffected

		return nil
	})
	return totalDeleted, err
}

func (r *userRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&database.User{}).Count(&count).Error
	return count, err
}

func (r *userRepository) WithTx(tx *gorm.DB) UserRepository {
	return &userRepository{db: tx}
}

func deleteUserVotes(tx *gorm.DB, userID uint) error {
	if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&database.Vote{}).Error; err != nil {
		return fmt.Errorf("delete user votes: %w", err)
	}

	var posts []database.Post
	if err := tx.Unscoped().Where("author_id = ?", userID).Find(&posts).Error; err != nil {
		return fmt.Errorf("get user posts: %w", err)
	}

	if len(posts) > 0 {
		postIDs := make([]uint, len(posts))
		for i, post := range posts {
			postIDs[i] = post.ID
		}
		if err := tx.Unscoped().Where("post_id IN (?)", postIDs).Delete(&database.Vote{}).Error; err != nil {
			return fmt.Errorf("delete votes on user posts: %w", err)
		}
	}

	return nil
}
