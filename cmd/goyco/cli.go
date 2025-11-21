package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"goyco/cmd/goyco/commands"

	"github.com/joho/godotenv"
)

func loadDotEnv() {
	if _, err := os.Stat(".env"); err == nil {
		_ = godotenv.Load()
		return
	}
}

func newFlagSet(name string, usage func()) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if usage != nil {
		fs.Usage = usage
	}
	return fs
}

func parseCommand(fs *flag.FlagSet, args []string, context string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return commands.ErrHelpRequested
		}
		return fmt.Errorf("failed to parse %s command: %w", context, err)
	}
	return nil
}

func printRootUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [<args>]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "\nCommands:")
	fmt.Fprintln(os.Stderr, "  run       start the web application in foreground")
	fmt.Fprintln(os.Stderr, "  start     start the web application in background")
	fmt.Fprintln(os.Stderr, "  stop      stop the daemon")
	fmt.Fprintln(os.Stderr, "  status    check if the daemon is running")
	fmt.Fprintln(os.Stderr, "  migrate   apply database migrations")
	fmt.Fprintln(os.Stderr, "  user      manage users (create, update, delete, lock, list)")
	fmt.Fprintln(os.Stderr, "  post      manage posts (delete, list, search)")
	fmt.Fprintln(os.Stderr, "  prune     hard delete users and posts (posts, all)")
	fmt.Fprintln(os.Stderr, "  seed      seed database with random data")
}

func printRunUsage() {
	fmt.Fprintln(os.Stderr, "Usage: goyco run")
	fmt.Fprintln(os.Stderr, "\nStart the web application in foreground.")
}
