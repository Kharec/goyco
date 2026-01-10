package dto

import (
	"goyco/internal/services"
)

type AuthResponseDTO struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	User         UserDTO `json:"user"`
}

func ToAuthResponseDTO(result *services.AuthResult) AuthResponseDTO {
	if result == nil {
		return AuthResponseDTO{}
	}

	return AuthResponseDTO{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		User:         ToUserDTO(result.User),
	}
}
