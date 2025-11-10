package dto

import (
	"time"

	"goyco/internal/database"
)

type UserDTO struct {
	ID              uint       `json:"id"`
	Username        string     `json:"username"`
	Email           string     `json:"email,omitempty"`
	EmailVerified   bool       `json:"email_verified,omitempty"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type UserListDTO struct {
	Users  []UserDTO `json:"users"`
	Count  int       `json:"count"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
}

func ToUserDTO(user *database.User) UserDTO {
	if user == nil {
		return UserDTO{}
	}

	return UserDTO{
		ID:              user.ID,
		Username:        user.Username,
		Email:           user.Email,
		EmailVerified:   user.EmailVerified,
		EmailVerifiedAt: user.EmailVerifiedAt,
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
	}
}

func ToUserDTOs(users []database.User) []UserDTO {
	dtos := make([]UserDTO, len(users))
	for i := range users {
		dtos[i] = ToUserDTO(&users[i])
	}
	return dtos
}

type SanitizedUserDTO struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func ToSanitizedUserDTO(user *database.User) SanitizedUserDTO {
	if user == nil {
		return SanitizedUserDTO{}
	}

	return SanitizedUserDTO{
		ID:        user.ID,
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func ToSanitizedUserDTOs(users []database.User) []SanitizedUserDTO {
	dtos := make([]SanitizedUserDTO, len(users))
	for i := range users {
		dtos[i] = ToSanitizedUserDTO(&users[i])
	}
	return dtos
}
