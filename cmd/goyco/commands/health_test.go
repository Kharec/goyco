package commands

import (
	"os"
	"strings"
	"testing"
	"time"

	"goyco/internal/config"
	"goyco/internal/health"
	"goyco/internal/testutils"
)

func TestHandleHealthCommand(t *testing.T) {
	cfg := testutils.NewTestConfig()

	t.Run("help requested", func(t *testing.T) {
		err := HandleHealthCommand(cfg, "health", []string{"--help"})

		if err != nil {
			t.Errorf("unexpected error for help: %v", err)
		}
	})

	t.Run("unexpected arguments", func(t *testing.T) {
		err := HandleHealthCommand(cfg, "health", []string{"extra", "args"})

		if err == nil {
			t.Error("expected error for unexpected arguments")
		}

		if !strings.Contains(err.Error(), "unexpected arguments") {
			t.Errorf("expected error containing 'unexpected arguments', got %q", err.Error())
		}
	})
}

func TestPrintHealthUsage(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	printHealthUsage()

	w.Close()
	os.Stderr = oldStderr

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "Usage: goyco health") {
		t.Errorf("expected usage to contain 'Usage: goyco health', got %q", output)
	}
}

func TestCreateSMTPChecker(t *testing.T) {
	t.Run("with valid smtp config", func(t *testing.T) {
		cfg := &config.Config{
			SMTP: config.SMTPConfig{
				Host: "smtp.example.com",
				Port: 587,
			},
		}

		checker := createSMTPChecker(cfg)

		if checker == nil {
			t.Error("expected checker to be created")
		}

		if checker.Name() != "smtp" {
			t.Errorf("expected name 'smtp', got %s", checker.Name())
		}
	})

	t.Run("with empty host", func(t *testing.T) {
		cfg := &config.Config{
			SMTP: config.SMTPConfig{
				Host: "",
				Port: 587,
			},
		}

		checker := createSMTPChecker(cfg)

		if checker != nil {
			t.Error("expected checker to be nil when host is empty")
		}
	})
}

func TestPrintHealthResult(t *testing.T) {
	t.Run("healthy result", func(t *testing.T) {
		result := health.OverallResult{
			Status:    health.StatusHealthy,
			Version:   "v1.0.0",
			Timestamp: time.Now().UTC(),
			Services: map[string]health.Result{
				"database": {
					Status:  health.StatusHealthy,
					Latency: 2 * time.Millisecond,
					Details: map[string]any{
						"ping_time": "2ms",
					},
				},
			},
		}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := printHealthResult(result)

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		if !strings.Contains(output, "Health Status: healthy") {
			t.Errorf("expected 'Health Status: healthy', got %q", output)
		}

		if !strings.Contains(output, "Version: v1.0.0") {
			t.Errorf("expected 'Version: v1.0.0', got %q", output)
		}
	})

	t.Run("degraded result with message", func(t *testing.T) {
		result := health.OverallResult{
			Status:    health.StatusDegraded,
			Version:   "v1.0.0",
			Timestamp: time.Now().UTC(),
			Services: map[string]health.Result{
				"smtp": {
					Status:  health.StatusDegraded,
					Message: "Connection failed",
					Latency: 5 * time.Millisecond,
				},
			},
		}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := printHealthResult(result)

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		if !strings.Contains(output, "Health Status: degraded") {
			t.Errorf("expected 'Health Status: degraded', got %q", output)
		}

		if !strings.Contains(output, "Connection failed") {
			t.Errorf("expected error message in output, got %q", output)
		}
	})

	t.Run("empty services", func(t *testing.T) {
		result := health.OverallResult{
			Status:    health.StatusHealthy,
			Version:   "v1.0.0",
			Timestamp: time.Now().UTC(),
			Services:  map[string]health.Result{},
		}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := printHealthResult(result)

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		if !strings.Contains(output, "No services configured") {
			t.Errorf("expected 'No services configured', got %q", output)
		}
	})
}
