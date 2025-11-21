package commands

import (
	"testing"

	"goyco/internal/testutils"
)

func TestHandleMigrateCommand(t *testing.T) {
	cfg := testutils.NewTestConfig()

	t.Run("help requested", func(t *testing.T) {
		err := HandleMigrateCommand(cfg, "migrate", []string{"--help"})

		if err != nil {
			t.Errorf("unexpected error for help: %v", err)
		}
	})

	t.Run("unexpected arguments", func(t *testing.T) {
		err := HandleMigrateCommand(cfg, "migrate", []string{"extra", "args"})

		if err == nil {
			t.Error("expected error for unexpected arguments")
		}

		if err.Error() != "unexpected arguments for migrate command" {
			t.Errorf("expected specific error, got: %v", err)
		}
	})

	t.Run("runs migrations", func(t *testing.T) {
		cfg := testutils.NewTestConfig()
		setInMemoryDBConnector(t)

		err := HandleMigrateCommand(cfg, "migrate", []string{})

		if err != nil {
			t.Fatalf("unexpected error running migrations: %v", err)
		}
	})

	t.Run("runs migrations with JSON output", func(t *testing.T) {
		SetJSONOutput(true)
		defer SetJSONOutput(false)

		cfg := testutils.NewTestConfig()
		setInMemoryDBConnector(t)

		err := HandleMigrateCommand(cfg, "migrate", []string{})

		if err != nil {
			t.Fatalf("unexpected error running migrations: %v", err)
		}
	})
}
