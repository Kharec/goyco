package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"
)

const CSPNonceKey contextKey = "csp_nonce"

type SecurityHeadersConfig struct {
	RelaxSwaggerCSP bool
}

func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{RelaxSwaggerCSP: true}
}

func GenerateCSPNonce() (string, error) {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", fmt.Errorf("failed to generate CSP nonce: %w", err)
	}
	return base64.StdEncoding.EncodeToString(nonceBytes), nil
}

func GetCSPNonceFromContext(ctx context.Context) string {
	if nonce, ok := ctx.Value(CSPNonceKey).(string); ok {
		return nonce
	}
	return ""
}

func SecurityHeadersMiddleware() func(http.Handler) http.Handler {
	return SecurityHeadersMiddlewareWithConfig(DefaultSecurityHeadersConfig())
}

func SecurityHeadersMiddlewareWithConfig(cfg SecurityHeadersConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			isSwaggerRoute := strings.HasPrefix(r.URL.Path, "/swagger")
			if isSwaggerRoute && cfg.RelaxSwaggerCSP {
				csp := "default-src 'self'; " +
					"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " +
					"style-src 'self' 'unsafe-inline'; " +
					"style-src-attr 'unsafe-inline'; " +
					"style-src-elem 'self' 'unsafe-inline'; " +
					"img-src 'self' data: https:; " +
					"font-src 'self' data:; " +
					"connect-src 'self'; " +
					"frame-ancestors 'none'; " +
					"base-uri 'self'; " +
					"form-action 'self'"
				w.Header().Set("Content-Security-Policy", csp)
			} else {
				nonce, err := GenerateCSPNonce()
				if err != nil {
					log.Printf("middleware security headers: CSP nonce: %v", err)
					nonce = ""
				}

				if nonce != "" {
					ctx := context.WithValue(r.Context(), CSPNonceKey, nonce)
					r = r.WithContext(ctx)
				}

				csp := "default-src 'self'; " +
					"img-src 'self' data: https:; " +
					"font-src 'self' data:; " +
					"connect-src 'self'; " +
					"frame-ancestors 'none'; " +
					"base-uri 'self'; " +
					"form-action 'self'"

				if nonce != "" {
					csp = "script-src 'self' 'nonce-" + nonce + "'; " +
						"style-src 'self' 'nonce-" + nonce + "'; " + csp
				} else {
					csp = "script-src 'self'; " +
						"style-src 'self'; " + csp
				}

				w.Header().Set("Content-Security-Policy", csp)
			}

			permissionsPolicy := "geolocation=(), " +
				"microphone=(), " +
				"camera=(), " +
				"payment=(), " +
				"usb=(), " +
				"magnetometer=(), " +
				"gyroscope=(), " +
				"speaker=(), " +
				"vibrate=(), " +
				"fullscreen=(self), " +
				"sync-xhr=()"
			w.Header().Set("Permissions-Policy", permissionsPolicy)

			w.Header().Set("Server", "")

			next.ServeHTTP(w, r)
		})
	}
}

func HSTSMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.TLS != nil {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			} else if TrustProxyHeaders {
				if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" {
					w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
