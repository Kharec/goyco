package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

const (
	CSRFTokenCookieName = "csrf_token"
	CSRFTokenFormName   = "csrf_token"
	CSRFTokenHeaderName = "X-CSRF-Token"
)

func CSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func SetCSRFToken(w http.ResponseWriter, r *http.Request, token string) {
	cookie := &http.Cookie{
		Name:     CSRFTokenCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   IsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   3600,
	}
	http.SetCookie(w, cookie)
}

func GetCSRFToken(r *http.Request) string {
	token := strings.TrimSpace(r.FormValue(CSRFTokenFormName))
	if token == "" {
		token = strings.TrimSpace(r.Header.Get(CSRFTokenHeaderName))
	}
	return token
}

func ValidateCSRFToken(r *http.Request) bool {
	formToken := GetCSRFToken(r)
	if formToken == "" {
		return false
	}

	cookie, err := r.Cookie(CSRFTokenCookieName)
	if err != nil {
		return false
	}

	cookieToken := strings.TrimSpace(cookie.Value)
	if cookieToken == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(formToken), []byte(cookieToken)) == 1
}

func CSRFMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
				next.ServeHTTP(w, r)
				return
			}

			if strings.HasPrefix(r.URL.Path, "/api/") && hasBearerToken(r) {
				next.ServeHTTP(w, r)
				return
			}

			if !ValidateCSRFToken(r) {
				http.Error(w, "Invalid CSRF token", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func hasBearerToken(r *http.Request) bool {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	return strings.HasPrefix(auth, "Bearer ")
}

func IsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}

	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "https" {
		return true
	}

	ssl := r.Header.Get("X-Forwarded-Ssl")
	if ssl == "on" {
		return true
	}

	scheme := r.Header.Get("X-Forwarded-Scheme")
	return scheme == "https"
}
