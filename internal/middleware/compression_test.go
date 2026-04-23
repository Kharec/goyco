package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompressionMiddleware(t *testing.T) {
	middleware := CompressionMiddleware()

	t.Run("Accepts gzip encoding", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/test", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Header().Get("Content-Encoding") != "gzip" {
			t.Error("Expected Content-Encoding to be gzip")
		}

		if !isGzipCompressed(recorder.Body.Bytes()) {
			t.Error("Expected response to be gzip compressed")
		}

		decompressed, err := decompressGzip(recorder.Body.Bytes())
		if err != nil {
			t.Fatalf("Failed to decompress response: %v", err)
		}

		if string(decompressed) != "test response" {
			t.Errorf("Expected decompressed content to be 'test response', got '%s'", string(decompressed))
		}
	})

	t.Run("Does not accept gzip encoding", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/test", nil)
		request.Header.Set("Accept-Encoding", "deflate")
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Header().Get("Content-Encoding") == "gzip" {
			t.Error("Expected Content-Encoding not to be gzip")
		}

		if recorder.Body.String() != "test response" {
			t.Errorf("Expected 'test response', got '%s'", recorder.Body.String())
		}
	})

	t.Run("No Accept-Encoding header", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/test", nil)
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Header().Get("Content-Encoding") == "gzip" {
			t.Error("Expected Content-Encoding not to be gzip")
		}

		if recorder.Body.String() != "test response" {
			t.Errorf("Expected 'test response', got '%s'", recorder.Body.String())
		}
	})

	t.Run("Small response compressed", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/test", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("hi"))
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Header().Get("Content-Encoding") != "gzip" {
			t.Error("Expected small response to be compressed")
		}

		decompressed, err := decompressGzip(recorder.Body.Bytes())
		if err != nil {
			t.Fatalf("Failed to decompress response: %v", err)
		}

		if string(decompressed) != "hi" {
			t.Errorf("Expected decompressed content to be 'hi', got '%s'", string(decompressed))
		}
	})

	t.Run("Already compressed response", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/test", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		request.Header.Set("Content-Encoding", "gzip")
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("already compressed"))
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Header().Get("Content-Encoding") == "gzip" {
			t.Error("Expected Content-Encoding not to be gzip for already compressed request")
		}

		if recorder.Body.String() != "already compressed" {
			t.Errorf("Expected 'already compressed', got '%s'", recorder.Body.String())
		}
	})
}

func TestShouldCompress(t *testing.T) {
	tests := []struct {
		name     string
		request  *http.Request
		expected bool
	}{
		{
			name: "GET request with gzip encoding",
			request: func() *http.Request {
				request := httptest.NewRequest("GET", "/test", nil)
				request.Header.Set("Accept-Encoding", "gzip")
				return request
			}(),
			expected: true,
		},
		{
			name: "POST request with gzip encoding",
			request: func() *http.Request {
				request := httptest.NewRequest("POST", "/test", nil)
				request.Header.Set("Accept-Encoding", "gzip")
				return request
			}(),
			expected: true,
		},
		{
			name: "GET request without gzip encoding",
			request: func() *http.Request {
				request := httptest.NewRequest("GET", "/test", nil)
				request.Header.Set("Accept-Encoding", "deflate")
				return request
			}(),
			expected: true,
		},
		{
			name: "GET request for image",
			request: func() *http.Request {
				request := httptest.NewRequest("GET", "/image.jpg", nil)
				request.Header.Set("Accept-Encoding", "gzip")
				request.Header.Set("Content-Type", "image/jpeg")
				return request
			}(),
			expected: true,
		},
		{
			name: "GET request for CSS",
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/style.css", nil)
				req.Header.Set("Accept-Encoding", "gzip")
				req.Header.Set("Content-Type", "text/css")
				return req
			}(),
			expected: true,
		},
		{
			name: "GET request for JavaScript",
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/script.js", nil)
				req.Header.Set("Accept-Encoding", "gzip")
				req.Header.Set("Content-Type", "application/javascript")
				return req
			}(),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultCompressionConfig()
			result := shouldCompress(tt.request, config)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func isGzipCompressed(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	return data[0] == 0x1f && data[1] == 0x8b
}

func decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

func TestCompressionMiddlewareWithConfig(t *testing.T) {
	t.Run("With default config", func(t *testing.T) {
		config := DefaultCompressionConfig()
		middleware := CompressionMiddlewareWithConfig(config)

		request := httptest.NewRequest("GET", "/test", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		request.Header.Set("Content-Type", "text/html")
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Header().Get("Content-Encoding") != "gzip" {
			t.Error("Expected Content-Encoding to be gzip")
		}

		if !isGzipCompressed(recorder.Body.Bytes()) {
			t.Error("Expected response to be gzip compressed")
		}

		decompressed, err := decompressGzip(recorder.Body.Bytes())
		if err != nil {
			t.Fatalf("Failed to decompress response: %v", err)
		}

		if string(decompressed) != "test response" {
			t.Errorf("Expected decompressed content to be 'test response', got '%s'", string(decompressed))
		}
	})

	t.Run("With custom config", func(t *testing.T) {
		config := &CompressionConfig{
			Level:   gzip.BestCompression,
			MinSize: 0,
			CompressibleTypes: []string{
				"text/",
				"application/json",
			},
		}
		middleware := CompressionMiddlewareWithConfig(config)

		request := httptest.NewRequest("GET", "/test", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Header().Get("Content-Encoding") != "gzip" {
			t.Error("Expected Content-Encoding to be gzip")
		}

		if !isGzipCompressed(recorder.Body.Bytes()) {
			t.Error("Expected response to be gzip compressed")
		}
	})

	t.Run("With nil config uses default", func(t *testing.T) {
		middleware := CompressionMiddlewareWithConfig(nil)

		request := httptest.NewRequest("GET", "/test", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		request.Header.Set("Content-Type", "text/html")
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Header().Get("Content-Encoding") != "gzip" {
			t.Error("Expected Content-Encoding to be gzip")
		}
	})

	t.Run("Non-compressible content type", func(t *testing.T) {
		config := DefaultCompressionConfig()
		middleware := CompressionMiddlewareWithConfig(config)

		request := httptest.NewRequest("GET", "/test", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Header().Get("Content-Encoding") == "gzip" {
			t.Error("Expected Content-Encoding not to be gzip for non-compressible content")
		}

		if recorder.Body.String() != "test response" {
			t.Errorf("Expected 'test response', got '%s'", recorder.Body.String())
		}
	})

	t.Run("Minimum size threshold - small response not compressed", func(t *testing.T) {
		config := &CompressionConfig{
			Level:   gzip.DefaultCompression,
			MinSize: 1000,
			CompressibleTypes: []string{
				"text/",
			},
		}
		middleware := CompressionMiddlewareWithConfig(config)

		request := httptest.NewRequest("GET", "/test", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		request.Header.Set("Content-Type", "text/html")
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("small"))
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Header().Get("Content-Encoding") == "gzip" {
			t.Error("Expected Content-Encoding not to be gzip for small response")
		}

		if recorder.Body.String() != "small" {
			t.Errorf("Expected 'small', got '%s'", recorder.Body.String())
		}
	})

	t.Run("Minimum size threshold - large response compressed", func(t *testing.T) {
		config := &CompressionConfig{
			Level:   gzip.DefaultCompression,
			MinSize: 10,
			CompressibleTypes: []string{
				"text/",
			},
		}
		middleware := CompressionMiddlewareWithConfig(config)

		request := httptest.NewRequest("GET", "/test", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		request.Header.Set("Content-Type", "text/html")
		recorder := httptest.NewRecorder()

		largeResponse := strings.Repeat("a", 100)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(largeResponse))
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Header().Get("Content-Encoding") != "gzip" {
			t.Error("Expected Content-Encoding to be gzip for large response")
		}

		if !isGzipCompressed(recorder.Body.Bytes()) {
			t.Error("Expected response to be gzip compressed")
		}
	})
}

func TestDecompressionMiddleware(t *testing.T) {
	t.Run("Decompresses gzip request body", func(t *testing.T) {
		middleware := DecompressionMiddleware()

		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		gz.Write([]byte("compressed data"))
		gz.Close()

		request := httptest.NewRequest("POST", "/test", &buf)
		request.Header.Set("Content-Encoding", "gzip")
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("Failed to read request body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			w.Write(body)
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Body.String() != "compressed data" {
			t.Errorf("Expected 'compressed data', got '%s'", recorder.Body.String())
		}

		if request.Header.Get("Content-Encoding") != "" {
			t.Error("Expected Content-Encoding header to be removed")
		}
	})

	t.Run("Handles non-gzip request", func(t *testing.T) {
		middleware := DecompressionMiddleware()

		request := httptest.NewRequest("POST", "/test", strings.NewReader("plain data"))
		request.Header.Set("Content-Type", "text/plain")
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("Failed to read request body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			w.Write(body)
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Body.String() != "plain data" {
			t.Errorf("Expected 'plain data', got '%s'", recorder.Body.String())
		}
	})

	t.Run("Handles invalid gzip data", func(t *testing.T) {
		middleware := DecompressionMiddleware()

		request := httptest.NewRequest("POST", "/test", strings.NewReader("invalid gzip data"))
		request.Header.Set("Content-Encoding", "gzip")
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Handler should not be called for invalid gzip data")
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", recorder.Code)
		}

		if !strings.Contains(recorder.Body.String(), "Invalid gzip encoding") {
			t.Error("Expected error message about invalid gzip encoding")
		}
	})

	t.Run("Handles empty request body", func(t *testing.T) {
		middleware := DecompressionMiddleware()

		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		gz.Close()

		request := httptest.NewRequest("POST", "/test", &buf)
		request.Header.Set("Content-Encoding", "gzip")
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("Failed to read request body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			w.Write(body)
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Body.String() != "" {
			t.Errorf("Expected empty body, got '%s'", recorder.Body.String())
		}
	})

	t.Run("Limits decompressed size", func(t *testing.T) {
		config := &DecompressionConfig{
			MaxDecompressedSize: 10,
		}
		middleware := DecompressionMiddlewareWithConfig(config)

		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		gz.Write([]byte("this is more than ten bytes of data"))
		gz.Close()

		request := httptest.NewRequest("POST", "/test", &buf)
		request.Header.Set("Content-Encoding", "gzip")
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("Failed to read request body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			w.Write(body)
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if len(recorder.Body.String()) > 10 {
			t.Errorf("Expected body to be truncated to <= 10 bytes, got %d bytes", len(recorder.Body.String()))
		}
	})
}

func TestShouldCompressWithConfig(t *testing.T) {
	config := DefaultCompressionConfig()

	tests := []struct {
		name     string
		request  *http.Request
		config   *CompressionConfig
		expected bool
	}{
		{
			name: "Compressible content type",
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Content-Type", "text/html")
				return req
			}(),
			config:   config,
			expected: true,
		},
		{
			name: "Non-compressible content type",
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Content-Type", "image/jpeg")
				return req
			}(),
			config:   config,
			expected: true,
		},
		{
			name: "Already compressed request",
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Content-Type", "text/html")
				req.Header.Set("Content-Encoding", "gzip")
				return req
			}(),
			config:   config,
			expected: false,
		},
		{
			name: "Custom compressible types",
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Content-Type", "application/custom")
				return req
			}(),
			config: &CompressionConfig{
				CompressibleTypes: []string{"application/custom"},
			},
			expected: true,
		},
		{
			name: "Non-compressible exact match",
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Content-Type", "application/zip")
				return req
			}(),
			config:   config,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldCompress(tt.request, tt.config)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDefaultCompressionConfig(t *testing.T) {
	config := DefaultCompressionConfig()

	if config.Level != gzip.DefaultCompression {
		t.Errorf("Expected level %d, got %d", gzip.DefaultCompression, config.Level)
	}

	if config.MinSize != 0 {
		t.Errorf("Expected min size 0, got %d", config.MinSize)
	}

	expectedTypes := []string{
		"text/",
		"application/json",
		"application/xml",
		"application/javascript",
		"application/css",
		"application/",
	}

	if len(config.CompressibleTypes) != len(expectedTypes) {
		t.Errorf("Expected %d compressible types, got %d", len(expectedTypes), len(config.CompressibleTypes))
	}

	for i, expectedType := range expectedTypes {
		if config.CompressibleTypes[i] != expectedType {
			t.Errorf("Expected compressible type %s at index %d, got %s", expectedType, i, config.CompressibleTypes[i])
		}
	}
}
