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
	case "production":
		if origins := os.Getenv("CORS_ALLOWED_ORIGINS"); origins == "" {
			config.AllowedOrigins = []string{}
		}
		config.AllowCredentials = true
	case "staging":
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

func CORSWithConfig(config *CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if r.Method == "OPTIONS" {
				if origin != "" {
					allowed := false
					hasWildcard := false
					for _, allowedOrigin := range config.AllowedOrigins {
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

					if !allowed {
						http.Error(w, "Origin not allowed", http.StatusForbidden)
						return
					}

					if hasWildcard && !config.AllowCredentials {
						w.Header().Set("Access-Control-Allow-Origin", "*")
					} else {
						w.Header().Set("Access-Control-Allow-Origin", origin)
					}

					w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
					w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))

					if config.AllowCredentials && !hasWildcard {
						w.Header().Set("Access-Control-Allow-Credentials", "true")
					}
				}

				w.WriteHeader(http.StatusOK)
				return
			}

			if origin != "" {
				allowed := false
				hasWildcard := false
				for _, allowedOrigin := range config.AllowedOrigins {
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

				if !allowed {
					http.Error(w, "Origin not allowed", http.StatusForbidden)
					return
				}

				if hasWildcard && !config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}

				if config.AllowCredentials && !hasWildcard {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func CORS(next http.Handler) http.Handler {
	config := NewCORSConfig()
	return CORSWithConfig(config)(next)
}
