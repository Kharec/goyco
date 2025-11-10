package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestInMemoryDBMonitor(t *testing.T) {
	monitor := NewInMemoryDBMonitor()

	stats := monitor.GetStats()
	if stats.TotalQueries != 0 {
		t.Errorf("Expected 0 total queries, got %d", stats.TotalQueries)
	}

	monitor.LogQuery("SELECT * FROM users", 50*time.Millisecond, nil)
	stats = monitor.GetStats()
	if stats.TotalQueries != 1 {
		t.Errorf("Expected 1 total query, got %d", stats.TotalQueries)
	}
	if stats.AverageDuration != 50*time.Millisecond {
		t.Errorf("Expected average duration 50ms, got %v", stats.AverageDuration)
	}
	if stats.MaxDuration != 50*time.Millisecond {
		t.Errorf("Expected max duration 50ms, got %v", stats.MaxDuration)
	}

	monitor.LogQuery("SELECT * FROM posts", 150*time.Millisecond, nil)
	stats = monitor.GetStats()
	if stats.TotalQueries != 2 {
		t.Errorf("Expected 2 total queries, got %d", stats.TotalQueries)
	}
	if stats.SlowQueries != 1 {
		t.Errorf("Expected 1 slow query, got %d", stats.SlowQueries)
	}

	monitor.LogQuery("SELECT * FROM invalid", 10*time.Millisecond, sql.ErrNoRows)
	stats = monitor.GetStats()
	if stats.TotalQueries != 3 {
		t.Errorf("Expected 3 total queries, got %d", stats.TotalQueries)
	}
	if stats.ErrorCount != 1 {
		t.Errorf("Expected 1 error, got %d", stats.ErrorCount)
	}

	expectedAvg := time.Duration((int64(50*time.Millisecond) + int64(150*time.Millisecond)) / 2)
	if stats.AverageDuration != expectedAvg {
		t.Errorf("Expected average duration %v, got %v", expectedAvg, stats.AverageDuration)
	}
}

func TestQueryLogger(t *testing.T) {

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	monitor := NewInMemoryDBMonitor()
	logger := NewQueryLogger(db, monitor)

	ctx := context.Background()
	rows, err := logger.QueryContext(ctx, "SELECT * FROM users")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if rows == nil {
		t.Fatal("Expected rows, got nil")
	}
	rows.Close()

	stats := monitor.GetStats()
	if stats.TotalQueries != 1 {
		t.Errorf("Expected 1 total query, got %d", stats.TotalQueries)
	}

	row := logger.QueryRowContext(ctx, "SELECT * FROM users WHERE id = ?", 1)
	if row == nil {
		t.Fatal("Expected row, got nil")
	}

	stats = monitor.GetStats()
	if stats.TotalQueries != 2 {
		t.Errorf("Expected 2 total queries, got %d", stats.TotalQueries)
	}

	_, err = logger.ExecContext(ctx, "INSERT INTO users (name) VALUES (?)", "test")

	if err == nil {
		t.Fatal("Expected error for INSERT into non-existent table")
	}

	stats = monitor.GetStats()
	if stats.TotalQueries != 3 {
		t.Errorf("Expected 3 total queries, got %d", stats.TotalQueries)
	}
	if stats.ErrorCount != 1 {
		t.Errorf("Expected 1 error, got %d", stats.ErrorCount)
	}
}

func TestDatabaseHealthChecker(t *testing.T) {

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	monitor := NewInMemoryDBMonitor()
	checker := NewDatabaseHealthChecker(db, monitor)

	health := checker.CheckHealth()
	if health["status"] != "healthy" {
		t.Errorf("Expected healthy status, got %v", health["status"])
	}
	if health["ping_time"] == nil {
		t.Error("Expected ping_time to be present")
	}

	monitor.LogQuery("SELECT * FROM users", 50*time.Millisecond, nil)
	monitor.LogQuery("SELECT * FROM posts", 150*time.Millisecond, nil)

	health = checker.CheckHealth()
	if health["database_stats"] == nil {
		t.Error("Expected database_stats to be present")
	}

	stats, ok := health["database_stats"].(map[string]any)
	if !ok {
		t.Fatal("Expected database_stats to be a map")
	}

	if stats["total_queries"] != int64(2) {
		t.Errorf("Expected 2 total queries, got %v", stats["total_queries"])
	}
	if stats["slow_queries"] != int64(1) {
		t.Errorf("Expected 1 slow query, got %v", stats["slow_queries"])
	}
}

func TestMetricsCollector(t *testing.T) {
	monitor := NewInMemoryDBMonitor()
	collector := NewMetricsCollector(monitor)

	metrics := collector.GetMetrics()
	if metrics.RequestCount != 0 {
		t.Errorf("Expected 0 requests, got %d", metrics.RequestCount)
	}

	collector.RecordRequest(100*time.Millisecond, false)
	collector.RecordRequest(200*time.Millisecond, false)
	collector.RecordRequest(50*time.Millisecond, true)

	metrics = collector.GetMetrics()
	if metrics.RequestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", metrics.RequestCount)
	}
	if metrics.ErrorCount != 1 {
		t.Errorf("Expected 1 error, got %d", metrics.ErrorCount)
	}
	if metrics.MaxResponse != 200*time.Millisecond {
		t.Errorf("Expected max response 200ms, got %v", metrics.MaxResponse)
	}

	expectedAvg := time.Duration((int64(100*time.Millisecond) + int64(200*time.Millisecond) + int64(50*time.Millisecond)) / 3)
	if metrics.AverageResponse != expectedAvg {
		t.Errorf("Expected average response %v, got %v", expectedAvg, metrics.AverageResponse)
	}
}

func TestMetricsMiddleware(t *testing.T) {
	monitor := NewInMemoryDBMonitor()
	collector := NewMetricsCollector(monitor)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	middleware := MetricsMiddleware(collector)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	metrics := collector.GetMetrics()
	if metrics.RequestCount != 1 {
		t.Errorf("Expected 1 request, got %d", metrics.RequestCount)
	}
	if metrics.ErrorCount != 0 {
		t.Errorf("Expected 0 errors, got %d", metrics.ErrorCount)
	}

	errorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})

	errorMiddleware := MetricsMiddleware(collector)
	errorWrappedHandler := errorMiddleware(errorHandler)

	req = httptest.NewRequest("GET", "/error", nil)
	w = httptest.NewRecorder()
	errorWrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	metrics = collector.GetMetrics()
	if metrics.RequestCount != 2 {
		t.Errorf("Expected 2 requests, got %d", metrics.RequestCount)
	}
	if metrics.ErrorCount != 1 {
		t.Errorf("Expected 1 error, got %d", metrics.ErrorCount)
	}
}

func TestDBMonitoringMiddleware(t *testing.T) {
	monitor := NewInMemoryDBMonitor()
	threshold := 50 * time.Millisecond

	var capturedCtx context.Context
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()

		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	middleware := DBMonitoringMiddleware(monitor, threshold)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if capturedCtx == nil {
		t.Fatal("Expected context to be captured")
	}
	if capturedCtx.Value(dbMonitorKey) == nil {
		t.Error("Expected dbMonitorKey to be set in context")
	}
	if capturedCtx.Value(slowQueryThresholdKey) == nil {
		t.Error("Expected slowQueryThresholdKey to be set in context")
	}

	actualThreshold := capturedCtx.Value(slowQueryThresholdKey).(time.Duration)
	if actualThreshold != threshold {
		t.Errorf("Expected threshold %v, got %v", threshold, actualThreshold)
	}
}

func TestMetricsResponseWriter(t *testing.T) {
	recorder := httptest.NewRecorder()
	writer := &metricsResponseWriter{
		ResponseWriter: recorder,
		statusCode:     http.StatusOK,
	}

	writer.WriteHeader(http.StatusNotFound)
	if writer.statusCode != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, writer.statusCode)
	}

	if recorder.Code != http.StatusNotFound {
		t.Errorf("Expected underlying writer to receive status %d, got %d", http.StatusNotFound, recorder.Code)
	}
}

func TestSlowQueryThreshold(t *testing.T) {
	monitor := NewInMemoryDBMonitor()

	monitor.LogQuery("SELECT * FROM users", 50*time.Millisecond, nil)
	monitor.LogQuery("SELECT * FROM posts", 150*time.Millisecond, nil)
	monitor.LogQuery("SELECT * FROM comments", 200*time.Millisecond, nil)

	stats := monitor.GetStats()
	if stats.SlowQueries != 2 {
		t.Errorf("Expected 2 slow queries with default 100ms threshold, got %d", stats.SlowQueries)
	}

	monitor2 := NewInMemoryDBMonitor()
	monitor2.LogQuery("SELECT * FROM users", 50*time.Millisecond, nil)
	monitor2.LogQuery("SELECT * FROM posts", 150*time.Millisecond, nil)

	stats2 := monitor2.GetStats()
	if stats2.SlowQueries != 1 {
		t.Errorf("Expected 1 slow query with default 100ms threshold, got %d", stats2.SlowQueries)
	}
}

func TestConcurrentAccess(t *testing.T) {
	monitor := NewInMemoryDBMonitor()
	collector := NewMetricsCollector(monitor)

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			monitor.LogQuery("SELECT * FROM users", 50*time.Millisecond, nil)
			collector.RecordRequest(100*time.Millisecond, false)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats := monitor.GetStats()
	if stats.TotalQueries != 10 {
		t.Errorf("Expected 10 total queries, got %d", stats.TotalQueries)
	}

	metrics := collector.GetMetrics()
	if metrics.RequestCount != 10 {
		t.Errorf("Expected 10 requests, got %d", metrics.RequestCount)
	}
}

func TestContextHelpers(t *testing.T) {
	monitor := NewInMemoryDBMonitor()
	threshold := 200 * time.Millisecond

	ctx := context.Background()
	ctx = context.WithValue(ctx, dbMonitorKey, monitor)
	ctx = context.WithValue(ctx, slowQueryThresholdKey, threshold)

	retrievedMonitor, ok := GetDBMonitorFromContext(ctx)
	if !ok {
		t.Error("Expected to retrieve monitor from context")
	}
	if retrievedMonitor != monitor {
		t.Error("Expected retrieved monitor to match original")
	}

	retrievedThreshold, ok := GetSlowQueryThresholdFromContext(ctx)
	if !ok {
		t.Error("Expected to retrieve threshold from context")
	}
	if retrievedThreshold != threshold {
		t.Errorf("Expected threshold %v, got %v", threshold, retrievedThreshold)
	}

	emptyCtx := context.Background()
	_, ok = GetDBMonitorFromContext(emptyCtx)
	if ok {
		t.Error("Expected not to retrieve monitor from empty context")
	}

	_, ok = GetSlowQueryThresholdFromContext(emptyCtx)
	if ok {
		t.Error("Expected not to retrieve threshold from empty context")
	}
}

func TestThreadSafety(t *testing.T) {
	monitor := NewInMemoryDBMonitor()
	collector := NewMetricsCollector(monitor)

	numGoroutines := 100
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {

			if id%2 == 0 {
				monitor.LogQuery("SELECT * FROM users", time.Duration(id)*time.Millisecond, nil)
				collector.RecordRequest(time.Duration(id)*time.Millisecond, false)
			} else {
				monitor.LogQuery("SELECT * FROM users", time.Duration(id)*time.Millisecond, sql.ErrNoRows)
				collector.RecordRequest(time.Duration(id)*time.Millisecond, true)
			}
			done <- true
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	stats := monitor.GetStats()
	if stats.TotalQueries != int64(numGoroutines) {
		t.Errorf("Expected %d total queries, got %d", numGoroutines, stats.TotalQueries)
	}

	metrics := collector.GetMetrics()
	if metrics.RequestCount != int64(numGoroutines) {
		t.Errorf("Expected %d requests, got %d", numGoroutines, metrics.RequestCount)
	}

	expectedErrors := int64(numGoroutines / 2)
	if stats.ErrorCount != expectedErrors {
		t.Errorf("Expected %d errors, got %d", expectedErrors, stats.ErrorCount)
	}
	if metrics.ErrorCount != expectedErrors {
		t.Errorf("Expected %d request errors, got %d", expectedErrors, metrics.ErrorCount)
	}
}
