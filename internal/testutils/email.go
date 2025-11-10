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

	"github.com/joho/godotenv"
	"goyco/internal/config"
	"goyco/internal/database"
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

	addr := listener.Addr().(*net.TCPAddr)
	server.port = addr.Port

	go server.serve()

	return server, nil
}

func (s *TestEmailServer) serve() {
	for {
		if s.closed {
			return
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if !s.closed {
			}
			return
		}
		go s.handleConnection(conn)
	}
}

func (s *TestEmailServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	conn.Write([]byte("220 Test SMTP server ready\r\n"))

	buffer := make([]byte, 1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			return
		}

		command := strings.TrimSpace(string(buffer[:n]))

		if s.delay > 0 {
			time.Sleep(s.delay)
		}

		switch {
		case strings.HasPrefix(command, "EHLO"), strings.HasPrefix(command, "HELO"):
			conn.Write([]byte("250-Hello\r\n250-AUTH PLAIN LOGIN\r\n250 OK\r\n"))
		case strings.HasPrefix(command, "AUTH PLAIN"):
			conn.Write([]byte("235 Authentication successful\r\n"))
		case strings.HasPrefix(command, "AUTH LOGIN"):
			conn.Write([]byte("334 VXNlcm5hbWU6\r\n"))
			if _, err := conn.Read(buffer); err != nil {
				return
			}
			conn.Write([]byte("334 UGFzc3dvcmQ6\r\n"))
			if _, err := conn.Read(buffer); err != nil {
				return
			}
			conn.Write([]byte("235 Authentication successful\r\n"))
		case strings.HasPrefix(command, "AUTH"):
			conn.Write([]byte("504 Unrecognized authentication type\r\n"))
		case strings.HasPrefix(command, "MAIL FROM"):
			if s.shouldFail {
				conn.Write([]byte("550 Mail from failed\r\n"))
				return
			}
			conn.Write([]byte("250 OK\r\n"))
		case strings.HasPrefix(command, "RCPT TO"):
			if s.shouldFail {
				conn.Write([]byte("550 Rcpt to failed\r\n"))
				return
			}
			conn.Write([]byte("250 OK\r\n"))
		case command == "DATA":
			conn.Write([]byte("354 Start mail input; end with <CRLF>.<CRLF>\r\n"))
			s.readEmailData(conn)
		case command == "QUIT":
			conn.Write([]byte("221 Bye\r\n"))
			return
		default:
			conn.Write([]byte("500 Unknown command\r\n"))
		}
	}
}

func (s *TestEmailServer) readEmailData(conn net.Conn) {
	var emailData strings.Builder
	buffer := make([]byte, 1024)

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			return
		}

		emailData.Write(buffer[:n])

		if strings.Contains(emailData.String(), "\r\n.\r\n") {
			break
		}
	}

	email := s.parseEmail(emailData.String())
	s.mu.Lock()
	s.emails = append(s.emails, email)
	s.mu.Unlock()

	conn.Write([]byte("250 OK\r\n"))
}

func (s *TestEmailServer) parseEmail(data string) TestEmail {
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
			bodyStart := strings.Index(data, "\r\n\r\n")
			if bodyStart != -1 {
				email.Body = data[bodyStart+4:]
				email.Body = strings.TrimSuffix(email.Body, "\r\n.\r\n")
			}
			break
		}
	}

	return email
}

func (s *TestEmailServer) Close() error {
	s.closed = true
	return s.listener.Close()
}

func (s *TestEmailServer) GetPort() int {
	return s.port
}

func (s *TestEmailServer) GetEmails() []TestEmail {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.emails
}

func (s *TestEmailServer) ClearEmails() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emails = make([]TestEmail, 0)
}

func (s *TestEmailServer) SetShouldFail(shouldFail bool) {
	s.shouldFail = shouldFail
}

func (s *TestEmailServer) SetDelay(delay time.Duration) {
	s.delay = delay
}

func (s *TestEmailServer) GetEmailCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.emails)
}

func (s *TestEmailServer) GetLastEmail() *TestEmail {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.emails) == 0 {
		return nil
	}
	return &s.emails[len(s.emails)-1]
}

func (s *TestEmailServer) WaitForEmails(count int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s.mu.RLock()
		emailCount := len(s.emails)
		s.mu.RUnlock()
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

func (b *TestEmailBuilder) From(from string) *TestEmailBuilder {
	b.email.From = from
	return b
}

func (b *TestEmailBuilder) To(to string) *TestEmailBuilder {
	b.email.To = []string{to}
	return b
}

func (b *TestEmailBuilder) Subject(subject string) *TestEmailBuilder {
	b.email.Subject = subject
	return b
}

func (b *TestEmailBuilder) Body(body string) *TestEmailBuilder {
	b.email.Body = body
	return b
}

func (b *TestEmailBuilder) Header(key, value string) *TestEmailBuilder {
	b.email.Headers[key] = value
	return b
}

func (b *TestEmailBuilder) Build() *TestEmail {
	return b.email
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
	for i := range emails {
		if m.MatchEmail(&emails[i], criteria) {
			return &emails[i]
		}
	}
	return nil
}

func (m *TestEmailMatcher) CountMatchingEmails(emails []TestEmail, criteria map[string]any) int {
	count := 0
	for i := range emails {
		if m.MatchEmail(&emails[i], criteria) {
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
	connexion, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		t.Skipf("Skipping SMTP integration tests: unable to reach %s: %v", address, err)
	}
	connexion.Close()

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
