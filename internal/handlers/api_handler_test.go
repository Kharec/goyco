package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"goyco/internal/database"
	"goyco/internal/middleware"
	"goyco/internal/repositories"
	"goyco/internal/services"
	"goyco/internal/testutils"
)

func TestAPIHandlerGetAPIInfo(t *testing.T) {
	mockPostRepo := testutils.NewPostRepositoryStub()
	mockUserRepo := testutils.NewUserRepositoryStub()
	handler := newAPIHandlerForTest(mockPostRepo, mockUserRepo)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api", nil)

	handler.GetAPIInfo(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	var resp APIInfo
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success || resp.Message == "" {
		t.Fatalf("expected success response, got %+v", resp)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok || data["name"] != fmt.Sprintf("%s API", testutils.AppTestConfig.App.Title) {
		t.Fatalf("unexpected data payload: %#v", resp.Data)
	}

	endpoints, ok := data["endpoints"].(map[string]any)
	if !ok {
		t.Fatalf("expected endpoints map, got %#v", data["endpoints"])
	}

	authEndpoints := endpoints["authentication"].(map[string]any)
	for _, route := range []string{
		"POST /api/auth/resend-verification",
		"POST /api/auth/account/confirm",
	} {
		if _, found := authEndpoints[route]; !found {
			t.Fatalf("expected authentication catalogue to include %s", route)
		}
	}

	systemEndpoints := endpoints["system"].(map[string]any)
	if _, found := systemEndpoints["GET /metrics"]; !found {
		t.Fatalf("expected system catalogue to include GET /metrics")
	}
}

func TestAPIHandlerGetHealth(t *testing.T) {
	mockPostRepo := testutils.NewPostRepositoryStub()
	mockUserRepo := testutils.NewUserRepositoryStub()
	handler := newAPIHandlerForTest(mockPostRepo, mockUserRepo)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	handler.GetHealth(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	var resp APIInfo
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if !resp.Success || resp.Message == "" {
		t.Fatalf("expected success message, got %+v", resp)
	}

	data := resp.Data.(map[string]any)
	if data["status"] != "healthy" {
		t.Fatalf("expected health status, got %+v", data)
	}
}

func TestAPIHandlerGetMetrics(t *testing.T) {
	mockPostRepo := testutils.NewPostRepositoryStub()
	mockPostRepo.CountFn = func() (int64, error) { return 10, nil }
	mockPostRepo.GetTopPostsFn = func(limit int) ([]database.Post, error) {
		return []database.Post{
			{ID: 1, Score: 100},
			{ID: 2, Score: 50},
			{ID: 3, Score: 25},
		}, nil
	}

	mockUserRepo := testutils.NewUserRepositoryStub()
	mockUserRepo.CountFn = func() (int64, error) { return 5, nil }

	handler := newAPIHandlerForTest(mockPostRepo, mockUserRepo)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	handler.GetMetrics(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	var resp APIInfo
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if !resp.Success || resp.Message == "" {
		t.Fatalf("expected success response, got %+v", resp)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected metrics data map, got %T", resp.Data)
	}

	if data["posts"] == nil {
		t.Fatalf("expected metrics payload to include posts")
	}
	if data["users"] == nil {
		t.Fatalf("expected metrics payload to include users")
	}
	if data["votes"] == nil {
		t.Fatalf("expected metrics payload to include votes")
	}
	if data["system"] == nil {
		t.Fatalf("expected metrics payload to include system")
	}

	posts, ok := data["posts"].(map[string]any)
	if !ok {
		t.Fatalf("expected posts to be a map, got %T", data["posts"])
	}
	if posts["total_count"] != float64(10) {
		t.Fatalf("expected posts total_count to be 10, got %v", posts["total_count"])
	}
}

func newAPIHandlerForTest(postRepo repositories.PostRepository, userRepo repositories.UserRepository) *APIHandler {
	voteRepo := testutils.NewMockVoteRepository()
	voteService := services.NewVoteService(voteRepo, postRepo, nil)
	return NewAPIHandler(testutils.AppTestConfig, postRepo, userRepo, voteService)
}

func TestAPIHandlerGetMetricsErrorHandling(t *testing.T) {
	mockPostRepo := testutils.NewPostRepositoryStub()
	mockPostRepo.CountFn = func() (int64, error) { return 0, errors.New("database error") }

	mockUserRepo := testutils.NewUserRepositoryStub()
	handler := newAPIHandlerForTest(mockPostRepo, mockUserRepo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	handler.GetMetrics(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusInternalServerError)

	var resp APIInfo
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.Success {
		t.Fatalf("expected error response, got %+v", resp)
	}
}

func TestAPIHandlerGetMetricsWithDatabaseMonitoring(t *testing.T) {
	mockPostRepo := testutils.NewPostRepositoryStub()
	mockPostRepo.CountFn = func() (int64, error) { return 10, nil }
	mockPostRepo.GetTopPostsFn = func(limit int) ([]database.Post, error) {
		return []database.Post{
			{ID: 1, Score: 100},
			{ID: 2, Score: 50},
		}, nil
	}

	mockUserRepo := testutils.NewUserRepositoryStub()
	mockUserRepo.CountFn = func() (int64, error) { return 5, nil }

	voteRepo := testutils.NewMockVoteRepository()
	voteService := services.NewVoteService(voteRepo, mockPostRepo, nil)

	handler := NewAPIHandler(testutils.AppTestConfig, mockPostRepo, mockUserRepo, voteService)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	handler.GetMetrics(recorder, request)

	testutils.AssertHTTPStatus(t, recorder, http.StatusOK)

	var resp APIInfo
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success response, got %+v", resp)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected metrics data map, got %T", resp.Data)
	}

	expectedSections := []string{"posts", "users", "votes", "system"}
	for _, section := range expectedSections {
		if data[section] == nil {
			t.Fatalf("expected metrics payload to include %s", section)
		}
	}
}

func TestNewAPIHandlerWithMonitoring(t *testing.T) {
	mockPostRepo := testutils.NewPostRepositoryStub()
	mockUserRepo := testutils.NewUserRepositoryStub()
	voteRepo := testutils.NewMockVoteRepository()
	voteService := services.NewVoteService(voteRepo, mockPostRepo, nil)
	monitor := middleware.NewInMemoryDBMonitor()

	db := testutils.NewTestDB(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	handler := NewAPIHandlerWithMonitoring(testutils.AppTestConfig, mockPostRepo, mockUserRepo, voteService, db, monitor)

	if handler == nil {
		t.Fatal("Expected handler to be created")
	}

	if handler.dbMonitor == nil {
		t.Error("Expected dbMonitor to be set")
	}

	if handler.healthChecker == nil {
		t.Error("Expected healthChecker to be set")
	}

	if handler.metricsCollector == nil {
		t.Error("Expected metricsCollector to be set")
	}
}

func TestNewAPIHandlerWithMonitoring_NilDB(t *testing.T) {
	mockPostRepo := testutils.NewPostRepositoryStub()
	mockUserRepo := testutils.NewUserRepositoryStub()
	voteRepo := testutils.NewMockVoteRepository()
	voteService := services.NewVoteService(voteRepo, mockPostRepo, nil)

	handler := NewAPIHandlerWithMonitoring(testutils.AppTestConfig, mockPostRepo, mockUserRepo, voteService, nil, nil)

	if handler == nil {
		t.Fatal("Expected handler to be created")
	}

	if handler.dbMonitor != nil {
		t.Error("Expected dbMonitor to be nil when db is nil")
	}

	if handler.healthChecker != nil {
		t.Error("Expected healthChecker to be nil when db is nil")
	}

	if handler.metricsCollector != nil {
		t.Error("Expected metricsCollector to be nil when db is nil")
	}
}
