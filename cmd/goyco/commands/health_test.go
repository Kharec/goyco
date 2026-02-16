package commands

import (
	"os"
	"strings"
	"testing"

	"goyco/internal/config"
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
