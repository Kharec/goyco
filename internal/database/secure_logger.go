package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm/logger"
)

type SecureLogger struct {
	writer           logger.Writer
	config           logger.Config
	sensitiveFields  []string
	sensitivePattern *regexp.Regexp
	productionMode   bool
}

func NewSecureLogger(writer logger.Writer, config logger.Config, productionMode bool) *SecureLogger {
	sensitiveFields := []string{
		"password", "token", "secret", "key", "hash", "salt",
		"email_verification_token", "password_reset_token",
		"token_hash", "jwt_secret", "api_key", "access_token",
		"refresh_token", "session_id", "cookie", "auth",
	}

	sensitivePattern := regexp.MustCompile(`(?i)(password|token|secret|key|hash|salt|email_verification_token|password_reset_token|token_hash|jwt_secret|api_key|access_token|refresh_token|session_id|cookie|auth)`)

	return &SecureLogger{
		writer:           writer,
		config:           config,
		sensitiveFields:  sensitiveFields,
		sensitivePattern: sensitivePattern,
		productionMode:   productionMode,
	}
}

func (l *SecureLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.config.LogLevel = level
	return &newLogger
}

func (l *SecureLogger) Info(ctx context.Context, msg string, data ...any) {
	if l.config.LogLevel >= logger.Info {
		l.log(ctx, "info", msg, data...)
	}
}

func (l *SecureLogger) Warn(ctx context.Context, msg string, data ...any) {
	if l.config.LogLevel >= logger.Warn {
		l.log(ctx, "warn", msg, data...)
	}
}

func (l *SecureLogger) Error(ctx context.Context, msg string, data ...any) {
	if l.config.LogLevel >= logger.Error {
		l.log(ctx, "error", msg, data...)
	}
}

func (l *SecureLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.config.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && l.config.LogLevel >= logger.Error && (!l.config.IgnoreRecordNotFoundError || !IsRecordNotFoundError(err)):
		sql, rows := fc()
		l.log(ctx, "error", fmt.Sprintf("[%.3fms] [rows:%v] %s", float64(elapsed.Nanoseconds())/1e6, rows, sql))
	case elapsed > l.config.SlowThreshold && l.config.SlowThreshold != 0 && l.config.LogLevel >= logger.Warn:
		sql, rows := fc()
		l.log(ctx, "warn", fmt.Sprintf("[%.3fms] [rows:%v] %s", float64(elapsed.Nanoseconds())/1e6, rows, sql))
	case l.config.LogLevel == logger.Info:
		sql, rows := fc()
		l.log(ctx, "info", fmt.Sprintf("[%.3fms] [rows:%v] %s", float64(elapsed.Nanoseconds())/1e6, rows, sql))
	}
}

func (l *SecureLogger) log(_ context.Context, level, msg string, data ...any) {
	if l.productionMode {
		msg = l.maskSensitiveData(msg)

		maskedData := make([]any, len(data))
		for i, d := range data {
			maskedData[i] = l.maskSensitiveData(fmt.Sprintf("%v", d))
		}
		data = maskedData
	}

	formattedMsg := fmt.Sprintf(msg, data...)

	l.writer.Printf("[%s] %s", strings.ToUpper(level), formattedMsg)
}

func (l *SecureLogger) maskSensitiveData(data string) string {
	if l.productionMode {
		data = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`).ReplaceAllString(data, "[EMAIL_MASKED]")

		data = regexp.MustCompile(`\b[A-Za-z0-9]{20,}\b`).ReplaceAllStringFunc(data, func(match string) string {
			if l.sensitivePattern.MatchString(match) {
				return "[TOKEN_MASKED]"
			}
			return match
		})

		data = l.maskSQLValues(data)
	}

	return data
}

func (l *SecureLogger) maskSQLValues(sql string) string {
	paramPattern := regexp.MustCompile(`'([^']*)'`)

	return paramPattern.ReplaceAllStringFunc(sql, func(match string) string {
		value := strings.Trim(match, "'")

		if l.isSensitiveValue(value) {
			return "'[MASKED]'"
		}

		return match
	})
}

func (l *SecureLogger) isSensitiveValue(value string) bool {
	if regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`).MatchString(value) {
		return true
	}

	if len(value) > 20 && regexp.MustCompile(`^[A-Za-z0-9+/]{20,}={0,2}$`).MatchString(value) {
		return true
	}

	if regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(value) {
		return true
	}

	if regexp.MustCompile(`^[A-Za-z0-9+/]+={0,2}$`).MatchString(value) && len(value) > 10 {
		return true
	}

	return false
}

func IsRecordNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "record not found") ||
		strings.Contains(strings.ToLower(err.Error()), "not found")
}

func CreateSecureLogger(productionMode bool) logger.Interface {
	config := logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	}

	if productionMode {
		config.LogLevel = logger.Error
		config.SlowThreshold = 2 * time.Second
	}

	writer := log.New(os.Stdout, "\r\n", log.LstdFlags)
	return NewSecureLogger(writer, config, productionMode)
}
