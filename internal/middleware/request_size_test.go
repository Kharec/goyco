package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestRequestSizeLimitMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		requestSize    int
		limitSize      int64
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "request within limit",
			requestSize:    100,
			limitSize:      1000,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "request exactly at limit",
			requestSize:    1000,
			limitSize:      1000,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "request exceeds limit",
			requestSize:    1500,
			limitSize:      1000,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "request significantly exceeds limit",
			requestSize:    5000,
			limitSize:      1000,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "zero limit",
			requestSize:    100,
			limitSize:      0,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "empty request body",
			requestSize:    0,
			limitSize:      1000,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {

					http.Error(w, "Request body too large", http.StatusBadRequest)
					return
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Body size: " + strconv.Itoa(len(body))))
			})

			middleware := RequestSizeLimitMiddleware(tt.limitSize)
			wrappedHandler := middleware(handler)

			var body io.Reader
			if tt.requestSize > 0 {
				body = strings.NewReader(strings.Repeat("A", tt.requestSize))
			} else {
				body = http.NoBody
			}

			request := httptest.NewRequest("POST", "/test", body)
			request.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(recorder, request)

			if recorder.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, recorder.Code)
			}

			if tt.expectError {
				if recorder.Code != http.StatusBadRequest {
					t.Errorf("Expected %d status for oversized request, got %d", http.StatusBadRequest, recorder.Code)
				}
			} else {
				if recorder.Code != http.StatusOK {
					t.Errorf("Expected %d status for valid request, got %d", http.StatusOK, recorder.Code)
				}
			}
		})
	}
}

func TestRequestSizeLimitMiddleware_NoBody(t *testing.T) {

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("No body"))
	})

	middleware := RequestSizeLimitMiddleware(1000)
	wrappedHandler := middleware(handler)

	request := httptest.NewRequest("GET", "/test", nil)
	request.Body = nil

	recorder := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d for nil body, got %d", http.StatusOK, recorder.Code)
	}
}

func TestRequestSizeLimitMiddleware_NoBodyHTTP(t *testing.T) {

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("No body"))
	})

	middleware := RequestSizeLimitMiddleware(1000)
	wrappedHandler := middleware(handler)

	request := httptest.NewRequest("GET", "/test", http.NoBody)

	recorder := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d for http.NoBody, got %d", http.StatusOK, recorder.Code)
	}
}

func TestRequestSizeLimitMiddleware_HandlerError(t *testing.T) {

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Handler error", http.StatusInternalServerError)
	})

	middleware := RequestSizeLimitMiddleware(1000)
	wrappedHandler := middleware(handler)

	request := httptest.NewRequest("POST", "/test", strings.NewReader("small body"))

	recorder := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d for handler error, got %d", http.StatusInternalServerError, recorder.Code)
	}
}

func TestRequestSizeLimitMiddleware_ReadBody(t *testing.T) {

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Read " + strconv.Itoa(len(body)) + " bytes"))
	})

	middleware := RequestSizeLimitMiddleware(100)
	wrappedHandler := middleware(handler)

	request := httptest.NewRequest("POST", "/test", strings.NewReader("Hello, World!"))

	recorder := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	expectedBody := "Read 13 bytes"
	if !strings.Contains(recorder.Body.String(), expectedBody) {
		t.Errorf("Expected response to contain %q, got %q", expectedBody, recorder.Body.String())
	}
}

func TestRequestSizeLimitMiddleware_PartialRead(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := make([]byte, 5)
		n, err := r.Body.Read(buffer)
		if err != nil && err != io.EOF {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Read " + strconv.Itoa(n) + " bytes: " + string(buffer[:n])))
	})

	middleware := RequestSizeLimitMiddleware(100)
	wrappedHandler := middleware(handler)

	request := httptest.NewRequest("POST", "/test", strings.NewReader("Hello, World!"))

	recorder := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	expectedBody := "Read 5 bytes: Hello"
	if !strings.Contains(recorder.Body.String(), expectedBody) {
		t.Errorf("Expected response to contain %q, got %q", expectedBody, recorder.Body.String())
	}
}

func TestDefaultRequestSizeLimitMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		requestSize    int
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "request within 1MB limit",
			requestSize:    100 * 1024,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "request exactly 1MB",
			requestSize:    1024 * 1024,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "request exceeds 1MB",
			requestSize:    2 * 1024 * 1024,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "Request body too large", http.StatusBadRequest)
					return
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Body size: " + strconv.Itoa(len(body))))
			})

			middleware := DefaultRequestSizeLimitMiddleware()
			wrappedHandler := middleware(handler)

			request := httptest.NewRequest("POST", "/test", strings.NewReader(strings.Repeat("A", tt.requestSize)))
			request.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(recorder, request)

			if recorder.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, recorder.Code)
			}

			if tt.expectError {
				if recorder.Code != http.StatusBadRequest {
					t.Errorf("Expected %d status for oversized request, got %d", http.StatusBadRequest, recorder.Code)
				}
			} else {
				if recorder.Code != http.StatusOK {
					t.Errorf("Expected %d status for valid request, got %d", http.StatusOK, recorder.Code)
				}
			}
		})
	}
}

func TestRequestSizeLimitMiddleware_ConcurrentRequests(t *testing.T) {

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		_ = len(body)
	})

	middleware := RequestSizeLimitMiddleware(1000)
	wrappedHandler := middleware(handler)

	done := make(chan bool, 10)

	for i := range 10 {
		go func(size int) {
			defer func() { done <- true }()

			request := httptest.NewRequest("POST", "/test", strings.NewReader(strings.Repeat("A", size)))
			recorder := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Errorf("Expected status %d for concurrent request, got %d", http.StatusOK, recorder.Code)
			}
		}(i * 100)
	}

	for range 10 {
		<-done
	}
}

func TestRequestSizeLimitMiddleware_LargeRequest(t *testing.T) {

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {

			http.Error(w, "Request body too large", http.StatusBadRequest)
			return
		}

		t.Error("Handler should not be called for oversized requests")
		_ = len(body)
	})

	middleware := RequestSizeLimitMiddleware(100)
	wrappedHandler := middleware(handler)

	largeBody := strings.NewReader(strings.Repeat("A", 10000))
	request := httptest.NewRequest("POST", "/test", largeBody)

	recorder := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for large request, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestRequestSizeLimitMiddleware_EmptyBodyAfterLimit(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 2000)
		n, err := r.Body.Read(body)

		if err != nil && err != io.EOF {
			http.Error(w, "Body too large", http.StatusRequestEntityTooLarge)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Read " + string(rune(n)) + " bytes"))
	})

	middleware := RequestSizeLimitMiddleware(100)
	wrappedHandler := middleware(handler)

	request := httptest.NewRequest("POST", "/test", strings.NewReader(strings.Repeat("A", 500)))

	recorder := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest && recorder.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status %d or %d for oversized request, got %d", http.StatusBadRequest, http.StatusRequestEntityTooLarge, recorder.Code)
	}
}

func TestRequestSizeLimitMiddleware_ChunkedBody(t *testing.T) {

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Read " + strconv.Itoa(len(body)) + " bytes"))
	})

	middleware := RequestSizeLimitMiddleware(1000)
	wrappedHandler := middleware(handler)

	request := httptest.NewRequest("POST", "/test", strings.NewReader("Hello, World!"))
	request.TransferEncoding = []string{"chunked"}

	recorder := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d for chunked request, got %d", http.StatusOK, recorder.Code)
	}
}

func TestRequestSizeLimitMiddleware_ContentLengthHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Read " + strconv.Itoa(len(body)) + " bytes"))
	})

	middleware := RequestSizeLimitMiddleware(1000)
	wrappedHandler := middleware(handler)

	body := strings.NewReader("Hello, World!")
	request := httptest.NewRequest("POST", "/test", body)
	request.ContentLength = int64(len("Hello, World!"))

	recorder := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d for request with Content-Length, got %d", http.StatusOK, recorder.Code)
	}
}

func TestRequestSizeLimitMiddleware_ZeroContentLength(t *testing.T) {

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Read " + strconv.Itoa(len(body)) + " bytes"))
	})

	middleware := RequestSizeLimitMiddleware(1000)
	wrappedHandler := middleware(handler)

	request := httptest.NewRequest("POST", "/test", http.NoBody)
	request.ContentLength = 0

	recorder := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d for zero Content-Length request, got %d", http.StatusOK, recorder.Code)
	}
}

func TestRequestSizeLimitMiddleware_InvalidContentLength(t *testing.T) {

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Read " + strconv.Itoa(len(body)) + " bytes"))
	})

	middleware := RequestSizeLimitMiddleware(1000)
	wrappedHandler := middleware(handler)

	request := httptest.NewRequest("POST", "/test", strings.NewReader("Hello"))
	request.ContentLength = -1

	recorder := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d for invalid Content-Length request, got %d", http.StatusOK, recorder.Code)
	}
}
