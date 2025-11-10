package commands

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"goyco/internal/config"
)

type ConfigValidator struct {
	auditLogger *AuditLogger
}

func NewConfigValidator(auditLogger *AuditLogger) *ConfigValidator {
	return &ConfigValidator{
		auditLogger: auditLogger,
	}
}

func (v *ConfigValidator) ValidateConfiguration(cfg *config.Config) error {
	var errors []string

	if err := v.validateDatabaseConfig(cfg); err != nil {
		errors = append(errors, fmt.Sprintf("Database: %v", err))
	}

	if err := v.validateSMTPConfig(cfg); err != nil {
		errors = append(errors, fmt.Sprintf("SMTP: %v", err))
	}

	if err := v.validateServerConfig(cfg); err != nil {
		errors = append(errors, fmt.Sprintf("Server: %v", err))
	}

	if err := v.validateSecurityConfig(cfg); err != nil {
		errors = append(errors, fmt.Sprintf("Security: %v", err))
	}

	if err := v.validateFilePaths(cfg); err != nil {
		errors = append(errors, fmt.Sprintf("File paths: %v", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n- %s", strings.Join(errors, "\n- "))
	}

	if v.auditLogger != nil {
		v.auditLogger.LogConfigurationChange("validation", "invalid", "valid", true, nil)
	}

	return nil
}

func (v *ConfigValidator) validateDatabaseConfig(cfg *config.Config) error {
	if cfg.Database.Host == "" {
		return fmt.Errorf("DB_HOST is required")
	}

	port, err := strconv.Atoi(cfg.Database.Port)
	if err != nil {
		return fmt.Errorf("DB_PORT must be a valid integer")
	}
	if port <= 0 || port > 65535 {
		return fmt.Errorf("DB_PORT must be between 1 and 65535")
	}

	if cfg.Database.Name == "" {
		return fmt.Errorf("DB_NAME is required")
	}

	if cfg.Database.User == "" {
		return fmt.Errorf("DB_USER is required")
	}

	if cfg.Database.Password == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}

	if !v.isValidHost(cfg.Database.Host) {
		return fmt.Errorf("DB_HOST has invalid format")
	}

	return nil
}

func (v *ConfigValidator) validateSMTPConfig(cfg *config.Config) error {
	if cfg.SMTP.Host == "" {
		return fmt.Errorf("SMTP_HOST is required")
	}

	if cfg.SMTP.Port <= 0 || cfg.SMTP.Port > 65535 {
		return fmt.Errorf("SMTP_PORT must be between 1 and 65535")
	}

	if cfg.SMTP.From == "" {
		return fmt.Errorf("SMTP_FROM is required")
	}

	if !v.isValidEmail(cfg.SMTP.From) {
		return fmt.Errorf("SMTP_FROM has invalid email format")
	}

	if cfg.App.AdminEmail == "" {
		return fmt.Errorf("ADMIN_EMAIL is required")
	}

	if !v.isValidEmail(cfg.App.AdminEmail) {
		return fmt.Errorf("ADMIN_EMAIL has invalid email format")
	}

	if !v.isValidHost(cfg.SMTP.Host) {
		return fmt.Errorf("SMTP_HOST has invalid format")
	}

	return nil
}

func (v *ConfigValidator) validateServerConfig(cfg *config.Config) error {
	serverPort, err := strconv.Atoi(cfg.Server.Port)
	if err != nil {
		return fmt.Errorf("SERVER_PORT must be a valid integer")
	}
	if serverPort <= 0 || serverPort > 65535 {
		return fmt.Errorf("SERVER_PORT must be between 1 and 65535")
	}

	if cfg.App.BaseURL != "" {
		if !v.isValidURL(cfg.App.BaseURL) {
			return fmt.Errorf("BASE_URL has invalid format")
		}
	}

	if cfg.Server.EnableTLS {
		if cfg.Server.TLSCertFile == "" {
			return fmt.Errorf("SERVER_TLS_CERT_FILE is required when TLS is enabled")
		}
		if cfg.Server.TLSKeyFile == "" {
			return fmt.Errorf("SERVER_TLS_KEY_FILE is required when TLS is enabled")
		}
	}

	return nil
}

func (v *ConfigValidator) validateSecurityConfig(cfg *config.Config) error {
	if cfg.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}

	if len(cfg.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters for security")
	}

	weakSecrets := []string{
		"your-secret-key", "secret", "jwt-secret", "my-secret",
		"change-me", "default-secret", "123456", "password",
		"admin", "test", "development", "production", "staging",
	}

	lowerSecret := strings.ToLower(cfg.JWT.Secret)
	for _, weak := range weakSecrets {
		if lowerSecret == weak {
			return fmt.Errorf("JWT_SECRET cannot be a common weak value: %s", weak)
		}
	}

	return nil
}

func (v *ConfigValidator) validateFilePaths(cfg *config.Config) error {
	if cfg.LogDir != "" {
		if err := v.validateDirectory(cfg.LogDir, "LOG_DIR"); err != nil {
			return err
		}
	}

	if cfg.PIDDir != "" {
		if err := v.validateDirectory(cfg.PIDDir, "PID_DIR"); err != nil {
			return err
		}
	}

	if cfg.Server.EnableTLS {
		if err := v.validateFile(cfg.Server.TLSCertFile, "SERVER_TLS_CERT_FILE"); err != nil {
			return err
		}
		if err := v.validateFile(cfg.Server.TLSKeyFile, "SERVER_TLS_KEY_FILE"); err != nil {
			return err
		}
	}

	return nil
}

func (v *ConfigValidator) validateDirectory(path, name string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("%s directory does not exist and cannot be created: %v", name, err)
		}
	}

	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s path exists but is not a directory", name)
		}
	}

	if err := v.checkWritePermission(path); err != nil {
		return fmt.Errorf("%s directory is not writable: %v", name, err)
	}

	return nil
}

func (v *ConfigValidator) validateFile(path, name string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("%s file does not exist: %s", name, path)
	}

	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return fmt.Errorf("%s path exists but is a directory, not a file", name)
		}
	}

	if err := v.checkReadPermission(path); err != nil {
		return fmt.Errorf("%s file is not readable: %v", name, err)
	}

	return nil
}

func (v *ConfigValidator) checkWritePermission(path string) error {
	testFile := filepath.Join(path, ".goyco_test_write")
	file, err := os.Create(testFile)
	if err != nil {
		return err
	}
	_ = file.Close()
	_ = os.Remove(testFile)
	return nil
}

func (v *ConfigValidator) checkReadPermission(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	_ = file.Close()
	return nil
}

func (v *ConfigValidator) isValidHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}

	if v.isValidHostname(host) {
		return true
	}

	return false
}

func (v *ConfigValidator) isValidHostname(hostname string) bool {
	if len(hostname) == 0 || len(hostname) > 253 {
		return false
	}

	hostnameRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	return hostnameRegex.MatchString(hostname)
}

func (v *ConfigValidator) isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func (v *ConfigValidator) isValidURL(url string) bool {
	urlRegex := regexp.MustCompile(`^https?://[a-zA-Z0-9.-]+(:[0-9]+)?(/.*)?$`)
	return urlRegex.MatchString(url)
}

func (v *ConfigValidator) ValidateEnvironmentVariables() error {
	requiredVars := []string{
		"DB_HOST", "DB_PORT", "DB_NAME", "DB_USER", "DB_PASSWORD",
		"SMTP_HOST", "SMTP_PORT", "SMTP_FROM", "ADMIN_EMAIL", "JWT_SECRET",
	}

	var missingVars []string
	for _, varName := range requiredVars {
		if os.Getenv(varName) == "" {
			missingVars = append(missingVars, varName)
		}
	}

	if len(missingVars) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missingVars, ", "))
	}

	return nil
}

func (v *ConfigValidator) ValidatePort(portStr, name string) (int, error) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer", name)
	}

	if port <= 0 || port > 65535 {
		return 0, fmt.Errorf("%s must be between 1 and 65535", name)
	}

	return port, nil
}

func (v *ConfigValidator) ValidateEmail(email, name string) error {
	if email == "" {
		return fmt.Errorf("%s is required", name)
	}

	if !v.isValidEmail(email) {
		return fmt.Errorf("%s has invalid email format", name)
	}

	return nil
}

func (v *ConfigValidator) ValidatePassword(password, name string) error {
	if password == "" {
		return fmt.Errorf("%s is required", name)
	}

	if len(password) < 8 {
		return fmt.Errorf("%s must be at least 8 characters", name)
	}

	if len(password) > 128 {
		return fmt.Errorf("%s must be 128 characters or less", name)
	}

	return nil
}
