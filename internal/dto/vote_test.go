package dto

import (
	"testing"
	"time"

	"goyco/internal/database"
)

func TestToVoteDTO(t *testing.T) {
	t.Run("nil vote", func(t *testing.T) {
		dto := ToVoteDTO(nil)
		if dto.ID != 0 {
			t.Errorf("Expected zero value for nil vote, got ID %d", dto.ID)
		}
	})

	t.Run("vote with user ID", func(t *testing.T) {
		userID := uint(42)
		vote := &database.Vote{
			ID:        1,
			UserID:    &userID,
			PostID:    10,
			Type:      database.VoteUp,
			CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		}

		dto := ToVoteDTO(vote)

		if dto.ID != vote.ID {
			t.Errorf("Expected ID %d, got %d", vote.ID, dto.ID)
		}
		if dto.UserID == nil || *dto.UserID != userID {
			t.Errorf("Expected UserID %d, got %v", userID, dto.UserID)
		}
		if dto.PostID != vote.PostID {
			t.Errorf("Expected PostID %d, got %d", vote.PostID, dto.PostID)
		}
		if dto.Type != string(vote.Type) {
			t.Errorf("Expected Type %q, got %q", vote.Type, dto.Type)
		}
		if !dto.CreatedAt.Equal(vote.CreatedAt) {
			t.Errorf("Expected CreatedAt %v, got %v", vote.CreatedAt, dto.CreatedAt)
		}
		if !dto.UpdatedAt.Equal(vote.UpdatedAt) {
			t.Errorf("Expected UpdatedAt %v, got %v", vote.UpdatedAt, dto.UpdatedAt)
		}
	})

	t.Run("vote without user ID", func(t *testing.T) {
		vote := &database.Vote{
			ID:        2,
			UserID:    nil,
			PostID:    20,
			Type:      database.VoteDown,
			CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		}

		dto := ToVoteDTO(vote)

		if dto.UserID != nil {
			t.Errorf("Expected UserID to be nil, got %v", dto.UserID)
		}
		if dto.Type != string(database.VoteDown) {
			t.Errorf("Expected Type %q, got %q", database.VoteDown, dto.Type)
		}
	})

	t.Run("all vote types", func(t *testing.T) {
		tests := []struct {
			name     string
			voteType database.VoteType
			expected string
		}{
			{"VoteUp", database.VoteUp, "up"},
			{"VoteDown", database.VoteDown, "down"},
			{"VoteNone", database.VoteNone, "none"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				vote := &database.Vote{
					ID:   1,
					Type: tt.voteType,
				}

				dto := ToVoteDTO(vote)

				if dto.Type != tt.expected {
					t.Errorf("Expected Type %q, got %q", tt.expected, dto.Type)
				}
			})
		}
	})
}

func TestToVoteDTOs(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		votes := []database.Vote{}
		dtos := ToVoteDTOs(votes)
		if len(dtos) != 0 {
			t.Errorf("Expected empty slice, got %d items", len(dtos))
		}
	})

	t.Run("multiple votes", func(t *testing.T) {
		userID1 := uint(1)
		votes := []database.Vote{
			{
				ID:     1,
				UserID: &userID1,
				PostID: 10,
				Type:   database.VoteUp,
			},
			{
				ID:     2,
				UserID: nil,
				PostID: 10,
				Type:   database.VoteDown,
			},
			{
				ID:     3,
				UserID: &userID1,
				PostID: 20,
				Type:   database.VoteUp,
			},
		}

		dtos := ToVoteDTOs(votes)

		if len(dtos) != len(votes) {
			t.Fatalf("Expected %d DTOs, got %d", len(votes), len(dtos))
		}

		for i := range votes {
			if dtos[i].ID != votes[i].ID {
				t.Errorf("Vote %d: Expected ID %d, got %d", i, votes[i].ID, dtos[i].ID)
			}
			if dtos[i].PostID != votes[i].PostID {
				t.Errorf("Vote %d: Expected PostID %d, got %d", i, votes[i].PostID, dtos[i].PostID)
			}
			if dtos[i].Type != string(votes[i].Type) {
				t.Errorf("Vote %d: Expected Type %q, got %q", i, votes[i].Type, dtos[i].Type)
			}
		}
	})
}
