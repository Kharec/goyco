package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type contextKey string

const UserIDKey contextKey = "user_id"

type TokenVerifier interface {
	VerifyToken(token string) (uint, error)
}

func sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error":   message,
		"message": message,
	})
}

func NewAuth(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authHeader == "" {
				if strings.HasPrefix(r.URL.Path, "/api/") {
					sendJSONError(w, "Authorization header required", http.StatusUnauthorized)
				} else {
					http.Error(w, "Authorization header required", http.StatusUnauthorized)
				}
				return
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				if strings.HasPrefix(r.URL.Path, "/api/") {
					sendJSONError(w, "Invalid authorization header", http.StatusUnauthorized)
				} else {
					http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
				}
				return
			}

			tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if tokenString == "" {
				if strings.HasPrefix(r.URL.Path, "/api/") {
					sendJSONError(w, "Invalid authorization token", http.StatusUnauthorized)
				} else {
					http.Error(w, "Invalid authorization token", http.StatusUnauthorized)
				}
				return
			}

			userID, err := verifier.VerifyToken(tokenString)
			if err != nil {
				if strings.HasPrefix(r.URL.Path, "/api/") {
					sendJSONError(w, "Invalid or expired token", http.StatusUnauthorized)
				} else {
					http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				}
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserIDFromContext(ctx context.Context) uint {
	if userID, ok := ctx.Value(UserIDKey).(uint); ok {
		return userID
	}
	return 0
}
