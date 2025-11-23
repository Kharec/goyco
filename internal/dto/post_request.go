package dto

type CreatePostRequest struct {
	Title   string `json:"title" validate:"omitempty,min=3,max=200"`
	URL     string `json:"url" validate:"required,url,max=2048"`
	Content string `json:"content" validate:"omitempty,max=10000"`
}

type UpdatePostRequest struct {
	Title   string `json:"title" validate:"required,min=3,max=200"`
	Content string `json:"content" validate:"omitempty,max=10000"`
}
