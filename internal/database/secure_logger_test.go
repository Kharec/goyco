package database

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm/logger"
)

func TestSecureLogger_MaskSensitiveData(t *testing.T) {
	writer := log.New(os.Stdout, "\r\n", log.LstdFlags)
	config := logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	}

	tests := []struct {
		name       string
		production bool
		input      string
		expected   string
	}{
		{
			name:       "development_mode_no_masking",
			production: false,
			input:      "SELECT * FROM users WHERE email = 'user@example.com'",
			expected:   "SELECT * FROM users WHERE email = 'user@example.com'",
		},
		{
			name:       "production_mode_mask_email",
			production: true,
			input:      "SELECT * FROM users WHERE email = 'user@example.com'",
			expected:   "SELECT * FROM users WHERE email = '[EMAIL_MASKED]'",
		},
		{
			name:       "production_mode_mask_token",
			production: true,
			input:      "SELECT * FROM users WHERE password_reset_token = 'abc123def456ghi789'",
			expected:   "SELECT * FROM users WHERE password_reset_token = '[TOKEN_MASKED]'",
		},
		{
			name:       "production_mode_mask_uuid",
			production: true,
			input:      "SELECT * FROM users WHERE id = '550e8400-e29b-41d4-a716-446655440000'",
			expected:   "SELECT * FROM users WHERE id = '[TOKEN_MASKED]'",
		},
		{
			name:       "production_mode_no_masking_short_values",
			production: true,
			input:      "SELECT * FROM users WHERE id = 123",
			expected:   "SELECT * FROM users WHERE id = 123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secureLogger := NewSecureLogger(writer, config, tt.production)
			result := secureLogger.maskSensitiveData(tt.input)

			if tt.production {
				if strings.Contains(result, "user@example.com") {
					t.Errorf("Email should be masked in production mode")
				}
				if strings.Contains(result, "abc123def456ghi789") {
					t.Errorf("Token should be masked in production mode")
				}
			} else {
				if result != tt.input {
					t.Errorf("Expected %q, got %q", tt.input, result)
				}
			}
		})
	}
}

func TestSecureLogger_IsSensitiveValue(t *testing.T) {
	writer := log.New(os.Stdout, "\r\n", log.LstdFlags)
	config := logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	}

	secureLogger := NewSecureLogger(writer, config, true)

	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{
			name:     "email_address",
			value:    "user@example.com",
			expected: true,
		},
		{
			name:     "long_token",
			value:    "abc123def456ghi789jkl012mno345pqr678stu901vwx234yz",
			expected: true,
		},
		{
			name:     "uuid",
			value:    "550e8400-e29b-41d4-a716-446655440000",
			expected: true,
		},
		{
			name:     "short_value",
			value:    "123",
			expected: false,
		},
		{
			name:     "normal_text",
			value:    "golang programming",
			expected: false,
		},
		{
			name:     "base64_like",
			value:    "SGVsbG8gV29ybGQ=",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := secureLogger.isSensitiveValue(tt.value)
			if result != tt.expected {
				t.Errorf("Expected %v for value %q, got %v", tt.expected, tt.value, result)
			}
		})
	}
}

func TestSecureLogger_LogLevels(t *testing.T) {
	writer := log.New(os.Stdout, "\r\n", log.LstdFlags)
	config := logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	}

	secureLogger := NewSecureLogger(writer, config, false)

	ctx := context.Background()

	secureLogger.Info(ctx, "Test info message")
	secureLogger.Warn(ctx, "Test warn message")
	secureLogger.Error(ctx, "Test error message")
}

func TestCreateSecureLogger(t *testing.T) {
	prodLogger := CreateSecureLogger(true)
	if prodLogger == nil {
		t.Error("Expected non-nil logger for production mode")
	}

	devLogger := CreateSecureLogger(false)
	if devLogger == nil {
		t.Error("Expected non-nil logger for development mode")
	}
}

func TestSecureLogger_LogMode(t *testing.T) {
	writer := log.New(os.Stdout, "\r\n", log.LstdFlags)
	config := logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	}

	secureLogger := NewSecureLogger(writer, config, false)

	newLogger := secureLogger.LogMode(logger.Error)
	if newLogger == nil {
		t.Error("Expected non-nil logger from LogMode")
	}

	if secureLogger.config.LogLevel != logger.Info {
		t.Error("Original logger should be unchanged")
	}
}

func TestSecureLogger_Trace(t *testing.T) {
	writer := log.New(os.Stdout, "\r\n", log.LstdFlags)
	config := logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	}

	secureLogger := NewSecureLogger(writer, config, false)
	ctx := context.Background()

	t.Run("silent_level", func(t *testing.T) {
		silentLogger := secureLogger.LogMode(logger.Silent)
		silentLogger.Trace(ctx, time.Now(), func() (string, int64) {
			return "SELECT * FROM users", 1
		}, nil)
	})

	t.Run("error_level_with_error", func(t *testing.T) {
		errorLogger := secureLogger.LogMode(logger.Error)
		errorLogger.Trace(ctx, time.Now(), func() (string, int64) {
			return "SELECT * FROM users", 1
		}, errors.New("test error"))
	})

	t.Run("warn_level_slow_query", func(t *testing.T) {
		warnLogger := secureLogger.LogMode(logger.Warn)

		startTime := time.Now().Add(-2 * time.Second)
		warnLogger.Trace(ctx, startTime, func() (string, int64) {
			return "SELECT * FROM users", 1
		}, nil)
	})

	t.Run("info_level", func(t *testing.T) {
		infoLogger := secureLogger.LogMode(logger.Info)
		infoLogger.Trace(ctx, time.Now(), func() (string, int64) {
			return "SELECT * FROM users", 1
		}, nil)
	})
}

func TestSecureLogger_MaskSQLValues(t *testing.T) {
	writer := log.New(os.Stdout, "\r\n", log.LstdFlags)
	config := logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	}

	secureLogger := NewSecureLogger(writer, config, true)

	tests := []struct {
		name     string
		sql      string
		expected string
	}{
		{
			name:     "email_in_sql",
			sql:      "SELECT * FROM users WHERE email = 'user@example.com'",
			expected: "SELECT * FROM users WHERE email = '[MASKED]'",
		},
		{
			name:     "token_in_sql",
			sql:      "SELECT * FROM users WHERE token = 'abc123def456ghi789'",
			expected: "SELECT * FROM users WHERE token = '[MASKED]'",
		},
		{
			name:     "uuid_in_sql",
			sql:      "SELECT * FROM users WHERE id = '550e8400-e29b-41d4-a716-446655440000'",
			expected: "SELECT * FROM users WHERE id = '[MASKED]'",
		},
		{
			name:     "normal_value",
			sql:      "SELECT * FROM users WHERE id = 123",
			expected: "SELECT * FROM users WHERE id = 123",
		},
		{
			name:     "multiple_values",
			sql:      "SELECT * FROM users WHERE email = 'user@example.com' AND id = 123",
			expected: "SELECT * FROM users WHERE email = '[MASKED]' AND id = 123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := secureLogger.maskSQLValues(tt.sql)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestSecureLogger_IsRecordNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "record_not_found",
			err:      errors.New("record not found"),
			expected: true,
		},
		{
			name:     "not_found",
			err:      errors.New("not found"),
			expected: true,
		},
		{
			name:     "RECORD NOT FOUND",
			err:      errors.New("RECORD NOT FOUND"),
			expected: true,
		},
		{
			name:     "NOT FOUND",
			err:      errors.New("NOT FOUND"),
			expected: true,
		},
		{
			name:     "other_error",
			err:      errors.New("connection failed"),
			expected: false,
		},
		{
			name:     "nil_error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRecordNotFoundError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v for error '%v', got %v", tt.expected, tt.err, result)
			}
		})
	}
}

func TestSecureLogger_ProductionMode(t *testing.T) {
	writer := log.New(os.Stdout, "\r\n", log.LstdFlags)
	config := logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Error,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	}

	secureLogger := NewSecureLogger(writer, config, true)
	ctx := context.Background()

	secureLogger.Info(ctx, "User login: %s", "user@example.com")
	secureLogger.Warn(ctx, "Token validation: %s", "abc123def456ghi789")
	secureLogger.Error(ctx, "Database error: %s", "connection failed")
}

func TestSecureLogger_DevelopmentMode(t *testing.T) {
	writer := log.New(os.Stdout, "\r\n", log.LstdFlags)
	config := logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	}

	secureLogger := NewSecureLogger(writer, config, false)
	ctx := context.Background()

	secureLogger.Info(ctx, "User login: %s", "user@example.com")
	secureLogger.Warn(ctx, "Token validation: %s", "abc123def456ghi789")
	secureLogger.Error(ctx, "Database error: %s", "connection failed")
}
