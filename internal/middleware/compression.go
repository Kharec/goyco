package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"slices"
	"strings"
)

func CompressionMiddleware() func(http.Handler) http.Handler {
	return CompressionMiddlewareWithConfig(nil)
}

func CompressionMiddlewareWithConfig(config *CompressionConfig) func(http.Handler) http.Handler {
	if config == nil {
		config = DefaultCompressionConfig()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			if !shouldCompress(r, config) {
				next.ServeHTTP(w, r)
				return
			}

			var buf bytes.Buffer
			bufferedWriter := &bufferedResponseWriter{
				ResponseWriter: w,
				buffer:         &buf,
			}

			next.ServeHTTP(bufferedWriter, r)

			if bufferedWriter.isRedirect {
				return
			}

			if buf.Len() < config.MinSize {
				bufferedWriter.flush()
				w.Write(buf.Bytes())
				return
			}

			responseContentType := w.Header().Get("Content-Type")
			if !shouldCompressResponse(responseContentType, config) {
				bufferedWriter.flush()
				w.Write(buf.Bytes())
				return
			}

			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")
			bufferedWriter.flush()

			gz, err := gzip.NewWriterLevel(w, config.Level)
			if err != nil {
				gz = gzip.NewWriter(w)
			}
			defer gz.Close()

			if _, err := gz.Write(buf.Bytes()); err != nil {
				return
			}
		})
	}
}

type bufferedResponseWriter struct {
	http.ResponseWriter
	buffer        *bytes.Buffer
	statusCode    int
	headerWritten bool
	isRedirect    bool
}

func (brw *bufferedResponseWriter) Write(b []byte) (int, error) {
	if brw.isRedirect {
		return brw.ResponseWriter.Write(b)
	}
	if !brw.headerWritten {
		brw.statusCode = http.StatusOK
	}
	return brw.buffer.Write(b)
}

func (brw *bufferedResponseWriter) WriteHeader(code int) {
	if brw.headerWritten {
		return
	}
	brw.statusCode = code
	if isRedirect(code) {
		brw.isRedirect = true
		brw.ResponseWriter.WriteHeader(code)
		brw.headerWritten = true
	}
}

func (brw *bufferedResponseWriter) Header() http.Header {
	return brw.ResponseWriter.Header()
}

func (brw *bufferedResponseWriter) flush() {
	if !brw.headerWritten {
		brw.ResponseWriter.WriteHeader(brw.statusCode)
		brw.headerWritten = true
	}
}

func isRedirect(statusCode int) bool {
	return statusCode >= 300 && statusCode < 400
}

func shouldCompress(r *http.Request, config *CompressionConfig) bool {
	return r.Header.Get("Content-Encoding") == ""
}

func shouldCompressResponse(contentType string, config *CompressionConfig) bool {
	if contentType == "" {
		return true
	}

	compressible := false
	for _, compressibleType := range config.CompressibleTypes {
		if strings.HasPrefix(contentType, compressibleType) {
			compressible = true
			break
		}
	}

	if !compressible {
		return false
	}

	nonCompressiblePrefixes := []string{"image/", "video/", "audio/"}
	nonCompressibleExact := []string{"application/zip", "application/gzip"}

	for _, prefix := range nonCompressiblePrefixes {
		if strings.HasPrefix(contentType, prefix) {
			return false
		}
	}
	return !slices.Contains(nonCompressibleExact, contentType)
}

func DecompressionMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Content-Encoding") == "gzip" {
				gz, err := gzip.NewReader(r.Body)
				if err != nil {
					http.Error(w, "Invalid gzip encoding", http.StatusBadRequest)
					return
				}
				defer gz.Close()

				r.Body = io.NopCloser(gz)
				r.Header.Del("Content-Encoding")
			}

			next.ServeHTTP(w, r)
		})
	}
}

type CompressionConfig struct {
	Level             int
	MinSize           int
	CompressibleTypes []string
}

func DefaultCompressionConfig() *CompressionConfig {
	return &CompressionConfig{
		Level:   gzip.DefaultCompression,
		MinSize: 0,
		CompressibleTypes: []string{
			"text/",
			"application/json",
			"application/xml",
			"application/javascript",
			"application/css",
			"application/",
		},
	}
}
