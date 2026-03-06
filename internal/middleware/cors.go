package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	MaxAge           int
	AllowCredentials bool
}

func NewCORSConfig() *CORSConfig {
	env := os.Getenv("GOYCO_ENV")

	config := &CORSConfig{
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Requested-With", "X-CSRF-Token"},
		MaxAge:           86400,
		AllowCredentials: false,
	}

	switch env {
	case "production", "staging":
		if origins := os.Getenv("CORS_ALLOWED_ORIGINS"); origins == "" {
			config.AllowedOrigins = []string{}
		}
		config.AllowCredentials = true
	default:
		config.AllowedOrigins = []string{
			"http://localhost:3000",
			"http://localhost:8080",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:8080",
		}
		config.AllowCredentials = true
	}

	if origins := os.Getenv("CORS_ALLOWED_ORIGINS"); origins != "" {
		config.AllowedOrigins = strings.Split(origins, ",")
	}

	return config
}

func checkOrigin(origin string, allowedOrigins []string) (allowed bool, hasWildcard bool) {
	for _, allowedOrigin := range allowedOrigins {
		if allowedOrigin == "*" {
			hasWildcard = true
			allowed = true
			break
		}
		if allowedOrigin == origin {
			allowed = true
			break
		}
	}
	return
}

func setCORSHeaders(w http.ResponseWriter, origin string, hasWildcard bool, config *CORSConfig) {
	if hasWildcard && !config.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}

	if config.AllowCredentials && !hasWildcard {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
}

func CORSWithConfig(config *CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin == "" {
				if r.Method == "OPTIONS" {
					w.WriteHeader(http.StatusOK)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			allowed, hasWildcard := checkOrigin(origin, config.AllowedOrigins)

			if !allowed {
				http.Error(w, "Origin not allowed", http.StatusForbidden)
				return
			}

			if r.Method == "OPTIONS" {
				setCORSHeaders(w, origin, hasWildcard, config)

				w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
				w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))

				w.WriteHeader(http.StatusOK)
				return
			}

			setCORSHeaders(w, origin, hasWildcard, config)
			next.ServeHTTP(w, r)
		})
	}
}

func CORS(next http.Handler) http.Handler {
	config := NewCORSConfig()
	return CORSWithConfig(config)(next)
}
