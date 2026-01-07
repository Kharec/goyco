package config

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLoadSuccess(t *testing.T) {
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "5439")
	t.Setenv("DB_USER", "goyco")
	t.Setenv("DB_PASSWORD", "super-secret")
	t.Setenv("DB_NAME", "goycodb")
	t.Setenv("DB_SSLMODE", "require")
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("SERVER_HOST", "127.0.0.1")
	t.Setenv("JWT_SECRET", "this-is-a-very-secure-jwt-secret-key-that-is-long-enough")
	t.Setenv("JWT_EXPIRATION", "12")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_USERNAME", "mailer")
	t.Setenv("SMTP_PASSWORD", "mail-secret")
	t.Setenv("SMTP_FROM", "no-reply@example.com")
	t.Setenv("APP_BASE_URL", "https://goyco.example.com")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("TITLE", "My Custom Site")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Database.Host != "db.example.com" || cfg.Database.Port != "5439" || cfg.Database.User != "goyco" {
		t.Fatalf("unexpected database config: %+v", cfg.Database)
	}

	if cfg.Database.Password != "super-secret" || cfg.Database.Name != "goycodb" || cfg.Database.SSLMode != "require" {
		t.Fatalf("unexpected database credentials: %+v", cfg.Database)
	}

	if cfg.Server.Port != "9090" || cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("unexpected server config: %+v", cfg.Server)
	}

	if cfg.JWT.Secret != "this-is-a-very-secure-jwt-secret-key-that-is-long-enough" {
		t.Fatalf("unexpected jwt secret: %q", cfg.JWT.Secret)
	}

	if cfg.JWT.Expiration != 12 {
		t.Fatalf("expected JWT expiration 12, got %d", cfg.JWT.Expiration)
	}

	if cfg.SMTP.Host != "smtp.example.com" || cfg.SMTP.Port != 2525 {
		t.Fatalf("unexpected smtp host/port: %+v", cfg.SMTP)
	}

	if cfg.SMTP.Username != "mailer" || cfg.SMTP.Password != "mail-secret" || cfg.SMTP.From != "no-reply@example.com" {
		t.Fatalf("unexpected smtp credentials: %+v", cfg.SMTP)
	}

	if cfg.App.BaseURL != "https://goyco.example.com" {
		t.Fatalf("expected base url to be overridden, got %q", cfg.App.BaseURL)
	}

	if cfg.App.Title != "My Custom Site" {
		t.Fatalf("expected title to be 'My Custom Site', got %q", cfg.App.Title)
	}
}

func TestLoadMissingPassword(t *testing.T) {
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_FROM", "no-reply@example.com")
	if _, err := Load(); err == nil {
		t.Fatalf("expected error when DB_PASSWORD is missing")
	}
}

func TestLoadDefaultBaseURL(t *testing.T) {
	t.Setenv("DB_PASSWORD", "pw")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_FROM", "no-reply@example.com")
	t.Setenv("JWT_SECRET", "this-is-a-very-secure-jwt-secret-key-that-is-long-enough")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected load to succeed, got %v", err)
	}

	if cfg.App.BaseURL != "http://0.0.0.0:8080" {
		t.Fatalf("expected default base url http://0.0.0.0:8080, got %q", cfg.App.BaseURL)
	}

	if cfg.App.Title != "Goyco" {
		t.Fatalf("expected default title to be 'Goyco', got %q", cfg.App.Title)
	}
}

func TestConfigGetConnectionString(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{
			Host:     "db",
			Port:     "5432",
			User:     "user",
			Password: "pass",
			Name:     "dbname",
			SSLMode:  "disable",
		},
	}

	got := cfg.GetConnectionString()
	expected := "host=db port=5432 user=user password=pass dbname=dbname sslmode=disable client_encoding=UTF8"

	if got != expected {
		t.Fatalf("expected connection string %q, got %q", expected, got)
	}
}

func TestGetEnv(t *testing.T) {
	const key = "CONFIG_TEST_ENV"

	t.Setenv(key, "value")
	if got := getEnv(key, "default"); got != "value" {
		t.Fatalf("expected %q, got %q", "value", got)
	}

	if got := getEnv(key+"_MISSING", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback value, got %q", got)
	}
}

func TestGetEnvAsInt(t *testing.T) {
	const key = "CONFIG_TEST_INT"

	t.Setenv(key, "42")
	if got := getEnvAsInt(key, 1); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}

	t.Setenv(key, "not-a-number")
	if got := getEnvAsInt(key, 5); got != 5 {
		t.Fatalf("expected default 5 when invalid int, got %d", got)
	}

	t.Setenv(key, "")
	if got := getEnvAsInt(key, 7); got != 7 {
		t.Fatalf("expected default 7 when env empty, got %d", got)
	}
}

func TestValidateJWTSecret(t *testing.T) {
	tests := []struct {
		name        string
		secret      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid long secret",
			secret:      "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
			expectError: false,
		},
		{
			name:        "valid secret with special chars",
			secret:      "MyV3ry$ecure&JWT!Secret#Key@2024-With-Special-Chars",
			expectError: false,
		},
		{
			name:        "empty secret",
			secret:      "",
			expectError: true,
			errorMsg:    "JWT secret is required and cannot be empty",
		},
		{
			name:        "whitespace only secret",
			secret:      "   ",
			expectError: true,
			errorMsg:    "JWT secret is required and cannot be empty",
		},
		{
			name:        "too short secret",
			secret:      "short",
			expectError: true,
			errorMsg:    "JWT secret must be at least 32 characters long for security",
		},
		{
			name:        "default placeholder secret",
			secret:      "your-secret-key",
			expectError: true,
			errorMsg:    "JWT secret cannot be a placeholder value like \"your-secret-key\"",
		},
		{
			name:        "common placeholder secret",
			secret:      "secret",
			expectError: true,
			errorMsg:    "JWT secret cannot be a placeholder value like \"secret\"",
		},
		{
			name:        "test placeholder secret",
			secret:      "test",
			expectError: true,
			errorMsg:    "JWT secret cannot be a placeholder value like \"test\"",
		},
		{
			name:        "development placeholder secret",
			secret:      "development",
			expectError: true,
			errorMsg:    "JWT secret cannot be a placeholder value like \"development\"",
		},
		{
			name:        "case insensitive placeholder",
			secret:      "SECRET",
			expectError: true,
			errorMsg:    "JWT secret cannot be a placeholder value like \"secret\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJWTSecret(tt.secret)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error for secret %q, got nil", tt.secret)
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Fatalf("expected error message to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Fatalf("unexpected error for secret %q: %v", tt.secret, err)
			}
		})
	}
}

func TestLoadWithInvalidJWTSecret(t *testing.T) {
	t.Setenv("DB_PASSWORD", "password")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_FROM", "no-reply@example.com")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")

	t.Setenv("JWT_SECRET", "your-secret-key")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET is placeholder value")
	}
	if !strings.Contains(err.Error(), "your-secret-key") {
		t.Fatalf("expected error message to mention placeholder value, got: %v", err)
	}
}

func TestValidateJWTConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      JWTConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        1,
				RefreshExpiration: 24,
				Issuer:            "goyco",
				Audience:          "goyco-users",
			},
			expectError: false,
		},
		{
			name: "empty issuer",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        1,
				RefreshExpiration: 24,
				Issuer:            "",
				Audience:          "goyco-users",
			},
			expectError: true,
			errorMsg:    "JWT_ISSUER is required and cannot be empty",
		},
		{
			name: "whitespace only issuer",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        1,
				RefreshExpiration: 24,
				Issuer:            "   ",
				Audience:          "goyco-users",
			},
			expectError: true,
			errorMsg:    "JWT_ISSUER is required and cannot be empty",
		},
		{
			name: "empty audience",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        1,
				RefreshExpiration: 24,
				Issuer:            "goyco",
				Audience:          "",
			},
			expectError: true,
			errorMsg:    "JWT_AUDIENCE is required and cannot be empty",
		},
		{
			name: "whitespace only audience",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        1,
				RefreshExpiration: 24,
				Issuer:            "goyco",
				Audience:          "   ",
			},
			expectError: true,
			errorMsg:    "JWT_AUDIENCE is required and cannot be empty",
		},
		{
			name: "zero expiration",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        0,
				RefreshExpiration: 24,
				Issuer:            "goyco",
				Audience:          "goyco-users",
			},
			expectError: true,
			errorMsg:    "JWT_EXPIRATION must be greater than 0",
		},
		{
			name: "negative expiration",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        -1,
				RefreshExpiration: 24,
				Issuer:            "goyco",
				Audience:          "goyco-users",
			},
			expectError: true,
			errorMsg:    "JWT_EXPIRATION must be greater than 0",
		},
		{
			name: "zero refresh expiration",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        1,
				RefreshExpiration: 0,
				Issuer:            "goyco",
				Audience:          "goyco-users",
			},
			expectError: true,
			errorMsg:    "JWT_REFRESH_EXPIRATION must be greater than 0",
		},
		{
			name: "negative refresh expiration",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        1,
				RefreshExpiration: -1,
				Issuer:            "goyco",
				Audience:          "goyco-users",
			},
			expectError: true,
			errorMsg:    "JWT_REFRESH_EXPIRATION must be greater than 0",
		},
		{
			name: "refresh expiration not greater than access expiration",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        24,
				RefreshExpiration: 24,
				Issuer:            "goyco",
				Audience:          "goyco-users",
			},
			expectError: true,
			errorMsg:    "JWT_REFRESH_EXPIRATION must be greater than JWT_EXPIRATION",
		},
		{
			name: "refresh expiration less than access expiration",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        24,
				RefreshExpiration: 12,
				Issuer:            "goyco",
				Audience:          "goyco-users",
			},
			expectError: true,
			errorMsg:    "JWT_REFRESH_EXPIRATION must be greater than JWT_EXPIRATION",
		},
		{
			name: "key rotation enabled but no current key",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        1,
				RefreshExpiration: 24,
				Issuer:            "goyco",
				Audience:          "goyco-users",
				KeyRotation: KeyRotationConfig{
					Enabled:     true,
					CurrentKey:  "",
					PreviousKey: "",
					KeyID:       "test-key",
				},
			},
			expectError: true,
			errorMsg:    "JWT_CURRENT_KEY is required when key rotation is enabled",
		},
		{
			name: "key rotation enabled but no key ID",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        1,
				RefreshExpiration: 24,
				Issuer:            "goyco",
				Audience:          "goyco-users",
				KeyRotation: KeyRotationConfig{
					Enabled:     true,
					CurrentKey:  "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
					PreviousKey: "",
					KeyID:       "",
				},
			},
			expectError: true,
			errorMsg:    "JWT_KEY_ID is required when key rotation is enabled",
		},
		{
			name: "key rotation enabled with invalid current key",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        1,
				RefreshExpiration: 24,
				Issuer:            "goyco",
				Audience:          "goyco-users",
				KeyRotation: KeyRotationConfig{
					Enabled:     true,
					CurrentKey:  "short",
					PreviousKey: "",
					KeyID:       "test-key",
				},
			},
			expectError: true,
			errorMsg:    "JWT_CURRENT_KEY validation failed",
		},
		{
			name: "key rotation enabled with invalid previous key",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        1,
				RefreshExpiration: 24,
				Issuer:            "goyco",
				Audience:          "goyco-users",
				KeyRotation: KeyRotationConfig{
					Enabled:     true,
					CurrentKey:  "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
					PreviousKey: "short",
					KeyID:       "test-key",
				},
			},
			expectError: true,
			errorMsg:    "JWT_PREVIOUS_KEY validation failed",
		},
		{
			name: "valid key rotation config",
			config: JWTConfig{
				Secret:            "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				Expiration:        1,
				RefreshExpiration: 24,
				Issuer:            "goyco",
				Audience:          "goyco-users",
				KeyRotation: KeyRotationConfig{
					Enabled:     true,
					CurrentKey:  "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
					PreviousKey: "this-is-another-very-secure-jwt-secret-key-that-is-long-enough",
					KeyID:       "test-key",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJWTConfig(&tt.config)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error for config %+v, got nil", tt.config)
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Fatalf("expected error message to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Fatalf("unexpected error for config %+v: %v", tt.config, err)
			}
		})
	}
}

func TestLoadWithInvalidJWTConfig(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name: "whitespace only issuer",
			envVars: map[string]string{
				"DB_PASSWORD":            "password",
				"SMTP_HOST":              "smtp.example.com",
				"SMTP_PORT":              "2525",
				"SMTP_FROM":              "no-reply@example.com",
				"ADMIN_EMAIL":            "admin@example.com",
				"JWT_SECRET":             "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				"JWT_ISSUER":             "   ",
				"JWT_AUDIENCE":           "goyco-users",
				"JWT_EXPIRATION":         "1",
				"JWT_REFRESH_EXPIRATION": "24",
			},
			expectError: true,
			errorMsg:    "JWT_ISSUER is required and cannot be empty",
		},
		{
			name: "whitespace only audience",
			envVars: map[string]string{
				"DB_PASSWORD":            "password",
				"SMTP_HOST":              "smtp.example.com",
				"SMTP_PORT":              "2525",
				"SMTP_FROM":              "no-reply@example.com",
				"ADMIN_EMAIL":            "admin@example.com",
				"JWT_SECRET":             "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				"JWT_ISSUER":             "goyco",
				"JWT_AUDIENCE":           "   ",
				"JWT_EXPIRATION":         "1",
				"JWT_REFRESH_EXPIRATION": "24",
			},
			expectError: true,
			errorMsg:    "JWT_AUDIENCE is required and cannot be empty",
		},
		{
			name: "zero expiration",
			envVars: map[string]string{
				"DB_PASSWORD":            "password",
				"SMTP_HOST":              "smtp.example.com",
				"SMTP_PORT":              "2525",
				"SMTP_FROM":              "no-reply@example.com",
				"ADMIN_EMAIL":            "admin@example.com",
				"JWT_SECRET":             "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				"JWT_ISSUER":             "goyco",
				"JWT_AUDIENCE":           "goyco-users",
				"JWT_EXPIRATION":         "0",
				"JWT_REFRESH_EXPIRATION": "24",
			},
			expectError: true,
			errorMsg:    "JWT_EXPIRATION must be greater than 0",
		},
		{
			name: "refresh expiration not greater than access expiration",
			envVars: map[string]string{
				"DB_PASSWORD":            "password",
				"SMTP_HOST":              "smtp.example.com",
				"SMTP_PORT":              "2525",
				"SMTP_FROM":              "no-reply@example.com",
				"ADMIN_EMAIL":            "admin@example.com",
				"JWT_SECRET":             "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				"JWT_ISSUER":             "goyco",
				"JWT_AUDIENCE":           "goyco-users",
				"JWT_EXPIRATION":         "24",
				"JWT_REFRESH_EXPIRATION": "24",
			},
			expectError: true,
			errorMsg:    "JWT_REFRESH_EXPIRATION must be greater than JWT_EXPIRATION",
		},
		{
			name: "key rotation enabled but no current key",
			envVars: map[string]string{
				"DB_PASSWORD":              "password",
				"SMTP_HOST":                "smtp.example.com",
				"SMTP_PORT":                "2525",
				"SMTP_FROM":                "no-reply@example.com",
				"ADMIN_EMAIL":              "admin@example.com",
				"JWT_SECRET":               "this-is-a-very-secure-jwt-secret-key-that-is-long-enough",
				"JWT_ISSUER":               "goyco",
				"JWT_AUDIENCE":             "goyco-users",
				"JWT_EXPIRATION":           "1",
				"JWT_REFRESH_EXPIRATION":   "24",
				"JWT_KEY_ROTATION_ENABLED": "true",
				"JWT_CURRENT_KEY":          "",
				"JWT_KEY_ID":               "test-key",
			},
			expectError: true,
			errorMsg:    "JWT_CURRENT_KEY is required when key rotation is enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			envVars := []string{
				"JWT_SECRET", "JWT_ISSUER", "JWT_AUDIENCE", "JWT_EXPIRATION", "JWT_REFRESH_EXPIRATION",
				"JWT_KEY_ROTATION_ENABLED", "JWT_CURRENT_KEY", "JWT_PREVIOUS_KEY", "JWT_KEY_ID",
			}
			for _, envVar := range envVars {
				t.Setenv(envVar, "")
			}

			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			_, err := Load()
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Fatalf("expected error message to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestServerConfigDefaults(t *testing.T) {
	envVars := []string{
		"SERVER_READ_TIMEOUT",
		"SERVER_WRITE_TIMEOUT",
		"SERVER_IDLE_TIMEOUT",
		"SERVER_MAX_HEADER_BYTES",
		"SERVER_ENABLE_TLS",
		"SERVER_TLS_CERT_FILE",
		"SERVER_TLS_KEY_FILE",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}

	os.Setenv("DB_PASSWORD", "testpassword")
	os.Setenv("SMTP_HOST", "smtp.example.com")
	os.Setenv("SMTP_FROM", "test@example.com")
	os.Setenv("ADMIN_EMAIL", "admin@example.com")
	os.Setenv("JWT_SECRET", "this-is-a-very-long-secret-key-for-testing-purposes-only")

	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Server.ReadTimeout != 30*time.Second {
		t.Errorf("Expected ReadTimeout to be 30s, got %v", config.Server.ReadTimeout)
	}

	if config.Server.WriteTimeout != 30*time.Second {
		t.Errorf("Expected WriteTimeout to be 30s, got %v", config.Server.WriteTimeout)
	}

	if config.Server.IdleTimeout != 120*time.Second {
		t.Errorf("Expected IdleTimeout to be 120s, got %v", config.Server.IdleTimeout)
	}

	if config.Server.MaxHeaderBytes != 1<<20 {
		t.Errorf("Expected MaxHeaderBytes to be 1MB, got %d", config.Server.MaxHeaderBytes)
	}

	if config.Server.EnableTLS {
		t.Error("Expected EnableTLS to be false by default")
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}

func TestServerConfigCustomValues(t *testing.T) {
	os.Setenv("DB_PASSWORD", "testpassword")
	os.Setenv("SMTP_HOST", "smtp.example.com")
	os.Setenv("SMTP_FROM", "test@example.com")
	os.Setenv("ADMIN_EMAIL", "admin@example.com")
	os.Setenv("JWT_SECRET", "this-is-a-very-long-secret-key-for-testing-purposes-only")
	os.Setenv("SERVER_READ_TIMEOUT", "60")
	os.Setenv("SERVER_WRITE_TIMEOUT", "45")
	os.Setenv("SERVER_IDLE_TIMEOUT", "180")
	os.Setenv("SERVER_MAX_HEADER_BYTES", "2097152")
	os.Setenv("SERVER_ENABLE_TLS", "true")
	os.Setenv("SERVER_TLS_CERT_FILE", "/path/to/cert.pem")
	os.Setenv("SERVER_TLS_KEY_FILE", "/path/to/key.pem")

	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Server.ReadTimeout != 60*time.Second {
		t.Errorf("Expected ReadTimeout to be 60s, got %v", config.Server.ReadTimeout)
	}

	if config.Server.WriteTimeout != 45*time.Second {
		t.Errorf("Expected WriteTimeout to be 45s, got %v", config.Server.WriteTimeout)
	}

	if config.Server.IdleTimeout != 180*time.Second {
		t.Errorf("Expected IdleTimeout to be 180s, got %v", config.Server.IdleTimeout)
	}

	if config.Server.MaxHeaderBytes != 2<<20 {
		t.Errorf("Expected MaxHeaderBytes to be 2MB, got %d", config.Server.MaxHeaderBytes)
	}

	if !config.Server.EnableTLS {
		t.Error("Expected EnableTLS to be true")
	}

	if config.Server.TLSCertFile != "/path/to/cert.pem" {
		t.Errorf("Expected TLSCertFile to be /path/to/cert.pem, got %s", config.Server.TLSCertFile)
	}

	if config.Server.TLSKeyFile != "/path/to/key.pem" {
		t.Errorf("Expected TLSKeyFile to be /path/to/key.pem, got %s", config.Server.TLSKeyFile)
	}

	envVars := []string{
		"SERVER_READ_TIMEOUT",
		"SERVER_WRITE_TIMEOUT",
		"SERVER_IDLE_TIMEOUT",
		"SERVER_MAX_HEADER_BYTES",
		"SERVER_ENABLE_TLS",
		"SERVER_TLS_CERT_FILE",
		"SERVER_TLS_KEY_FILE",
	}
	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}

func TestServerConfigEdgeCases(t *testing.T) {
	os.Setenv("DB_PASSWORD", "testpassword")
	os.Setenv("SMTP_HOST", "smtp.example.com")
	os.Setenv("SMTP_FROM", "test@example.com")
	os.Setenv("ADMIN_EMAIL", "admin@example.com")
	os.Setenv("JWT_SECRET", "this-is-a-very-long-secret-key-for-testing-purposes-only")
	os.Setenv("SERVER_READ_TIMEOUT", "0")
	os.Setenv("SERVER_WRITE_TIMEOUT", "0")
	os.Setenv("SERVER_IDLE_TIMEOUT", "0")

	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Server.ReadTimeout != 0 {
		t.Errorf("Expected ReadTimeout to be 0, got %v", config.Server.ReadTimeout)
	}

	if config.Server.WriteTimeout != 0 {
		t.Errorf("Expected WriteTimeout to be 0, got %v", config.Server.WriteTimeout)
	}

	if config.Server.IdleTimeout != 0 {
		t.Errorf("Expected IdleTimeout to be 0, got %v", config.Server.IdleTimeout)
	}

	os.Setenv("SERVER_MAX_HEADER_BYTES", "10485760")

	config, err = Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Server.MaxHeaderBytes != 10485760 {
		t.Errorf("Expected MaxHeaderBytes to be 10MB, got %d", config.Server.MaxHeaderBytes)
	}

	envVars := []string{
		"SERVER_READ_TIMEOUT",
		"SERVER_WRITE_TIMEOUT",
		"SERVER_IDLE_TIMEOUT",
		"SERVER_MAX_HEADER_BYTES",
	}
	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}

func TestTLSValidation(t *testing.T) {
	os.Setenv("DB_PASSWORD", "testpassword")
	os.Setenv("SMTP_HOST", "smtp.example.com")
	os.Setenv("SMTP_FROM", "test@example.com")
	os.Setenv("ADMIN_EMAIL", "admin@example.com")
	os.Setenv("JWT_SECRET", "this-is-a-very-long-secret-key-for-testing-purposes-only")
	os.Setenv("SERVER_ENABLE_TLS", "true")

	_, err := Load()
	if err == nil {
		t.Error("Expected error when TLS is enabled without cert files")
	}

	if err.Error() != "SERVER_TLS_CERT_FILE is required when SERVER_ENABLE_TLS is true" {
		t.Errorf("Expected specific error message, got: %v", err)
	}

	os.Setenv("SERVER_TLS_CERT_FILE", "/path/to/cert.pem")

	_, err = Load()
	if err == nil {
		t.Error("Expected error when TLS is enabled without key file")
	}

	if err.Error() != "SERVER_TLS_KEY_FILE is required when SERVER_ENABLE_TLS is true" {
		t.Errorf("Expected specific error message, got: %v", err)
	}

	os.Setenv("SERVER_TLS_KEY_FILE", "/path/to/key.pem")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config with TLS: %v", err)
	}

	if !cfg.Server.EnableTLS {
		t.Error("Expected EnableTLS to be true")
	}

	envVars := []string{
		"SERVER_ENABLE_TLS",
		"SERVER_TLS_CERT_FILE",
		"SERVER_TLS_KEY_FILE",
	}
	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}

func TestValidateBcryptCost(t *testing.T) {
	tests := []struct {
		name        string
		bcryptCost  int
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid cost at minimum (10)",
			bcryptCost:  10,
			expectError: false,
		},
		{
			name:        "valid cost at maximum (14)",
			bcryptCost:  14,
			expectError: false,
		},
		{
			name:        "valid cost in middle (12)",
			bcryptCost:  12,
			expectError: false,
		},
		{
			name:        "cost too low (9)",
			bcryptCost:  9,
			expectError: true,
			errorMsg:    "BCRYPT_COST must be at least 10 for security",
		},
		{
			name:        "cost too low (5)",
			bcryptCost:  5,
			expectError: true,
			errorMsg:    "BCRYPT_COST must be at least 10 for security",
		},
		{
			name:        "cost too low (0)",
			bcryptCost:  0,
			expectError: true,
			errorMsg:    "BCRYPT_COST must be at least 10 for security",
		},
		{
			name:        "cost too high (15)",
			bcryptCost:  15,
			expectError: true,
			errorMsg:    "BCRYPT_COST must be at most 14 to avoid performance issues",
		},
		{
			name:        "cost too high (20)",
			bcryptCost:  20,
			expectError: true,
			errorMsg:    "BCRYPT_COST must be at most 14 to avoid performance issues",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appConfig := AppConfig{
				BcryptCost: tt.bcryptCost,
			}
			err := validateAppConfig(&appConfig)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error for BCRYPT_COST %d, got nil", tt.bcryptCost)
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Fatalf("expected error message to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Fatalf("unexpected error for BCRYPT_COST %d: %v", tt.bcryptCost, err)
			}
		})
	}
}

func TestLoadWithInvalidBcryptCost(t *testing.T) {
	tests := []struct {
		name        string
		bcryptCost  string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "cost too low",
			bcryptCost:  "9",
			expectError: true,
			errorMsg:    "BCRYPT_COST must be at least 10",
		},
		{
			name:        "cost too high",
			bcryptCost:  "15",
			expectError: true,
			errorMsg:    "BCRYPT_COST must be at most 14",
		},
		{
			name:        "valid cost",
			bcryptCost:  "12",
			expectError: false,
		},
		{
			name:        "default cost",
			bcryptCost:  "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			t.Setenv("DB_PASSWORD", "password")
			t.Setenv("SMTP_HOST", "smtp.example.com")
			t.Setenv("SMTP_PORT", "2525")
			t.Setenv("SMTP_FROM", "no-reply@example.com")
			t.Setenv("ADMIN_EMAIL", "admin@example.com")
			t.Setenv("JWT_SECRET", "this-is-a-very-secure-jwt-secret-key-that-is-long-enough")

			if tt.bcryptCost != "" {
				t.Setenv("BCRYPT_COST", tt.bcryptCost)
			} else {
				os.Unsetenv("BCRYPT_COST")
			}

			cfg, err := Load()
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Fatalf("expected error message to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			} else {
				expectedCost := 12
				if tt.bcryptCost == "" {
					expectedCost = 10
				} else if costInt, err := strconv.Atoi(tt.bcryptCost); err == nil {
					expectedCost = costInt
				}
				if cfg.App.BcryptCost != expectedCost {
					t.Fatalf("expected BCRYPT_COST %d, got %d", expectedCost, cfg.App.BcryptCost)
				}
			}
		})
	}
}

func TestCLIConfigJSONOutput(t *testing.T) {
	tests := []struct {
		name           string
		envValue       string
		expectedOutput bool
	}{
		{
			name:           "default false when not set",
			envValue:       "",
			expectedOutput: false,
		},
		{
			name:           "true when set to true",
			envValue:       "true",
			expectedOutput: true,
		},
		{
			name:           "false when set to false",
			envValue:       "false",
			expectedOutput: false,
		},
		{
			name:           "true when set to 1",
			envValue:       "1",
			expectedOutput: true,
		},
		{
			name:           "false when set to 0",
			envValue:       "0",
			expectedOutput: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DB_PASSWORD", "password")
			t.Setenv("SMTP_HOST", "smtp.example.com")
			t.Setenv("SMTP_PORT", "2525")
			t.Setenv("SMTP_FROM", "no-reply@example.com")
			t.Setenv("ADMIN_EMAIL", "admin@example.com")
			t.Setenv("JWT_SECRET", "this-is-a-very-secure-jwt-secret-key-that-is-long-enough")

			if tt.envValue != "" {
				t.Setenv("CLI_JSON_OUTPUT", tt.envValue)
			} else {
				os.Unsetenv("CLI_JSON_OUTPUT")
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.CLI.JSONOutputDefault != tt.expectedOutput {
				t.Fatalf("expected CLI.JSONOutputDefault to be %v, got %v", tt.expectedOutput, cfg.CLI.JSONOutputDefault)
			}
		})
	}
}
