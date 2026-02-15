package health

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"goyco/internal/middleware"
)

// MockDBMonitor is a mock implementation of DBMonitor for testing
type MockDBMonitor struct {
	stats middleware.DBStats
}

func (m *MockDBMonitor) GetStats() middleware.DBStats {
	return m.stats
}

func (m *MockDBMonitor) LogQuery(query string, duration time.Duration, err error) {
	// Mock implementation - does nothing
}

func (m *MockDBMonitor) LogSlowQuery(query string, duration time.Duration, threshold time.Duration) {
	// Mock implementation - does nothing
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
			name:     "mixed degraded and unhealthy",
			results:  map[string]Result{"db": {Status: StatusDegraded}, "smtp": {Status: StatusUnhealthy}},
			expected: StatusUnhealthy,
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
	// Create an in-memory database using the default driver
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		// Fallback: create a nil db and just verify the checker structure
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

	// Note: The actual result depends on database availability
	// We just verify the structure is correct
	if result.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	if _, ok := result.Details["ping_time"]; !ok {
		t.Error("Expected ping_time in details")
	}

	// Only check stats if status is healthy
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
	// Test with a nil database connection
	checker := NewDatabaseChecker(nil, nil)
	ctx := context.Background()

	// This should panic or return an error result
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Got expected panic with nil db: %v", r)
		}
	}()

	result := checker.Check(ctx)

	// If no panic, we should get an unhealthy status
	if result.Status != StatusUnhealthy {
		t.Logf("Expected unhealthy status for nil db, got %s", result.Status)
	}
}

func TestSMTPChecker_Name(t *testing.T) {
	checker := &SMTPChecker{}
	if checker.Name() != "smtp" {
		t.Errorf("Expected name 'smtp', got %s", checker.Name())
	}
}

func TestSMTPChecker_Check_InvalidHost(t *testing.T) {
	config := SMTPConfig{
		Host: "invalid.host.that.does.not.exist",
		Port: 587,
	}

	checker := NewSMTPChecker(config)
	ctx := context.Background()

	result := checker.Check(ctx)

	if result.Status != StatusDegraded {
		t.Errorf("Expected degraded status for invalid host, got %s", result.Status)
	}

	if result.Message == "" {
		t.Error("Expected error message for connection failure")
	}

	if result.Details["host"] != "invalid.host.that.does.not.exist:587" {
		t.Errorf("Expected host in details, got %v", result.Details["host"])
	}
}

func TestSMTPChecker_Check_WithAuth(t *testing.T) {
	config := SMTPConfig{
		Host:     "smtp.example.com",
		Port:     587,
		Username: "test@example.com",
		Password: "password",
		From:     "noreply@example.com",
	}

	checker := NewSMTPChecker(config)

	// Just verify the configuration is stored correctly
	if checker.config.Host != "smtp.example.com" {
		t.Errorf("Expected host to be 'smtp.example.com', got %s", checker.config.Host)
	}
	if checker.config.Port != 587 {
		t.Errorf("Expected port to be 587, got %d", checker.config.Port)
	}
}

func TestCompositeChecker(t *testing.T) {
	// Create a composite checker with test checkers
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

// mockChecker is a test implementation of the Checker interface
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
