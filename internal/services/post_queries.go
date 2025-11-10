package services

import (
	"goyco/internal/database"
	"goyco/internal/repositories"
)

type PostQueries struct {
	postRepo    repositories.PostRepository
	voteService *VoteService
}

func NewPostQueries(postRepo repositories.PostRepository, voteService *VoteService) *PostQueries {
	return &PostQueries{
		postRepo:    postRepo,
		voteService: voteService,
	}
}

type QueryOptions struct {
	Limit  int
	Offset int
	Sort   string
}

type VoteContext struct {
	UserID    uint
	IPAddress string
	UserAgent string
}

func (pq *PostQueries) enrichPostsWithVotes(posts []database.Post, ctx VoteContext) []database.Post {
	if pq.voteService == nil {
		return posts
	}

	enriched := make([]database.Post, len(posts))
	for i := range posts {
		enriched[i] = posts[i]
		vote, err := pq.voteService.GetUserVote(ctx.UserID, posts[i].ID, ctx.IPAddress, ctx.UserAgent)
		if err == nil && vote != nil {
			enriched[i].CurrentVote = vote.Type
		}
	}

	return enriched
}

func (pq *PostQueries) enrichPostWithVote(post *database.Post, ctx VoteContext) *database.Post {
	if pq.voteService == nil || post == nil {
		return post
	}

	vote, err := pq.voteService.GetUserVote(ctx.UserID, post.ID, ctx.IPAddress, ctx.UserAgent)
	if err == nil && vote != nil {
		post.CurrentVote = vote.Type
	}

	return post
}

func (pq *PostQueries) GetAll(opts QueryOptions, ctx VoteContext) ([]database.Post, error) {
	posts, err := pq.postRepo.GetAll(opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}

	return pq.enrichPostsWithVotes(posts, ctx), nil
}

func (pq *PostQueries) GetTop(limit int, ctx VoteContext) ([]database.Post, error) {
	posts, err := pq.postRepo.GetTopPosts(limit)
	if err != nil {
		return nil, err
	}

	return pq.enrichPostsWithVotes(posts, ctx), nil
}

func (pq *PostQueries) GetNewest(limit int, ctx VoteContext) ([]database.Post, error) {
	posts, err := pq.postRepo.GetNewestPosts(limit)
	if err != nil {
		return nil, err
	}

	return pq.enrichPostsWithVotes(posts, ctx), nil
}

func (pq *PostQueries) GetBySort(sort string, limit int, ctx VoteContext) ([]database.Post, error) {
	switch sort {
	case "new", "newest", "latest":
		return pq.GetNewest(limit, ctx)
	default:
		return pq.GetTop(limit, ctx)
	}
}

func (pq *PostQueries) GetSearch(query string, opts QueryOptions, ctx VoteContext) ([]database.Post, error) {
	posts, err := pq.postRepo.Search(query, opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}

	return pq.enrichPostsWithVotes(posts, ctx), nil
}

func (pq *PostQueries) GetByID(postID uint, ctx VoteContext) (*database.Post, error) {
	post, err := pq.postRepo.GetByID(postID)
	if err != nil {
		return nil, err
	}

	return pq.enrichPostWithVote(post, ctx), nil
}

func (pq *PostQueries) GetByUserID(userID uint, opts QueryOptions, ctx VoteContext) ([]database.Post, error) {
	posts, err := pq.postRepo.GetByUserID(userID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}

	return pq.enrichPostsWithVotes(posts, ctx), nil
}
