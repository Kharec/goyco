package middleware

import (
	"net/http"
)

func RequestSizeLimitMiddleware(maxSize int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body == nil || r.Body == http.NoBody {
				next.ServeHTTP(w, r)
				return
			}

			limitedBody := http.MaxBytesReader(w, r.Body, maxSize)
			r.Body = limitedBody
			defer func() {
				if err := limitedBody.Close(); err != nil {
					return
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func DefaultRequestSizeLimitMiddleware() func(http.Handler) http.Handler {
	return RequestSizeLimitMiddleware(1024 * 1024)
}
