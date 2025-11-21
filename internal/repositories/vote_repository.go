package repositories

import (
	"errors"

	"gorm.io/gorm"
	"goyco/internal/database"
)

type VoteRepository interface {
	Create(vote *database.Vote) error
	CreateOrUpdate(vote *database.Vote) error
	GetByID(id uint) (*database.Vote, error)
	GetByUserAndPost(userID, postID uint) (*database.Vote, error)
	GetByVoteHash(voteHash string) (*database.Vote, error)
	GetByPostID(postID uint) ([]database.Vote, error)
	GetByUserID(userID uint) ([]database.Vote, error)
	Update(vote *database.Vote) error
	Delete(id uint) error
	Count() (int64, error)
	CountByPostID(postID uint) (int64, error)
	CountByUserID(userID uint) (int64, error)
	GetVoteCountsByPostID(postID uint) (upVotes int, downVotes int, err error)
	WithTx(tx *gorm.DB) VoteRepository
}

type voteRepository struct {
	db *gorm.DB
}

func NewVoteRepository(db *gorm.DB) VoteRepository {
	return &voteRepository{db: db}
}

func (r *voteRepository) Create(vote *database.Vote) error {
	return r.db.Create(vote).Error
}

func (r *voteRepository) CreateOrUpdate(vote *database.Vote) error {
	var existingVote *database.Vote
	var err error
	var lookupByUserID bool

	if vote.UserID != nil {
		existingVote, err = r.GetByUserAndPost(*vote.UserID, vote.PostID)
		lookupByUserID = true
	} else if vote.VoteHash != nil {
		existingVote, err = r.GetByVoteHash(*vote.VoteHash)
		lookupByUserID = false
	} else {
		return errors.New("vote must have either user_id or vote_hash")
	}

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err := r.Create(vote)
			if err != nil && errors.Is(err, gorm.ErrDuplicatedKey) {
				if lookupByUserID {
					existingVote, err = r.GetByUserAndPost(*vote.UserID, vote.PostID)
				} else {
					existingVote, err = r.GetByVoteHash(*vote.VoteHash)
				}
				if err != nil {
					return err
				}
				existingVote.Type = vote.Type
				existingVote.UpdatedAt = vote.UpdatedAt
				return r.Update(existingVote)
			}
			return err
		}
		return err
	}

	existingVote.Type = vote.Type
	existingVote.UpdatedAt = vote.UpdatedAt
	return r.Update(existingVote)
}

func (r *voteRepository) GetByID(id uint) (*database.Vote, error) {
	var vote database.Vote
	err := r.db.Preload("User").Preload("Post").First(&vote, id).Error
	if err != nil {
		return nil, err
	}
	return &vote, nil
}

func (r *voteRepository) GetByUserAndPost(userID, postID uint) (*database.Vote, error) {
	var vote database.Vote
	err := r.db.Where("user_id = ? AND post_id = ?", userID, postID).First(&vote).Error
	if err != nil {
		return nil, err
	}
	return &vote, nil
}

func (r *voteRepository) GetByVoteHash(voteHash string) (*database.Vote, error) {
	var vote database.Vote
	err := r.db.Where("vote_hash = ?", voteHash).First(&vote).Error
	if err != nil {
		return nil, err
	}
	return &vote, nil
}

func (r *voteRepository) GetByPostID(postID uint) ([]database.Vote, error) {
	var votes []database.Vote
	err := r.db.Where("post_id = ?", postID).Find(&votes).Error
	return votes, err
}

func (r *voteRepository) GetByUserID(userID uint) ([]database.Vote, error) {
	var votes []database.Vote
	err := r.db.Where("user_id = ?", userID).Preload("Post").Find(&votes).Error
	return votes, err
}

func (r *voteRepository) Update(vote *database.Vote) error {
	return r.db.Save(vote).Error
}

func (r *voteRepository) Delete(id uint) error {
	return r.db.Delete(&database.Vote{}, id).Error
}

func (r *voteRepository) CountByPostID(postID uint) (int64, error) {
	var count int64
	err := r.db.Model(&database.Vote{}).Where("post_id = ?", postID).Count(&count).Error
	return count, err
}

func (r *voteRepository) CountByUserID(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&database.Vote{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *voteRepository) WithTx(tx *gorm.DB) VoteRepository {
	return &voteRepository{db: tx}
}

func (r *voteRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&database.Vote{}).Count(&count).Error
	return count, err
}

func (r *voteRepository) GetVoteCountsByPostID(postID uint) (int, int, error) {
	var result struct {
		UpVotes   int64
		DownVotes int64
	}

	err := r.db.Model(&database.Vote{}).
		Select("COUNT(CASE WHEN type = ? THEN 1 END) as up_votes, COUNT(CASE WHEN type = ? THEN 1 END) as down_votes", database.VoteUp, database.VoteDown).
		Where("post_id = ?", postID).
		Scan(&result).Error
	if err != nil {
		return 0, 0, err
	}

	return int(result.UpVotes), int(result.DownVotes), nil
}
