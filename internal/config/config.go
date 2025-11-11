package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Database  DatabaseConfig
	Server    ServerConfig
	JWT       JWTConfig
	SMTP      SMTPConfig
	App       AppConfig
	RateLimit RateLimitConfig
	LogDir    string
	PIDDir    string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type ServerConfig struct {
	Port           string
	Host           string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	MaxHeaderBytes int
	EnableTLS      bool
	TLSCertFile    string
	TLSKeyFile     string
}

type JWTConfig struct {
	Secret            string
	Expiration        int
	RefreshExpiration int
	Issuer            string
	Audience          string
	KeyRotation       KeyRotationConfig
}

type KeyRotationConfig struct {
	Enabled     bool
	CurrentKey  string
	PreviousKey string
	KeyID       string
}

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	Timeout  time.Duration
}

type AppConfig struct {
	BaseURL    string
	Debug      bool
	AdminEmail string
	BcryptCost int
	Title      string
}

type RateLimitConfig struct {
	AuthLimit         int
	GeneralLimit      int
	HealthLimit       int
	MetricsLimit      int
	TrustProxyHeaders bool
}

func Load() (*Config, error) {
	config := &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", "goyco"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Server: ServerConfig{
			Port:           getEnv("SERVER_PORT", "8080"),
			Host:           getEnv("SERVER_HOST", "0.0.0.0"),
			ReadTimeout:    time.Duration(getEnvAsInt("SERVER_READ_TIMEOUT", 30)) * time.Second,
			WriteTimeout:   time.Duration(getEnvAsInt("SERVER_WRITE_TIMEOUT", 30)) * time.Second,
			IdleTimeout:    time.Duration(getEnvAsInt("SERVER_IDLE_TIMEOUT", 120)) * time.Second,
			MaxHeaderBytes: getEnvAsInt("SERVER_MAX_HEADER_BYTES", 1<<20),
			EnableTLS:      getEnvAsBool("SERVER_ENABLE_TLS", false),
			TLSCertFile:    getEnv("SERVER_TLS_CERT_FILE", ""),
			TLSKeyFile:     getEnv("SERVER_TLS_KEY_FILE", ""),
		},
		JWT: JWTConfig{
			Secret:            getEnv("JWT_SECRET", "your-secret-key"),
			Expiration:        getEnvAsInt("JWT_EXPIRATION", 1),
			RefreshExpiration: getEnvAsInt("JWT_REFRESH_EXPIRATION", 168),
			Issuer:            getEnv("JWT_ISSUER", "goyco"),
			Audience:          getEnv("JWT_AUDIENCE", "goyco-users"),
			KeyRotation: KeyRotationConfig{
				Enabled:     getEnvAsBool("JWT_KEY_ROTATION_ENABLED", false),
				CurrentKey:  getEnv("JWT_CURRENT_KEY", ""),
				PreviousKey: getEnv("JWT_PREVIOUS_KEY", ""),
				KeyID:       getEnv("JWT_KEY_ID", "default"),
			},
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", ""),
			Port:     getEnvAsInt("SMTP_PORT", 587),
			Username: getEnv("SMTP_USERNAME", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", ""),
			Timeout:  time.Duration(getEnvAsInt("SMTP_TIMEOUT", 30)) * time.Second,
		},
		App: AppConfig{
			BaseURL:    getEnv("APP_BASE_URL", ""),
			Debug:      getEnvAsBool("DEBUG", false),
			AdminEmail: getEnv("ADMIN_EMAIL", ""),
			BcryptCost: getEnvAsInt("BCRYPT_COST", 10),
			Title:      getEnv("TITLE", "Goyco"),
		},
		RateLimit: RateLimitConfig{
			AuthLimit:         getEnvAsInt("RATE_LIMIT_AUTH", 10),
			GeneralLimit:      getEnvAsInt("RATE_LIMIT_GENERAL", 200),
			HealthLimit:       getEnvAsInt("RATE_LIMIT_HEALTH", 120),
			MetricsLimit:      getEnvAsInt("RATE_LIMIT_METRICS", 20),
			TrustProxyHeaders: getEnvAsBool("RATE_LIMIT_TRUST_PROXY", false),
		},
		LogDir: getEnv("LOG_DIR", "/var/log/"),
		PIDDir: getEnv("PID_DIR", "/run"),
	}

	if config.App.BaseURL == "" {
		config.App.BaseURL = fmt.Sprintf("http://%s:%s", config.Server.Host, config.Server.Port)
	}

	if config.Database.Password == "" {
		return nil, fmt.Errorf("DB_PASSWORD is required")
	}

	if strings.TrimSpace(config.SMTP.Host) == "" {
		return nil, fmt.Errorf("SMTP_HOST is required")
	}

	if config.SMTP.Port <= 0 {
		return nil, fmt.Errorf("SMTP_PORT must be greater than 0")
	}

	if strings.TrimSpace(config.SMTP.From) == "" {
		return nil, fmt.Errorf("SMTP_FROM is required")
	}

	if strings.TrimSpace(config.App.AdminEmail) == "" {
		return nil, fmt.Errorf("ADMIN_EMAIL is required")
	}

	if config.Server.EnableTLS {
		if strings.TrimSpace(config.Server.TLSCertFile) == "" {
			return nil, fmt.Errorf("SERVER_TLS_CERT_FILE is required when SERVER_ENABLE_TLS is true")
		}
		if strings.TrimSpace(config.Server.TLSKeyFile) == "" {
			return nil, fmt.Errorf("SERVER_TLS_KEY_FILE is required when SERVER_ENABLE_TLS is true")
		}
	}

	if err := validateJWTConfig(&config.JWT); err != nil {
		return nil, err
	}

	if err := validateAppConfig(&config.App); err != nil {
		return nil, err
	}

	return config, nil
}

func (c *Config) GetConnectionString() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s client_encoding=UTF8",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.Name,
		c.Database.SSLMode,
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func validateJWTConfig(jwt *JWTConfig) error {
	if err := validateJWTSecret(jwt.Secret); err != nil {
		return err
	}

	if strings.TrimSpace(jwt.Issuer) == "" {
		return fmt.Errorf("JWT_ISSUER is required and cannot be empty")
	}

	if strings.TrimSpace(jwt.Audience) == "" {
		return fmt.Errorf("JWT_AUDIENCE is required and cannot be empty")
	}

	if jwt.Expiration <= 0 {
		return fmt.Errorf("JWT_EXPIRATION must be greater than 0")
	}

	if jwt.RefreshExpiration <= 0 {
		return fmt.Errorf("JWT_REFRESH_EXPIRATION must be greater than 0")
	}

	if jwt.RefreshExpiration <= jwt.Expiration {
		return fmt.Errorf("JWT_REFRESH_EXPIRATION must be greater than JWT_EXPIRATION")
	}

	if jwt.KeyRotation.Enabled {
		if strings.TrimSpace(jwt.KeyRotation.CurrentKey) == "" {
			return fmt.Errorf("JWT_CURRENT_KEY is required when key rotation is enabled")
		}

		if err := validateJWTSecret(jwt.KeyRotation.CurrentKey); err != nil {
			return fmt.Errorf("JWT_CURRENT_KEY validation failed: %w", err)
		}

		if jwt.KeyRotation.PreviousKey != "" {
			if err := validateJWTSecret(jwt.KeyRotation.PreviousKey); err != nil {
				return fmt.Errorf("JWT_PREVIOUS_KEY validation failed: %w", err)
			}
		}

		if strings.TrimSpace(jwt.KeyRotation.KeyID) == "" {
			return fmt.Errorf("JWT_KEY_ID is required when key rotation is enabled")
		}
	}

	return nil
}

func validateJWTSecret(secret string) error {
	trimmed := strings.TrimSpace(secret)

	if trimmed == "" {
		return fmt.Errorf("JWT secret is required and cannot be empty")
	}

	invalidSecrets := []string{
		"your-secret-key",
		"secret",
		"jwt-secret",
		"my-secret",
		"change-me",
		"default-secret",
		"123456",
		"password",
		"admin",
		"test",
		"development",
		"production",
		"staging",
	}

	for _, invalid := range invalidSecrets {
		if strings.EqualFold(trimmed, invalid) {
			return fmt.Errorf("JWT secret cannot be a placeholder value like %q - please set a secure, random secret", invalid)
		}
	}

	if len(trimmed) < 32 {
		return fmt.Errorf("JWT secret must be at least 32 characters long for security (current length: %d)", len(trimmed))
	}

	return nil
}

func validateAppConfig(app *AppConfig) error {

	if app.BcryptCost < 10 {
		return fmt.Errorf("BCRYPT_COST must be at least 10 for security (current: %d)", app.BcryptCost)
	}
	if app.BcryptCost > 14 {
		return fmt.Errorf("BCRYPT_COST must be at most 14 to avoid performance issues (current: %d)", app.BcryptCost)
	}

	return nil
}
