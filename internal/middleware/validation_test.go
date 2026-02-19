package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type TestUser struct {
	Username string `json:"username" validate:"required,min=3,max=20"`
	Email    string `json:"email" validate:"required,email"`
	Age      int    `json:"age" validate:"min=18,max=120"`
	URL      string `json:"url" validate:"url"`
	Status   string `json:"status" validate:"oneof=active inactive pending"`
}

type TestPost struct {
	Title   string `json:"title" validate:"required,min=1,max=200"`
	Content string `json:"content" validate:"required,min=10"`
	Tags    string `json:"tags" validate:"omitempty,min=1"`
}

func TestValidationMiddleware(t *testing.T) {
	middleware := ValidationMiddleware()

	t.Run("Valid POST request", func(t *testing.T) {
		user := TestUser{
			Username: "testuser",
			Email:    "test@example.com",
			Age:      25,
			URL:      "https://example.com",
			Status:   "active",
		}

		body, _ := json.Marshal(user)
		request := httptest.NewRequest("POST", "/users", bytes.NewBuffer(body))
		request.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(request.Context(), DTOTypeKey, reflect.TypeFor[TestUser]())
		request = request.WithContext(ctx)
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Body.String() != "success" {
			t.Errorf("Expected 'success', got '%s'", recorder.Body.String())
		}
	})

	t.Run("Invalid POST request - missing required field", func(t *testing.T) {
		user := TestUser{
			Email:  "test@example.com",
			Age:    25,
			URL:    "https://example.com",
			Status: "active",
		}

		body, _ := json.Marshal(user)
		request := httptest.NewRequest("POST", "/users", bytes.NewBuffer(body))
		request.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(request.Context(), DTOTypeKey, reflect.TypeFor[TestUser]())
		request = request.WithContext(ctx)
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Handler should not be called for invalid request")
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", recorder.Code)
		}
	})

	t.Run("GET request bypasses validation", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/users", nil)
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Body.String() != "success" {
			t.Errorf("Expected 'success', got '%s'", recorder.Body.String())
		}
	})

	t.Run("Invalid POST request - invalid URL format", func(t *testing.T) {
		user := TestUser{
			Username: "testuser",
			Email:    "test@example.com",
			Age:      25,
			URL:      "http://",
			Status:   "active",
		}

		body, _ := json.Marshal(user)
		request := httptest.NewRequest("POST", "/users", bytes.NewBuffer(body))
		request.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(request.Context(), DTOTypeKey, reflect.TypeFor[TestUser]())
		request = request.WithContext(ctx)
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Handler should not be called for invalid request")
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", recorder.Code)
		}
	})

	t.Run("Invalid POST request - URL without protocol", func(t *testing.T) {
		user := TestUser{
			Username: "testuser",
			Email:    "test@example.com",
			Age:      25,
			URL:      "example.com",
			Status:   "active",
		}

		body, _ := json.Marshal(user)
		request := httptest.NewRequest("POST", "/users", bytes.NewBuffer(body))
		request.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(request.Context(), DTOTypeKey, reflect.TypeFor[TestUser]())
		request = request.WithContext(ctx)
		recorder := httptest.NewRecorder()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Handler should not be called for invalid request")
		}))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", recorder.Code)
		}
	})
}
