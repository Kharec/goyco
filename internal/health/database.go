package health

import (
	"context"
	"database/sql"
	"time"

	"goyco/internal/middleware"
)

type DatabaseChecker struct {
	db      *sql.DB
	monitor middleware.DBMonitor
}

func NewDatabaseChecker(db *sql.DB, monitor middleware.DBMonitor) *DatabaseChecker {
	return &DatabaseChecker{
		db:      db,
		monitor: monitor,
	}
}

func (c *DatabaseChecker) Name() string {
	return "database"
}

func (c *DatabaseChecker) Check(ctx context.Context) Result {
	start := time.Now()

	err := c.db.Ping()
	latency := time.Since(start)

	result := Result{
		Status:    StatusHealthy,
		Latency:   latency,
		Timestamp: time.Now().UTC(),
		Details: map[string]any{
			"ping_time": latency.String(),
		},
	}

	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = err.Error()
		return result
	}

	if c.monitor != nil {
		stats := c.monitor.GetStats()
		result.Details["stats"] = map[string]any{
			"total_queries":    stats.TotalQueries,
			"slow_queries":     stats.SlowQueries,
			"average_duration": stats.AverageDuration.String(),
			"max_duration":     stats.MaxDuration.String(),
			"error_count":      stats.ErrorCount,
			"last_query_time":  stats.LastQueryTime.Format(time.RFC3339),
		}
	}

	return result
}
