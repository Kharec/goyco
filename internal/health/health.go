package health

import (
	"context"
	"time"
)

type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

type Result struct {
	Status    Status         `json:"status"`
	Message   string         `json:"message,omitempty"`
	Latency   time.Duration  `json:"latency"`
	Timestamp time.Time      `json:"timestamp"`
	Details   map[string]any `json:"details,omitempty"`
}

type Checker interface {
	Name() string
	Check(ctx context.Context) Result
}

type OverallResult struct {
	Status    Status            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Version   string            `json:"version"`
	Services  map[string]Result `json:"services"`
}

func determineOverallStatus(results map[string]Result) Status {
	hasUnhealthy := false
	hasSMTPUnhealthy := false
	hasDegraded := false

	for name, result := range results {
		switch result.Status {
		case StatusUnhealthy:
			if name == "smtp" {
				hasSMTPUnhealthy = true
			} else {
				hasUnhealthy = true
			}
		case StatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return StatusUnhealthy
	}
	if hasDegraded || hasSMTPUnhealthy {
		return StatusDegraded
	}
	return StatusHealthy
}
