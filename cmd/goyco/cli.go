package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"

	"goyco/cmd/goyco/commands"
	"goyco/internal/config"

	"github.com/joho/godotenv"
	"github.com/urfave/cli/v3"
)

var (
	helpPrinterOnce    sync.Once
	defaultHelpPrinter func(io.Writer, string, interface{})
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
	fmt.Fprintln(os.Stderr, "  health    check the health of the application and its dependencies")
}

func printRunUsage() {
	fmt.Fprintln(os.Stderr, "Usage: goyco run")
	fmt.Fprintln(os.Stderr, "\nStart the web application in foreground.")
}

func buildRootCommand(cfg *config.Config) *cli.Command {
	helpPrinterOnce.Do(func() {
		defaultHelpPrinter = cli.HelpPrinter
	})
	cli.HelpPrinter = func(w io.Writer, templ string, data interface{}) {
		if cmd, ok := data.(*cli.Command); ok && cmd.Root() == cmd {
			printRootUsage()
			return
		}
		defaultHelpPrinter(w, templ, data)
	}

	root := &cli.Command{
		Name:        "goyco",
		Usage:       "Y Combinator-style news aggregation platform API",
		UsageText:   "goyco <command> [<args>]",
		HideVersion: true,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "json",
				Usage: "output results in JSON format",
				Value: cfg.CLI.JSONOutputDefault,
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			commands.SetJSONOutput(cmd.Bool("json"))
			return ctx, nil
		},
		After: func(ctx context.Context, cmd *cli.Command) error {
			cli.HelpPrinter = defaultHelpPrinter
			return nil
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() == 0 {
				printRootUsage()
				return nil
			}
			printRootUsage()
			return fmt.Errorf("unknown command: %s", cmd.Args().First())
		},
		Commands: []*cli.Command{
			{
				Name:            "run",
				Usage:           "start the web application in foreground",
				SkipFlagParsing: true,
				Action: func(_ context.Context, cmd *cli.Command) error {
					return handleRunCommand(cfg, cmd.Args().Slice())
				},
			},
			{
				Name:            "start",
				Usage:           "start the web application in background",
				SkipFlagParsing: true,
				Action: func(_ context.Context, cmd *cli.Command) error {
					return commands.HandleStartCommand(cfg, cmd.Args().Slice())
				},
			},
			{
				Name:            "stop",
				Usage:           "stop the daemon",
				SkipFlagParsing: true,
				Action: func(_ context.Context, cmd *cli.Command) error {
					return commands.HandleStopCommand(cfg, cmd.Args().Slice())
				},
			},
			{
				Name:            "status",
				Usage:           "check if the daemon is running",
				SkipFlagParsing: true,
				Action: func(_ context.Context, cmd *cli.Command) error {
					return commands.HandleStatusCommand(cfg, cmd.Name, cmd.Args().Slice())
				},
			},
			{
				Name:            "migrate",
				Aliases:         []string{"migrations"},
				Usage:           "apply database migrations",
				SkipFlagParsing: true,
				Action: func(_ context.Context, cmd *cli.Command) error {
					return commands.HandleMigrateCommand(cfg, cmd.Name, cmd.Args().Slice())
				},
			},
			{
				Name:            "user",
				Usage:           "manage users (create, update, delete, lock, list)",
				SkipFlagParsing: true,
				Action: func(_ context.Context, cmd *cli.Command) error {
					return commands.HandleUserCommand(cfg, cmd.Name, cmd.Args().Slice())
				},
			},
			{
				Name:            "post",
				Usage:           "manage posts (delete, list, search)",
				SkipFlagParsing: true,
				Action: func(_ context.Context, cmd *cli.Command) error {
					return commands.HandlePostCommand(cfg, cmd.Name, cmd.Args().Slice())
				},
			},
			{
				Name:            "prune",
				Usage:           "hard delete users and posts (posts, all)",
				SkipFlagParsing: true,
				Action: func(_ context.Context, cmd *cli.Command) error {
					return commands.HandlePruneCommand(cfg, cmd.Name, cmd.Args().Slice())
				},
			},
			{
				Name:            "seed",
				Usage:           "seed database with random data",
				SkipFlagParsing: true,
				Action: func(_ context.Context, cmd *cli.Command) error {
					return commands.HandleSeedCommand(cfg, cmd.Name, cmd.Args().Slice())
				},
			},
			{
				Name:            "health",
				Usage:           "check the health of the application and its dependencies",
				SkipFlagParsing: true,
				Action: func(_ context.Context, cmd *cli.Command) error {
					return commands.HandleHealthCommand(cfg, cmd.Name, cmd.Args().Slice())
				},
			},
		},
		Writer:    os.Stdout,
		ErrWriter: os.Stderr,
	}

	return root
}
