package commands

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sync"

	"gorm.io/gorm"
	"goyco/internal/config"
	"goyco/internal/database"
)

var ErrHelpRequested = errors.New("help requested")

type DBConnector func(cfg *config.Config) (*gorm.DB, func() error, error)

var (
	jsonOutputMu sync.RWMutex
	jsonOutput   bool
)

func SetJSONOutput(enabled bool) {
	jsonOutputMu.Lock()
	defer jsonOutputMu.Unlock()
	jsonOutput = enabled
}

func IsJSONOutput() bool {
	jsonOutputMu.RLock()
	defer jsonOutputMu.RUnlock()
	return jsonOutput
}

var (
	dbConnectorMu      sync.RWMutex
	currentDBConnector = defaultDBConnector
)

func defaultDBConnector(cfg *config.Config) (*gorm.DB, func() error, error) {
	db, err := database.Connect(cfg)
	if err != nil {
		return nil, nil, err
	}
	return db, func() error { return database.Close(db) }, nil
}

func SetDBConnector(connector DBConnector) {
	dbConnectorMu.Lock()
	defer dbConnectorMu.Unlock()

	if connector == nil {
		currentDBConnector = defaultDBConnector
		return
	}

	currentDBConnector = connector
}

func getDBConnector() DBConnector {
	dbConnectorMu.RLock()
	defer dbConnectorMu.RUnlock()
	return currentDBConnector
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
			return ErrHelpRequested
		}
		return fmt.Errorf("failed to parse %s command: %w", context, err)
	}
	return nil
}

func withDatabase(cfg *config.Config, fn func(db *gorm.DB) error) error {
	connector := getDBConnector()
	db, cleanup, err := connector(cfg)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	if cleanup != nil {
		defer func() {
			if err := cleanup(); err != nil {
				fmt.Printf("Warning: closing database: %v\n", err)
			}
		}()
	}

	return fn(db)
}

func truncate(in string, max int) string {
	if len(in) <= max {
		return in
	}
	if max <= 3 {
		return in[:max]
	}
	return in[:max-3] + "..."
}

func outputJSON(v interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

func outputMessage(message string, args ...interface{}) {
	if IsJSONOutput() {
		outputJSON(map[string]interface{}{
			"message": fmt.Sprintf(message, args...),
		})
	} else {
		fmt.Printf(message+"\n", args...)
	}
}

func outputError(message string, args ...interface{}) {
	if IsJSONOutput() {
		outputJSON(map[string]interface{}{
			"error": fmt.Sprintf(message, args...),
		})
	} else {
		fmt.Fprintf(os.Stderr, message+"\n", args...)
	}
}

func outputWarning(message string, args ...interface{}) {
	if IsJSONOutput() {
		outputJSON(map[string]interface{}{
			"warning": fmt.Sprintf(message, args...),
		})
	} else {
		fmt.Printf("Warning: "+message+"\n", args...)
	}
}
