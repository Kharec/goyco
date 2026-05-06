package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubVerifier struct {
	userID uint
	err    error
	token  string
}

func (s *stubVerifier) VerifyToken(token string) (uint, error) {
	s.token = token
	if s.err != nil {
		return 0, s.err
	}
	return s.userID, nil
}

func TestNewAuthWithoutAuthorization(t *testing.T) {
	verifier := &stubVerifier{userID: 42}
	called := false

	middleware := NewAuth(verifier)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if id := GetUserIDFromContext(r.Context()); id != nil {
			t.Fatalf("unexpected user id %v", id)
		}
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(recorder, request)

	if called {
		t.Fatal("expected next handler NOT to be called when no authorization header")
	}

	if recorder.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Result().StatusCode)
	}
}

func TestNewAuthValidToken(t *testing.T) {
	verifier := &stubVerifier{userID: 99}
	middleware := NewAuth(verifier)

	handlerCalled := false
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		id := GetUserIDFromContext(r.Context())
		if id == nil || *id != 99 {
			v := uint(0)
			if id != nil {
				v = *id
			}
			t.Fatalf("expected user id 99, got %d", v)
		}
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/secure", nil)
	request.Header.Set("Authorization", "Bearer token-123")

	handler.ServeHTTP(recorder, request)

	if !handlerCalled {
		t.Fatal("expected handler to be called for valid token")
	}

	if verifier.token != "token-123" {
		t.Fatalf("expected verifier to receive token-123, got %q", verifier.token)
	}

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Result().StatusCode)
	}
}

func TestNewAuthInvalidHeaders(t *testing.T) {
	tests := []struct {
		name   string
		header string
		status int
	}{
		{name: "MissingBearer", header: "Token value", status: http.StatusUnauthorized},
		{name: "EmptyToken", header: "Bearer   ", status: http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			verifier := &stubVerifier{userID: 1}
			middleware := NewAuth(verifier)
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("handler should not be called")
			}))

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("Authorization", tc.header)

			handler.ServeHTTP(recorder, request)

			if recorder.Result().StatusCode != tc.status {
				t.Fatalf("expected status %d, got %d", tc.status, recorder.Result().StatusCode)
			}
		})
	}
}

func TestNewAuthVerifierError(t *testing.T) {
	verifier := &stubVerifier{err: http.ErrNoCookie}
	middleware := NewAuth(verifier)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when verifier fails")
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer token-xyz")

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 when verifier fails, got %d", recorder.Result().StatusCode)
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDKey, uint(55))

	id := GetUserIDFromContext(ctx)
	if id == nil || *id != 55 {
		t.Fatalf("expected id 55, got %v", id)
	}

	if ptr := GetUserIDFromContext(context.Background()); ptr != nil {
		t.Fatalf("expected nil when id missing, got %v", ptr)
	}
}
