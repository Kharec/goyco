package testutils

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"goyco/internal/config"
	"goyco/internal/database"

	"github.com/joho/godotenv"
)

type TestEmailServer struct {
	listener   net.Listener
	port       int
	emails     []TestEmail
	shouldFail bool
	delay      time.Duration
	closed     bool
	mu         sync.RWMutex
}

type TestEmail struct {
	From    string
	To      []string
	Subject string
	Body    string
	Headers map[string]string
	Raw     string
}

func NewTestEmailServer() (*TestEmailServer, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}

	server := &TestEmailServer{
		listener: listener,
		emails:   make([]TestEmail, 0),
		delay:    0,
		closed:   false,
	}

	address := listener.Addr().(*net.TCPAddr)
	server.port = address.Port

	go server.serve()

	return server, nil
}

func (server *TestEmailServer) serve() {
	for {
		if server.closed {
			return
		}

		connection, err := server.listener.Accept()
		if err != nil {
			if !server.closed {
			}
			return
		}
		go server.handleConnection(connection)
	}
}

func (server *TestEmailServer) handleConnection(connection net.Conn) {
	defer connection.Close()

	connection.Write([]byte("220 Test SMTP server ready\r\n"))

	buffer := make([]byte, 1024)
	for {
		n, err := connection.Read(buffer)
		if err != nil {
			return
		}

		command := strings.TrimSpace(string(buffer[:n]))

		if server.delay > 0 {
			time.Sleep(server.delay)
		}

		switch {
		case strings.HasPrefix(command, "EHLO"), strings.HasPrefix(command, "HELO"):
			connection.Write([]byte("250-Hello\r\n250-AUTH PLAIN LOGIN\r\n250 OK\r\n"))
		case strings.HasPrefix(command, "AUTH PLAIN"):
			connection.Write([]byte("235 Authentication successful\r\n"))
		case strings.HasPrefix(command, "AUTH LOGIN"):
			connection.Write([]byte("334 VXNlcm5hbWU6\r\n"))
			if _, err := connection.Read(buffer); err != nil {
				return
			}
			connection.Write([]byte("334 UGFzc3dvcmQ6\r\n"))
			if _, err := connection.Read(buffer); err != nil {
				return
			}
			connection.Write([]byte("235 Authentication successful\r\n"))
		case strings.HasPrefix(command, "AUTH"):
			connection.Write([]byte("504 Unrecognized authentication type\r\n"))
		case strings.HasPrefix(command, "MAIL FROM"):
			if server.shouldFail {
				connection.Write([]byte("550 Mail from failed\r\n"))
				return
			}
			connection.Write([]byte("250 OK\r\n"))
		case strings.HasPrefix(command, "RCPT TO"):
			if server.shouldFail {
				connection.Write([]byte("550 Rcpt to failed\r\n"))
				return
			}
			connection.Write([]byte("250 OK\r\n"))
		case command == "DATA":
			connection.Write([]byte("354 Start mail input; end with <CRLF>.<CRLF>\r\n"))
			server.readEmailData(connection)
		case command == "QUIT":
			connection.Write([]byte("221 Bye\r\n"))
			return
		default:
			connection.Write([]byte("500 Unknown command\r\n"))
		}
	}
}

func (server *TestEmailServer) readEmailData(connection net.Conn) {
	var emailData strings.Builder
	buffer := make([]byte, 1024)

	for {
		n, err := connection.Read(buffer)
		if err != nil {
			return
		}

		emailData.Write(buffer[:n])

		if strings.Contains(emailData.String(), "\r\n.\r\n") {
			break
		}
	}

	email := server.parseEmail(emailData.String())
	server.mu.Lock()
	server.emails = append(server.emails, email)
	server.mu.Unlock()

	connection.Write([]byte("250 OK\r\n"))
}

func (server *TestEmailServer) parseEmail(data string) TestEmail {
	lines := strings.Split(data, "\r\n")
	email := TestEmail{
		Headers: make(map[string]string),
		Raw:     data,
	}

	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				email.Headers[key] = value

				switch key {
				case "From":
					email.From = value
				case "To":
					email.To = []string{value}
				case "Subject":
					email.Subject = value
				}
			}
		} else if line == "" {
			_, after, ok := strings.Cut(data, "\r\n\r\n")
			if ok {
				email.Body = after
				email.Body = strings.TrimSuffix(email.Body, "\r\n.\r\n")
			}
			break
		}
	}

	return email
}

func (server *TestEmailServer) Close() error {
	server.closed = true
	return server.listener.Close()
}

func (server *TestEmailServer) GetPort() int {
	return server.port
}

func (server *TestEmailServer) GetEmails() []TestEmail {
	server.mu.RLock()
	defer server.mu.RUnlock()
	return server.emails
}

func (server *TestEmailServer) ClearEmails() {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.emails = make([]TestEmail, 0)
}

func (server *TestEmailServer) SetShouldFail(shouldFail bool) {
	server.shouldFail = shouldFail
}

func (server *TestEmailServer) SetDelay(delay time.Duration) {
	server.delay = delay
}

func (server *TestEmailServer) GetEmailCount() int {
	server.mu.RLock()
	defer server.mu.RUnlock()
	return len(server.emails)
}

func (server *TestEmailServer) GetLastEmail() *TestEmail {
	server.mu.RLock()
	defer server.mu.RUnlock()
	if len(server.emails) == 0 {
		return nil
	}
	return &server.emails[len(server.emails)-1]
}

func (server *TestEmailServer) WaitForEmails(count int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		server.mu.RLock()
		emailCount := len(server.emails)
		server.mu.RUnlock()
		if emailCount >= count {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

type TestEmailValidator struct{}

func NewTestEmailValidator() *TestEmailValidator {
	return &TestEmailValidator{}
}

func (v *TestEmailValidator) ValidateEmail(email *TestEmail, expectedTo, expectedSubject, expectedBody string) []string {
	var errors []string

	if email == nil {
		errors = append(errors, "email is nil")
		return errors
	}

	if len(email.To) == 0 {
		errors = append(errors, "no recipients")
	} else if email.To[0] != expectedTo {
		errors = append(errors, fmt.Sprintf("to = %v, want %v", email.To[0], expectedTo))
	}

	if email.Subject != expectedSubject {
		errors = append(errors, fmt.Sprintf("subject = %v, want %v", email.Subject, expectedSubject))
	}

	if email.Body != expectedBody {
		errors = append(errors, fmt.Sprintf("body = %v, want %v", email.Body, expectedBody))
	}

	return errors
}

func (v *TestEmailValidator) ValidateEmailContains(email *TestEmail, expectedTo, expectedSubjectContains, expectedBodyContains string) []string {
	var errors []string

	if email == nil {
		errors = append(errors, "email is nil")
		return errors
	}

	if len(email.To) == 0 {
		errors = append(errors, "no recipients")
	} else if email.To[0] != expectedTo {
		errors = append(errors, fmt.Sprintf("to = %v, want %v", email.To[0], expectedTo))
	}

	if !strings.Contains(email.Subject, expectedSubjectContains) {
		errors = append(errors, fmt.Sprintf("subject does not contain %v", expectedSubjectContains))
	}

	if !strings.Contains(email.Body, expectedBodyContains) {
		errors = append(errors, fmt.Sprintf("body does not contain %v", expectedBodyContains))
	}

	return errors
}

func (v *TestEmailValidator) ValidateEmailHeaders(email *TestEmail, expectedHeaders map[string]string) []string {
	var errors []string

	if email == nil {
		errors = append(errors, "email is nil")
		return errors
	}

	for key, expectedValue := range expectedHeaders {
		actualValue, exists := email.Headers[key]
		if !exists {
			errors = append(errors, fmt.Sprintf("header %v not found", key))
		} else if actualValue != expectedValue {
			errors = append(errors, fmt.Sprintf("header %v = %v, want %v", key, actualValue, expectedValue))
		}
	}

	return errors
}

type TestEmailBuilder struct {
	email *TestEmail
}

func NewTestEmailBuilder() *TestEmailBuilder {
	return &TestEmailBuilder{
		email: &TestEmail{
			Headers: make(map[string]string),
		},
	}
}

func (builder *TestEmailBuilder) From(from string) *TestEmailBuilder {
	builder.email.From = from
	return builder
}

func (builder *TestEmailBuilder) To(to string) *TestEmailBuilder {
	builder.email.To = []string{to}
	return builder
}

func (builder *TestEmailBuilder) Subject(subject string) *TestEmailBuilder {
	builder.email.Subject = subject
	return builder
}

func (builder *TestEmailBuilder) Body(body string) *TestEmailBuilder {
	builder.email.Body = body
	return builder
}

func (builder *TestEmailBuilder) Header(key, value string) *TestEmailBuilder {
	builder.email.Headers[key] = value
	return builder
}

func (builder *TestEmailBuilder) Build() *TestEmail {
	return builder.email
}

type TestEmailMatcher struct{}

func NewTestEmailMatcher() *TestEmailMatcher {
	return &TestEmailMatcher{}
}

func (m *TestEmailMatcher) MatchEmail(email *TestEmail, criteria map[string]any) bool {
	if email == nil {
		return false
	}

	for key, expectedValue := range criteria {
		switch key {
		case "from":
			if email.From != expectedValue {
				return false
			}
		case "to":
			if len(email.To) == 0 || email.To[0] != expectedValue {
				return false
			}
		case "subject":
			if email.Subject != expectedValue {
				return false
			}
		case "body":
			if email.Body != expectedValue {
				return false
			}
		case "subject_contains":
			if !strings.Contains(email.Subject, expectedValue.(string)) {
				return false
			}
		case "body_contains":
			if !strings.Contains(email.Body, expectedValue.(string)) {
				return false
			}
		case "header":
			headerMap := expectedValue.(map[string]string)
			for headerKey, headerValue := range headerMap {
				if email.Headers[headerKey] != headerValue {
					return false
				}
			}
		}
	}

	return true
}

func (m *TestEmailMatcher) FindEmail(emails []TestEmail, criteria map[string]any) *TestEmail {
	for idx := range emails {
		if m.MatchEmail(&emails[idx], criteria) {
			return &emails[idx]
		}
	}
	return nil
}

func (m *TestEmailMatcher) CountMatchingEmails(emails []TestEmail, criteria map[string]any) int {
	count := 0
	for idx := range emails {
		if m.MatchEmail(&emails[idx], criteria) {
			count++
		}
	}
	return count
}

type MockEmailSenderWithError struct {
	Err error
}

func NewMockEmailSenderWithError(err error) *MockEmailSenderWithError {
	return &MockEmailSenderWithError{Err: err}
}

func (m *MockEmailSenderWithError) Send(to, subject, body string) error {
	return m.Err
}

func NewEmailTestUser(username, email string) *database.User {
	return &database.User{
		ID:       1,
		Username: username,
		Email:    email,
	}
}

func NewEmailTestConfig(baseURL string) *config.Config {
	return &config.Config{
		App: config.AppConfig{
			BaseURL:    baseURL,
			AdminEmail: "admin@example.com",
		},
	}
}

type SMTPSender struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	timeout  time.Duration
}

func GetSMTPSenderFromEnv(t *testing.T) *SMTPSender {
	t.Helper()

	envPaths := []string{".env", "../.env", "../../.env", "../../../.env"}
	for _, envPath := range envPaths {
		if _, err := os.Stat(envPath); err == nil {
			_ = godotenv.Load(envPath)
			break
		}
	}

	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	if host == "" {
		t.Skip("Skipping SMTP integration tests: SMTP_HOST is not configured")
	}

	portStr := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if portStr == "" {
		t.Skip("Skipping SMTP integration tests: SMTP_PORT is not configured")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Skipf("Skipping SMTP integration tests: invalid SMTP_PORT '%s': %v", portStr, err)
	}

	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))
	if from == "" {
		t.Skip("Skipping SMTP integration tests: SMTP_FROM is not configured")
	}

	sender := &SMTPSender{
		Host:     host,
		Port:     port,
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     from,
		timeout:  5 * time.Second,
	}

	address := net.JoinHostPort(sender.Host, strconv.Itoa(sender.Port))
	connection, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		t.Skipf("Skipping SMTP integration tests: unable to reach %s: %v", address, err)
	}
	connection.Close()

	return sender
}

func (s *SMTPSender) Send(to, subject, body string) error {
	if to == "" {
		return fmt.Errorf("recipient email is required")
	}
	if subject == "" {
		return fmt.Errorf("subject is required")
	}
	if body == "" {
		return fmt.Errorf("body is required")
	}
	return nil
}
