package dto

type LoginRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=8,max=128"`
}

type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,min=8,max=128"`
}

type ResendVerificationRequest struct {
	Email string `json:"email" validate:"required,email,max=254"`
}

type ForgotPasswordRequest struct {
	UsernameOrEmail string `json:"username_or_email" validate:"required"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=128"`
}

type UpdateEmailRequest struct {
	Email string `json:"email" validate:"required,email,max=254"`
}

type UpdateUsernameRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=128"`
}

type ConfirmAccountDeletionRequest struct {
	Token       string `json:"token" validate:"required"`
	DeletePosts bool   `json:"delete_posts"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" example:"f94d4ddc7d9b4fcb9d3a2c44c400b780c3e1f1a5c2b7d4e6a0b1c2d3e4f5a6b7" validate:"required"`
}

type RevokeTokenRequest struct {
	RefreshToken string `json:"refresh_token" example:"f94d4ddc7d9b4fcb9d3a2c44c400b780c3e1f1a5c2b7d4e6a0b1c2d3e4f5a6b7" validate:"required"`
}
