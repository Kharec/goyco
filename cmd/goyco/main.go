// @title Goyco API
// @version 0.1.0
// @description Goyco is a Y Combinator-style news aggregation platform API.
// @contact.name Goyco Team
// @contact.email sandro@cazzaniga.fr
// @license.name GPLv3
// @license.url https://www.gnu.org/licenses/gpl-3.0.html
// @host localhost:8080
// @schemes http
// @BasePath /api

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"goyco/cmd/goyco/commands"
	"goyco/docs"
	"goyco/internal/config"
	"goyco/internal/version"
)

func main() {
	loadDotEnv()

	commands.SetRunServer(runServerImpl)

	if len(os.Args) > 1 && os.Args[len(os.Args)-1] == "--daemon" {
		args := os.Args[1 : len(os.Args)-1]
		if err := commands.RunDaemonProcessDirect(args); err != nil {
			log.Fatalf("daemon error: %v", err)
		}
		return
	}

	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	validator := commands.NewConfigValidator(nil)
	if err := validator.ValidateConfiguration(cfg); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	docs.SwaggerInfo.Title = fmt.Sprintf("%s API", cfg.App.Title)
	docs.SwaggerInfo.Description = "Y Combinator-style news board API."
	docs.SwaggerInfo.Version = version.Version
	docs.SwaggerInfo.BasePath = "/api"
	docs.SwaggerInfo.Host = fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	docs.SwaggerInfo.Schemes = []string{"http"}
	if cfg.Server.EnableTLS {
		docs.SwaggerInfo.Schemes = append(docs.SwaggerInfo.Schemes, "https")
	}

	rootFS := flag.NewFlagSet("goyco", flag.ContinueOnError)
	rootFS.SetOutput(os.Stderr)
	rootFS.Usage = printRootUsage
	showHelp := rootFS.Bool("help", false, "show this help message")

	if err := rootFS.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("failed to parse arguments: %w", err)
	}

	if *showHelp {
		printRootUsage()
		return nil
	}

	remaining := rootFS.Args()
	if len(remaining) == 0 {
		printRootUsage()
		return nil
	}

	return dispatchCommand(cfg, remaining[0], remaining[1:])
}

func dispatchCommand(cfg *config.Config, name string, args []string) error {
	switch name {
	case "run":
		return handleRunCommand(cfg, args)
	case "start":
		return commands.HandleStartCommand(cfg, args)
	case "stop":
		return commands.HandleStopCommand(cfg, args)
	case "status":
		return commands.HandleStatusCommand(cfg, name, args)
	case "user":
		return commands.HandleUserCommand(cfg, name, args)
	case "post":
		return commands.HandlePostCommand(cfg, name, args)
	case "prune":
		return commands.HandlePruneCommand(cfg, name, args)
	case "migrate", "migrations":
		return commands.HandleMigrateCommand(cfg, name, args)
	case "seed":
		return commands.HandleSeedCommand(cfg, name, args)
	case "help", "-h", "--help":
		printRootUsage()
		return nil
	default:
		printRootUsage()
		return fmt.Errorf("unknown command: %s", name)
	}
}

func handleRunCommand(cfg *config.Config, args []string) error {
	fs := newFlagSet("run", printRunUsage)
	if err := parseCommand(fs, args, "run"); err != nil {
		if errors.Is(err, commands.ErrHelpRequested) {
			return nil
		}
		return err
	}

	if fs.NArg() > 0 {
		printRunUsage()
		return errors.New("unexpected arguments for run command")
	}

	return runServer(cfg, false)
}
