package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"goyco/internal/config"
)

type ConnectionPoolConfig struct {
	MaxOpenConns        int
	MaxIdleConns        int
	ConnMaxLifetime     time.Duration
	ConnMaxIdleTime     time.Duration
	ConnTimeout         time.Duration
	HealthCheckInterval time.Duration
}

func DefaultConnectionPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxOpenConns:        25,
		MaxIdleConns:        10,
		ConnMaxLifetime:     5 * time.Minute,
		ConnMaxIdleTime:     1 * time.Minute,
		ConnTimeout:         30 * time.Second,
		HealthCheckInterval: 30 * time.Second,
	}
}

func ProductionConnectionPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxOpenConns:        100,
		MaxIdleConns:        25,
		ConnMaxLifetime:     10 * time.Minute,
		ConnMaxIdleTime:     2 * time.Minute,
		ConnTimeout:         10 * time.Second,
		HealthCheckInterval: 15 * time.Second,
	}
}

func HighTrafficConnectionPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxOpenConns:        200,
		MaxIdleConns:        50,
		ConnMaxLifetime:     15 * time.Minute,
		ConnMaxIdleTime:     5 * time.Minute,
		ConnTimeout:         5 * time.Second,
		HealthCheckInterval: 10 * time.Second,
	}
}

type ConnectionPoolManager struct {
	db     *gorm.DB
	sqlDB  *sql.DB
	config ConnectionPoolConfig
	ctx    context.Context
	cancel context.CancelFunc
}

func NewConnectionPoolManager(cfg *config.Config, poolConfig ConnectionPoolConfig) (*ConnectionPoolManager, error) {
	dsn := cfg.GetConnectionString()

	secureLogger := CreateSecureLogger(!cfg.App.Debug)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: secureLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(poolConfig.MaxOpenConns)
	sqlDB.SetMaxIdleConns(poolConfig.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(poolConfig.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(poolConfig.ConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), poolConfig.ConnTimeout)
	if err := sqlDB.PingContext(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	cancel()

	managerCtx, managerCancel := context.WithCancel(context.Background())

	manager := &ConnectionPoolManager{
		db:     db,
		sqlDB:  sqlDB,
		config: poolConfig,
		ctx:    managerCtx,
		cancel: managerCancel,
	}

	go manager.startHealthCheck()

	return manager, nil
}

func (m *ConnectionPoolManager) GetDB() *gorm.DB {
	return m.db
}

func (m *ConnectionPoolManager) GetSQLDB() *sql.DB {
	return m.sqlDB
}

func (m *ConnectionPoolManager) GetPoolStats() sql.DBStats {
	return m.sqlDB.Stats()
}

func (m *ConnectionPoolManager) startHealthCheck() {
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

func (m *ConnectionPoolManager) performHealthCheck() {
	ctx, cancel := context.WithTimeout(m.ctx, m.config.ConnTimeout)
	defer cancel()

	if err := m.sqlDB.PingContext(ctx); err != nil {
		log.Printf("Database health check failed: %v", err)
	}
}

func (m *ConnectionPoolManager) Close() error {
	if m.cancel != nil {
		m.cancel()
	}

	if m.sqlDB != nil {
		return m.sqlDB.Close()
	}

	return nil
}

func ConnectWithPool(cfg *config.Config) (*ConnectionPoolManager, error) {
	var poolConfig ConnectionPoolConfig

	if cfg.App.Debug {
		poolConfig = DefaultConnectionPoolConfig()
	} else {
		poolConfig = ProductionConnectionPoolConfig()
	}

	if cfg.App.BaseURL != "" && !cfg.App.Debug {
		poolConfig = HighTrafficConnectionPoolConfig()
	}

	return NewConnectionPoolManager(cfg, poolConfig)
}
