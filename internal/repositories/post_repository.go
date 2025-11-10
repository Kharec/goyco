package repositories

import (
	"gorm.io/gorm"
	"goyco/internal/database"
)

type PostRepository interface {
	Create(post *database.Post) error
	GetByID(id uint) (*database.Post, error)
	GetAll(limit, offset int) ([]database.Post, error)
	GetByUserID(userID uint, limit, offset int) ([]database.Post, error)
	Update(post *database.Post) error
	Delete(id uint) error
	Count() (int64, error)
	CountByUserID(userID uint) (int64, error)
	GetTopPosts(limit int) ([]database.Post, error)
	GetNewestPosts(limit int) ([]database.Post, error)
	Search(query string, limit, offset int) ([]database.Post, error)
	GetPostsByDeletedUsers() ([]database.Post, error)
	HardDeletePostsByDeletedUsers() (int64, error)
	HardDeleteAll() (int64, error)
	WithTx(tx *gorm.DB) PostRepository
}

type postRepository struct {
	db        *gorm.DB
	sanitizer *SearchSanitizer
}

func NewPostRepository(db *gorm.DB) PostRepository {
	return &postRepository{
		db:        db,
		sanitizer: NewSearchSanitizer(),
	}
}

func (r *postRepository) Create(post *database.Post) error {
	return r.db.Create(post).Error
}

func (r *postRepository) GetByID(id uint) (*database.Post, error) {
	var post database.Post
	err := r.db.Preload("Author").First(&post, id).Error
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *postRepository) GetAll(limit, offset int) ([]database.Post, error) {
	var posts []database.Post
	query := r.db.Preload("Author").Order("score DESC, created_at DESC, id DESC")
	query = ApplyPagination(query, limit, offset)

	err := query.Find(&posts).Error
	return posts, err
}

func (r *postRepository) GetByUserID(userID uint, limit, offset int) ([]database.Post, error) {
	var posts []database.Post
	query := r.db.Where("author_id = ?", userID).Preload("Author").Order("created_at DESC")
	query = ApplyPagination(query, limit, offset)

	err := query.Find(&posts).Error
	return posts, err
}

func (r *postRepository) Update(post *database.Post) error {
	return r.db.Save(post).Error
}

func (r *postRepository) Delete(id uint) error {
	return r.db.Delete(&database.Post{}, id).Error
}

func (r *postRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&database.Post{}).Count(&count).Error
	return count, err
}

func (r *postRepository) CountByUserID(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&database.Post{}).Where("author_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *postRepository) GetTopPosts(limit int) ([]database.Post, error) {
	var posts []database.Post
	query := r.db.Preload("Author").Order("score DESC, created_at DESC, id DESC")
	query = ApplyPagination(query, limit, 0)

	err := query.Find(&posts).Error
	return posts, err
}

func (r *postRepository) GetNewestPosts(limit int) ([]database.Post, error) {
	var posts []database.Post
	query := r.db.Preload("Author").Order("created_at DESC")
	query = ApplyPagination(query, limit, 0)

	err := query.Find(&posts).Error
	return posts, err
}

func (r *postRepository) Search(query string, limit, offset int) ([]database.Post, error) {
	var posts []database.Post

	sanitizedQuery, err := r.sanitizer.SanitizeSearchQuery(query)
	if err != nil {
		return nil, err
	}

	if err := r.sanitizer.ValidateSearchQuery(sanitizedQuery); err != nil {
		return nil, err
	}

	if sanitizedQuery == "" {
		return posts, nil
	}

	dbQuery := r.db.Preload("Author").Where("UPPER(title) LIKE UPPER(?) OR UPPER(content) LIKE UPPER(?)", "%"+sanitizedQuery+"%", "%"+sanitizedQuery+"%").Order("score DESC, created_at DESC, id DESC")
	dbQuery = ApplyPagination(dbQuery, limit, offset)

	err = dbQuery.Find(&posts).Error
	return posts, err
}

func (r *postRepository) WithTx(tx *gorm.DB) PostRepository {
	return &postRepository{db: tx, sanitizer: r.sanitizer}
}

func (r *postRepository) GetPostsByDeletedUsers() ([]database.Post, error) {
	var posts []database.Post
	err := r.db.Unscoped().
		Preload("Author").
		Where("author_id IS NULL OR author_id IN (SELECT id FROM users WHERE deleted_at IS NOT NULL)").
		Find(&posts).Error
	return posts, err
}

func (r *postRepository) HardDeletePostsByDeletedUsers() (int64, error) {
	result := r.db.Unscoped().
		Where("author_id IS NULL OR author_id IN (SELECT id FROM users WHERE deleted_at IS NOT NULL)").
		Delete(&database.Post{})
	return result.RowsAffected, result.Error
}

func (r *postRepository) HardDeleteAll() (int64, error) {
	var rowsAffected int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Unscoped().Where("1 = 1").Delete(&database.Post{})
		rowsAffected = result.RowsAffected
		return result.Error
	})
	return rowsAffected, err
}
