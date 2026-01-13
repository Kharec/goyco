package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"goyco/cmd/goyco/commands"
	"goyco/internal/config"
	"goyco/internal/testutils"

	"github.com/urfave/cli/v3"
	"gorm.io/gorm"
)

func FuzzCLIArgs(f *testing.F) {
	f.Add("")
	f.Add("run")
	f.Add("--help")
	f.Add("user list")
	f.Add("post search")
	f.Add("migrate")

	f.Fuzz(func(t *testing.T, input string) {
		if !isValidUTF8(input) {
			return
		}

		if len(input) > 1000 {
			input = input[:1000]
		}

		args := strings.Fields(input)
		if len(args) == 0 {
			return
		}
		cmd := buildRootCommand(testutils.NewTestConfig())
		for _, sub := range cmd.Commands {
			sub.Action = func(context.Context, *cli.Command) error { return nil }
		}

		err := cmd.Run(context.Background(), append([]string{"goyco"}, args...))
		if err != nil {
			if !strings.Contains(err.Error(), "flag") && !strings.Contains(err.Error(), "unknown command") {
				t.Logf("Unexpected error format from command parsing: %v", err)
			}
		}
	})
}

func FuzzCommandDispatch(f *testing.F) {
	cfg := testutils.NewTestConfig()

	setRunServer(func(_ *config.Config, _ bool) error {
		return nil
	})
	defer setRunServer(runServerImpl)

	originalRunServer := runServerImpl
	commands.SetRunServer(func(_ *config.Config, _ bool) error {
		return nil
	})
	defer commands.SetRunServer(originalRunServer)

	commands.SetDaemonize(func() (int, error) {
		return 999, nil
	})
	defer commands.SetDaemonize(nil)

	commands.SetSetupDaemonLogging(func(_ *config.Config, _ string) error {
		return nil
	})
	defer commands.SetSetupDaemonLogging(nil)

	commands.SetDBConnector(func(_ *config.Config) (*gorm.DB, func() error, error) {
		return nil, nil, fmt.Errorf("database connection disabled in fuzzer")
	})
	defer commands.SetDBConnector(nil)

	daemonCommands := map[string]bool{
		"start":  true,
		"stop":   true,
		"status": true,
	}

	f.Add("run")
	f.Add("help")
	f.Add("user")
	f.Add("post")
	f.Add("migrate")
	f.Add("unknown_command")
	f.Add("--help")
	f.Add("-h")

	f.Fuzz(func(t *testing.T, input string) {
		if !isValidUTF8(input) {
			return
		}

		parts := strings.Fields(input)
		if len(parts) == 0 {
			return
		}

		cmdName := parts[0]
		args := parts[1:]

		if daemonCommands[cmdName] {
			return
		}

		err := dispatchCommand(cfg, cmdName, args)

		knownCommands := map[string]bool{
			"run": true, "user": true, "post": true, "prune": true, "migrate": true,
			"migrations": true, "seed": true, "help": true, "-h": true, "--help": true,
		}

		if knownCommands[cmdName] {
			if err != nil && !strings.Contains(err.Error(), cmdName) {
				t.Logf("Known command %q returned unexpected error: %v", cmdName, err)
			}
		} else {
			if err == nil {
				t.Fatalf("Unknown command %q should return an error", cmdName)
			}
			if !strings.Contains(err.Error(), cmdName) {
				t.Fatalf("Error for unknown command should contain command name: %v", err)
			}
		}
	})
}

func FuzzRunCommandHandler(f *testing.F) {
	cfg := testutils.NewTestConfig()

	setRunServer(func(_ *config.Config, _ bool) error {
		return nil
	})
	defer setRunServer(runServerImpl)

	f.Add("")
	f.Add("--help")
	f.Add("extra arg")
	f.Add("--invalid")

	f.Fuzz(func(t *testing.T, input string) {
		if !isValidUTF8(input) {
			return
		}

		args := strings.Fields(input)

		err := handleRunCommand(cfg, args)

		switch {
		case len(args) > 0 && args[0] == "--help":
			if err != nil {
				t.Logf("Help flag should not error, got: %v", err)
			}
		case len(args) > 0:
			if err == nil {
				return
			}

			errMsg := err.Error()
			if strings.Contains(errMsg, "flag provided but not defined") ||
				strings.Contains(errMsg, "failed to parse") {
				return
			}

			if !strings.Contains(errMsg, "unexpected arguments") {
				t.Logf("Got error (may be acceptable for server setup): %v", err)
			}
		default:
			if err != nil && strings.Contains(err.Error(), "unexpected arguments") {
				t.Fatalf("Empty args should not trigger 'unexpected arguments' error: %v", err)
			}
		}
	})
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == utf8.RuneError {
			return false
		}
	}
	return true
}
