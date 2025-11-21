package commands

import (
	"errors"
	"flag"
	"os"
	"testing"

	"gorm.io/gorm"
	"goyco/internal/config"
	"goyco/internal/testutils"
)

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

			if tt.expectHelp && !errors.Is(err, ErrHelpRequested) {
				t.Error("expected help requested error")
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{
			name:     "string shorter than max",
			input:    "short",
			max:      10,
			expected: "short",
		},
		{
			name:     "string equal to max",
			input:    "exactly",
			max:      7,
			expected: "exactly",
		},
		{
			name:     "string longer than max",
			input:    "this is a very long string",
			max:      10,
			expected: "this is...",
		},
		{
			name:     "string longer than max with small max",
			input:    "hello",
			max:      3,
			expected: "hel",
		},
		{
			name:     "string longer than max with very small max",
			input:    "hello",
			max:      1,
			expected: "h",
		},
		{
			name:     "empty string",
			input:    "",
			max:      5,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.max)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestWithDatabase(t *testing.T) {
	cfg := testutils.NewTestConfig()

	t.Run("custom connector success", func(t *testing.T) {
		setInMemoryDBConnector(t)

		var called bool
		err := withDatabase(cfg, func(db *gorm.DB) error {
			called = true
			if db == nil {
				t.Fatal("expected non-nil database")
			}
			return nil
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !called {
			t.Fatal("expected database function to be called")
		}
	})

	t.Run("default connector failure", func(t *testing.T) {
		SetDBConnector(nil)
		var called bool
		err := withDatabase(cfg, func(db *gorm.DB) error {
			called = true
			return nil
		})

		if err == nil {
			t.Error("expected database connection error in test environment")
		}

		if called {
			t.Error("expected database function not to be called when connection fails")
		}
	})
}

func setInMemoryDBConnector(t *testing.T) {
	t.Helper()

	SetDBConnector(func(_ *config.Config) (*gorm.DB, func() error, error) {
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
		SetDBConnector(nil)
	})
}

func TestSetJSONOutput(t *testing.T) {
	t.Run("set and get JSON output", func(t *testing.T) {
		SetJSONOutput(true)
		if !IsJSONOutput() {
			t.Error("expected JSON output to be enabled")
		}

		SetJSONOutput(false)
		if IsJSONOutput() {
			t.Error("expected JSON output to be disabled")
		}
	})

	t.Run("concurrent access", func(t *testing.T) {
		SetJSONOutput(false)
		done := make(chan bool)

		go func() {
			for i := 0; i < 100; i++ {
				SetJSONOutput(true)
				_ = IsJSONOutput()
				SetJSONOutput(false)
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 100; i++ {
				_ = IsJSONOutput()
			}
			done <- true
		}()

		<-done
		<-done
	})
}
