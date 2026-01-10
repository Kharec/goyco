package dto

import (
	"time"

	"goyco/internal/database"
)

type PostDTO struct {
	ID          uint      `json:"id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Content     string    `json:"content,omitempty"`
	AuthorID    *uint     `json:"author_id,omitempty"`
	AuthorName  string    `json:"author_name,omitempty"`
	Author      *UserDTO  `json:"author,omitempty"`
	UpVotes     int       `json:"up_votes"`
	DownVotes   int       `json:"down_votes"`
	Score       int       `json:"score"`
	CurrentVote string    `json:"current_vote,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PostListDTO struct {
	Posts  []PostDTO `json:"posts"`
	Count  int       `json:"count"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
}

type SearchPostListDTO struct {
	Posts  []PostDTO `json:"posts"`
	Count  int       `json:"count"`
	Query  string    `json:"query"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
}

func ToPostDTO(post *database.Post) PostDTO {
	if post == nil {
		return PostDTO{}
	}

	dto := PostDTO{
		ID:         post.ID,
		Title:      post.Title,
		URL:        post.URL,
		Content:    post.Content,
		AuthorID:   post.AuthorID,
		AuthorName: post.AuthorName,
		UpVotes:    post.UpVotes,
		DownVotes:  post.DownVotes,
		Score:      post.Score,
		CreatedAt:  post.CreatedAt,
		UpdatedAt:  post.UpdatedAt,
	}

	if post.CurrentVote != "" {
		dto.CurrentVote = string(post.CurrentVote)
	}

	if post.Author.ID != 0 {
		authorDTO := ToUserDTO(&post.Author)
		dto.Author = &authorDTO
	}

	return dto
}

func ToPostDTOs(posts []database.Post) []PostDTO {
	dtos := make([]PostDTO, len(posts))
	for i := range posts {
		dtos[i] = ToPostDTO(&posts[i])
	}
	return dtos
}

func ToPostListDTO(posts []database.Post, limit, offset int) PostListDTO {
	postDTOs := ToPostDTOs(posts)
	return PostListDTO{
		Posts:  postDTOs,
		Count:  len(postDTOs),
		Limit:  limit,
		Offset: offset,
	}
}

func ToSearchPostListDTO(posts []database.Post, query string, limit, offset int) SearchPostListDTO {
	postDTOs := ToPostDTOs(posts)
	return SearchPostListDTO{
		Posts:  postDTOs,
		Count:  len(postDTOs),
		Query:  query,
		Limit:  limit,
		Offset: offset,
	}
}

type TitleResponseDTO struct {
	Title string `json:"title"`
}
