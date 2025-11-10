package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
	"goyco/internal/database"
	"goyco/internal/repositories"
)

type VoteService struct {
	voteRepo  repositories.VoteRepository
	postRepo  repositories.PostRepository
	db        *gorm.DB
	voteMutex sync.RWMutex
}

type VoteRequest struct {
	UserID    uint              `json:"user_id,omitempty"`
	PostID    uint              `json:"post_id"`
	Type      database.VoteType `json:"type"`
	IPAddress string            `json:"-"`
	UserAgent string            `json:"-"`
}

type VoteResponse struct {
	PostID            uint              `json:"post_id"`
	Type              database.VoteType `json:"type"`
	UpVotes           int               `json:"up_votes"`
	DownVotes         int               `json:"down_votes"`
	Score             int               `json:"score"`
	Message           string            `json:"message"`
	IsUnauthenticated bool              `json:"is_unauthenticated"`
}

func NewVoteService(voteRepo repositories.VoteRepository, postRepo repositories.PostRepository, db *gorm.DB) *VoteService {
	return &VoteService{
		voteRepo: voteRepo,
		postRepo: postRepo,
		db:       db,
	}
}

func (vs *VoteService) GenerateVoteHash(ipAddress, userAgent string, postID uint) string {
	data := fmt.Sprintf("%s:%s:%d", ipAddress, userAgent, postID)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (vs *VoteService) CastVote(req VoteRequest) (*VoteResponse, error) {
	if err := vs.validateVoteRequest(req); err != nil {
		return nil, err
	}

	vs.voteMutex.Lock()
	defer vs.voteMutex.Unlock()

	var response *VoteResponse

	if vs.db == nil {
		return vs.castVoteWithoutTransaction(req)
	}

	err := vs.db.Transaction(func(tx *gorm.DB) error {
		txVoteRepo := vs.voteRepo.WithTx(tx)
		txPostRepo := vs.postRepo.WithTx(tx)

		post, err := txPostRepo.GetByID(req.PostID)
		if err != nil {
			if IsRecordNotFound(err) {
				return errors.New("post not found")
			}
			return fmt.Errorf("failed to get post: %w", err)
		}

		isUnauthenticated := req.UserID == 0

		if req.Type == database.VoteNone {

			var existingVote *database.Vote
			var err error

			if isUnauthenticated {
				voteHash := vs.GenerateVoteHash(req.IPAddress, req.UserAgent, req.PostID)
				existingVote, err = txVoteRepo.GetByVoteHash(voteHash)
			} else {
				existingVote, err = txVoteRepo.GetByUserAndPost(req.UserID, req.PostID)
			}

			if err != nil {
				if IsRecordNotFound(err) {
					response = vs.buildVoteResponse(post, database.VoteNone, isUnauthenticated)
					return nil
				}
				return fmt.Errorf("failed to get existing vote: %w", err)
			}

			if err := txVoteRepo.Delete(existingVote.ID); err != nil {
				return fmt.Errorf("failed to delete vote: %w", err)
			}
		} else {

			var vote *database.Vote

			if isUnauthenticated {
				voteHash := vs.GenerateVoteHash(req.IPAddress, req.UserAgent, req.PostID)
				vote = &database.Vote{
					PostID:    req.PostID,
					Type:      req.Type,
					VoteHash:  &voteHash,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
			} else {
				vote = &database.Vote{
					UserID:    &req.UserID,
					PostID:    req.PostID,
					Type:      req.Type,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
			}

			if err := txVoteRepo.CreateOrUpdate(vote); err != nil {
				return fmt.Errorf("failed to create or update vote: %w", err)
			}
		}

		if err := vs.updatePostVoteCountsWithTx(tx, req.PostID); err != nil {
			return fmt.Errorf("failed to update post vote counts: %w", err)
		}

		updatedPost, err := txPostRepo.GetByID(req.PostID)
		if err != nil {
			return fmt.Errorf("failed to get updated post: %w", err)
		}

		response = vs.buildVoteResponse(updatedPost, req.Type, isUnauthenticated)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return response, nil
}

func (vs *VoteService) castVoteWithoutTransaction(req VoteRequest) (*VoteResponse, error) {
	post, err := vs.postRepo.GetByID(req.PostID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("post not found")
		}
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	isUnauthenticated := req.UserID == 0

	if req.Type == database.VoteNone {

		var existingVote *database.Vote
		var err error

		if isUnauthenticated {
			voteHash := vs.GenerateVoteHash(req.IPAddress, req.UserAgent, req.PostID)
			existingVote, err = vs.voteRepo.GetByVoteHash(voteHash)
		} else {
			existingVote, err = vs.voteRepo.GetByUserAndPost(req.UserID, req.PostID)
		}

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return vs.buildVoteResponse(post, database.VoteNone, isUnauthenticated), nil
			}
			return nil, fmt.Errorf("failed to get existing vote: %w", err)
		}

		if err := vs.voteRepo.Delete(existingVote.ID); err != nil {
			return nil, fmt.Errorf("failed to delete vote: %w", err)
		}
	} else {

		var vote *database.Vote

		if isUnauthenticated {
			voteHash := vs.GenerateVoteHash(req.IPAddress, req.UserAgent, req.PostID)
			vote = &database.Vote{
				PostID:    req.PostID,
				Type:      req.Type,
				VoteHash:  &voteHash,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
		} else {
			vote = &database.Vote{
				UserID:    &req.UserID,
				PostID:    req.PostID,
				Type:      req.Type,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
		}

		if err := vs.voteRepo.CreateOrUpdate(vote); err != nil {
			return nil, fmt.Errorf("failed to create or update vote: %w", err)
		}
	}

	if err := vs.updatePostVoteCounts(req.PostID); err != nil {
		return nil, fmt.Errorf("failed to update post vote counts: %w", err)
	}

	updatedPost, err := vs.postRepo.GetByID(req.PostID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated post: %w", err)
	}

	return vs.buildVoteResponse(updatedPost, req.Type, isUnauthenticated), nil
}

func (vs *VoteService) GetUserVote(userID uint, postID uint, ipAddress, userAgent string) (*database.Vote, error) {

	if userID > 0 {
		vote, err := vs.voteRepo.GetByUserAndPost(userID, postID)
		if err == nil && vote != nil {
			return vote, nil
		}
	}

	voteHash := vs.GenerateVoteHash(ipAddress, userAgent, postID)
	vote, err := vs.voteRepo.GetByVoteHash(voteHash)
	if err == nil && vote != nil {
		return vote, nil
	}

	return nil, gorm.ErrRecordNotFound
}

func (vs *VoteService) GetPostVotes(postID uint) ([]database.Vote, error) {
	votes, err := vs.voteRepo.GetByPostID(postID)
	if err != nil {
		return nil, err
	}

	return votes, nil
}

func (vs *VoteService) DeleteVotesByPostID(postID uint) error {
	if vs.db != nil {
		if err := vs.db.Unscoped().Where("post_id = ?", postID).Delete(&database.Vote{}).Error; err != nil {
			return fmt.Errorf("failed to delete votes for post: %w", err)
		}
		return nil
	}

	votes, err := vs.voteRepo.GetByPostID(postID)
	if err != nil {
		return fmt.Errorf("failed to get votes: %w", err)
	}

	for _, vote := range votes {
		if err := vs.voteRepo.Delete(vote.ID); err != nil {
			return fmt.Errorf("failed to delete vote %d: %w", vote.ID, err)
		}
	}

	return nil
}

func (vs *VoteService) validateVoteRequest(req VoteRequest) error {
	if req.PostID == 0 {
		return errors.New("post ID is required")
	}
	if req.Type != database.VoteUp && req.Type != database.VoteDown && req.Type != database.VoteNone {
		return errors.New("invalid vote type")
	}
	return nil
}

func (vs *VoteService) buildVoteResponse(post *database.Post, voteType database.VoteType, isUnauthenticated bool) *VoteResponse {
	message := "Vote updated successfully"
	if voteType == database.VoteNone {
		message = "Vote removed successfully"
	}

	return &VoteResponse{
		PostID:            post.ID,
		Type:              voteType,
		UpVotes:           post.UpVotes,
		DownVotes:         post.DownVotes,
		Score:             post.Score,
		Message:           message,
		IsUnauthenticated: isUnauthenticated,
	}
}

func (vs *VoteService) updatePostVoteCounts(postID uint) error {
	if vs.db == nil {

		votes, err := vs.voteRepo.GetByPostID(postID)
		if err != nil {
			return fmt.Errorf("failed to get votes: %w", err)
		}

		upVotes, downVotes := vs.countVotes(votes)
		score := upVotes - downVotes

		post, err := vs.postRepo.GetByID(postID)
		if err != nil {
			return fmt.Errorf("failed to get post: %w", err)
		}

		post.UpVotes = upVotes
		post.DownVotes = downVotes
		post.Score = score

		return vs.postRepo.Update(post)
	}
	return vs.updatePostVoteCountsWithTx(vs.db, postID)
}

func (vs *VoteService) updatePostVoteCountsWithTx(tx *gorm.DB, postID uint) error {
	txVoteRepo := vs.voteRepo.WithTx(tx)
	txPostRepo := vs.postRepo.WithTx(tx)

	votes, err := txVoteRepo.GetByPostID(postID)
	if err != nil {
		return fmt.Errorf("failed to get votes: %w", err)
	}

	upVotes, downVotes := vs.countVotes(votes)
	score := upVotes - downVotes

	post, err := txPostRepo.GetByID(postID)
	if err != nil {
		return fmt.Errorf("failed to get post: %w", err)
	}

	post.UpVotes = upVotes
	post.DownVotes = downVotes
	post.Score = score

	return txPostRepo.Update(post)
}

func (vs *VoteService) countVotes(votes []database.Vote) (int, int) {
	upVotes := 0
	downVotes := 0

	for _, vote := range votes {
		switch vote.Type {
		case database.VoteUp:
			upVotes++
		case database.VoteDown:
			downVotes++
		}
	}

	return upVotes, downVotes
}

func (vs *VoteService) GetVoteStatistics() (int64, int64, error) {

	totalCount, err := vs.voteRepo.Count()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get vote count: %w", err)
	}

	return totalCount, 0, nil
}
