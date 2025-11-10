package handlers

import (
	"fmt"
	"net/http"
	"time"

	"goyco/internal/config"
	"goyco/internal/middleware"
	"goyco/internal/repositories"
	"goyco/internal/services"
	"goyco/internal/version"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type APIHandler struct {
	config           *config.Config
	postRepo         repositories.PostRepository
	userRepo         repositories.UserRepository
	voteService      *services.VoteService
	dbMonitor        middleware.DBMonitor
	healthChecker    *middleware.DatabaseHealthChecker
	metricsCollector *middleware.MetricsCollector
}

func NewAPIHandler(config *config.Config, postRepo repositories.PostRepository, userRepo repositories.UserRepository, voteService *services.VoteService) *APIHandler {
	return &APIHandler{
		config:      config,
		postRepo:    postRepo,
		userRepo:    userRepo,
		voteService: voteService,
	}
}

func NewAPIHandlerWithMonitoring(config *config.Config, postRepo repositories.PostRepository, userRepo repositories.UserRepository, voteService *services.VoteService, db *gorm.DB, dbMonitor middleware.DBMonitor) *APIHandler {
	if db == nil {
		return NewAPIHandler(config, postRepo, userRepo, voteService)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return NewAPIHandler(config, postRepo, userRepo, voteService)
	}

	healthChecker := middleware.NewDatabaseHealthChecker(sqlDB, dbMonitor)
	metricsCollector := middleware.NewMetricsCollector(dbMonitor)

	return &APIHandler{
		config:           config,
		postRepo:         postRepo,
		userRepo:         userRepo,
		voteService:      voteService,
		dbMonitor:        dbMonitor,
		healthChecker:    healthChecker,
		metricsCollector: metricsCollector,
	}
}

type APIInfo = CommonResponse

// @Summary Get API information
// @Description Retrieve API metadata, available endpoints, authentication details, and response formats
// @Tags api
// @Accept json
// @Produce json
// @Success 200 {object} APIInfo "API information retrieved successfully"
// @Router /api [get]
func (h *APIHandler) GetAPIInfo(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api" {
		http.NotFound(w, r)
		return
	}

	apiInfo := map[string]any{
		"name":        fmt.Sprintf("%s API", h.config.App.Title),
		"version":     version.Version,
		"description": "Y Combinator-style news board API",
		"endpoints": map[string]any{
			"authentication": map[string]any{
				"POST /api/auth/register":            "Register new user",
				"POST /api/auth/login":               "Login user",
				"GET /api/auth/confirm":              "Confirm email address",
				"POST /api/auth/resend-verification": "Resend verification email",
				"POST /api/auth/forgot-password":     "Request password reset",
				"POST /api/auth/reset-password":      "Reset password",
				"POST /api/auth/account/confirm":     "Confirm account deletion",
				"GET /api/auth/me":                   "Get current user profile",
				"POST /api/auth/logout":              "Logout user",
				"PUT /api/auth/email":                "Update email address",
				"PUT /api/auth/username":             "Update username",
				"PUT /api/auth/password":             "Update password",
				"DELETE /api/auth/account":           "Request account deletion",
			},
			"posts": map[string]any{
				"GET /api/posts":         "List all posts",
				"GET /api/posts/search":  "Search posts",
				"GET /api/posts/title":   "Fetch title from URL",
				"GET /api/posts/{id}":    "Get specific post",
				"POST /api/posts":        "Create new post",
				"PUT /api/posts/{id}":    "Update post",
				"DELETE /api/posts/{id}": "Delete post",
			},
			"votes": map[string]any{
				"POST /api/posts/{id}/vote":   "Cast a vote",
				"DELETE /api/posts/{id}/vote": "Remove vote",
				"GET /api/posts/{id}/vote":    "Get user's vote",
				"GET /api/posts/{id}/votes":   "Get all votes for post",
			},
			"users": map[string]any{
				"GET /api/users":            "List all users",
				"POST /api/users":           "Create new user",
				"GET /api/users/{id}":       "Get specific user",
				"GET /api/users/{id}/posts": "Get user's posts",
			},
			"system": map[string]any{
				"GET /health":  "Health check",
				"GET /metrics": "Service metrics",
			},
		},
		"authentication": map[string]any{
			"type": "Bearer Token (JWT)",
			"note": "Include Authorization header with 'Bearer <token>' for protected endpoints",
		},
		"response_format": map[string]any{
			"success": "boolean",
			"message": "string",
			"data":    "object or array",
			"error":   "string (on error)",
		},
	}

	SendSuccessResponse(w, "API information retrieved successfully", apiInfo)
}

// @Summary Health check
// @Description Check the API health status along with database connectivity details
// @Tags api
// @Accept json
// @Produce json
// @Success 200 {object} CommonResponse "Health check successful"
// @Router /health [get]
func (h *APIHandler) GetHealth(w http.ResponseWriter, r *http.Request) {

	if h.healthChecker != nil {
		health := h.healthChecker.CheckHealth()
		health["version"] = version.Version
		SendSuccessResponse(w, "Health check successful", health)
		return
	}

	currentTimestamp := time.Now().UTC().Format(time.RFC3339)

	health := map[string]any{
		"status":    "healthy",
		"timestamp": currentTimestamp,
		"version":   version.Version,
		"services": map[string]any{
			"database": "connected",
			"api":      "running",
		},
	}

	SendSuccessResponse(w, "Health check successful", health)
}

// @Summary Get metrics
// @Description Retrieve application metrics including aggregate counts and database performance data
// @Tags api
// @Accept json
// @Produce json
// @Success 200 {object} CommonResponse "Application metrics retrieved successfully"
// @Router /metrics [get]
func (h *APIHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {

	postCount, err := h.postRepo.Count()
	if err != nil {
		SendErrorResponse(w, "Failed to get post count", http.StatusInternalServerError)
		return
	}

	userCount, err := h.userRepo.Count()
	if err != nil {
		SendErrorResponse(w, "Failed to get user count", http.StatusInternalServerError)
		return
	}

	totalVoteCount, _, err := h.voteService.GetVoteStatistics()
	if err != nil {
		SendErrorResponse(w, "Failed to get vote statistics", http.StatusInternalServerError)
		return
	}

	topPosts, err := h.postRepo.GetTopPosts(5)
	if err != nil {
		SendErrorResponse(w, "Failed to get top posts", http.StatusInternalServerError)
		return
	}

	var avgVotesPerPost float64
	if postCount > 0 {
		avgVotesPerPost = float64(totalVoteCount) / float64(postCount)
	}

	var totalScore int
	for _, post := range topPosts {
		totalScore += post.Score
	}

	var avgScore float64
	if len(topPosts) > 0 {
		avgScore = float64(totalScore) / float64(len(topPosts))
	}

	metrics := map[string]any{
		"posts": map[string]any{
			"total_count":     postCount,
			"top_posts_count": len(topPosts),
			"total_score":     totalScore,
			"average_score":   avgScore,
		},
		"users": map[string]any{
			"total_count": userCount,
		},
		"votes": map[string]any{
			"total_count":      totalVoteCount,
			"average_per_post": avgVotesPerPost,
			"note":             "All votes are counted together",
		},
		"system": map[string]any{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   version.Version,
		},
	}

	if h.metricsCollector != nil {
		performanceMetrics := h.metricsCollector.GetMetrics()
		metrics["database"] = map[string]any{
			"total_queries":    performanceMetrics.DBStats.TotalQueries,
			"slow_queries":     performanceMetrics.DBStats.SlowQueries,
			"average_duration": performanceMetrics.DBStats.AverageDuration.String(),
			"max_duration":     performanceMetrics.DBStats.MaxDuration.String(),
			"error_count":      performanceMetrics.DBStats.ErrorCount,
			"last_query_time":  performanceMetrics.DBStats.LastQueryTime.Format(time.RFC3339),
		}
		metrics["performance"] = map[string]any{
			"request_count":    performanceMetrics.RequestCount,
			"average_response": performanceMetrics.AverageResponse.String(),
			"max_response":     performanceMetrics.MaxResponse.String(),
			"error_count":      performanceMetrics.ErrorCount,
		}
	}

	SendSuccessResponse(w, "Metrics retrieved successfully", metrics)
}

func (h *APIHandler) MountRoutes(r chi.Router, config RouteModuleConfig) {
}
