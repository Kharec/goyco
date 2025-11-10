package server

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
	"goyco/internal/config"
	"goyco/internal/handlers"
	"goyco/internal/middleware"
)

type RouterConfig struct {
	AuthHandler        *handlers.AuthHandler
	PostHandler        *handlers.PostHandler
	VoteHandler        *handlers.VoteHandler
	UserHandler        *handlers.UserHandler
	APIHandler         *handlers.APIHandler
	AuthService        middleware.TokenVerifier
	PageHandler        *handlers.PageHandler
	StaticDir          string
	Debug              bool
	DisableCache       bool
	DisableCompression bool
	DBMonitor          middleware.DBMonitor
	RateLimitConfig    config.RateLimitConfig
}

func NewRouter(cfg RouterConfig) http.Handler {
	middleware.SetTrustProxyHeaders(cfg.RateLimitConfig.TrustProxyHeaders)

	router := chi.NewRouter()
	router.Use(middleware.Logging(cfg.Debug))
	router.Use(middleware.SecurityHeadersMiddleware())
	router.Use(middleware.HSTSMiddleware())
	router.Use(middleware.CORS)

	if !cfg.DisableCompression {
		router.Use(middleware.CompressionMiddleware())
		router.Use(middleware.DecompressionMiddleware())
	}
	router.Use(middleware.DefaultRequestSizeLimitMiddleware())

	if !cfg.DisableCache {
		cache := middleware.NewInMemoryCache()
		cacheConfig := middleware.DefaultCacheConfig()
		router.Use(middleware.CacheMiddleware(cache, cacheConfig))
		router.Use(middleware.CacheInvalidationMiddleware(cache))
	}

	var dbMonitor middleware.DBMonitor
	if cfg.DBMonitor != nil {
		dbMonitor = cfg.DBMonitor
	} else {
		dbMonitor = middleware.NewInMemoryDBMonitor()
	}

	router.Use(middleware.DBMonitoringMiddleware(dbMonitor, 100*time.Millisecond))

	metricsCollector := middleware.NewMetricsCollector(dbMonitor)
	router.Use(middleware.MetricsMiddleware(metricsCollector))

	routeConfig := handlers.RouteModuleConfig{
		AuthService: cfg.AuthService,
		GeneralRateLimit: func(r chi.Router) chi.Router {
			return r.With(middleware.GeneralRateLimitMiddlewareWithLimit(cfg.RateLimitConfig.GeneralLimit))
		},
		AuthRateLimit: func(r chi.Router) chi.Router {
			return r.With(middleware.AuthRateLimitMiddlewareWithLimit(cfg.RateLimitConfig.AuthLimit))
		},
		CSRFMiddleware: middleware.CSRFMiddleware(),
		AuthMiddleware: middleware.NewAuth(cfg.AuthService),
	}

	if cfg.PageHandler != nil {
		cfg.PageHandler.MountRoutes(router, routeConfig)
	}

	if cfg.APIHandler != nil {
		healthRateLimited := router.With(middleware.HealthRateLimitMiddleware(cfg.RateLimitConfig.HealthLimit))
		healthRateLimited.Get("/health", cfg.APIHandler.GetHealth)

		metricsRateLimited := router.With(middleware.MetricsRateLimitMiddleware(cfg.RateLimitConfig.MetricsLimit))
		metricsRateLimited.Get("/metrics", cfg.APIHandler.GetMetrics)
	}

	swaggerRateLimited := router.With(middleware.GeneralRateLimitMiddlewareWithLimit(cfg.RateLimitConfig.GeneralLimit))
	swaggerRateLimited.Get("/swagger/*", httpSwagger.Handler())

	router.Get("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(cfg.StaticDir, "robots.txt"))
	})

	router.Route("/api", func(api chi.Router) {
		modules := []handlers.RouteModule{}

		if cfg.AuthHandler != nil {
			modules = append(modules, cfg.AuthHandler)
		}
		if cfg.PostHandler != nil {
			modules = append(modules, cfg.PostHandler)
		}
		if cfg.VoteHandler != nil {
			modules = append(modules, cfg.VoteHandler)
		}
		if cfg.UserHandler != nil {
			modules = append(modules, cfg.UserHandler)
		}

		for _, module := range modules {
			module.MountRoutes(api, routeConfig)
		}

		if cfg.APIHandler != nil {
			apiRateLimited := api.With(middleware.GeneralRateLimitMiddlewareWithLimit(cfg.RateLimitConfig.GeneralLimit))
			apiRateLimited.Get("/", cfg.APIHandler.GetAPIInfo)
		}
	})

	staticDir := cfg.StaticDir
	if staticDir == "" {
		staticDir = "./internal/static/"
	}

	router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	return router
}
