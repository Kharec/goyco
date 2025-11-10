package dto

import (
	"testing"
	"time"

	"goyco/internal/database"
)

func TestToUserDTO(t *testing.T) {
	t.Run("nil user", func(t *testing.T) {
		dto := ToUserDTO(nil)
		if dto.ID != 0 {
			t.Errorf("Expected zero value for nil user, got ID %d", dto.ID)
		}
	})

	t.Run("valid user", func(t *testing.T) {
		verifiedAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		user := &database.User{
			ID:              42,
			Username:        "testuser",
			Email:           "test@example.com",
			EmailVerified:   true,
			EmailVerifiedAt: &verifiedAt,
			CreatedAt:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:       time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		}

		dto := ToUserDTO(user)

		if dto.ID != user.ID {
			t.Errorf("Expected ID %d, got %d", user.ID, dto.ID)
		}
		if dto.Username != user.Username {
			t.Errorf("Expected Username %q, got %q", user.Username, dto.Username)
		}
		if dto.Email != user.Email {
			t.Errorf("Expected Email %q, got %q", user.Email, dto.Email)
		}
		if dto.EmailVerified != user.EmailVerified {
			t.Errorf("Expected EmailVerified %v, got %v", user.EmailVerified, dto.EmailVerified)
		}
		if dto.EmailVerifiedAt == nil || !dto.EmailVerifiedAt.Equal(*user.EmailVerifiedAt) {
			t.Errorf("Expected EmailVerifiedAt %v, got %v", user.EmailVerifiedAt, dto.EmailVerifiedAt)
		}
		if !dto.CreatedAt.Equal(user.CreatedAt) {
			t.Errorf("Expected CreatedAt %v, got %v", user.CreatedAt, dto.CreatedAt)
		}
		if !dto.UpdatedAt.Equal(user.UpdatedAt) {
			t.Errorf("Expected UpdatedAt %v, got %v", user.UpdatedAt, dto.UpdatedAt)
		}
	})

	t.Run("user without email verified at", func(t *testing.T) {
		user := &database.User{
			ID:              1,
			Username:        "testuser",
			Email:           "test@example.com",
			EmailVerified:   false,
			EmailVerifiedAt: nil,
			CreatedAt:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:       time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		}

		dto := ToUserDTO(user)

		if dto.EmailVerifiedAt != nil {
			t.Error("Expected EmailVerifiedAt to be nil")
		}
	})
}

func TestToUserDTOs(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		users := []database.User{}
		dtos := ToUserDTOs(users)
		if len(dtos) != 0 {
			t.Errorf("Expected empty slice, got %d items", len(dtos))
		}
	})

	t.Run("multiple users", func(t *testing.T) {
		users := []database.User{
			{
				ID:       1,
				Username: "user1",
				Email:    "user1@example.com",
			},
			{
				ID:       2,
				Username: "user2",
				Email:    "user2@example.com",
			},
		}

		dtos := ToUserDTOs(users)

		if len(dtos) != len(users) {
			t.Fatalf("Expected %d DTOs, got %d", len(users), len(dtos))
		}

		for i := range users {
			if dtos[i].ID != users[i].ID {
				t.Errorf("User %d: Expected ID %d, got %d", i, users[i].ID, dtos[i].ID)
			}
			if dtos[i].Username != users[i].Username {
				t.Errorf("User %d: Expected Username %q, got %q", i, users[i].Username, dtos[i].Username)
			}
		}
	})
}

func TestToSanitizedUserDTO(t *testing.T) {
	t.Run("nil user", func(t *testing.T) {
		dto := ToSanitizedUserDTO(nil)
		if dto.ID != 0 {
			t.Errorf("Expected zero value for nil user, got ID %d", dto.ID)
		}
	})

	t.Run("valid user", func(t *testing.T) {
		user := &database.User{
			ID:            42,
			Username:      "testuser",
			Email:         "test@example.com",
			EmailVerified: true,
			CreatedAt:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		}

		dto := ToSanitizedUserDTO(user)

		if dto.ID != user.ID {
			t.Errorf("Expected ID %d, got %d", user.ID, dto.ID)
		}
		if dto.Username != user.Username {
			t.Errorf("Expected Username %q, got %q", user.Username, dto.Username)
		}
		if !dto.CreatedAt.Equal(user.CreatedAt) {
			t.Errorf("Expected CreatedAt %v, got %v", user.CreatedAt, dto.CreatedAt)
		}
		if !dto.UpdatedAt.Equal(user.UpdatedAt) {
			t.Errorf("Expected UpdatedAt %v, got %v", user.UpdatedAt, dto.UpdatedAt)
		}
	})
}

func TestToSanitizedUserDTOs(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		users := []database.User{}
		dtos := ToSanitizedUserDTOs(users)
		if len(dtos) != 0 {
			t.Errorf("Expected empty slice, got %d items", len(dtos))
		}
	})

	t.Run("multiple users", func(t *testing.T) {
		users := []database.User{
			{
				ID:       1,
				Username: "user1",
				Email:    "user1@example.com",
			},
			{
				ID:       2,
				Username: "user2",
				Email:    "user2@example.com",
			},
		}

		dtos := ToSanitizedUserDTOs(users)

		if len(dtos) != len(users) {
			t.Fatalf("Expected %d DTOs, got %d", len(users), len(dtos))
		}

		for i := range users {
			if dtos[i].ID != users[i].ID {
				t.Errorf("User %d: Expected ID %d, got %d", i, users[i].ID, dtos[i].ID)
			}
			if dtos[i].Username != users[i].Username {
				t.Errorf("User %d: Expected Username %q, got %q", i, users[i].Username, dtos[i].Username)
			}
		}
	})
}
