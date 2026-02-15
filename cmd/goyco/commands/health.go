package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/health"
	"goyco/internal/middleware"
	"goyco/internal/version"
)

func HandleHealthCommand(cfg *config.Config, name string, args []string) error {
	fs := newFlagSet(name, printHealthUsage)
	if err := parseCommand(fs, args, name); err != nil {
		if errors.Is(err, ErrHelpRequested) {
			return nil
		}
		return err
	}

	if fs.NArg() > 0 {
		printHealthUsage()
		return errors.New("unexpected arguments for health command")
	}

	return runHealthCheck(cfg)
}

func printHealthUsage() {
	fmt.Fprintln(os.Stderr, "Usage: goyco health")
	fmt.Fprintln(os.Stderr, "\nCheck the health status of the application and its dependencies.")
}

func runHealthCheck(cfg *config.Config) error {
	compositeChecker := health.NewCompositeChecker()

	dbChecker, err := createDatabaseChecker(cfg)
	if err != nil {
		return fmt.Errorf("failed to create database checker: %w", err)
	}
	if dbChecker != nil {
		compositeChecker.AddChecker(dbChecker)
	}

	smtpChecker := createSMTPChecker(cfg)
	if smtpChecker != nil {
		compositeChecker.AddChecker(smtpChecker)
	}

	ctx := context.Background()
	result := compositeChecker.CheckWithVersion(ctx, version.GetVersion())

	if IsJSONOutput() {
		return outputJSON(result)
	}

	return printHealthResult(result)
}

func createDatabaseChecker(cfg *config.Config) (health.Checker, error) {
	db, err := database.Connect(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	sqlDB.SetConnMaxLifetime(5 * time.Second)
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	monitor := middleware.NewInMemoryDBMonitor()
	return health.NewDatabaseChecker(sqlDB, monitor), nil
}

func createSMTPChecker(cfg *config.Config) health.Checker {
	if cfg.SMTP.Host == "" {
		return nil
	}

	smtpConfig := health.SMTPConfig{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		Username: cfg.SMTP.Username,
		Password: cfg.SMTP.Password,
		From:     cfg.SMTP.From,
	}

	return health.NewSMTPChecker(smtpConfig)
}

func printHealthResult(result health.OverallResult) error {
	fmt.Printf("Health Status: %s\n", result.Status)
	fmt.Printf("Version: %s\n", result.Version)
	fmt.Printf("Timestamp: %s\n", result.Timestamp.Format(time.RFC3339))
	fmt.Println()

	if len(result.Services) == 0 {
		fmt.Println("No services configured.")
		return nil
	}

	fmt.Println("Services:")
	for name, service := range result.Services {
		fmt.Printf("  %s:\n", name)
		fmt.Printf("    Status: %s\n", service.Status)
		fmt.Printf("    Latency: %s\n", service.Latency)
		if service.Message != "" {
			fmt.Printf("    Message: %s\n", service.Message)
		}
		if len(service.Details) > 0 {
			fmt.Printf("    Details:\n")
			for key, value := range service.Details {
				fmt.Printf("      %s: %v\n", key, value)
			}
		}
		fmt.Println()
	}

	return nil
}
