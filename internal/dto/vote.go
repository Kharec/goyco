package dto

import (
	"time"

	"goyco/internal/database"
)

type VoteRequest struct {
	Type string `json:"type" validate:"required,oneof=up down none"`
}

type VoteDTO struct {
	ID        uint      `json:"id"`
	UserID    *uint     `json:"user_id,omitempty"`
	PostID    uint      `json:"post_id"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func ToVoteDTO(vote *database.Vote) VoteDTO {
	if vote == nil {
		return VoteDTO{}
	}

	return VoteDTO{
		ID:        vote.ID,
		UserID:    vote.UserID,
		PostID:    vote.PostID,
		Type:      string(vote.Type),
		CreatedAt: vote.CreatedAt,
		UpdatedAt: vote.UpdatedAt,
	}
}

func ToVoteDTOs(votes []database.Vote) []VoteDTO {
	dtos := make([]VoteDTO, len(votes))
	for i := range votes {
		dtos[i] = ToVoteDTO(&votes[i])
	}
	return dtos
}
