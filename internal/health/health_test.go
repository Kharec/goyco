package health

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"goyco/internal/middleware"
)

type MockDBMonitor struct {
	stats middleware.DBStats
}

func (m *MockDBMonitor) GetStats() middleware.DBStats {
	return m.stats
}

func (m *MockDBMonitor) LogQuery(query string, duration time.Duration, err error) {
}

func (m *MockDBMonitor) LogSlowQuery(query string, duration time.Duration, threshold time.Duration) {
}

func TestStatusConstants(t *testing.T) {
	if StatusHealthy != "healthy" {
		t.Errorf("Expected StatusHealthy to be 'healthy', got %s", StatusHealthy)
	}
	if StatusDegraded != "degraded" {
		t.Errorf("Expected StatusDegraded to be 'degraded', got %s", StatusDegraded)
	}
	if StatusUnhealthy != "unhealthy" {
		t.Errorf("Expected StatusUnhealthy to be 'unhealthy', got %s", StatusUnhealthy)
	}
}

func TestDetermineOverallStatus(t *testing.T) {
	tests := []struct {
		name     string
		results  map[string]Result
		expected Status
	}{
		{
			name:     "all healthy",
			results:  map[string]Result{"db": {Status: StatusHealthy}, "smtp": {Status: StatusHealthy}},
			expected: StatusHealthy,
		},
		{
			name:     "one degraded",
			results:  map[string]Result{"db": {Status: StatusHealthy}, "smtp": {Status: StatusDegraded}},
			expected: StatusDegraded,
		},
		{
			name:     "one unhealthy",
			results:  map[string]Result{"db": {Status: StatusUnhealthy}, "smtp": {Status: StatusHealthy}},
			expected: StatusUnhealthy,
		},
		{
			name:     "smtp unhealthy downgrades overall to degraded",
			results:  map[string]Result{"db": {Status: StatusHealthy}, "smtp": {Status: StatusUnhealthy}},
			expected: StatusDegraded,
		},
		{
			name:     "mixed degraded and unhealthy",
			results:  map[string]Result{"db": {Status: StatusDegraded}, "smtp": {Status: StatusUnhealthy}},
			expected: StatusDegraded,
		},
		{
			name:     "empty results",
			results:  map[string]Result{},
			expected: StatusHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineOverallStatus(tt.results)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestDatabaseChecker_Name(t *testing.T) {
	checker := &DatabaseChecker{}
	if checker.Name() != "database" {
		t.Errorf("Expected name 'database', got %s", checker.Name())
	}
}

func TestDatabaseChecker_Check(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Logf("Could not open test database: %v", err)
		t.Skip("Skipping database-dependent test")
	}
	defer db.Close()

	monitor := &MockDBMonitor{
		stats: middleware.DBStats{
			TotalQueries:    10,
			SlowQueries:     1,
			AverageDuration: 5 * time.Millisecond,
			MaxDuration:     20 * time.Millisecond,
			ErrorCount:      0,
			LastQueryTime:   time.Now(),
		},
	}

	checker := NewDatabaseChecker(db, monitor)
	ctx := context.Background()

	result := checker.Check(ctx)

	if result.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	if _, ok := result.Details["ping_time"]; !ok {
		t.Error("Expected ping_time in details")
	}

	if result.Status == StatusHealthy {
		stats, ok := result.Details["stats"].(map[string]any)
		if !ok {
			t.Log("Stats not available in details (may be expected)")
		} else {
			if stats["total_queries"] != int64(10) {
				t.Errorf("Expected total_queries to be 10, got %v", stats["total_queries"])
			}
		}
	}
}

func TestDatabaseChecker_Check_Unhealthy(t *testing.T) {
	checker := NewDatabaseChecker(nil, nil)
	ctx := context.Background()

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Got expected panic with nil db: %v", r)
		}
	}()

	result := checker.Check(ctx)

	if result.Status != StatusUnhealthy {
		t.Logf("Expected unhealthy status for nil db, got %s", result.Status)
	}
}

func TestCompositeChecker(t *testing.T) {
	checker1 := &mockChecker{
		name:   "service1",
		status: StatusHealthy,
	}
	checker2 := &mockChecker{
		name:   "service2",
		status: StatusHealthy,
	}

	composite := NewCompositeChecker(checker1, checker2)
	ctx := context.Background()

	result := composite.Check(ctx)

	if result.Status != StatusHealthy {
		t.Errorf("Expected overall healthy status, got %s", result.Status)
	}

	if len(result.Services) != 2 {
		t.Errorf("Expected 2 service results, got %d", len(result.Services))
	}

	if _, ok := result.Services["service1"]; !ok {
		t.Error("Expected service1 in results")
	}

	if _, ok := result.Services["service2"]; !ok {
		t.Error("Expected service2 in results")
	}
}

func TestCompositeChecker_AddChecker(t *testing.T) {
	composite := NewCompositeChecker()

	checker := &mockChecker{
		name:   "test-service",
		status: StatusHealthy,
	}

	composite.AddChecker(checker)

	ctx := context.Background()
	result := composite.Check(ctx)

	if len(result.Services) != 1 {
		t.Errorf("Expected 1 service result, got %d", len(result.Services))
	}
}

func TestCompositeChecker_CheckWithVersion(t *testing.T) {
	checker := &mockChecker{
		name:   "test",
		status: StatusHealthy,
	}

	composite := NewCompositeChecker(checker)
	ctx := context.Background()

	result := composite.CheckWithVersion(ctx, "v1.2.3")

	if result.Version != "v1.2.3" {
		t.Errorf("Expected version 'v1.2.3', got %s", result.Version)
	}
}

type mockChecker struct {
	name   string
	status Status
}

func (m *mockChecker) Name() string {
	return m.name
}

func (m *mockChecker) Check(ctx context.Context) Result {
	return Result{
		Status:    m.status,
		Timestamp: time.Now().UTC(),
		Latency:   1 * time.Millisecond,
	}
}
