package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/gorm"
	"goyco/internal/database"
	"goyco/internal/middleware"
	"goyco/internal/services"
	"goyco/internal/testutils"
)

func newVoteHandlerWithRepos() *VoteHandler {
	handler, _, _ := newVoteHandlerWithReposRefs()
	return handler
}

func newVoteHandlerWithReposRefs() (*VoteHandler, *testutils.MockVoteRepository, map[uint]*database.Post) {
	voteRepo := testutils.NewMockVoteRepository()
	posts := map[uint]*database.Post{
		1: {ID: 1},
	}
	postRepo := testutils.NewPostRepositoryStub()
	postRepo.GetByIDFn = func(id uint) (*database.Post, error) {
		if post, ok := posts[id]; ok {
			copy := *post
			return &copy, nil
		}
		return nil, gorm.ErrRecordNotFound
	}
	postRepo.UpdateFn = func(post *database.Post) error {
		copy := *post
		posts[post.ID] = &copy
		return nil
	}
	postRepo.DeleteFn = func(id uint) error {
		if _, ok := posts[id]; !ok {
			return gorm.ErrRecordNotFound
		}
		delete(posts, id)
		return nil
	}
	postRepo.CreateFn = func(post *database.Post) error {
		copy := *post
		posts[post.ID] = &copy
		return nil
	}
	service := services.NewVoteService(voteRepo, postRepo, nil)
	return NewVoteHandler(service), voteRepo, posts
}

func TestVoteHandlerCastVote(t *testing.T) {
	handler := newVoteHandlerWithRepos()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"up"}`))
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	handler.CastVote(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusUnauthorized)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/posts/abc/vote", bytes.NewBufferString(`{"type":"up"}`))
	request = testutils.WithURLParams(request, map[string]string{"id": "abc"})
	ctx := context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	handler.CastVote(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`invalid`))
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	handler.CastVote(recorder, request)
	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid json, got %d", recorder.Result().StatusCode)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"maybe"}`))
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	handler.CastVote(recorder, request)
	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid vote type, got %d", recorder.Result().StatusCode)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"up"}`))
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	handler.CastVote(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"down"}`))
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(2))
	request = request.WithContext(ctx)
	handler.CastVote(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for successful down vote, got %d", recorder.Result().StatusCode)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"none"}`))
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(3))
	request = request.WithContext(ctx)
	handler.CastVote(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for successful none vote, got %d", recorder.Result().StatusCode)
	}
}

func TestVoteHandlerCastVotePostNotFound(t *testing.T) {
	handler, _, posts := newVoteHandlerWithReposRefs()
	delete(posts, 1)

	request := httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"up"}`))
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx := context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	recorder := httptest.NewRecorder()

	handler.CastVote(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusNotFound)
}

func TestVoteHandlerRemoveVote(t *testing.T) {
	handler := newVoteHandlerWithRepos()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/posts/1/vote", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	handler.RemoveVote(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusUnauthorized)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/api/posts/abc/vote", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "abc"})
	ctx := context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	handler.RemoveVote(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/api/posts/1/vote", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	handler.RemoveVote(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for removing non-existent vote (idempotent), got %d", recorder.Result().StatusCode)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"up"}`))
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	handler.CastVote(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for creating vote, got %d", recorder.Result().StatusCode)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/api/posts/1/vote", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	handler.RemoveVote(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 when removing vote, got %d", recorder.Result().StatusCode)
	}
}

func TestVoteHandlerRemoveVotePostNotFound(t *testing.T) {
	handler, _, posts := newVoteHandlerWithReposRefs()
	delete(posts, 1)

	request := httptest.NewRequest(http.MethodDelete, "/api/posts/1/vote", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx := context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	recorder := httptest.NewRecorder()

	handler.RemoveVote(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusNotFound)
}

func TestVoteHandlerRemoveVoteUnexpectedError(t *testing.T) {
	handler, voteRepo, _ := newVoteHandlerWithReposRefs()

	request := httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"up"}`))
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx := context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	recorder := httptest.NewRecorder()
	handler.CastVote(recorder, request)

	voteRepo.DeleteErr = fmt.Errorf("database unavailable")

	request = httptest.NewRequest(http.MethodDelete, "/api/posts/1/vote", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	recorder = httptest.NewRecorder()

	handler.RemoveVote(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusInternalServerError)
}

func TestVoteHandlerGetUserVote(t *testing.T) {
	handler := newVoteHandlerWithRepos()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/posts/1/vote", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	handler.GetUserVote(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusUnauthorized)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/posts/abc/vote", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "abc"})
	ctx := context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	handler.GetUserVote(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/posts/1/vote", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	handler.GetUserVote(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 when vote missing, got %d", recorder.Result().StatusCode)
	}

	var resp VoteResponse
	_ = json.NewDecoder(recorder.Body).Decode(&resp)
	data := resp.Data.(map[string]any)
	if data["has_vote"].(bool) {
		t.Fatalf("expected has_vote false, got true")
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"up"}`))
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	handler.CastVote(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for creating vote, got %d", recorder.Result().StatusCode)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/posts/1/vote", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	handler.GetUserVote(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 when vote exists, got %d", recorder.Result().StatusCode)
	}

	_ = json.NewDecoder(recorder.Body).Decode(&resp)
	data = resp.Data.(map[string]any)
	if !data["has_vote"].(bool) {
		t.Fatalf("expected has_vote true, got false")
	}
}

func TestVoteHandlerGetPostVotes(t *testing.T) {
	handler := newVoteHandlerWithRepos()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/posts/abc/votes", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "abc"})
	handler.GetPostVotes(recorder, request)
	testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/posts/1/votes", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	handler.GetPostVotes(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for empty votes, got %d", recorder.Result().StatusCode)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"up"}`))
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx := context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
	request = request.WithContext(ctx)
	handler.CastVote(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for creating vote, got %d", recorder.Result().StatusCode)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"down"}`))
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(2))
	request = request.WithContext(ctx)
	handler.CastVote(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for creating vote, got %d", recorder.Result().StatusCode)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/posts/1/votes", nil)
	request = testutils.WithURLParams(request, map[string]string{"id": "1"})
	handler.GetPostVotes(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}

	var resp VoteResponse
	_ = json.NewDecoder(recorder.Body).Decode(&resp)
	data := resp.Data.(map[string]any)
	votes := data["votes"].([]any)
	if len(votes) != 2 {
		t.Fatalf("expected 2 votes, got %d", len(votes))
	}
}

func TestVoteFlowRegression(t *testing.T) {
	handler := newVoteHandlerWithRepos()

	t.Run("CompleteVoteLifecycle", func(t *testing.T) {
		userID := uint(1)
		postID := "1"

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"up"}`))
		request = testutils.WithURLParams(request, map[string]string{"id": postID})
		ctx := context.WithValue(request.Context(), middleware.UserIDKey, userID)
		request = request.WithContext(ctx)
		handler.CastVote(recorder, request)
		testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

		recorder = httptest.NewRecorder()
		request = httptest.NewRequest(http.MethodGet, "/api/posts/1/vote", nil)
		request = testutils.WithURLParams(request, map[string]string{"id": postID})
		ctx = context.WithValue(request.Context(), middleware.UserIDKey, userID)
		request = request.WithContext(ctx)
		handler.GetUserVote(recorder, request)
		if recorder.Result().StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for getting vote, got %d", recorder.Result().StatusCode)
		}

		recorder = httptest.NewRecorder()
		request = httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"down"}`))
		request = testutils.WithURLParams(request, map[string]string{"id": postID})
		ctx = context.WithValue(request.Context(), middleware.UserIDKey, userID)
		request = request.WithContext(ctx)
		handler.CastVote(recorder, request)
		if recorder.Result().StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for changing to downvote, got %d", recorder.Result().StatusCode)
		}

		recorder = httptest.NewRecorder()
		request = httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"none"}`))
		request = testutils.WithURLParams(request, map[string]string{"id": postID})
		ctx = context.WithValue(request.Context(), middleware.UserIDKey, userID)
		request = request.WithContext(ctx)
		handler.CastVote(recorder, request)
		if recorder.Result().StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for removing vote, got %d", recorder.Result().StatusCode)
		}

		recorder = httptest.NewRecorder()
		request = httptest.NewRequest(http.MethodGet, "/api/posts/1/vote", nil)
		request = testutils.WithURLParams(request, map[string]string{"id": postID})
		ctx = context.WithValue(request.Context(), middleware.UserIDKey, userID)
		request = request.WithContext(ctx)
		handler.GetUserVote(recorder, request)
		if recorder.Result().StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for getting removed vote, got %d", recorder.Result().StatusCode)
		}

		var resp VoteResponse
		_ = json.NewDecoder(recorder.Body).Decode(&resp)
		data := resp.Data.(map[string]any)
		if data["has_vote"].(bool) {
			t.Fatalf("expected has_vote false after removal, got true")
		}
	})

	t.Run("MultipleUsersVoting", func(t *testing.T) {
		postID := "1"

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"up"}`))
		request = testutils.WithURLParams(request, map[string]string{"id": postID})
		ctx := context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
		request = request.WithContext(ctx)
		handler.CastVote(recorder, request)
		if recorder.Result().StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for user 1 upvote, got %d", recorder.Result().StatusCode)
		}

		recorder = httptest.NewRecorder()
		request = httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"down"}`))
		request = testutils.WithURLParams(request, map[string]string{"id": postID})
		ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(2))
		request = request.WithContext(ctx)
		handler.CastVote(recorder, request)
		if recorder.Result().StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for user 2 downvote, got %d", recorder.Result().StatusCode)
		}

		recorder = httptest.NewRecorder()
		request = httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"up"}`))
		request = testutils.WithURLParams(request, map[string]string{"id": postID})
		ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(3))
		request = request.WithContext(ctx)
		handler.CastVote(recorder, request)
		if recorder.Result().StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for user 3 upvote, got %d", recorder.Result().StatusCode)
		}

		recorder = httptest.NewRecorder()
		request = httptest.NewRequest(http.MethodGet, "/api/posts/1/votes", nil)
		request = testutils.WithURLParams(request, map[string]string{"id": postID})
		handler.GetPostVotes(recorder, request)
		if recorder.Result().StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for getting all votes, got %d", recorder.Result().StatusCode)
		}

		var resp VoteResponse
		_ = json.NewDecoder(recorder.Body).Decode(&resp)
		data := resp.Data.(map[string]any)
		votes := data["votes"].([]any)
		if len(votes) != 3 {
			t.Fatalf("expected 3 votes, got %d", len(votes))
		}
	})

	t.Run("ErrorHandlingEdgeCases", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(``))
		request = testutils.WithURLParams(request, map[string]string{"id": "1"})
		ctx := context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
		request = request.WithContext(ctx)
		handler.CastVote(recorder, request)
		testutils.AssertHTTPStatus(t, recorder, http.StatusBadRequest)

		recorder = httptest.NewRecorder()
		request = httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{}`))
		request = testutils.WithURLParams(request, map[string]string{"id": "1"})
		ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
		request = request.WithContext(ctx)
		handler.CastVote(recorder, request)
		if recorder.Result().StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing type field, got %d", recorder.Result().StatusCode)
		}

		recorder = httptest.NewRecorder()
		request = httptest.NewRequest(http.MethodPost, "/api/posts/1/vote", bytes.NewBufferString(`{"type":"invalid"}`))
		request = testutils.WithURLParams(request, map[string]string{"id": "1"})
		ctx = context.WithValue(request.Context(), middleware.UserIDKey, uint(1))
		request = request.WithContext(ctx)
		handler.CastVote(recorder, request)
		if recorder.Result().StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid vote type, got %d", recorder.Result().StatusCode)
		}
	})
}
