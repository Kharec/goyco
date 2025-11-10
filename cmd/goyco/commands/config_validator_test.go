package commands

import (
	"os"
	"path/filepath"
	"testing"

	"goyco/internal/config"
)

func TestNewConfigValidator(t *testing.T) {
	tests := []struct {
		name        string
		auditLogger *AuditLogger
	}{
		{
			name:        "with audit logger",
			auditLogger: &AuditLogger{},
		},
		{
			name:        "without audit logger",
			auditLogger: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewConfigValidator(tt.auditLogger)
			if validator == nil {
				t.Error("expected validator to not be nil")
				return
			}
			if validator.auditLogger != tt.auditLogger {
				t.Errorf("expected audit logger %v, got %v", tt.auditLogger, validator.auditLogger)
			}
		})
	}
}

func TestValidateConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.Config
		expectError bool
	}{
		{
			name: "valid configuration",
			cfg: &config.Config{
				Database: config.DatabaseConfig{
					Host:     "localhost",
					Port:     "5432",
					User:     "postgres",
					Password: "password123",
					Name:     "testdb",
				},
				SMTP: config.SMTPConfig{
					Host: "smtp.example.com",
					Port: 587,
					From: "test@example.com",
				},
				Server: config.ServerConfig{
					Port:      "8080",
					EnableTLS: false,
				},
				JWT: config.JWTConfig{
					Secret: "this-is-a-very-secure-secret-key-that-is-long-enough",
				},
				App: config.AppConfig{
					AdminEmail: "admin@example.com",
				},
			},
			expectError: false,
		},
		{
			name: "invalid database config",
			cfg: &config.Config{
				Database: config.DatabaseConfig{
					Host:     "",
					Port:     "5432",
					User:     "postgres",
					Password: "password123",
					Name:     "testdb",
				},
				SMTP: config.SMTPConfig{
					Host: "smtp.example.com",
					Port: 587,
					From: "test@example.com",
				},
				Server: config.ServerConfig{
					Port:      "8080",
					EnableTLS: false,
				},
				JWT: config.JWTConfig{
					Secret: "this-is-a-very-secure-secret-key-that-is-long-enough",
				},
				App: config.AppConfig{
					AdminEmail: "admin@example.com",
				},
			},
			expectError: true,
		},
		{
			name: "invalid SMTP config",
			cfg: &config.Config{
				Database: config.DatabaseConfig{
					Host:     "localhost",
					Port:     "5432",
					User:     "postgres",
					Password: "password123",
					Name:     "testdb",
				},
				SMTP: config.SMTPConfig{
					Host: "",
					Port: 587,
					From: "test@example.com",
				},
				Server: config.ServerConfig{
					Port:      "8080",
					EnableTLS: false,
				},
				JWT: config.JWTConfig{
					Secret: "this-is-a-very-secure-secret-key-that-is-long-enough",
				},
				App: config.AppConfig{
					AdminEmail: "admin@example.com",
				},
			},
			expectError: true,
		},
		{
			name: "multiple validation errors",
			cfg: &config.Config{
				Database: config.DatabaseConfig{
					Host:     "",
					Port:     "invalid",
					User:     "",
					Password: "",
					Name:     "",
				},
				SMTP: config.SMTPConfig{
					Host: "",
					Port: 0,
					From: "",
				},
				Server: config.ServerConfig{
					Port:      "invalid",
					EnableTLS: false,
				},
				JWT: config.JWTConfig{
					Secret: "",
				},
				App: config.AppConfig{
					AdminEmail: "",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewConfigValidator(nil)
			err := validator.ValidateConfiguration(tt.cfg)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateDatabaseConfig(t *testing.T) {
	validator := NewConfigValidator(nil)

	tests := []struct {
		name        string
		cfg         config.DatabaseConfig
		expectError bool
	}{
		{
			name: "valid database config",
			cfg: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "postgres",
				Password: "password123",
				Name:     "testdb",
			},
			expectError: false,
		},
		{
			name: "missing host",
			cfg: config.DatabaseConfig{
				Host:     "",
				Port:     "5432",
				User:     "postgres",
				Password: "password123",
				Name:     "testdb",
			},
			expectError: true,
		},
		{
			name: "invalid port - not a number",
			cfg: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "not-a-port",
				User:     "postgres",
				Password: "password123",
				Name:     "testdb",
			},
			expectError: true,
		},
		{
			name: "invalid port - zero",
			cfg: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "0",
				User:     "postgres",
				Password: "password123",
				Name:     "testdb",
			},
			expectError: true,
		},
		{
			name: "invalid port - too large",
			cfg: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "65536",
				User:     "postgres",
				Password: "password123",
				Name:     "testdb",
			},
			expectError: true,
		},
		{
			name: "missing database name",
			cfg: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "postgres",
				Password: "password123",
				Name:     "",
			},
			expectError: true,
		},
		{
			name: "missing user",
			cfg: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "",
				Password: "password123",
				Name:     "testdb",
			},
			expectError: true,
		},
		{
			name: "missing password",
			cfg: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "postgres",
				Password: "",
				Name:     "testdb",
			},
			expectError: true,
		},
		{
			name: "invalid host format",
			cfg: config.DatabaseConfig{
				Host:     "invalid..host",
				Port:     "5432",
				User:     "postgres",
				Password: "password123",
				Name:     "testdb",
			},
			expectError: true,
		},
		{
			name: "valid IP host",
			cfg: config.DatabaseConfig{
				Host:     "127.0.0.1",
				Port:     "5432",
				User:     "postgres",
				Password: "password123",
				Name:     "testdb",
			},
			expectError: false,
		},
		{
			name: "valid hostname",
			cfg: config.DatabaseConfig{
				Host:     "db.example.com",
				Port:     "5432",
				User:     "postgres",
				Password: "password123",
				Name:     "testdb",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Database: tt.cfg}
			err := validator.validateDatabaseConfig(cfg)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateSMTPConfig(t *testing.T) {
	validator := NewConfigValidator(nil)

	tests := []struct {
		name        string
		cfg         *config.Config
		expectError bool
	}{
		{
			name: "valid SMTP config",
			cfg: &config.Config{
				SMTP: config.SMTPConfig{
					Host: "smtp.example.com",
					Port: 587,
					From: "test@example.com",
				},
				App: config.AppConfig{
					AdminEmail: "admin@example.com",
				},
			},
			expectError: false,
		},
		{
			name: "missing SMTP host",
			cfg: &config.Config{
				SMTP: config.SMTPConfig{
					Host: "",
					Port: 587,
					From: "test@example.com",
				},
				App: config.AppConfig{
					AdminEmail: "admin@example.com",
				},
			},
			expectError: true,
		},
		{
			name: "invalid SMTP port - zero",
			cfg: &config.Config{
				SMTP: config.SMTPConfig{
					Host: "smtp.example.com",
					Port: 0,
					From: "test@example.com",
				},
				App: config.AppConfig{
					AdminEmail: "admin@example.com",
				},
			},
			expectError: true,
		},
		{
			name: "invalid SMTP port - too large",
			cfg: &config.Config{
				SMTP: config.SMTPConfig{
					Host: "smtp.example.com",
					Port: 65536,
					From: "test@example.com",
				},
				App: config.AppConfig{
					AdminEmail: "admin@example.com",
				},
			},
			expectError: true,
		},
		{
			name: "missing SMTP from",
			cfg: &config.Config{
				SMTP: config.SMTPConfig{
					Host: "smtp.example.com",
					Port: 587,
					From: "",
				},
				App: config.AppConfig{
					AdminEmail: "admin@example.com",
				},
			},
			expectError: true,
		},
		{
			name: "invalid SMTP from email",
			cfg: &config.Config{
				SMTP: config.SMTPConfig{
					Host: "smtp.example.com",
					Port: 587,
					From: "not-an-email",
				},
				App: config.AppConfig{
					AdminEmail: "admin@example.com",
				},
			},
			expectError: true,
		},
		{
			name: "missing admin email",
			cfg: &config.Config{
				SMTP: config.SMTPConfig{
					Host: "smtp.example.com",
					Port: 587,
					From: "test@example.com",
				},
				App: config.AppConfig{
					AdminEmail: "",
				},
			},
			expectError: true,
		},
		{
			name: "invalid admin email",
			cfg: &config.Config{
				SMTP: config.SMTPConfig{
					Host: "smtp.example.com",
					Port: 587,
					From: "test@example.com",
				},
				App: config.AppConfig{
					AdminEmail: "not-an-email",
				},
			},
			expectError: true,
		},
		{
			name: "invalid SMTP host format",
			cfg: &config.Config{
				SMTP: config.SMTPConfig{
					Host: "invalid..host",
					Port: 587,
					From: "test@example.com",
				},
				App: config.AppConfig{
					AdminEmail: "admin@example.com",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateSMTPConfig(tt.cfg)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateServerConfig(t *testing.T) {
	validator := NewConfigValidator(nil)

	tempDir := t.TempDir()
	certFile := filepath.Join(tempDir, "cert.pem")
	keyFile := filepath.Join(tempDir, "key.pem")

	_ = os.WriteFile(certFile, []byte("test cert"), 0644)
	_ = os.WriteFile(keyFile, []byte("test key"), 0644)

	tests := []struct {
		name        string
		cfg         *config.Config
		expectError bool
	}{
		{
			name: "valid server config without TLS",
			cfg: &config.Config{
				Server: config.ServerConfig{
					Port:      "8080",
					EnableTLS: false,
				},
			},
			expectError: false,
		},
		{
			name: "valid server config with TLS",
			cfg: &config.Config{
				Server: config.ServerConfig{
					Port:        "8443",
					EnableTLS:   true,
					TLSCertFile: certFile,
					TLSKeyFile:  keyFile,
				},
			},
			expectError: false,
		},
		{
			name: "invalid port - not a number",
			cfg: &config.Config{
				Server: config.ServerConfig{
					Port:      "not-a-port",
					EnableTLS: false,
				},
			},
			expectError: true,
		},
		{
			name: "invalid port - zero",
			cfg: &config.Config{
				Server: config.ServerConfig{
					Port:      "0",
					EnableTLS: false,
				},
			},
			expectError: true,
		},
		{
			name: "invalid port - too large",
			cfg: &config.Config{
				Server: config.ServerConfig{
					Port:      "65536",
					EnableTLS: false,
				},
			},
			expectError: true,
		},
		{
			name: "TLS enabled but missing cert file",
			cfg: &config.Config{
				Server: config.ServerConfig{
					Port:        "8443",
					EnableTLS:   true,
					TLSCertFile: "",
					TLSKeyFile:  keyFile,
				},
			},
			expectError: true,
		},
		{
			name: "TLS enabled but missing key file",
			cfg: &config.Config{
				Server: config.ServerConfig{
					Port:        "8443",
					EnableTLS:   true,
					TLSCertFile: certFile,
					TLSKeyFile:  "",
				},
			},
			expectError: true,
		},
		{
			name: "valid base URL",
			cfg: &config.Config{
				Server: config.ServerConfig{
					Port:      "8080",
					EnableTLS: false,
				},
				App: config.AppConfig{
					BaseURL: "https://example.com",
				},
			},
			expectError: false,
		},
		{
			name: "invalid base URL",
			cfg: &config.Config{
				Server: config.ServerConfig{
					Port:      "8080",
					EnableTLS: false,
				},
				App: config.AppConfig{
					BaseURL: "not-a-url",
				},
			},
			expectError: true,
		},
		{
			name: "base URL with port",
			cfg: &config.Config{
				Server: config.ServerConfig{
					Port:      "8080",
					EnableTLS: false,
				},
				App: config.AppConfig{
					BaseURL: "http://example.com:8080",
				},
			},
			expectError: false,
		},
		{
			name: "base URL with path",
			cfg: &config.Config{
				Server: config.ServerConfig{
					Port:      "8080",
					EnableTLS: false,
				},
				App: config.AppConfig{
					BaseURL: "https://example.com/api/v1",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateServerConfig(tt.cfg)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateSecurityConfig(t *testing.T) {
	validator := NewConfigValidator(nil)

	tests := []struct {
		name        string
		cfg         *config.Config
		expectError bool
	}{
		{
			name: "valid JWT secret",
			cfg: &config.Config{
				JWT: config.JWTConfig{
					Secret: "this-is-a-very-secure-secret-key-that-is-long-enough",
				},
			},
			expectError: false,
		},
		{
			name: "missing JWT secret",
			cfg: &config.Config{
				JWT: config.JWTConfig{
					Secret: "",
				},
			},
			expectError: true,
		},
		{
			name: "JWT secret too short",
			cfg: &config.Config{
				JWT: config.JWTConfig{
					Secret: "short",
				},
			},
			expectError: true,
		},
		{
			name: "JWT secret exactly 32 characters",
			cfg: &config.Config{
				JWT: config.JWTConfig{
					Secret: "12345678901234567890123456789012",
				},
			},
			expectError: false,
		},
		{
			name: "JWT secret - weak value: your-secret-key",
			cfg: &config.Config{
				JWT: config.JWTConfig{
					Secret: "your-secret-key",
				},
			},
			expectError: true,
		},
		{
			name: "JWT secret - weak value: secret",
			cfg: &config.Config{
				JWT: config.JWTConfig{
					Secret: "secret",
				},
			},
			expectError: true,
		},
		{
			name: "JWT secret - weak value: jwt-secret",
			cfg: &config.Config{
				JWT: config.JWTConfig{
					Secret: "jwt-secret",
				},
			},
			expectError: true,
		},
		{
			name: "JWT secret - weak value: change-me",
			cfg: &config.Config{
				JWT: config.JWTConfig{
					Secret: "change-me",
				},
			},
			expectError: true,
		},
		{
			name: "JWT secret - weak value: development",
			cfg: &config.Config{
				JWT: config.JWTConfig{
					Secret: "development",
				},
			},
			expectError: true,
		},
		{
			name: "JWT secret - weak value: production",
			cfg: &config.Config{
				JWT: config.JWTConfig{
					Secret: "production",
				},
			},
			expectError: true,
		},
		{
			name: "JWT secret case insensitive weak check",
			cfg: &config.Config{
				JWT: config.JWTConfig{
					Secret: "DEVELOPMENT",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateSecurityConfig(tt.cfg)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateFilePaths(t *testing.T) {
	validator := NewConfigValidator(nil)

	tempDir := t.TempDir()
	validDir := filepath.Join(tempDir, "logs")
	validCertFile := filepath.Join(tempDir, "cert.pem")
	validKeyFile := filepath.Join(tempDir, "key.pem")

	_ = os.MkdirAll(validDir, 0755)
	_ = os.WriteFile(validCertFile, []byte("test cert"), 0644)
	_ = os.WriteFile(validKeyFile, []byte("test key"), 0644)

	tests := []struct {
		name        string
		cfg         *config.Config
		expectError bool
	}{
		{
			name: "valid file paths",
			cfg: &config.Config{
				LogDir: validDir,
				PIDDir: validDir,
				Server: config.ServerConfig{
					EnableTLS:   true,
					TLSCertFile: validCertFile,
					TLSKeyFile:  validKeyFile,
				},
			},
			expectError: false,
		},
		{
			name: "empty paths - no TLS",
			cfg: &config.Config{
				LogDir: "",
				PIDDir: "",
				Server: config.ServerConfig{
					EnableTLS: false,
				},
			},
			expectError: false,
		},
		{
			name: "TLS enabled but cert file missing",
			cfg: &config.Config{
				Server: config.ServerConfig{
					EnableTLS:   true,
					TLSCertFile: "/nonexistent/cert.pem",
					TLSKeyFile:  validKeyFile,
				},
			},
			expectError: true,
		},
		{
			name: "TLS enabled but key file missing",
			cfg: &config.Config{
				Server: config.ServerConfig{
					EnableTLS:   true,
					TLSCertFile: validCertFile,
					TLSKeyFile:  "/nonexistent/key.pem",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateFilePaths(tt.cfg)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateDirectory(t *testing.T) {
	validator := NewConfigValidator(nil)

	tempDir := t.TempDir()
	existingDir := filepath.Join(tempDir, "existing")
	_ = os.MkdirAll(existingDir, 0755)

	tests := []struct {
		name        string
		setup       func() string
		expectError bool
		cleanup     func(string)
	}{
		{
			name: "existing writable directory",
			setup: func() string {
				return existingDir
			},
			expectError: false,
			cleanup:     func(string) {},
		},
		{
			name: "non-existent directory that can be created",
			setup: func() string {
				return filepath.Join(tempDir, "new-dir")
			},
			expectError: false,
			cleanup:     func(path string) { _ = os.RemoveAll(path) },
		},
		{
			name: "path exists but is a file",
			setup: func() string {
				filePath := filepath.Join(tempDir, "notadir")
				_ = os.WriteFile(filePath, []byte("test"), 0644)
				return filePath
			},
			expectError: true,
			cleanup:     func(path string) { _ = os.Remove(path) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			defer tt.cleanup(path)

			err := validator.validateDirectory(path, "TEST_DIR")

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateFile(t *testing.T) {
	validator := NewConfigValidator(nil)

	tempDir := t.TempDir()
	existingFile := filepath.Join(tempDir, "test.txt")
	_ = os.WriteFile(existingFile, []byte("test content"), 0644)

	tests := []struct {
		name        string
		setup       func() string
		expectError bool
		cleanup     func(string)
	}{
		{
			name: "existing readable file",
			setup: func() string {
				return existingFile
			},
			expectError: false,
			cleanup:     func(string) {},
		},
		{
			name: "non-existent file",
			setup: func() string {
				return filepath.Join(tempDir, "nonexistent.txt")
			},
			expectError: true,
			cleanup:     func(string) {},
		},
		{
			name: "path exists but is a directory",
			setup: func() string {
				dirPath := filepath.Join(tempDir, "notafile")
				_ = os.MkdirAll(dirPath, 0755)
				return dirPath
			},
			expectError: true,
			cleanup:     func(path string) { _ = os.RemoveAll(path) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			defer tt.cleanup(path)

			err := validator.validateFile(path, "TEST_FILE")

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestIsValidHost(t *testing.T) {
	validator := NewConfigValidator(nil)

	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		{"valid IPv4", "127.0.0.1", true},
		{"valid IPv4", "192.168.1.1", true},
		{"valid IPv4", "0.0.0.0", true},
		{"valid IPv6", "::1", true},
		{"valid IPv6", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", true},
		{"valid hostname", "localhost", true},
		{"valid hostname", "example.com", true},
		{"valid hostname", "subdomain.example.com", true},
		{"valid hostname", "a", true},
		{"invalid hostname - double dot", "invalid..host", false},
		{"invalid hostname - starts with dot", ".example.com", false},
		{"invalid hostname - ends with dot", "example.com.", false},
		{"invalid hostname - starts with dash", "-example.com", false},
		{"invalid hostname - too long", string(make([]byte, 254)), false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.isValidHost(tt.host)
			if result != tt.expected {
				t.Errorf("isValidHost(%q) = %v, expected %v", tt.host, result, tt.expected)
			}
		})
	}
}

func TestIsValidHostname(t *testing.T) {
	validator := NewConfigValidator(nil)

	tests := []struct {
		name     string
		hostname string
		expected bool
	}{
		{"valid simple hostname", "localhost", true},
		{"valid domain", "example.com", true},
		{"valid subdomain", "subdomain.example.com", true},
		{"valid single char", "a", true},
		{"valid with numbers", "host123", true},
		{"valid with dashes", "my-host.example.com", true},
		{"empty string", "", false},
		{"too long", string(make([]byte, 254)), false},
		{"double dot", "invalid..host", false},
		{"starts with dot", ".example.com", false},
		{"ends with dot", "example.com.", false},
		{"starts with dash", "-example.com", false},
		{"ends with dash", "example-.com", false},
		{"invalid chars", "host_name", false},
		{"invalid chars", "host@name", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.isValidHostname(tt.hostname)
			if result != tt.expected {
				t.Errorf("isValidHostname(%q) = %v, expected %v", tt.hostname, result, tt.expected)
			}
		})
	}
}

func TestIsValidEmail(t *testing.T) {
	validator := NewConfigValidator(nil)

	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{"valid email", "test@example.com", true},
		{"valid email with subdomain", "user@mail.example.com", true},
		{"valid email with plus", "user+tag@example.com", true},
		{"valid email with dots", "first.last@example.com", true},
		{"valid email with underscore", "user_name@example.com", true},
		{"valid email with numbers", "user123@example.com", true},
		{"valid email with percent", "user%name@example.com", true},
		{"missing @", "notanemail.com", false},
		{"missing domain", "user@", false},
		{"missing local part", "@example.com", false},
		{"invalid domain", "user@invalid", false},
		{"empty string", "", false},
		{"no TLD", "user@example", false},
		{"short TLD", "user@example.c", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.isValidEmail(tt.email)
			if result != tt.expected {
				t.Errorf("isValidEmail(%q) = %v, expected %v", tt.email, result, tt.expected)
			}
		})
	}
}

func TestIsValidURL(t *testing.T) {
	validator := NewConfigValidator(nil)

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"valid http URL", "http://example.com", true},
		{"valid https URL", "https://example.com", true},
		{"valid URL with port", "http://example.com:8080", true},
		{"valid URL with path", "https://example.com/path", true},
		{"valid URL with port and path", "http://example.com:8080/api/v1", true},
		{"valid URL with subdomain", "https://api.example.com", true},
		{"valid URL with numbers", "http://127.0.0.1", true},
		{"invalid - no protocol", "example.com", false},
		{"invalid - wrong protocol", "ftp://example.com", false},
		{"invalid - no domain", "http://", false},
		{"empty string", "", false},
		{"invalid format", "not a url", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.isValidURL(tt.url)
			if result != tt.expected {
				t.Errorf("isValidURL(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestValidateEnvironmentVariables(t *testing.T) {
	validator := NewConfigValidator(nil)

	originalEnv := make(map[string]string)
	requiredVars := []string{
		"DB_HOST", "DB_PORT", "DB_NAME", "DB_USER", "DB_PASSWORD",
		"SMTP_HOST", "SMTP_PORT", "SMTP_FROM", "ADMIN_EMAIL", "JWT_SECRET",
	}

	for _, varName := range requiredVars {
		originalEnv[varName] = os.Getenv(varName)
	}

	defer func() {
		for _, varName := range requiredVars {
			if val, ok := originalEnv[varName]; ok {
				if val == "" {
					_ = os.Unsetenv(varName)
				} else {
					_ = os.Setenv(varName, val)
				}
			}
		}
	}()

	tests := []struct {
		name        string
		setup       func()
		expectError bool
	}{
		{
			name: "all variables set",
			setup: func() {
				for _, varName := range requiredVars {
					_ = os.Setenv(varName, "test-value")
				}
			},
			expectError: false,
		},
		{
			name: "missing one variable",
			setup: func() {
				for _, varName := range requiredVars {
					_ = os.Setenv(varName, "test-value")
				}
				_ = os.Unsetenv("DB_HOST")
			},
			expectError: true,
		},
		{
			name: "missing multiple variables",
			setup: func() {
				for _, varName := range requiredVars {
					_ = os.Unsetenv(varName)
				}
			},
			expectError: true,
		},
		{
			name: "empty variable value",
			setup: func() {
				for _, varName := range requiredVars {
					_ = os.Setenv(varName, "")
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, varName := range requiredVars {
				_ = os.Unsetenv(varName)
			}
			tt.setup()

			err := validator.ValidateEnvironmentVariables()

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	validator := NewConfigValidator(nil)

	tests := []struct {
		name         string
		portStr      string
		portName     string
		expectError  bool
		expectedPort int
	}{
		{"valid port", "8080", "TEST_PORT", false, 8080},
		{"valid port minimum", "1", "TEST_PORT", false, 1},
		{"valid port maximum", "65535", "TEST_PORT", false, 65535},
		{"invalid - not a number", "not-a-port", "TEST_PORT", true, 0},
		{"invalid - zero", "0", "TEST_PORT", true, 0},
		{"invalid - negative", "-1", "TEST_PORT", true, 0},
		{"invalid - too large", "65536", "TEST_PORT", true, 0},
		{"invalid - empty", "", "TEST_PORT", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, err := validator.ValidatePort(tt.portStr, tt.portName)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if port != tt.expectedPort {
					t.Errorf("expected port %d, got %d", tt.expectedPort, port)
				}
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	validator := NewConfigValidator(nil)

	tests := []struct {
		name        string
		email       string
		emailName   string
		expectError bool
	}{
		{"valid email", "test@example.com", "TEST_EMAIL", false},
		{"valid email with subdomain", "user@mail.example.com", "TEST_EMAIL", false},
		{"missing email", "", "TEST_EMAIL", true},
		{"invalid email format", "notanemail", "TEST_EMAIL", true},
		{"invalid email - no @", "notanemail.com", "TEST_EMAIL", true},
		{"invalid email - no domain", "user@", "TEST_EMAIL", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateEmail(tt.email, tt.emailName)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	validator := NewConfigValidator(nil)

	tests := []struct {
		name         string
		password     string
		passwordName string
		expectError  bool
	}{
		{"valid password - minimum length", "12345678", "TEST_PASSWORD", false},
		{"valid password - longer", "this-is-a-valid-password", "TEST_PASSWORD", false},
		{"valid password - maximum length", string(make([]byte, 128)), "TEST_PASSWORD", false},
		{"missing password", "", "TEST_PASSWORD", true},
		{"too short", "short", "TEST_PASSWORD", true},
		{"too long", string(make([]byte, 129)), "TEST_PASSWORD", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidatePassword(tt.password, tt.passwordName)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateConfiguration_AuditLogging(t *testing.T) {
	tempDir := t.TempDir()
	auditLogger, err := NewAuditLogger(tempDir)
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}
	defer func() {
		_ = auditLogger.Close()
	}()

	validator := NewConfigValidator(auditLogger)

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "postgres",
			Password: "password123",
			Name:     "testdb",
		},
		SMTP: config.SMTPConfig{
			Host: "smtp.example.com",
			Port: 587,
			From: "test@example.com",
		},
		Server: config.ServerConfig{
			Port:      "8080",
			EnableTLS: false,
		},
		JWT: config.JWTConfig{
			Secret: "this-is-a-very-secure-secret-key-that-is-long-enough",
		},
		App: config.AppConfig{
			AdminEmail: "admin@example.com",
		},
	}

	err = validator.ValidateConfiguration(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	logFile := auditLogger.GetLogFile()
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("audit log file was not created")
	}
}
