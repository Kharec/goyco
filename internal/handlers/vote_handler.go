package handlers

import (
	"net/http"

	"goyco/internal/database"
	"goyco/internal/services"

	"github.com/go-chi/chi/v5"
)

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// @tag.name votes
// @tag.description Voting system endpoints. All votes are handled through the same API with identical behavior.

// @tag.name posts
// @tag.description Post management endpoints with integrated vote statistics.

// @tag.name auth
// @tag.description Authentication and user management endpoints.

// @tag.name users
// @tag.description User management endpoints.

// @tag.name api
// @tag.description API information and system metrics.

type VoteHandler struct {
	voteService *services.VoteService
}

func NewVoteHandler(voteService *services.VoteService) *VoteHandler {
	return &VoteHandler{
		voteService: voteService,
	}
}

// @Description Vote request with type field. All votes are handled the same way.
type VoteRequest struct {
	Type string `json:"type" example:"up" enums:"up,down,none" description:"Vote type: 'up' for upvote, 'down' for downvote, 'none' to remove vote"`
}

type VoteResponse = CommonResponse

// @Summary Cast a vote on a post
// @Description Vote on a post (upvote, downvote, or remove vote). Authentication is required; the vote is performed on behalf of the current user.
// @Description
// @Description **Vote Types:**
// @Description - `up`: Upvote the post
// @Description - `down`: Downvote the post
// @Description - `none`: Remove existing vote
// @Description
// @Description **Response includes:**
// @Description - Updated post vote counts (up_votes, down_votes, score)
// @Description - Success message
// @Tags votes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Post ID"
// @Param request body VoteRequest true "Vote data (type: 'up', 'down', or 'none' to remove)"
// @Success 200 {object} VoteResponse "Vote cast successfully with updated post statistics"
// @Failure 401 {object} VoteResponse "Authentication required"
// @Failure 400 {object} VoteResponse "Invalid request data or vote type"
// @Failure 404 {object} VoteResponse "Post not found"
// @Failure 500 {object} VoteResponse "Internal server error"
// @Example 200 {"success": true, "message": "Vote cast successfully", "data": {"post_id": 1, "type": "up", "up_votes": 5, "down_votes": 2, "score": 3, "is_anonymous": false}}
// @Example 400 {"success": false, "error": "Invalid vote type. Must be 'up', 'down', or 'none'"}
// @Router /api/posts/{id}/vote [post]
func (h *VoteHandler) CastVote(w http.ResponseWriter, r *http.Request) {
	userID, ok := RequireAuth(w, r)
	if !ok {
		return
	}

	postID, ok := ParseUintParam(w, r, "id", "Post")
	if !ok {
		return
	}

	var req VoteRequest
	if !DecodeJSONRequest(w, r, &req) {
		return
	}

	var voteType database.VoteType
	switch req.Type {
	case "up":
		voteType = database.VoteUp
	case "down":
		voteType = database.VoteDown
	case "none":
		voteType = database.VoteNone
	default:
		SendErrorResponse(w, "Invalid vote type. Must be 'up', 'down', or 'none'", http.StatusBadRequest)
		return
	}

	ipAddress := GetClientIP(r)
	userAgent := r.UserAgent()

	serviceReq := services.VoteRequest{
		UserID:    userID,
		PostID:    postID,
		Type:      voteType,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}

	response, err := h.voteService.CastVote(serviceReq)
	if err != nil {
		if err.Error() == "post not found" {
			SendErrorResponse(w, err.Error(), http.StatusNotFound)
			return
		}
		if err.Error() == "post ID is required" || err.Error() == "invalid vote type" {
			SendErrorResponse(w, err.Error(), http.StatusBadRequest)
			return
		}
		SendErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	SendSuccessResponse(w, "Vote cast successfully", response)
}

// @Summary Remove a vote
// @Description Remove a vote from a post for the authenticated user. This is equivalent to casting a vote with type 'none'.
// @Tags votes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Post ID"
// @Success 200 {object} VoteResponse "Vote removed successfully with updated post statistics"
// @Failure 401 {object} VoteResponse "Authentication required"
// @Failure 400 {object} VoteResponse "Invalid post ID"
// @Failure 404 {object} VoteResponse "Post not found"
// @Failure 500 {object} VoteResponse "Internal server error"
// @Router /api/posts/{id}/vote [delete]
func (h *VoteHandler) RemoveVote(w http.ResponseWriter, r *http.Request) {
	userID, ok := RequireAuth(w, r)
	if !ok {
		return
	}

	postID, ok := ParseUintParam(w, r, "id", "Post")
	if !ok {
		return
	}

	ipAddress := GetClientIP(r)
	userAgent := r.UserAgent()

	serviceReq := services.VoteRequest{
		UserID:    userID,
		PostID:    postID,
		Type:      database.VoteNone,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}

	response, err := h.voteService.CastVote(serviceReq)
	if err != nil {
		if err.Error() == "post not found" {
			SendErrorResponse(w, err.Error(), http.StatusNotFound)
			return
		}
		if err.Error() == "post ID is required" {
			SendErrorResponse(w, err.Error(), http.StatusBadRequest)
			return
		}
		SendErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	SendSuccessResponse(w, "Vote removed successfully", response)
}

// @Summary Get current user's vote
// @Description Retrieve the current user's vote for a specific post. Requires authentication and returns the vote type if it exists.
// @Description
// @Description **Response:**
// @Description - If vote exists: Returns vote details with contextual metadata (including `is_anonymous`)
// @Description - If no vote: Returns success with null vote data and metadata
// @Tags votes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Post ID"
// @Success 200 {object} VoteResponse "Vote retrieved successfully"
// @Success 200 {object} VoteResponse "No vote found for this user/post combination"
// @Failure 401 {object} VoteResponse "Authentication required"
// @Failure 400 {object} VoteResponse "Invalid post ID"
// @Failure 500 {object} VoteResponse "Internal server error"
// @Example 200 {"success": true, "message": "Vote retrieved successfully", "data": {"has_vote": true, "vote": {"type": "up", "user_id": 123}, "is_anonymous": false}}
// @Example 200 {"success": true, "message": "No vote found", "data": {"has_vote": false, "vote": null, "is_anonymous": false}}
// @Router /api/posts/{id}/vote [get]
func (h *VoteHandler) GetUserVote(w http.ResponseWriter, r *http.Request) {
	userID, ok := RequireAuth(w, r)
	if !ok {
		return
	}

	postID, ok := ParseUintParam(w, r, "id", "Post")
	if !ok {
		return
	}

	ipAddress := GetClientIP(r)
	userAgent := r.UserAgent()

	vote, err := h.voteService.GetUserVote(userID, postID, ipAddress, userAgent)
	if err != nil {
		if err.Error() == "record not found" {
			SendSuccessResponse(w, "No vote found", map[string]any{
				"has_vote":     false,
				"vote":         nil,
				"is_anonymous": false,
			})
			return
		}
		SendErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	SendSuccessResponse(w, "Vote retrieved successfully", map[string]any{
		"has_vote":     true,
		"vote":         vote,
		"is_anonymous": false,
	})
}

// @Summary Get post votes
// @Description Retrieve all votes for a specific post. Returns all votes in a single format.
// @Description
// @Description **Authentication Required:** Yes (Bearer token)
// @Description
// @Description **Response includes:**
// @Description - Array of all votes
// @Description - Total vote count
// @Description - Each vote includes type and unauthenticated status
// @Tags votes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Post ID"
// @Success 200 {object} VoteResponse "Votes retrieved successfully with count"
// @Failure 400 {object} VoteResponse "Invalid post ID"
// @Failure 401 {object} VoteResponse "Authentication required"
// @Failure 500 {object} VoteResponse "Internal server error"
// @Example 200 {"success": true, "message": "Votes retrieved successfully", "data": {"votes": [{"type": "up", "user_id": 123}, {"type": "down", "vote_hash": "abc123"}], "count": 2}}
// @Router /api/posts/{id}/votes [get]
func (h *VoteHandler) GetPostVotes(w http.ResponseWriter, r *http.Request) {
	postID, ok := ParseUintParam(w, r, "id", "Post")
	if !ok {
		return
	}

	votes, err := h.voteService.GetPostVotes(postID)
	if err != nil {
		SendErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	allVotes := make([]any, 0, len(votes))
	for _, vote := range votes {
		allVotes = append(allVotes, vote)
	}

	SendSuccessResponse(w, "Votes retrieved successfully", map[string]any{
		"votes": allVotes,
		"count": len(allVotes),
	})
}

func (h *VoteHandler) MountRoutes(r chi.Router, config RouteModuleConfig) {
	protected := r
	if config.AuthMiddleware != nil {
		protected = r.With(config.AuthMiddleware)
	}
	if config.GeneralRateLimit != nil {
		protected = config.GeneralRateLimit(protected)
	}

	protected.Post("/posts/{id}/vote", h.CastVote)
	protected.Delete("/posts/{id}/vote", h.RemoveVote)
	protected.Get("/posts/{id}/vote", h.GetUserVote)
	protected.Get("/posts/{id}/votes", h.GetPostVotes)
}
