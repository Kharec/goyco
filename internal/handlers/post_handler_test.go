package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgconn"
	"gorm.io/gorm"
	"goyco/internal/database"
	"goyco/internal/middleware"
	"goyco/internal/repositories"
	"goyco/internal/services"
	"goyco/internal/testutils"
)

func decodeHandlerResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return payload
}

func TestPostHandlerGetPostsWithVoteService(t *testing.T) {
	repo := testutils.NewPostRepositoryStub()
	repo.GetAllFn = func(limit, offset int) ([]database.Post, error) {
		return []database.Post{
			{ID: 1, Title: "Test Post 1"},
			{ID: 2, Title: "Test Post 2"},
		}, nil
	}

	voteRepo := testutils.NewMockVoteRepository()
	voteService := services.NewVoteService(voteRepo, repo, nil)
	handler := NewPostHandler(repo, nil, voteService)

	request := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	request = testutils.WithUserContext(request, middleware.UserIDKey, uint(1))
	recorder := httptest.NewRecorder()

	handler.GetPosts(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	payload := decodeHandlerResponse(t, recorder)
	if !payload["success"].(bool) {
		t.Fatalf("expected success response, got %v", payload)
	}
}

func TestPostHandlerCreatePostWithTitleFetcher(t *testing.T) {
	repo := testutils.NewPostRepositoryStub()
	var storedPost *database.Post
	repo.CreateFn = func(post *database.Post) error {
		storedPost = post
		return nil
	}

	titleFetcher := &testutils.MockTitleFetcher{}
	titleFetcher.SetTitle("Fetched Title")

	handler := NewPostHandler(repo, titleFetcher, nil)

	request := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBufferString(`{"url":"https://example.com","content":"Test content"}`))
	request = testutils.WithUserContext(request, middleware.UserIDKey, uint(1))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.CreatePost(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusCreated)

	if storedPost == nil {
		t.Fatal("expected post to be created")
	}

	if storedPost.Title != "Fetched Title" {
		t.Errorf("expected title 'Fetched Title', got %s", storedPost.Title)
	}
}

func TestPostHandlerCreatePostTitleFetcherError(t *testing.T) {
	repo := testutils.NewPostRepositoryStub()
	titleFetcher := &testutils.MockTitleFetcher{}
	titleFetcher.SetError(services.ErrUnsupportedScheme)

	handler := NewPostHandler(repo, titleFetcher, nil)

	request := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBufferString(`{"url":"ftp://example.com"}`))
	request = testutils.WithUserContext(request, middleware.UserIDKey, uint(1))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.CreatePost(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

	payload := decodeHandlerResponse(t, recorder)
	if payload["success"].(bool) {
		t.Fatalf("expected error response, got %v", payload)
	}
}

func TestPostHandlerSearchPosts(t *testing.T) {
	repo := testutils.NewPostRepositoryStub()
	repo.SearchFn = func(query string, limit, offset int) ([]database.Post, error) {
		return []database.Post{
			{ID: 1, Title: "Search Result 1"},
			{ID: 2, Title: "Search Result 2"},
		}, nil
	}

	handler := NewPostHandler(repo, nil, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/posts/search?q=test", nil)
	recorder := httptest.NewRecorder()

	handler.SearchPosts(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	payload := decodeHandlerResponse(t, recorder)
	if !payload["success"].(bool) {
		t.Fatalf("expected success response, got %v", payload)
	}
}

func TestPostHandlerFetchTitleFromURL(t *testing.T) {
	titleFetcher := &testutils.MockTitleFetcher{}
	titleFetcher.SetTitle("Test Title")

	handler := NewPostHandler(nil, titleFetcher, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/posts/title?url=https://example.com", nil)
	recorder := httptest.NewRecorder()

	handler.FetchTitleFromURL(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	payload := decodeHandlerResponse(t, recorder)
	if !payload["success"].(bool) {
		t.Fatalf("expected success response, got %v", payload)
	}
}

func TestPostHandlerFetchTitleFromURLNoFetcher(t *testing.T) {
	handler := NewPostHandler(nil, nil, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/posts/title?url=https://example.com", nil)
	recorder := httptest.NewRecorder()

	handler.FetchTitleFromURL(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusNotImplemented)
}

func TestPostHandlerUpdatePostUnauthorized(t *testing.T) {
	repo := testutils.NewPostRepositoryStub()
	repo.GetByIDFn = func(id uint) (*database.Post, error) {
		return &database.Post{ID: id, AuthorID: func() *uint { u := uint(2); return &u }()}, nil
	}

	handler := NewPostHandler(repo, nil, nil)

	request := httptest.NewRequest(http.MethodPut, "/api/posts/1", bytes.NewBufferString(`{"title":"Updated Title","content":"Updated content"}`))
	request = testutils.WithUserContext(request, middleware.UserIDKey, uint(1))
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.UpdatePost(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusForbidden)
}

func TestPostHandlerDeletePostUnauthorized(t *testing.T) {
	repo := testutils.NewPostRepositoryStub()
	repo.GetByIDFn = func(id uint) (*database.Post, error) {
		return &database.Post{ID: id, AuthorID: func() *uint { u := uint(2); return &u }()}, nil
	}

	voteRepo := testutils.NewMockVoteRepository()
	voteService := services.NewVoteService(voteRepo, repo, nil)
	handler := NewPostHandler(repo, nil, voteService)

	request := httptest.NewRequest(http.MethodDelete, "/api/posts/1", nil)
	request = testutils.WithUserContext(request, middleware.UserIDKey, uint(1))
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	recorder := httptest.NewRecorder()

	handler.DeletePost(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusForbidden)
}

func TestPostHandlerGetPosts(t *testing.T) {
	var receivedLimit, receivedOffset int
	repo := testutils.NewPostRepositoryStub()
	repo.GetAllFn = func(limit, offset int) ([]database.Post, error) {
		receivedLimit = limit
		receivedOffset = offset
		return []database.Post{{ID: 1}}, nil
	}

	handler := NewPostHandler(repo, nil, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/posts?limit=5&offset=2", nil)
	recorder := httptest.NewRecorder()

	handler.GetPosts(recorder, request)

	if receivedLimit != 5 || receivedOffset != 2 {
		t.Fatalf("expected limit=5 offset=2, got %d %d", receivedLimit, receivedOffset)
	}

	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	payload := decodeHandlerResponse(t, recorder)
	if !payload["success"].(bool) {
		t.Fatalf("expected success response, got %v", payload)
	}
}

func TestPostHandlerGetPostErrors(t *testing.T) {
	repo := testutils.NewPostRepositoryStub()
	handler := NewPostHandler(repo, nil, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	recorder := httptest.NewRecorder()
	handler.GetPost(recorder, request)
	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing id, got %d", recorder.Result().StatusCode)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/posts/abc", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "abc"})
	recorder = httptest.NewRecorder()
	handler.GetPost(recorder, request)
	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", recorder.Result().StatusCode)
	}

	repo.GetByIDFn = func(uint) (*database.Post, error) {
		return nil, gorm.ErrRecordNotFound
	}
	request = httptest.NewRequest(http.MethodGet, "/api/posts/1", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	recorder = httptest.NewRecorder()
	handler.GetPost(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusNotFound)
}

func TestPostHandlerCreatePostSuccess(t *testing.T) {
	var storedPost *database.Post
	repo := testutils.NewPostRepositoryStub()
	repo.CreateFn = func(post *database.Post) error {
		storedPost = &database.Post{
			Title:    post.Title,
			URL:      post.URL,
			Content:  post.Content,
			AuthorID: post.AuthorID,
		}
		storedPost.ID = 1
		return nil
	}
	fetcher := &testutils.TitleFetcherStub{FetchTitleFn: func(ctx context.Context, rawURL string) (string, error) {
		return "Fetched Title", nil
	}}

	handler := NewPostHandler(repo, fetcher, nil)

	body := bytes.NewBufferString(`{"title":"  ","url":"https://example.com","content":"Go"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/posts", body)
	ctx := context.WithValue(request.Context(), middleware.UserIDKey, uint(42))
	request = request.WithContext(ctx)

	recorder := httptest.NewRecorder()
	handler.CreatePost(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusCreated)

	if storedPost == nil || storedPost.Title != "Fetched Title" || storedPost.AuthorID == nil || *storedPost.AuthorID != 42 {
		t.Fatalf("unexpected stored post: %#v", storedPost)
	}
}

func TestPostHandlerCreatePostValidation(t *testing.T) {
	handler := NewPostHandler(testutils.NewPostRepositoryStub(), &testutils.TitleFetcherStub{}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBufferString(`{"title":"","url":"","content":""}`))
	request = request.WithContext(context.WithValue(request.Context(), middleware.UserIDKey, uint(1)))
	handler.CreatePost(recorder, request)
	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing url, got %d", recorder.Result().StatusCode)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBufferString(`invalid json`))
	handler.CreatePost(recorder, request)
	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", recorder.Result().StatusCode)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBufferString(`{"title":"ok","url":"https://example.com"}`))
	handler.CreatePost(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusUnauthorized)
}

func TestPostHandlerCreatePostTitleFetcherErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{name: "Unsupported", err: services.ErrUnsupportedScheme, wantStatus: http.StatusBadRequest, wantMsg: "Only HTTP and HTTPS URLs are supported"},
		{name: "TitleMissing", err: services.ErrTitleNotFound, wantStatus: http.StatusBadRequest, wantMsg: "Title could not be extracted"},
		{name: "Generic", err: errors.New("timeout"), wantStatus: http.StatusBadGateway, wantMsg: "Failed to fetch title"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := testutils.NewPostRepositoryStub()
			fetcher := &testutils.TitleFetcherStub{FetchTitleFn: func(ctx context.Context, rawURL string) (string, error) {
				return "", tc.err
			}}
			handler := NewPostHandler(repo, fetcher, nil)
			body := bytes.NewBufferString(`{"title":" ","url":"https://example.com"}`)
			request := httptest.NewRequest(http.MethodPost, "/api/posts", body)
			request = request.WithContext(context.WithValue(request.Context(), middleware.UserIDKey, uint(1)))

			recorder := httptest.NewRecorder()
			handler.CreatePost(recorder, request)

			testutils.AssertHTTPStatus(t, recorder, tc.wantStatus)

			if !strings.Contains(recorder.Body.String(), tc.wantMsg) {
				t.Fatalf("expected message to contain %q, got %q", tc.wantMsg, recorder.Body.String())
			}
		})
	}
}

func TestPostHandlerFetchTitleFromURLErrors(t *testing.T) {
	handler := NewPostHandler(testutils.NewPostRepositoryStub(), nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/posts/title?url=https://example.com", nil)
	recorder := httptest.NewRecorder()
	handler.FetchTitleFromURL(recorder, request)
	if recorder.Result().StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501 when fetcher unavailable, got %d", recorder.Result().StatusCode)
	}

	handler = NewPostHandler(testutils.NewPostRepositoryStub(), &testutils.TitleFetcherStub{}, nil)
	request = httptest.NewRequest(http.MethodGet, "/api/posts/title", nil)
	recorder = httptest.NewRecorder()
	handler.FetchTitleFromURL(recorder, request)
	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing url query, got %d", recorder.Result().StatusCode)
	}

	handler = NewPostHandler(testutils.NewPostRepositoryStub(), &testutils.TitleFetcherStub{FetchTitleFn: func(ctx context.Context, rawURL string) (string, error) {
		return "", errors.New("failed")
	}}, nil)
	request = httptest.NewRequest(http.MethodGet, "/api/posts/title?url=https://example.com", nil)
	recorder = httptest.NewRecorder()
	handler.FetchTitleFromURL(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusBadGateway)
}

func TestTranslatePostCreateError(t *testing.T) {
	conflictErr := &pgconn.PgError{Code: "23505"}
	msg, status := translatePostCreateError(conflictErr)
	if status != http.StatusConflict || !strings.Contains(msg, "already been submitted") {
		t.Fatalf("unexpected conflict translation: status=%d msg=%q", status, msg)
	}

	fkErr := &pgconn.PgError{Code: "23503"}
	msg, status = translatePostCreateError(fkErr)
	if status != http.StatusUnauthorized || !strings.Contains(msg, "Author account not found") {
		t.Fatalf("unexpected foreign key translation: status=%d msg=%q", status, msg)
	}

	msg, status = translatePostCreateError(errors.New("other"))
	if status != 0 || msg != "" {
		t.Fatalf("expected passthrough for unrelated errors, got status=%d msg=%q", status, msg)
	}
}

func TestPostHandlerUpdatePost(t *testing.T) {
	tests := []struct {
		name           string
		postID         string
		requestBody    string
		userID         uint
		mockSetup      func(*testutils.PostRepositoryStub)
		expectedStatus int
		expectedError  string
	}{
		{
			name:        "valid post update",
			postID:      "1",
			requestBody: `{"title": "Updated Title", "content": "Updated content"}`,
			userID:      1,
			mockSetup: func(repo *testutils.PostRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.Post, error) {
					authorID := uint(1)
					return &database.Post{ID: id, Title: "Old Title", AuthorID: &authorID}, nil
				}
				repo.UpdateFn = func(post *database.Post) error { return nil }
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing user context",
			postID:         "1",
			requestBody:    `{"title": "Updated Title", "content": "Updated content"}`,
			userID:         0,
			mockSetup:      func(repo *testutils.PostRepositoryStub) {},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authentication required",
		},
		{
			name:        "post not found",
			postID:      "999",
			requestBody: `{"title": "Updated Title", "content": "Updated content"}`,
			userID:      1,
			mockSetup: func(repo *testutils.PostRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.Post, error) {
					return nil, gorm.ErrRecordNotFound
				}
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "Post not found",
		},
		{
			name:        "not author",
			postID:      "1",
			requestBody: `{"title": "Updated Title", "content": "Updated content"}`,
			userID:      2,
			mockSetup: func(repo *testutils.PostRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.Post, error) {
					authorID := uint(1)
					return &database.Post{ID: id, Title: "Old Title", AuthorID: &authorID}, nil
				}
			},
			expectedStatus: http.StatusForbidden,
			expectedError:  "You can only edit your own posts",
		},
		{
			name:        "empty title",
			postID:      "1",
			requestBody: `{"title": "", "content": "Updated content"}`,
			userID:      1,
			mockSetup: func(repo *testutils.PostRepositoryStub) {
				authorID := uint(1)
				repo.GetByIDFn = func(id uint) (*database.Post, error) {
					return &database.Post{ID: id, Title: "Old Title", AuthorID: &authorID}, nil
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Title is required",
		},
		{
			name:        "short title",
			postID:      "1",
			requestBody: `{"title": "ab", "content": "Updated content"}`,
			userID:      1,
			mockSetup: func(repo *testutils.PostRepositoryStub) {
				authorID := uint(1)
				repo.GetByIDFn = func(id uint) (*database.Post, error) {
					return &database.Post{ID: id, Title: "Old Title", AuthorID: &authorID}, nil
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Title must be at least 3 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testutils.NewPostRepositoryStub()
			if tt.mockSetup != nil {
				tt.mockSetup(repo)
			}
			handler := NewPostHandler(repo, &testutils.TitleFetcherStub{}, nil)

			request := httptest.NewRequest(http.MethodPut, "/api/posts/"+tt.postID, bytes.NewBufferString(tt.requestBody))
			if tt.userID > 0 {
				ctx := context.WithValue(request.Context(), middleware.UserIDKey, tt.userID)
				request = request.WithContext(ctx)
			}

			ctx := chi.NewRouteContext()
			ctx.URLParams.Add("id", tt.postID)
			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, ctx))

			recorder := httptest.NewRecorder()

			handler.UpdatePost(recorder, request)

			testutils.AssertHTTPStatus(t, recorder, tt.expectedStatus)

			if tt.expectedError != "" {
				if !strings.Contains(recorder.Body.String(), tt.expectedError) {
					t.Fatalf("expected error to contain %q, got %q", tt.expectedError, recorder.Body.String())
				}
			}
		})
	}
}

func TestPostHandlerDeletePost(t *testing.T) {
	tests := []struct {
		name           string
		postID         string
		userID         uint
		mockSetup      func(*testutils.PostRepositoryStub)
		expectedStatus int
		expectedError  string
	}{
		{
			name:   "valid post deletion",
			postID: "1",
			userID: 1,
			mockSetup: func(repo *testutils.PostRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.Post, error) {
					authorID := uint(1)
					return &database.Post{ID: id, Title: "Test Post", AuthorID: &authorID}, nil
				}
				repo.DeleteFn = func(id uint) error { return nil }
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing user context",
			postID:         "1",
			userID:         0,
			mockSetup:      func(repo *testutils.PostRepositoryStub) {},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authentication required",
		},
		{
			name:   "post not found",
			postID: "999",
			userID: 1,
			mockSetup: func(repo *testutils.PostRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.Post, error) {
					return nil, gorm.ErrRecordNotFound
				}
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "Post not found",
		},
		{
			name:   "not author",
			postID: "1",
			userID: 2,
			mockSetup: func(repo *testutils.PostRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.Post, error) {
					authorID := uint(1)
					return &database.Post{ID: id, Title: "Test Post", AuthorID: &authorID}, nil
				}
			},
			expectedStatus: http.StatusForbidden,
			expectedError:  "You can only delete your own posts",
		},
		{
			name:   "delete error",
			postID: "1",
			userID: 1,
			mockSetup: func(repo *testutils.PostRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.Post, error) {
					authorID := uint(1)
					return &database.Post{ID: id, Title: "Test Post", AuthorID: &authorID}, nil
				}
				repo.DeleteFn = func(id uint) error { return errors.New("database error") }
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Failed to delete post",
		},
		{
			name:   "delete votes error",
			postID: "1",
			userID: 1,
			mockSetup: func(repo *testutils.PostRepositoryStub) {
				repo.GetByIDFn = func(id uint) (*database.Post, error) {
					authorID := uint(1)
					return &database.Post{ID: id, Title: "Test Post", AuthorID: &authorID}, nil
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Failed to delete post votes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testutils.NewPostRepositoryStub()
			if tt.mockSetup != nil {
				tt.mockSetup(repo)
			}

			var voteService *services.VoteService
			if tt.name == "delete votes error" {
				voteRepo := &errorVoteRepository{}
				voteService = services.NewVoteService(voteRepo, repo, nil)
			} else {
				voteRepo := testutils.NewMockVoteRepository()
				voteService = services.NewVoteService(voteRepo, repo, nil)
			}

			handler := NewPostHandler(repo, &testutils.TitleFetcherStub{}, voteService)

			request := httptest.NewRequest(http.MethodDelete, "/api/posts/"+tt.postID, nil)
			if tt.userID > 0 {
				ctx := context.WithValue(request.Context(), middleware.UserIDKey, tt.userID)
				request = request.WithContext(ctx)
			}

			ctx := chi.NewRouteContext()
			ctx.URLParams.Add("id", tt.postID)
			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, ctx))

			recorder := httptest.NewRecorder()

			handler.DeletePost(recorder, request)

			testutils.AssertHTTPStatus(t, recorder, tt.expectedStatus)

			if tt.expectedError != "" {
				if !strings.Contains(recorder.Body.String(), tt.expectedError) {
					t.Fatalf("expected error to contain %q, got %q", tt.expectedError, recorder.Body.String())
				}
			}
		})
	}
}

type errorVoteRepository struct{}

func (e *errorVoteRepository) Create(*database.Vote) error         { return nil }
func (e *errorVoteRepository) CreateOrUpdate(*database.Vote) error { return nil }
func (e *errorVoteRepository) GetByID(uint) (*database.Vote, error) {
	return nil, gorm.ErrRecordNotFound
}
func (e *errorVoteRepository) GetByUserAndPost(uint, uint) (*database.Vote, error) {
	return nil, gorm.ErrRecordNotFound
}
func (e *errorVoteRepository) GetByVoteHash(string) (*database.Vote, error) {
	return nil, gorm.ErrRecordNotFound
}
func (e *errorVoteRepository) GetByPostID(uint) ([]database.Vote, error) {
	return nil, errors.New("database error")
}
func (e *errorVoteRepository) GetByUserID(uint) ([]database.Vote, error)   { return nil, nil }
func (e *errorVoteRepository) Update(*database.Vote) error                 { return nil }
func (e *errorVoteRepository) Delete(uint) error                           { return nil }
func (e *errorVoteRepository) Count() (int64, error)                       { return 0, nil }
func (e *errorVoteRepository) CountByPostID(uint) (int64, error)           { return 0, nil }
func (e *errorVoteRepository) CountByUserID(uint) (int64, error)           { return 0, nil }
func (e *errorVoteRepository) WithTx(*gorm.DB) repositories.VoteRepository { return e }

func TestPostHandler_EdgeCases(t *testing.T) {
	postRepo := testutils.NewPostRepositoryStub()
	titleFetcher := &testutils.TitleFetcherStub{}
	handler := NewPostHandler(postRepo, titleFetcher, nil)

	t.Run("GetPosts with zero limit", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts?limit=0", nil)
		w := httptest.NewRecorder()

		handler.GetPosts(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for zero limit, got %d", w.Code)
		}
	})

	t.Run("GetPosts with negative limit", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts?limit=-1", nil)
		w := httptest.NewRecorder()

		handler.GetPosts(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for negative limit, got %d", w.Code)
		}
	})

	t.Run("GetPosts with negative offset", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts?offset=-1", nil)
		w := httptest.NewRecorder()

		handler.GetPosts(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for negative offset, got %d", w.Code)
		}
	})
}
