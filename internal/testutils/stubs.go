package testutils

import (
	"context"

	"gorm.io/gorm"
	"goyco/internal/database"
	"goyco/internal/repositories"
)

type PostRepositoryStub struct {
	CreateFn                        func(*database.Post) error
	GetByIDFn                       func(uint) (*database.Post, error)
	GetAllFn                        func(int, int) ([]database.Post, error)
	GetByUserIDFn                   func(uint, int, int) ([]database.Post, error)
	UpdateFn                        func(*database.Post) error
	DeleteFn                        func(uint) error
	CountFn                         func() (int64, error)
	CountByUserIDFn                 func(uint) (int64, error)
	GetTopPostsFn                   func(int) ([]database.Post, error)
	GetNewestPostsFn                func(int) ([]database.Post, error)
	SearchFn                        func(string, int, int) ([]database.Post, error)
	GetPostsByDeletedUsersFn        func() ([]database.Post, error)
	HardDeletePostsByDeletedUsersFn func() (int64, error)
	HardDeleteAllFn                 func() (int64, error)
	WithTxFn                        func(*gorm.DB) repositories.PostRepository
}

func NewPostRepositoryStub() *PostRepositoryStub {
	return &PostRepositoryStub{}
}

func (s *PostRepositoryStub) Create(post *database.Post) error {
	if s != nil && s.CreateFn != nil {
		return s.CreateFn(post)
	}
	return nil
}

func (s *PostRepositoryStub) GetByID(id uint) (*database.Post, error) {
	if s != nil && s.GetByIDFn != nil {
		return s.GetByIDFn(id)
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *PostRepositoryStub) GetAll(limit, offset int) ([]database.Post, error) {
	if s != nil && s.GetAllFn != nil {
		return s.GetAllFn(limit, offset)
	}
	return nil, nil
}

func (s *PostRepositoryStub) GetByUserID(userID uint, limit, offset int) ([]database.Post, error) {
	if s != nil && s.GetByUserIDFn != nil {
		return s.GetByUserIDFn(userID, limit, offset)
	}
	return nil, nil
}

func (s *PostRepositoryStub) Update(post *database.Post) error {
	if s != nil && s.UpdateFn != nil {
		return s.UpdateFn(post)
	}
	return nil
}

func (s *PostRepositoryStub) Delete(id uint) error {
	if s != nil && s.DeleteFn != nil {
		return s.DeleteFn(id)
	}
	return nil
}

func (s *PostRepositoryStub) Count() (int64, error) {
	if s != nil && s.CountFn != nil {
		return s.CountFn()
	}
	return 0, nil
}

func (s *PostRepositoryStub) CountByUserID(userID uint) (int64, error) {
	if s != nil && s.CountByUserIDFn != nil {
		return s.CountByUserIDFn(userID)
	}
	return 0, nil
}

func (s *PostRepositoryStub) GetTopPosts(limit int) ([]database.Post, error) {
	if s != nil && s.GetTopPostsFn != nil {
		return s.GetTopPostsFn(limit)
	}
	return s.GetAll(limit, 0)
}

func (s *PostRepositoryStub) GetNewestPosts(limit int) ([]database.Post, error) {
	if s != nil && s.GetNewestPostsFn != nil {
		return s.GetNewestPostsFn(limit)
	}
	return s.GetAll(limit, 0)
}

func (s *PostRepositoryStub) Search(query string, limit, offset int) ([]database.Post, error) {
	if s != nil && s.SearchFn != nil {
		return s.SearchFn(query, limit, offset)
	}
	return nil, nil
}

func (s *PostRepositoryStub) GetPostsByDeletedUsers() ([]database.Post, error) {
	if s != nil && s.GetPostsByDeletedUsersFn != nil {
		return s.GetPostsByDeletedUsersFn()
	}
	return nil, nil
}

func (s *PostRepositoryStub) HardDeletePostsByDeletedUsers() (int64, error) {
	if s != nil && s.HardDeletePostsByDeletedUsersFn != nil {
		return s.HardDeletePostsByDeletedUsersFn()
	}
	return 0, nil
}

func (s *PostRepositoryStub) HardDeleteAll() (int64, error) {
	if s != nil && s.HardDeleteAllFn != nil {
		return s.HardDeleteAllFn()
	}
	return 0, nil
}

func (s *PostRepositoryStub) WithTx(tx *gorm.DB) repositories.PostRepository {
	if s != nil && s.WithTxFn != nil {
		return s.WithTxFn(tx)
	}
	return s
}

type UserRepositoryStub struct {
	CreateFn                  func(*database.User) error
	GetByIDFn                 func(uint) (*database.User, error)
	GetByIDIncludingDeletedFn func(uint) (*database.User, error)
	GetByUsernameFn           func(string) (*database.User, error)
	GetByUsernameIncludingFn  func(string) (*database.User, error)
	GetByEmailFn              func(string) (*database.User, error)
	GetByVerificationFn       func(string) (*database.User, error)
	GetByPasswordResetFn      func(string) (*database.User, error)
	GetAllFn                  func(int, int) ([]database.User, error)
	UpdateFn                  func(*database.User) error
	DeleteFn                  func(uint) error
	HardDeleteFn              func(uint) error
	SoftDeleteWithPostsFn     func(uint) error
	LockFn                    func(uint) error
	UnlockFn                  func(uint) error
	GetPostsFn                func(uint, int, int) ([]database.Post, error)
	GetDeletedUsersFn         func() ([]database.User, error)
	GetByUsernamePrefixFn     func(string) (*database.User, error)
	HardDeleteAllFn           func() (int64, error)
	CountFn                   func() (int64, error)
	WithTxFn                  func(*gorm.DB) repositories.UserRepository
}

func NewUserRepositoryStub() *UserRepositoryStub {
	return &UserRepositoryStub{}
}

func (s *UserRepositoryStub) Create(user *database.User) error {
	if s != nil && s.CreateFn != nil {
		return s.CreateFn(user)
	}
	return nil
}

func (s *UserRepositoryStub) GetByID(id uint) (*database.User, error) {
	if s != nil && s.GetByIDFn != nil {
		return s.GetByIDFn(id)
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *UserRepositoryStub) GetByIDIncludingDeleted(id uint) (*database.User, error) {
	if s != nil && s.GetByIDIncludingDeletedFn != nil {
		return s.GetByIDIncludingDeletedFn(id)
	}
	return s.GetByID(id)
}

func (s *UserRepositoryStub) GetByUsername(username string) (*database.User, error) {
	if s != nil && s.GetByUsernameFn != nil {
		return s.GetByUsernameFn(username)
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *UserRepositoryStub) GetByUsernameIncludingDeleted(username string) (*database.User, error) {
	if s != nil && s.GetByUsernameIncludingFn != nil {
		return s.GetByUsernameIncludingFn(username)
	}
	return s.GetByUsername(username)
}

func (s *UserRepositoryStub) GetByEmail(email string) (*database.User, error) {
	if s != nil && s.GetByEmailFn != nil {
		return s.GetByEmailFn(email)
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *UserRepositoryStub) GetByVerificationToken(token string) (*database.User, error) {
	if s != nil && s.GetByVerificationFn != nil {
		return s.GetByVerificationFn(token)
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *UserRepositoryStub) GetByPasswordResetToken(token string) (*database.User, error) {
	if s != nil && s.GetByPasswordResetFn != nil {
		return s.GetByPasswordResetFn(token)
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *UserRepositoryStub) GetAll(limit, offset int) ([]database.User, error) {
	if s != nil && s.GetAllFn != nil {
		return s.GetAllFn(limit, offset)
	}
	return nil, nil
}

func (s *UserRepositoryStub) Update(user *database.User) error {
	if s != nil && s.UpdateFn != nil {
		return s.UpdateFn(user)
	}
	return nil
}

func (s *UserRepositoryStub) Delete(id uint) error {
	if s != nil && s.DeleteFn != nil {
		return s.DeleteFn(id)
	}
	return nil
}

func (s *UserRepositoryStub) HardDelete(id uint) error {
	if s != nil && s.HardDeleteFn != nil {
		return s.HardDeleteFn(id)
	}
	return nil
}

func (s *UserRepositoryStub) SoftDeleteWithPosts(id uint) error {
	if s != nil && s.SoftDeleteWithPostsFn != nil {
		return s.SoftDeleteWithPostsFn(id)
	}
	return nil
}

func (s *UserRepositoryStub) Lock(id uint) error {
	if s != nil && s.LockFn != nil {
		return s.LockFn(id)
	}
	return nil
}

func (s *UserRepositoryStub) Unlock(id uint) error {
	if s != nil && s.UnlockFn != nil {
		return s.UnlockFn(id)
	}
	return nil
}

func (s *UserRepositoryStub) GetPosts(userID uint, limit, offset int) ([]database.Post, error) {
	if s != nil && s.GetPostsFn != nil {
		return s.GetPostsFn(userID, limit, offset)
	}
	return nil, nil
}

func (s *UserRepositoryStub) GetDeletedUsers() ([]database.User, error) {
	if s != nil && s.GetDeletedUsersFn != nil {
		return s.GetDeletedUsersFn()
	}
	return nil, nil
}

func (s *UserRepositoryStub) GetByUsernamePrefix(prefix string) (*database.User, error) {
	if s != nil && s.GetByUsernamePrefixFn != nil {
		return s.GetByUsernamePrefixFn(prefix)
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *UserRepositoryStub) HardDeleteAll() (int64, error) {
	if s != nil && s.HardDeleteAllFn != nil {
		return s.HardDeleteAllFn()
	}
	return 0, nil
}

func (s *UserRepositoryStub) Count() (int64, error) {
	if s != nil && s.CountFn != nil {
		return s.CountFn()
	}
	return 0, nil
}

func (s *UserRepositoryStub) WithTx(tx *gorm.DB) repositories.UserRepository {
	if s != nil && s.WithTxFn != nil {
		return s.WithTxFn(tx)
	}
	return s
}

type EmailSenderStub struct {
	SendFn func(to, subject, body string) error
}

func (s *EmailSenderStub) Send(to, subject, body string) error {
	if s != nil && s.SendFn != nil {
		return s.SendFn(to, subject, body)
	}
	return nil
}

type TitleFetcherStub struct {
	FetchTitleFn func(ctx context.Context, rawURL string) (string, error)
}

func (s *TitleFetcherStub) FetchTitle(ctx context.Context, rawURL string) (string, error) {
	if s != nil && s.FetchTitleFn != nil {
		return s.FetchTitleFn(ctx, rawURL)
	}
	return "", nil
}
