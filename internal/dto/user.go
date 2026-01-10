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

type SanitizedUserListDTO struct {
	Users  []SanitizedUserDTO `json:"users"`
	Count  int                `json:"count"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

func ToUserListDTO(users []database.User, limit, offset int) UserListDTO {
	userDTOs := ToUserDTOs(users)
	return UserListDTO{
		Users:  userDTOs,
		Count:  len(userDTOs),
		Limit:  limit,
		Offset: offset,
	}
}

func ToSanitizedUserListDTO(users []database.User, limit, offset int) SanitizedUserListDTO {
	userDTOs := ToSanitizedUserDTOs(users)
	return SanitizedUserListDTO{
		Users:  userDTOs,
		Count:  len(userDTOs),
		Limit:  limit,
		Offset: offset,
	}
}

type RegistrationResponseDTO struct {
	User             UserDTO `json:"user"`
	VerificationSent bool    `json:"verification_sent"`
}

func ToRegistrationResponseDTO(user *database.User, verificationSent bool) RegistrationResponseDTO {
	return RegistrationResponseDTO{
		User:             ToUserDTO(user),
		VerificationSent: verificationSent,
	}
}

type AccountDeletionResponseDTO struct {
	PostsDeleted bool `json:"posts_deleted"`
}
