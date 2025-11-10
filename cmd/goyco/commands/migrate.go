package commands

import (
	"errors"
	"fmt"
	"os"

	"gorm.io/gorm"
	"goyco/internal/config"
	"goyco/internal/database"
)

func HandleMigrateCommand(cfg *config.Config, name string, args []string) error {
	fs := newFlagSet(name, printMigrateUsage)
	if err := parseCommand(fs, args, name); err != nil {
		if errors.Is(err, ErrHelpRequested) {
			return nil
		}
		return err
	}

	if fs.NArg() > 0 {
		printMigrateUsage()
		return errors.New("unexpected arguments for migrate command")
	}

	return withDatabase(cfg, func(db *gorm.DB) error {
		return runMigrateCommand(db)
	})
}

func runMigrateCommand(db *gorm.DB) error {
	fmt.Println("Running database migrations...")
	if err := database.Migrate(db); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	fmt.Println("Migrations applied successfully")
	return nil
}

func printMigrateUsage() {
	fmt.Fprintln(os.Stderr, "Usage: goyco migrate")
	fmt.Fprintln(os.Stderr, "\nApply database migrations.")
}
