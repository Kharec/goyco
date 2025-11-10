package commands

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

type AuditLogger struct {
	logFile string
	logger  *log.Logger
}

type AuditEvent struct {
	Timestamp  time.Time      `json:"timestamp"`
	Action     string         `json:"action"`
	Resource   string         `json:"resource"`
	ResourceID string         `json:"resource_id,omitempty"`
	Details    string         `json:"details,omitempty"`
	User       string         `json:"user,omitempty"`
	IPAddress  string         `json:"ip_address,omitempty"`
	UserAgent  string         `json:"user_agent,omitempty"`
	Success    bool           `json:"success"`
	Error      string         `json:"error,omitempty"`
	Changes    map[string]any `json:"changes,omitempty"`
}

func NewAuditLogger(logDir string) (*AuditLogger, error) {
	if logDir == "" {
		logDir = "/var/log"
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create audit log directory: %w", err)
	}

	logFile := filepath.Join(logDir, "goyco-audit.log")

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open audit log file: %w", err)
	}

	logger := log.New(file, "", 0)

	return &AuditLogger{
		logFile: logFile,
		logger:  logger,
	}, nil
}

func (a *AuditLogger) LogEvent(event AuditEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		a.logger.Printf("AUDIT: %s %s %s %s",
			event.Timestamp.Format(time.RFC3339),
			event.Action,
			event.Resource,
			event.Details)
		return
	}

	a.logger.Printf("%s", string(jsonData))
}

func (a *AuditLogger) LogUserCreation(userID uint, username, email string, success bool, err error) {
	event := AuditEvent{
		Action:     "user_create",
		Resource:   "user",
		ResourceID: fmt.Sprintf("%d", userID),
		Details:    fmt.Sprintf("Created user: %s (%s)", username, email),
		Success:    success,
	}

	if err != nil {
		event.Error = err.Error()
	}

	a.LogEvent(event)
}

func (a *AuditLogger) LogUserUpdate(userID uint, username string, changes map[string]any, success bool, err error) {
	event := AuditEvent{
		Action:     "user_update",
		Resource:   "user",
		ResourceID: fmt.Sprintf("%d", userID),
		Details:    fmt.Sprintf("Updated user: %s", username),
		Changes:    changes,
		Success:    success,
	}

	if err != nil {
		event.Error = err.Error()
	}

	a.LogEvent(event)
}

func (a *AuditLogger) LogUserDeletion(userID uint, username string, deletePosts bool, success bool, err error) {
	event := AuditEvent{
		Action:     "user_delete",
		Resource:   "user",
		ResourceID: fmt.Sprintf("%d", userID),
		Details:    fmt.Sprintf("Deleted user: %s (delete_posts: %t)", username, deletePosts),
		Success:    success,
	}

	if err != nil {
		event.Error = err.Error()
	}

	a.LogEvent(event)
}

func (a *AuditLogger) LogUserLock(userID uint, username string, locked bool, success bool, err error) {
	action := "user_lock"
	if !locked {
		action = "user_unlock"
	}

	event := AuditEvent{
		Action:     action,
		Resource:   "user",
		ResourceID: fmt.Sprintf("%d", userID),
		Details:    fmt.Sprintf("User %s: %s", username, action),
		Success:    success,
	}

	if err != nil {
		event.Error = err.Error()
	}

	a.LogEvent(event)
}

func (a *AuditLogger) LogPostDeletion(postID uint, title string, success bool, err error) {
	event := AuditEvent{
		Action:     "post_delete",
		Resource:   "post",
		ResourceID: fmt.Sprintf("%d", postID),
		Details:    fmt.Sprintf("Deleted post: %s", title),
		Success:    success,
	}

	if err != nil {
		event.Error = err.Error()
	}

	a.LogEvent(event)
}

func (a *AuditLogger) LogDataPruning(operation string, count int, success bool, err error) {
	event := AuditEvent{
		Action:   "data_prune",
		Resource: "data",
		Details:  fmt.Sprintf("Pruned %d records via %s", count, operation),
		Success:  success,
	}

	if err != nil {
		event.Error = err.Error()
	}

	a.LogEvent(event)
}

func (a *AuditLogger) LogDatabaseMigration(operation string, success bool, err error) {
	event := AuditEvent{
		Action:   "database_migrate",
		Resource: "database",
		Details:  fmt.Sprintf("Database migration: %s", operation),
		Success:  success,
	}

	if err != nil {
		event.Error = err.Error()
	}

	a.LogEvent(event)
}

func (a *AuditLogger) LogDatabaseSeeding(users, posts, votes int, success bool, err error) {
	event := AuditEvent{
		Action:   "database_seed",
		Resource: "database",
		Details:  fmt.Sprintf("Seeded database: %d users, %d posts, %d votes", users, posts, votes),
		Success:  success,
	}

	if err != nil {
		event.Error = err.Error()
	}

	a.LogEvent(event)
}

func (a *AuditLogger) LogDaemonOperation(operation string, pid int, success bool, err error) {
	event := AuditEvent{
		Action:   "daemon_" + operation,
		Resource: "daemon",
		Details:  fmt.Sprintf("Daemon %s (PID: %d)", operation, pid),
		Success:  success,
	}

	if err != nil {
		event.Error = err.Error()
	}

	a.LogEvent(event)
}

func (a *AuditLogger) LogSecurityEvent(eventType, details string, severity string) {
	event := AuditEvent{
		Action:   "security_event",
		Resource: "security",
		Details:  fmt.Sprintf("[%s] %s: %s", severity, eventType, details),
		Success:  true,
	}

	a.LogEvent(event)
}

func (a *AuditLogger) LogConfigurationChange(setting, oldValue, newValue string, success bool, err error) {
	event := AuditEvent{
		Action:   "config_change",
		Resource: "configuration",
		Details:  fmt.Sprintf("Changed %s from '%s' to '%s'", setting, oldValue, newValue),
		Success:  success,
	}

	if err != nil {
		event.Error = err.Error()
	}

	a.LogEvent(event)
}

func (a *AuditLogger) GetLogFile() string {
	return a.logFile
}

func (a *AuditLogger) Close() error {
	a.LogEvent(AuditEvent{
		Action:   "audit_logger_close",
		Resource: "audit",
		Details:  "Audit logger closed",
		Success:  true,
	})
	return nil
}
