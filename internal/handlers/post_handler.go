package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"goyco/internal/database"
	"goyco/internal/dto"
	"goyco/internal/repositories"
	"goyco/internal/security"
	"goyco/internal/services"
	"goyco/internal/validation"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgconn"
)

type PostHandler struct {
	postRepo     repositories.PostRepository
	titleFetcher services.TitleFetcher
	voteService  *services.VoteService
	postQueries  *services.PostQueries
}

func NewPostHandler(postRepo repositories.PostRepository, titleFetcher services.TitleFetcher, voteService *services.VoteService) *PostHandler {
	return &PostHandler{
		postRepo:     postRepo,
		titleFetcher: titleFetcher,
		voteService:  voteService,
		postQueries:  services.NewPostQueries(postRepo, voteService),
	}
}

type PostResponse = CommonResponse

type UpdatePostRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// @Summary Get posts
// @Description Get a list of posts with pagination. Posts include vote statistics (up_votes, down_votes, score) and current user's vote status.
// @Tags posts
// @Accept json
// @Produce json
// @Param limit query int false "Number of posts to return" default(20)
// @Param offset query int false "Number of posts to skip" default(0)
// @Success 200 {object} PostResponse "Posts retrieved successfully with vote statistics"
// @Failure 400 {object} PostResponse "Invalid pagination parameters"
// @Failure 500 {object} PostResponse "Internal server error"
// @Router /posts [get]
func (h *PostHandler) GetPosts(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	opts := services.QueryOptions{
		Limit:  limit,
		Offset: offset,
	}

	ctx := NewVoteContext(r)

	posts, err := h.postQueries.GetAll(opts, ctx)
	if err != nil {
		SendErrorResponse(w, "Failed to fetch posts", http.StatusInternalServerError)
		return
	}

	postDTOs := dto.ToPostDTOs(posts)
	SendSuccessResponse(w, "Posts retrieved successfully", map[string]any{
		"posts":  postDTOs,
		"count":  len(postDTOs),
		"limit":  limit,
		"offset": offset,
	})
}

// @Summary Get a single post
// @Description Get a post by ID with vote statistics and current user's vote status
// @Tags posts
// @Accept json
// @Produce json
// @Param id path int true "Post ID"
// @Success 200 {object} PostResponse "Post retrieved successfully with vote statistics"
// @Failure 400 {object} PostResponse "Invalid post ID"
// @Failure 404 {object} PostResponse "Post not found"
// @Failure 500 {object} PostResponse "Internal server error"
// @Router /posts/{id} [get]
func (h *PostHandler) GetPost(w http.ResponseWriter, r *http.Request) {
	postID, ok := ParseUintParam(w, r, "id", "Post")
	if !ok {
		return
	}

	ctx := NewVoteContext(r)

	post, err := h.postQueries.GetByID(postID, ctx)
	if !HandleRepoError(w, err, "Post") {
		return
	}

	postDTO := dto.ToPostDTO(post)
	SendSuccessResponse(w, "Post retrieved successfully", postDTO)
}

// @Summary Create a new post
// @Description Create a new post with URL and optional title
// @Tags posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreatePostRequest true "Post data"
// @Success 201 {object} PostResponse
// @Failure 400 {object} PostResponse "Invalid request data or validation failed"
// @Failure 401 {object} PostResponse "Authentication required"
// @Failure 409 {object} PostResponse "URL already submitted"
// @Failure 502 {object} PostResponse "Failed to fetch title from URL"
// @Failure 500 {object} PostResponse "Internal server error"
// @Router /posts [post]
func (h *PostHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	}

	if !DecodeJSONRequest(w, r, &req) {
		return
	}

	req.Title = security.SanitizeInput(req.Title)
	req.URL = security.SanitizeURL(req.URL)
	req.Content = security.SanitizePostContent(req.Content)

	if req.URL == "" {
		SendErrorResponse(w, "URL is required", http.StatusBadRequest)
		return
	}

	if len(req.Title) > 200 {
		SendErrorResponse(w, "Title must be no more than 200 characters", http.StatusBadRequest)
		return
	}

	if len(req.Content) > 10000 {
		SendErrorResponse(w, "Content must be no more than 10,000 characters", http.StatusBadRequest)
		return
	}

	userID, ok := RequireAuth(w, r)
	if !ok {
		return
	}

	title := req.Title

	if title == "" && h.titleFetcher != nil {
		titleCtx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
		defer cancel()

		fetchedTitle, err := h.titleFetcher.FetchTitle(titleCtx, req.URL)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrUnsupportedScheme):
				SendErrorResponse(w, "Only HTTP and HTTPS URLs are supported", http.StatusBadRequest)
			case errors.Is(err, services.ErrTitleNotFound):
				SendErrorResponse(w, "Title could not be extracted from the provided URL", http.StatusBadRequest)
			default:
				SendErrorResponse(w, "Failed to fetch title from URL", http.StatusBadGateway)
			}
			return
		}

		title = fetchedTitle
	}

	if title == "" {
		SendErrorResponse(w, "Title is required", http.StatusBadRequest)
		return
	}

	if len(title) < 3 {
		SendErrorResponse(w, "Title must be at least 3 characters", http.StatusBadRequest)
		return
	}

	post := &database.Post{
		Title:    title,
		URL:      req.URL,
		Content:  req.Content,
		AuthorID: &userID,
	}

	if err := h.postRepo.Create(post); err != nil {
		if errMsg, status := translatePostCreateError(err); status != 0 {
			SendErrorResponse(w, errMsg, status)
			return
		}

		SendErrorResponse(w, "Failed to create post", http.StatusInternalServerError)
		return
	}

	postDTO := dto.ToPostDTO(post)
	SendCreatedResponse(w, "Post created successfully", postDTO)
}

// @Summary Search posts
// @Description Search posts by title or content keywords. Results include vote statistics and current user's vote status.
// @Tags posts
// @Accept json
// @Produce json
// @Param q query string false "Search term"
// @Param limit query int false "Number of posts to return" default(20)
// @Param offset query int false "Number of posts to skip" default(0)
// @Success 200 {object} PostResponse "Search results with vote statistics"
// @Failure 400 {object} PostResponse "Invalid search parameters"
// @Failure 500 {object} PostResponse "Internal server error"
// @Router /posts/search [get]
func (h *PostHandler) SearchPosts(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit, offset := parsePagination(r)

	opts := services.QueryOptions{
		Limit:  limit,
		Offset: offset,
	}

	ctx := NewVoteContext(r)

	posts, err := h.postQueries.GetSearch(query, opts, ctx)
	if err != nil {
		if searchErr, ok := err.(*repositories.SearchError); ok {
			SendErrorResponse(w, "Invalid search query: "+searchErr.Message, http.StatusBadRequest)
			return
		}
		SendErrorResponse(w, "Failed to search posts", http.StatusInternalServerError)
		return
	}

	postDTOs := dto.ToPostDTOs(posts)
	SendSuccessResponse(w, "Search results retrieved successfully", map[string]any{
		"posts":  postDTOs,
		"count":  len(postDTOs),
		"query":  query,
		"limit":  limit,
		"offset": offset,
	})
}

// @Summary Update a post
// @Description Update the title and content of a post owned by the authenticated user
// @Tags posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Post ID"
// @Param request body UpdatePostRequest true "Post update data"
// @Success 200 {object} PostResponse "Post updated successfully"
// @Failure 400 {object} PostResponse "Invalid request data or validation failed"
// @Failure 401 {object} PostResponse "Authentication required"
// @Failure 403 {object} PostResponse "Not authorized to update this post"
// @Failure 404 {object} PostResponse "Post not found"
// @Failure 500 {object} PostResponse "Internal server error"
// @Router /posts/{id} [put]
func (h *PostHandler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	userID, ok := RequireAuth(w, r)
	if !ok {
		return
	}

	postID, ok := ParseUintParam(w, r, "id", "Post")
	if !ok {
		return
	}

	post, err := h.postRepo.GetByID(postID)
	if !HandleRepoError(w, err, "Post") {
		return
	}

	if post.AuthorID == nil || *post.AuthorID != userID {
		SendErrorResponse(w, "You can only edit your own posts", http.StatusForbidden)
		return
	}

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	if !DecodeJSONRequest(w, r, &req) {
		return
	}

	req.Title = security.SanitizeInput(req.Title)
	req.Content = security.SanitizePostContent(req.Content)

	if len(req.Title) > 200 {
		SendErrorResponse(w, "Title must be no more than 200 characters", http.StatusBadRequest)
		return
	}

	if len(req.Content) > 10000 {
		SendErrorResponse(w, "Content must be no more than 10,000 characters", http.StatusBadRequest)
		return
	}

	if err := validation.ValidateTitle(req.Title); err != nil {
		SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validation.ValidateContent(req.Content); err != nil {
		SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	post.Title = req.Title
	post.Content = req.Content

	if err := h.postRepo.Update(post); err != nil {
		SendErrorResponse(w, "Failed to update post", http.StatusInternalServerError)
		return
	}

	postDTO := dto.ToPostDTO(post)
	SendSuccessResponse(w, "Post updated successfully", postDTO)
}

// @Summary Delete a post
// @Description Delete a post owned by the authenticated user
// @Tags posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Post ID"
// @Success 200 {object} PostResponse "Post deleted successfully"
// @Failure 400 {object} PostResponse "Invalid post ID"
// @Failure 401 {object} PostResponse "Authentication required"
// @Failure 403 {object} PostResponse "Not authorized to delete this post"
// @Failure 404 {object} PostResponse "Post not found"
// @Failure 500 {object} PostResponse "Internal server error"
// @Router /posts/{id} [delete]
func (h *PostHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
	userID, ok := RequireAuth(w, r)
	if !ok {
		return
	}

	postID, ok := ParseUintParam(w, r, "id", "Post")
	if !ok {
		return
	}

	post, err := h.postRepo.GetByID(postID)
	if !HandleRepoError(w, err, "Post") {
		return
	}

	if post.AuthorID == nil || *post.AuthorID != userID {
		SendErrorResponse(w, "You can only delete your own posts", http.StatusForbidden)
		return
	}

	if err := h.voteService.DeleteVotesByPostID(postID); err != nil {
		SendErrorResponse(w, "Failed to delete post votes", http.StatusInternalServerError)
		return
	}

	if err := h.postRepo.Delete(postID); err != nil {
		SendErrorResponse(w, "Failed to delete post", http.StatusInternalServerError)
		return
	}

	SendSuccessResponse(w, "Post deleted successfully", nil)
}

// @Summary Fetch title from URL
// @Description Fetch the HTML title for the provided URL
// @Tags posts
// @Accept json
// @Produce json
// @Param url query string true "URL to inspect"
// @Success 200 {object} PostResponse "Title fetched successfully"
// @Failure 400 {object} PostResponse "Invalid URL or URL parameter missing"
// @Failure 501 {object} PostResponse "Title fetching is not available"
// @Failure 502 {object} PostResponse "Failed to fetch title from URL"
// @Router /posts/title [get]
func (h *PostHandler) FetchTitleFromURL(w http.ResponseWriter, r *http.Request) {
	if h.titleFetcher == nil {
		SendErrorResponse(w, "Title fetching is not available", http.StatusNotImplemented)
		return
	}

	requestedURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if requestedURL == "" {
		SendErrorResponse(w, "URL query parameter is required", http.StatusBadRequest)
		return
	}

	titleCtx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()

	title, err := h.titleFetcher.FetchTitle(titleCtx, requestedURL)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrUnsupportedScheme):
			SendErrorResponse(w, "Only HTTP and HTTPS URLs are supported", http.StatusBadRequest)
		case errors.Is(err, services.ErrTitleNotFound):
			SendErrorResponse(w, "Title could not be extracted from the provided URL", http.StatusBadRequest)
		default:
			SendErrorResponse(w, "Failed to fetch title from URL", http.StatusBadGateway)
		}
		return
	}

	SendSuccessResponse(w, "Title fetched successfully", map[string]string{
		"title": title,
	})
}

func translatePostCreateError(err error) (string, int) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return "This URL has already been submitted.", http.StatusConflict
		case "23503":
			return "Author account not found. Please sign in again.", http.StatusUnauthorized
		}
	}

	errStr := err.Error()
	if strings.Contains(errStr, "UNIQUE constraint") || strings.Contains(errStr, "duplicate") {
		return "This URL has already been submitted.", http.StatusConflict
	}

	return "", 0
}

func (h *PostHandler) MountRoutes(r chi.Router, config RouteModuleConfig) {
	public := r
	if config.GeneralRateLimit != nil {
		public = config.GeneralRateLimit(r)
	}
	public.Get("/posts", h.GetPosts)
	public.Get("/posts/search", h.SearchPosts)
	public.Get("/posts/title", h.FetchTitleFromURL)
	public.Get("/posts/{id}", h.GetPost)

	protected := r
	if config.AuthMiddleware != nil {
		protected = r.With(config.AuthMiddleware)
	}
	if config.GeneralRateLimit != nil {
		protected = config.GeneralRateLimit(protected)
	}
	protected.Post("/posts", h.CreatePost)
	protected.Put("/posts/{id}", h.UpdatePost)
	protected.Delete("/posts/{id}", h.DeletePost)
}
