package database

import (
	"strings"
	"testing"
	"time"

	"goyco/internal/config"
)

func TestConnectionPoolConfig(t *testing.T) {
	t.Run("default_config", func(t *testing.T) {
		config := DefaultConnectionPoolConfig()

		if config.MaxOpenConns <= 0 {
			t.Error("MaxOpenConns should be positive")
		}
		if config.MaxIdleConns <= 0 {
			t.Error("MaxIdleConns should be positive")
		}
		if config.ConnMaxLifetime <= 0 {
			t.Error("ConnMaxLifetime should be positive")
		}
		if config.ConnMaxIdleTime <= 0 {
			t.Error("ConnMaxIdleTime should be positive")
		}
		if config.ConnTimeout <= 0 {
			t.Error("ConnTimeout should be positive")
		}
		if config.HealthCheckInterval <= 0 {
			t.Error("HealthCheckInterval should be positive")
		}
	})

	t.Run("production_config", func(t *testing.T) {
		config := ProductionConnectionPoolConfig()

		if config.MaxOpenConns < 50 {
			t.Error("Production MaxOpenConns should be higher")
		}
		if config.MaxIdleConns < 10 {
			t.Error("Production MaxIdleConns should be higher")
		}
	})

	t.Run("high_traffic_config", func(t *testing.T) {
		config := HighTrafficConnectionPoolConfig()

		if config.MaxOpenConns < 100 {
			t.Error("High traffic MaxOpenConns should be very high")
		}
		if config.MaxIdleConns < 25 {
			t.Error("High traffic MaxIdleConns should be high")
		}
	})
}

func TestConnectionPoolManager_Stats(t *testing.T) {

	t.Run("config_validation", func(t *testing.T) {
		config := DefaultConnectionPoolConfig()

		if config.MaxOpenConns < config.MaxIdleConns {
			t.Error("MaxOpenConns should be >= MaxIdleConns")
		}

		if config.ConnMaxLifetime < config.ConnMaxIdleTime {
			t.Error("ConnMaxLifetime should be >= ConnMaxIdleTime")
		}

		if config.ConnTimeout > 60*time.Second {
			t.Error("ConnTimeout should be reasonable")
		}
	})
}

func TestConnectionPoolConfig_Values(t *testing.T) {
	tests := []struct {
		name   string
		config ConnectionPoolConfig
	}{
		{
			name:   "default",
			config: DefaultConnectionPoolConfig(),
		},
		{
			name:   "production",
			config: ProductionConnectionPoolConfig(),
		},
		{
			name:   "high_traffic",
			config: HighTrafficConnectionPoolConfig(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.config

			if config.MaxOpenConns <= 0 {
				t.Errorf("MaxOpenConns should be positive, got %d", config.MaxOpenConns)
			}
			if config.MaxIdleConns <= 0 {
				t.Errorf("MaxIdleConns should be positive, got %d", config.MaxIdleConns)
			}
			if config.ConnMaxLifetime <= 0 {
				t.Errorf("ConnMaxLifetime should be positive, got %v", config.ConnMaxLifetime)
			}
			if config.ConnMaxIdleTime <= 0 {
				t.Errorf("ConnMaxIdleTime should be positive, got %v", config.ConnMaxIdleTime)
			}
			if config.ConnTimeout <= 0 {
				t.Errorf("ConnTimeout should be positive, got %v", config.ConnTimeout)
			}
			if config.HealthCheckInterval <= 0 {
				t.Errorf("HealthCheckInterval should be positive, got %v", config.HealthCheckInterval)
			}

			if config.MaxOpenConns < config.MaxIdleConns {
				t.Errorf("MaxOpenConns (%d) should be >= MaxIdleConns (%d)", config.MaxOpenConns, config.MaxIdleConns)
			}

			if config.ConnMaxLifetime < config.ConnMaxIdleTime {
				t.Errorf("ConnMaxLifetime (%v) should be >= ConnMaxIdleTime (%v)", config.ConnMaxLifetime, config.ConnMaxIdleTime)
			}
		})
	}
}

func TestNewConnectionPoolManager(t *testing.T) {
	t.Run("invalid_database_config", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "invalid-host",
				Port:     "9999",
				User:     "invalid",
				Password: "invalid",
				Name:     "invalid",
				SSLMode:  "disable",
			},
			App: config.AppConfig{
				Debug: true,
			},
		}

		poolConfig := DefaultConnectionPoolConfig()
		manager, err := NewConnectionPoolManager(cfg, poolConfig)

		if err == nil {
			t.Error("Expected error with invalid database config")
		}
		if manager != nil {
			t.Error("Expected nil manager with invalid database config")
		}
		if !strings.Contains(err.Error(), "failed to connect to database") {
			t.Errorf("Expected connection error, got: %v", err)
		}
	})
}

func TestConnectionPoolManager_Methods(t *testing.T) {
	t.Run("get_db_methods", func(t *testing.T) {

		manager := &ConnectionPoolManager{
			db:    nil,
			sqlDB: nil,
		}

		if manager.GetDB() != nil {
			t.Error("Expected nil DB from uninitialized manager")
		}

		if manager.GetSQLDB() != nil {
			t.Error("Expected nil SQLDB from uninitialized manager")
		}

	})
}

func TestConnectWithPool(t *testing.T) {
	t.Run("debug_mode_config", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "test",
				Password: "test",
				Name:     "test_db",
				SSLMode:  "disable",
			},
			App: config.AppConfig{
				Debug: true,
			},
		}

		manager, err := ConnectWithPool(cfg)
		if err == nil {
			t.Error("Expected error with invalid database config")
		}
		if manager != nil {
			t.Error("Expected nil manager with invalid database config")
		}
	})

	t.Run("production_mode_config", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "test",
				Password: "test",
				Name:     "test_db",
				SSLMode:  "disable",
			},
			App: config.AppConfig{
				Debug: false,
			},
		}

		manager, err := ConnectWithPool(cfg)
		if err == nil {
			t.Error("Expected error with invalid database config")
		}
		if manager != nil {
			t.Error("Expected nil manager with invalid database config")
		}
	})

	t.Run("high_traffic_config", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "test",
				Password: "test",
				Name:     "test_db",
				SSLMode:  "disable",
			},
			App: config.AppConfig{
				Debug:   false,
				BaseURL: "https://example.com",
			},
		}

		manager, err := ConnectWithPool(cfg)
		if err == nil {
			t.Error("Expected error with invalid database config")
		}
		if manager != nil {
			t.Error("Expected nil manager with invalid database config")
		}
	})
}
