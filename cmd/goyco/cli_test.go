package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"strings"
	"testing"

	"gorm.io/gorm"
	"goyco/cmd/goyco/commands"
	"goyco/internal/config"
	"goyco/internal/testutils"
)

func TestLoadDotEnv(t *testing.T) {
	t.Run("no .env file", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("loadDotEnv panicked: %v", r)
			}
		}()
		loadDotEnv()
	})
}

func TestNewFlagSet(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
		usage    func()
	}{
		{
			name:     "with usage function",
			flagName: "test",
			usage:    func() { _, _ = os.Stderr.WriteString("test usage") },
		},
		{
			name:     "without usage function",
			flagName: "test2",
			usage:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFlagSet(tt.flagName, tt.usage)

			if fs.Name() != tt.flagName {
				t.Errorf("expected flag set name %q, got %q", tt.flagName, fs.Name())
			}

			if tt.usage != nil && fs.Usage == nil {
				t.Error("expected usage function to be set")
			}

		})
	}
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		context     string
		expectError bool
		expectHelp  bool
	}{
		{
			name:        "valid arguments",
			args:        []string{"--help"},
			context:     "test",
			expectError: true,
			expectHelp:  true,
		},
		{
			name:        "invalid flag",
			args:        []string{"--invalid-flag"},
			context:     "test",
			expectError: true,
			expectHelp:  false,
		},
		{
			name:        "empty arguments",
			args:        []string{},
			context:     "test",
			expectError: false,
			expectHelp:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.SetOutput(os.Stderr)

			err := parseCommand(fs, tt.args, tt.context)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.expectHelp && !errors.Is(err, commands.ErrHelpRequested) {
				t.Error("expected help requested error")
			}
		})
	}
}

func TestPrintRootUsage(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printRootUsage panicked: %v", r)
		}
	}()

	printRootUsage()
}

func TestPrintRunUsage(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printRunUsage panicked: %v", r)
		}
	}()

	printRunUsage()
}

func TestDispatchCommand(t *testing.T) {
	t.Run("unknown command", func(t *testing.T) {
		cfg := testutils.NewTestConfig()
		cmd := buildRootCommand(cfg)
		err := cmd.Run(context.Background(), []string{"goyco", "unknown"})

		if err == nil {
			t.Error("expected error for unknown command")
		}

		expectedErr := "unknown command: unknown"
		if err != nil && err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("help command", func(t *testing.T) {
		cfg := testutils.NewTestConfig()
		cmd := buildRootCommand(cfg)
		err := cmd.Run(context.Background(), []string{"goyco", "help"})

		if err != nil {
			t.Errorf("unexpected error for help command: %v", err)
		}
	})

	t.Run("h command", func(t *testing.T) {
		cfg := testutils.NewTestConfig()
		cmd := buildRootCommand(cfg)
		err := cmd.Run(context.Background(), []string{"goyco", "-h"})

		if err != nil {
			t.Errorf("unexpected error for -h command: %v", err)
		}
	})

	t.Run("--help command", func(t *testing.T) {
		cfg := testutils.NewTestConfig()
		cmd := buildRootCommand(cfg)
		err := cmd.Run(context.Background(), []string{"goyco", "--help"})

		if err != nil {
			t.Errorf("unexpected error for --help command: %v", err)
		}
	})

	t.Run("post list with injected database", func(t *testing.T) {
		cfg := testutils.NewTestConfig()

		useInMemoryCommandsConnector(t)

		cmd := buildRootCommand(cfg)
		err := cmd.Run(context.Background(), []string{"goyco", "post", "list"})

		if err != nil {
			t.Errorf("unexpected error for post list: %v", err)
		}
	})
}

func TestHandleRunCommand(t *testing.T) {

	t.Run("help requested", func(t *testing.T) {
		cfg := testutils.NewTestConfig()
		err := handleRunCommand(cfg, []string{"--help"})

		if err != nil {
			t.Errorf("unexpected error for help: %v", err)
		}
	})

	t.Run("unexpected arguments", func(t *testing.T) {
		cfg := testutils.NewTestConfig()
		err := handleRunCommand(cfg, []string{"extra", "args"})

		if err == nil {
			t.Error("expected error for unexpected arguments")
		}

		expectedErr := "unexpected arguments for run command"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})
}

func TestRun(t *testing.T) {

	t.Run("no arguments", func(t *testing.T) {
		err := run([]string{})

		if err != nil {
			t.Logf("Expected error in test environment: %v", err)
		}
	})

	t.Run("help flag", func(t *testing.T) {
		err := run([]string{"--help"})

		if err == nil {
			t.Error("expected config loading error in test environment")
		}
	})

	t.Run("invalid flag", func(t *testing.T) {
		err := run([]string{"--invalid-flag"})

		if err == nil {
			t.Error("expected error for invalid flag")
		}
	})
}

func TestRunE2E_CommandParsing(t *testing.T) {
	setupTestEnv(t)

	t.Run("help command succeeds", func(t *testing.T) {
		err := run([]string{"help"})
		if err != nil {
			t.Errorf("Expected help command to succeed, got error: %v", err)
		}
	})

	t.Run("unknown command fails with error", func(t *testing.T) {
		err := run([]string{"unknown-command"})
		if err == nil {
			t.Error("Expected error for unknown command")
		}
		if err != nil && !strings.Contains(err.Error(), "unknown command") {
			t.Errorf("Expected error about unknown command, got: %v", err)
		}
	})

	t.Run("migrate command parses correctly", func(t *testing.T) {
		err := run([]string{"migrate", "up"})
		if err != nil && strings.Contains(err.Error(), "unknown command") {
			t.Errorf("Expected migrate command to be recognized, got parsing error: %v", err)
		}
	})

	t.Run("post command parses correctly", func(t *testing.T) {
		useInMemoryCommandsConnector(t)
		err := run([]string{"post", "list"})
		if err != nil && strings.Contains(err.Error(), "unknown command") {
			t.Errorf("Expected post command to be recognized, got parsing error: %v", err)
		}
	})
}

func TestRunE2E_ConfigurationLoading(t *testing.T) {
	t.Run("missing required env vars fails gracefully", func(t *testing.T) {
		originalDBPwd := os.Getenv("DB_PASSWORD")
		originalSMTPHost := os.Getenv("SMTP_HOST")
		originalSMTPFrom := os.Getenv("SMTP_FROM")
		originalAdminEmail := os.Getenv("ADMIN_EMAIL")
		originalJWTSecret := os.Getenv("JWT_SECRET")

		defer func() {
			if originalDBPwd != "" {
				_ = os.Setenv("DB_PASSWORD", originalDBPwd)
			}
			if originalSMTPHost != "" {
				_ = os.Setenv("SMTP_HOST", originalSMTPHost)
			}
			if originalSMTPFrom != "" {
				_ = os.Setenv("SMTP_FROM", originalSMTPFrom)
			}
			if originalAdminEmail != "" {
				_ = os.Setenv("ADMIN_EMAIL", originalAdminEmail)
			}
			if originalJWTSecret != "" {
				_ = os.Setenv("JWT_SECRET", originalJWTSecret)
			}
		}()

		_ = os.Unsetenv("DB_PASSWORD")
		_ = os.Unsetenv("SMTP_HOST")
		_ = os.Unsetenv("SMTP_FROM")
		_ = os.Unsetenv("ADMIN_EMAIL")
		_ = os.Unsetenv("JWT_SECRET")

		err := run([]string{"help"})
		if err == nil {
			t.Error("Expected error when required env vars are missing")
		}
		if err != nil && !strings.Contains(err.Error(), "configuration") && !strings.Contains(err.Error(), "config") {
			t.Logf("Got error (may be expected): %v", err)
		}
	})

	t.Run("valid configuration loads successfully", func(t *testing.T) {
		setupTestEnv(t)
		err := run([]string{"help"})
		if err != nil {
			t.Errorf("Expected help command to succeed with valid config, got: %v", err)
		}
	})
}

func TestRunE2E_ArgumentParsing(t *testing.T) {
	setupTestEnv(t)

	t.Run("root help flag", func(t *testing.T) {
		err := run([]string{"--help"})
		if err != nil && !strings.Contains(err.Error(), "flag") {
			t.Logf("Got error (may be expected in test env): %v", err)
		}
	})

	t.Run("command with help flag", func(t *testing.T) {
		err := run([]string{"migrate", "--help"})
		if err != nil && strings.Contains(err.Error(), "unknown command") {
			t.Errorf("Expected migrate command to be recognized, got: %v", err)
		}
	})

	t.Run("command with invalid arguments", func(t *testing.T) {
		err := run([]string{"run", "extra", "args"})
		if err == nil {
			t.Error("Expected error for unexpected arguments")
		}
		if err != nil && !strings.Contains(err.Error(), "unexpected arguments") {
			t.Errorf("Expected error about unexpected arguments, got: %v", err)
		}
	})
}

func setupTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DB_PASSWORD", "test-password")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_FROM", "test@example.com")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("JWT_SECRET", "test-jwt-secret-key-that-is-long-enough-for-validation-purposes")
	tmpDir := os.TempDir()
	t.Setenv("LOG_DIR", tmpDir)
	t.Setenv("PID_DIR", tmpDir)
}

func useInMemoryCommandsConnector(t *testing.T) {
	t.Helper()

	commands.SetDBConnector(func(_ *config.Config) (*gorm.DB, func() error, error) {
		db := testutils.NewTestDB(t)

		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("failed to access underlying sql.DB: %v", err)
		}

		cleanup := func() error {
			return sqlDB.Close()
		}

		return db, cleanup, nil
	})

	t.Cleanup(func() {
		commands.SetDBConnector(nil)
	})
}
