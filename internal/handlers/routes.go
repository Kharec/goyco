package handlers

import (
	"net/http"

	"goyco/internal/middleware"

	"github.com/go-chi/chi/v5"
)

type RouteModule interface {
	MountRoutes(r chi.Router, config RouteModuleConfig)
}

type RouteModuleConfig struct {
	AuthService      middleware.TokenVerifier
	GeneralRateLimit func(chi.Router) chi.Router
	AuthRateLimit    func(chi.Router) chi.Router
	CSRFMiddleware   func(http.Handler) http.Handler
	AuthMiddleware   func(http.Handler) http.Handler
}
