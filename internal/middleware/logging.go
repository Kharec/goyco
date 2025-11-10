package middleware

import (
	"log"
	"net/http"
	"time"
)

func Logging(debug bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			if debug {
				log.Printf(
					"%s %s %d %v %s",
					r.Method,
					r.URL.Path,
					wrapped.statusCode,
					duration,
					r.UserAgent(),
				)
			} else {
				if wrapped.statusCode >= 400 || duration > time.Second {
					log.Printf(
						"%s %s %d %v %s",
						r.Method,
						r.URL.Path,
						wrapped.statusCode,
						duration,
						r.UserAgent(),
					)
				}
			}
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
