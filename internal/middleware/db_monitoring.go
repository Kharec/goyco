package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"time"
)

const (
	dbMonitorKey          contextKey = "db_monitor"
	slowQueryThresholdKey contextKey = "slow_query_threshold"
)

type DBMonitor interface {
	LogQuery(query string, duration time.Duration, err error)
	LogSlowQuery(query string, duration time.Duration, threshold time.Duration)
	GetStats() DBStats
}

type DBStats struct {
	TotalQueries    int64         `json:"total_queries"`
	SlowQueries     int64         `json:"slow_queries"`
	AverageDuration time.Duration `json:"average_duration"`
	MaxDuration     time.Duration `json:"max_duration"`
	ErrorCount      int64         `json:"error_count"`
	LastQueryTime   time.Time     `json:"last_query_time"`
}

type InMemoryDBMonitor struct {
	stats DBStats
	mu    sync.RWMutex
}

func NewInMemoryDBMonitor() *InMemoryDBMonitor {
	return &InMemoryDBMonitor{
		stats: DBStats{},
	}
}

func (m *InMemoryDBMonitor) LogQuery(query string, duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalQueries++
	m.stats.LastQueryTime = time.Now()

	if err != nil {
		m.stats.ErrorCount++
		return
	}

	if m.stats.TotalQueries == 1 {
		m.stats.AverageDuration = duration
	} else {

		totalDuration := int64(m.stats.AverageDuration) * (m.stats.TotalQueries - 1)
		totalDuration += int64(duration)
		m.stats.AverageDuration = time.Duration(totalDuration / m.stats.TotalQueries)
	}

	if duration > m.stats.MaxDuration {
		m.stats.MaxDuration = duration
	}

	slowThreshold := 100 * time.Millisecond
	if duration > slowThreshold {
		m.stats.SlowQueries++
	}
}

func (m *InMemoryDBMonitor) LogSlowQuery(query string, duration time.Duration, threshold time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats.SlowQueries++
}

func (m *InMemoryDBMonitor) GetStats() DBStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

func DBMonitoringMiddleware(monitor DBMonitor, slowQueryThreshold time.Duration) func(http.Handler) http.Handler {
	if slowQueryThreshold == 0 {
		slowQueryThreshold = 100 * time.Millisecond
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ctx := context.WithValue(r.Context(), dbMonitorKey, monitor)
			ctx = context.WithValue(ctx, slowQueryThresholdKey, slowQueryThreshold)

			next.ServeHTTP(w, r.WithContext(ctx))

			duration := time.Since(start)
			if duration > slowQueryThreshold {

				monitor.LogSlowQuery(r.URL.Path, duration, slowQueryThreshold)
			}
		})
	}
}

type QueryLogger struct {
	DB      *sql.DB
	Monitor DBMonitor
}

func NewQueryLogger(db *sql.DB, monitor DBMonitor) *QueryLogger {
	return &QueryLogger{
		DB:      db,
		Monitor: monitor,
	}
}

func (ql *QueryLogger) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := ql.DB.QueryContext(ctx, query, args...)
	duration := time.Since(start)

	ql.Monitor.LogQuery(query, duration, err)
	return rows, err
}

func (ql *QueryLogger) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	start := time.Now()
	row := ql.DB.QueryRowContext(ctx, query, args...)
	duration := time.Since(start)

	ql.Monitor.LogQuery(query, duration, nil)
	return row
}

func (ql *QueryLogger) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := ql.DB.ExecContext(ctx, query, args...)
	duration := time.Since(start)

	ql.Monitor.LogQuery(query, duration, err)
	return result, err
}

type DatabaseHealthChecker struct {
	DB      *sql.DB
	Monitor DBMonitor
}

func NewDatabaseHealthChecker(db *sql.DB, monitor DBMonitor) *DatabaseHealthChecker {
	return &DatabaseHealthChecker{
		DB:      db,
		Monitor: monitor,
	}
}

func (dhc *DatabaseHealthChecker) CheckHealth() map[string]any {
	start := time.Now()

	err := dhc.DB.Ping()
	duration := time.Since(start)

	health := map[string]any{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"ping_time": duration.String(),
	}

	if err != nil {
		health["status"] = "unhealthy"
		health["error"] = err.Error()
		return health
	}

	stats := dhc.Monitor.GetStats()
	health["database_stats"] = map[string]any{
		"total_queries":    stats.TotalQueries,
		"slow_queries":     stats.SlowQueries,
		"average_duration": stats.AverageDuration.String(),
		"max_duration":     stats.MaxDuration.String(),
		"error_count":      stats.ErrorCount,
		"last_query_time":  stats.LastQueryTime.Format(time.RFC3339),
	}

	return health
}

type PerformanceMetrics struct {
	RequestCount    int64         `json:"request_count"`
	AverageResponse time.Duration `json:"average_response"`
	MaxResponse     time.Duration `json:"max_response"`
	ErrorCount      int64         `json:"error_count"`
	DBStats         DBStats       `json:"database_stats"`
}

type MetricsCollector struct {
	monitor DBMonitor
	metrics PerformanceMetrics
	mu      sync.RWMutex
}

func NewMetricsCollector(monitor DBMonitor) *MetricsCollector {
	return &MetricsCollector{
		monitor: monitor,
		metrics: PerformanceMetrics{},
	}
}

func (mc *MetricsCollector) RecordRequest(duration time.Duration, hasError bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.metrics.RequestCount++

	if hasError {
		mc.metrics.ErrorCount++
	}

	if mc.metrics.RequestCount == 1 {
		mc.metrics.AverageResponse = duration
	} else {

		totalDuration := int64(mc.metrics.AverageResponse) * (mc.metrics.RequestCount - 1)
		totalDuration += int64(duration)
		mc.metrics.AverageResponse = time.Duration(totalDuration / mc.metrics.RequestCount)
	}

	if duration > mc.metrics.MaxResponse {
		mc.metrics.MaxResponse = duration
	}
}

func (mc *MetricsCollector) GetMetrics() PerformanceMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	mc.metrics.DBStats = mc.monitor.GetStats()
	return mc.metrics
}

func MetricsMiddleware(collector *MetricsCollector) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rw := &metricsResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)
			hasError := rw.statusCode >= 400
			collector.RecordRequest(duration, hasError)
		})
	}
}

type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *metricsResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func GetDBMonitorFromContext(ctx context.Context) (DBMonitor, bool) {
	monitor, ok := ctx.Value(dbMonitorKey).(DBMonitor)
	return monitor, ok
}

func GetSlowQueryThresholdFromContext(ctx context.Context) (time.Duration, bool) {
	threshold, ok := ctx.Value(slowQueryThresholdKey).(time.Duration)
	return threshold, ok
}
