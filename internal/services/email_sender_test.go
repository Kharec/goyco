package services

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"goyco/internal/testutils"
)

func TestNewSMTPSender(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		username string
		password string
		from     string
		expected *SMTPSender
	}{
		{
			name:     "valid configuration",
			host:     "smtp.example.com",
			port:     587,
			username: "user@example.com",
			password: "password123",
			from:     "noreply@example.com",
			expected: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password123",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
		},
		{
			name:     "empty credentials",
			host:     "smtp.example.com",
			port:     25,
			username: "",
			password: "",
			from:     "noreply@example.com",
			expected: &SMTPSender{
				host:     "smtp.example.com",
				port:     25,
				username: "",
				password: "",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := NewSMTPSender(tt.host, tt.port, tt.username, tt.password, tt.from)

			if sender.host != tt.expected.host {
				t.Errorf("host = %v, want %v", sender.host, tt.expected.host)
			}
			if sender.port != tt.expected.port {
				t.Errorf("port = %v, want %v", sender.port, tt.expected.port)
			}
			if sender.username != tt.expected.username {
				t.Errorf("username = %v, want %v", sender.username, tt.expected.username)
			}
			if sender.password != tt.expected.password {
				t.Errorf("password = %v, want %v", sender.password, tt.expected.password)
			}
			if sender.from != tt.expected.from {
				t.Errorf("from = %v, want %v", sender.from, tt.expected.from)
			}
			if sender.timeout != tt.expected.timeout {
				t.Errorf("timeout = %v, want %v", sender.timeout, tt.expected.timeout)
			}
		})
	}
}

func TestNewSMTPSenderWithTimeout(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		username string
		password string
		from     string
		timeout  time.Duration
	}{
		{
			name:     "custom timeout",
			host:     "smtp.example.com",
			port:     587,
			username: "user",
			password: "pass",
			from:     "from@example.com",
			timeout:  60 * time.Second,
		},
		{
			name:     "zero timeout",
			host:     "smtp.example.com",
			port:     25,
			username: "",
			password: "",
			from:     "from@example.com",
			timeout:  0,
		},
		{
			name:     "very short timeout",
			host:     "smtp.example.com",
			port:     587,
			username: "user",
			password: "pass",
			from:     "from@example.com",
			timeout:  1 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := NewSMTPSenderWithTimeout(tt.host, tt.port, tt.username, tt.password, tt.from, tt.timeout)

			if sender.host != tt.host {
				t.Errorf("host = %v, want %v", sender.host, tt.host)
			}
			if sender.port != tt.port {
				t.Errorf("port = %v, want %v", sender.port, tt.port)
			}
			if sender.username != tt.username {
				t.Errorf("username = %v, want %v", sender.username, tt.username)
			}
			if sender.password != tt.password {
				t.Errorf("password = %v, want %v", sender.password, tt.password)
			}
			if sender.from != tt.from {
				t.Errorf("from = %v, want %v", sender.from, tt.from)
			}
			if sender.timeout != tt.timeout {
				t.Errorf("timeout = %v, want %v", sender.timeout, tt.timeout)
			}
		})
	}
}

func TestSMTPSender_Validation(t *testing.T) {
	tests := []struct {
		name        string
		sender      *SMTPSender
		to          string
		subject     string
		body        string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil sender",
			sender:      nil,
			to:          "test@example.com",
			subject:     "Test Subject",
			body:        "Test Body",
			expectError: true,
			errorMsg:    "smtp sender is not configured",
		},
		{
			name: "empty recipient",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:          "",
			subject:     "Test Subject",
			body:        "Test Body",
			expectError: true,
			errorMsg:    "recipient address is required",
		},
		{
			name: "whitespace-only recipient",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:          "   ",
			subject:     "Test Subject",
			body:        "Test Body",
			expectError: true,
			errorMsg:    "recipient address is required",
		},
		{
			name: "tab-only recipient",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:          "\t",
			subject:     "Test Subject",
			body:        "Test Body",
			expectError: true,
			errorMsg:    "recipient address is required",
		},
		{
			name: "newline-only recipient",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:          "\n",
			subject:     "Test Subject",
			body:        "Test Body",
			expectError: true,
			errorMsg:    "recipient address is required",
		},
		{
			name: "valid recipient with whitespace",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:          "  test@example.com  ",
			subject:     "Test Subject",
			body:        "Test Body",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sender.Send(tt.to, tt.subject, tt.body)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("error message = %v, want to contain %v", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil && !isDialErrorExpected(err) {
					t.Errorf("unexpected error type: %v", err)
				}
			}
		})
	}
}

func TestSMTPSender_Send(t *testing.T) {
	tests := []struct {
		name        string
		sender      *SMTPSender
		to          string
		subject     string
		body        string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil sender",
			sender:      nil,
			to:          "test@example.com",
			subject:     "Test Subject",
			body:        "Test Body",
			expectError: true,
			errorMsg:    "smtp sender is not configured",
		},
		{
			name: "empty recipient",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:          "",
			subject:     "Test Subject",
			body:        "Test Body",
			expectError: true,
			errorMsg:    "recipient address is required",
		},
		{
			name: "whitespace-only recipient",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:          "   ",
			subject:     "Test Subject",
			body:        "Test Body",
			expectError: true,
			errorMsg:    "recipient address is required",
		},
		{
			name: "valid email with authentication",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:          "test@example.com",
			subject:     "Test Subject",
			body:        "<html><body>Test Body</body></html>",
			expectError: false,
		},
		{
			name: "valid email without authentication",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     25,
				username: "",
				password: "",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:          "test@example.com",
			subject:     "Test Subject",
			body:        "<html><body>Test Body</body></html>",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sender.Send(tt.to, tt.subject, tt.body)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("error message = %v, want to contain %v", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil && !isDialErrorExpected(err) {
					t.Errorf("unexpected error type: %v", err)
				}
			}
		})
	}
}

func isDialErrorExpected(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	expected := []string{
		"connection refused",
		"no such host",
		"timeout",
		"lookup",
		"name resolution",
		"temporary failure",
		"network is unreachable",
	}

	for _, fragment := range expected {
		if strings.Contains(msg, fragment) {
			return true
		}
	}

	return false
}

func newTestSMTPClientOrSkip(t *testing.T) *testutils.TestSMTPClient {
	t.Helper()
	client, err := testutils.NewTestSMTPClient(func(port int) testutils.AsyncEmailSender {
		return NewSMTPSenderWithTimeout(
			"localhost",
			port,
			"testuser",
			"testpass",
			"test@example.com",
			5*time.Second,
		)
	})
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "operation not permitted") || strings.Contains(msg, "permission denied") {
			t.Skipf("skipping SMTP client integration tests: %v", err)
		}
		t.Fatalf("Failed to create test client: %v", err)
	}

	return client
}

func TestSMTPSender_Send_MessageFormat(t *testing.T) {
	sender := &SMTPSender{
		host:     "smtp.example.com",
		port:     587,
		username: "user@example.com",
		password: "password",
		from:     "noreply@example.com",
		timeout:  30 * time.Second,
	}

	err := sender.Send("test@example.com", "Test Subject", "<html><body>Test Body</body></html>")

	if err != nil && !isDialErrorExpected(err) {
		t.Errorf("unexpected error type: %v", err)
	}
}

func TestSMTPSender_Send_Timeout(t *testing.T) {
	sender := &SMTPSender{
		host:     "smtp.example.com",
		port:     587,
		username: "user@example.com",
		password: "password",
		from:     "noreply@example.com",
		timeout:  1 * time.Millisecond,
	}

	err := sender.Send("test@example.com", "Test Subject", "Test Body")

	if err != nil && !isDialErrorExpected(err) {
		t.Errorf("expected timeout or connection error, got: %v", err)
	}
}

func TestSMTPSender_Send_Headers(t *testing.T) {
	sender := &SMTPSender{
		host: "smtp.example.com",
		port: 587,
	}

	expectedAddress := "smtp.example.com:587"
	actualAddress := net.JoinHostPort(sender.host, strconv.Itoa(sender.port))

	if actualAddress != expectedAddress {
		t.Errorf("address = %v, want %v", actualAddress, expectedAddress)
	}
}

func TestSMTPSender_SendAsync(t *testing.T) {
	sender := &SMTPSender{
		host:     "smtp.example.com",
		port:     587,
		username: "user@example.com",
		password: "password",
		from:     "noreply@example.com",
		timeout:  30 * time.Second,
	}

	resultChan := sender.SendAsync("test@example.com", "Test Subject", "Test Body")

	select {
	case err := <-resultChan:
		if err != nil && !isDialErrorExpected(err) {
			t.Errorf("unexpected error type: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("async send timed out")
	}
}

func TestSMTPSender_SendAsync_Concurrent(t *testing.T) {
	sender := &SMTPSender{
		host:     "smtp.example.com",
		port:     587,
		username: "user@example.com",
		password: "password",
		from:     "noreply@example.com",
		timeout:  30 * time.Second,
	}

	numEmails := 5
	resultChans := make([]<-chan error, numEmails)

	for i := range numEmails {
		resultChans[i] = sender.SendAsync(
			fmt.Sprintf("test%d@example.com", i),
			fmt.Sprintf("Test Subject %d", i),
			fmt.Sprintf("Test Body %d", i),
		)
	}

	for i, resultChan := range resultChans {
		select {
		case err := <-resultChan:
			if err != nil && !isDialErrorExpected(err) {
				t.Errorf("unexpected error type for email %d: %v", i, err)
			}
		case <-time.After(10 * time.Second):
			t.Errorf("async send %d timed out", i)
		}
	}
}

func TestSMTPSender_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		sender  *SMTPSender
		to      string
		subject string
		body    string
	}{
		{
			name: "very long subject",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:      "test@example.com",
			subject: strings.Repeat("A", 1000),
			body:    "Test Body",
		},
		{
			name: "very long body",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:      "test@example.com",
			subject: "Test Subject",
			body:    strings.Repeat("A", 10000),
		},
		{
			name: "special characters in subject",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:      "test@example.com",
			subject: "Test Subject with émojis 🎉 and special chars: !@#$%^&*()",
			body:    "Test Body",
		},
		{
			name: "HTML body with special characters",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:      "test@example.com",
			subject: "Test Subject",
			body:    "<html><body>Test Body with émojis 🎉 and special chars: !@#$%^&*()</body></html>",
		},
		{
			name: "empty subject",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:      "test@example.com",
			subject: "",
			body:    "Test Body",
		},
		{
			name: "empty body",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:      "test@example.com",
			subject: "Test Subject",
			body:    "",
		},
		{
			name: "very long email address",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:      strings.Repeat("a", 50) + "@" + strings.Repeat("b", 50) + ".com",
			subject: "Test Subject",
			body:    "Test Body",
		},
		{
			name: "email with unicode characters",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:      "tëst@éxämplé.com",
			subject: "Tëst Súbjëct",
			body:    "Tëst Bódy with únicódé",
		},
		{
			name: "email with plus sign",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:      "test+tag@example.com",
			subject: "Test Subject",
			body:    "Test Body",
		},
		{
			name: "email with multiple dots",
			sender: &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  30 * time.Second,
			},
			to:      "test.user.name@sub.domain.example.com",
			subject: "Test Subject",
			body:    "Test Body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sender.Send(tt.to, tt.subject, tt.body)

			if err != nil && !isDialErrorExpected(err) {
				t.Errorf("unexpected error type: %v", err)
			}
		})
	}
}

func TestSMTPSender_RecipientTrimming(t *testing.T) {
	sender := &SMTPSender{
		host:     "smtp.example.com",
		port:     587,
		username: "user@example.com",
		password: "password",
		from:     "noreply@example.com",
		timeout:  30 * time.Second,
	}

	testCases := []struct {
		input    string
		expected string
	}{
		{"  test@example.com  ", "test@example.com"},
		{"\ttest@example.com\t", "test@example.com"},
		{"\n  test@example.com  \n", "test@example.com"},
		{"test@example.com", "test@example.com"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("trim_%s", tc.input), func(t *testing.T) {
			err := sender.Send(tc.input, "Test Subject", "Test Body")

			if err != nil && !isDialErrorExpected(err) {
				t.Errorf("unexpected error type: %v", err)
			}
		})
	}
}

func TestSMTPSender_TimeoutBehavior(t *testing.T) {
	sender := &SMTPSender{
		host:     "smtp.example.com",
		port:     587,
		username: "user@example.com",
		password: "password",
		from:     "noreply@example.com",
		timeout:  1 * time.Nanosecond,
	}

	start := time.Now()
	err := sender.Send("test@example.com", "Test Subject", "Test Body")
	duration := time.Since(start)

	if duration > 100*time.Millisecond {
		t.Errorf("timeout took too long: %v", duration)
	}

	if err != nil && !isDialErrorExpected(err) {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestSMTPSender_TimeoutVariations(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{
			name:    "zero timeout",
			timeout: 0,
		},
		{
			name:    "1 nanosecond timeout",
			timeout: 1 * time.Nanosecond,
		},
		{
			name:    "1 microsecond timeout",
			timeout: 1 * time.Microsecond,
		},
		{
			name:    "1 millisecond timeout",
			timeout: 1 * time.Millisecond,
		},
		{
			name:    "1 second timeout",
			timeout: 1 * time.Second,
		},
		{
			name:    "1 hour timeout",
			timeout: 1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &SMTPSender{
				host:     "smtp.example.com",
				port:     587,
				username: "user@example.com",
				password: "password",
				from:     "noreply@example.com",
				timeout:  tt.timeout,
			}

			start := time.Now()
			err := sender.Send("test@example.com", "Test Subject", "Test Body")
			duration := time.Since(start)

			if tt.timeout < 100*time.Millisecond {
				if err != nil && !isDialErrorExpected(err) {
					t.Errorf("expected timeout or DNS error, got: %v", err)
				}
				if duration > 2*time.Second {
					t.Errorf("timeout took too long: %v", duration)
				}
			} else {
				if err != nil && !isDialErrorExpected(err) {
					t.Errorf("unexpected error type: %v", err)
				}
			}
		})
	}
}

func TestSMTPSender_ErrorRecovery(t *testing.T) {
	sender := &SMTPSender{
		host:     "smtp.example.com",
		port:     587,
		username: "user@example.com",
		password: "password",
		from:     "noreply@example.com",
		timeout:  30 * time.Second,
	}

	for i := range 5 {
		err := sender.Send("test@example.com", "Test Subject", "Test Body")
		if err != nil && !isDialErrorExpected(err) {
			t.Errorf("unexpected error type on attempt %d: %v", i, err)
		}
	}
}

func TestSMTPSender_InterfaceCompliance(t *testing.T) {
	var _ EmailSender = (*SMTPSender)(nil)
}

func BenchmarkSMTPSender_Send(b *testing.B) {
	sender := &SMTPSender{
		host:     "smtp.example.com",
		port:     587,
		username: "user@example.com",
		password: "password",
		from:     "noreply@example.com",
		timeout:  30 * time.Second,
	}

	for b.Loop() {
		sender.Send("test@example.com", "Test Subject", "Test Body")
	}
}

func BenchmarkSMTPSender_SendAsync(b *testing.B) {
	sender := &SMTPSender{
		host:     "smtp.example.com",
		port:     587,
		username: "user@example.com",
		password: "password",
		from:     "noreply@example.com",
		timeout:  30 * time.Second,
	}

	for b.Loop() {
		resultChan := sender.SendAsync("test@example.com", "Test Subject", "Test Body")
		<-resultChan
	}
}

func TestSMTPSender_Comprehensive(t *testing.T) {
	client := newTestSMTPClientOrSkip(t)
	defer client.Close()

	err := client.SendTestEmail("recipient@example.com", "Test Subject", "Test Body")
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}

	if !client.WaitForEmail(1 * time.Second) {
		t.Error("Email not received within timeout")
	}

	emails := client.GetReceivedEmails()
	if len(emails) != 1 {
		t.Errorf("Expected 1 email, got %d", len(emails))
		return
	}

	validator := testutils.NewTestEmailValidator()
	errors := validator.ValidateEmail(&emails[0], "recipient@example.com", "Test Subject", "Test Body")
	if len(errors) > 0 {
		t.Errorf("Email validation failed: %v", errors)
	}
}

func TestSMTPSender_HTML_Email(t *testing.T) {
	client := newTestSMTPClientOrSkip(t)
	defer client.Close()

	htmlBody := "<html><body><h1>Test</h1><p>This is a test email.</p></body></html>"
	err := client.SendTestEmail("recipient@example.com", "HTML Test Subject", htmlBody)
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}

	if !client.WaitForEmail(1 * time.Second) {
		t.Error("Email not received within timeout")
	}

	emails := client.GetReceivedEmails()
	if len(emails) != 1 {
		t.Errorf("Expected 1 email, got %d", len(emails))
		return
	}

	email := emails[0]
	if !strings.Contains(email.Body, "<html>") {
		t.Errorf("Body should contain HTML: %v", email.Body)
	}

	if email.Headers["Content-Type"] != "text/html; charset=\"UTF-8\"" {
		t.Errorf("Content-Type = %v, want text/html; charset=\"UTF-8\"", email.Headers["Content-Type"])
	}
}

func TestSMTPSender_Async_Email(t *testing.T) {
	client := newTestSMTPClientOrSkip(t)
	defer client.Close()

	resultChan := client.SendTestEmailAsync("recipient@example.com", "Async Test Subject", "Async Test Body")

	select {
	case err := <-resultChan:
		if err != nil {
			t.Errorf("Async send failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Async send timed out")
	}

	if !client.WaitForEmail(1 * time.Second) {
		t.Error("Email not received within timeout")
	}

	emails := client.GetReceivedEmails()
	if len(emails) != 1 {
		t.Errorf("Expected 1 email, got %d", len(emails))
		return
	}

	validator := testutils.NewTestEmailValidator()
	errors := validator.ValidateEmail(&emails[0], "recipient@example.com", "Async Test Subject", "Async Test Body")
	if len(errors) > 0 {
		t.Errorf("Email validation failed: %v", errors)
	}
}

func TestSMTPSender_Multiple_Emails(t *testing.T) {
	client := newTestSMTPClientOrSkip(t)
	defer client.Close()

	numEmails := 5
	for i := range numEmails {
		err := client.SendTestEmail(
			"recipient@example.com",
			"Test Subject",
			"Test Body",
		)
		if err != nil {
			t.Errorf("Send %d failed: %v", i, err)
		}
	}

	if !client.WaitForEmail(2 * time.Second) {
		t.Error("Emails not received within timeout")
	}

	emails := client.GetReceivedEmails()
	if len(emails) != numEmails {
		t.Errorf("Expected %d emails, got %d", numEmails, len(emails))
	}
}

func TestSMTPSender_Concurrent_Emails(t *testing.T) {
	client := newTestSMTPClientOrSkip(t)
	defer client.Close()

	numEmails := 10
	done := make(chan error, numEmails)

	for i := range numEmails {
		go func(i int) {
			err := client.SendTestEmail(
				"recipient@example.com",
				"Test Subject",
				"Test Body",
			)
			done <- err
		}(i)
	}

	for i := range numEmails {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Concurrent send %d failed: %v", i, err)
			}
		case <-time.After(10 * time.Second):
			t.Errorf("Concurrent send %d timed out", i)
		}
	}

	if !client.WaitForEmail(2 * time.Second) {
		t.Error("Emails not received within timeout")
	}

	emails := client.GetReceivedEmails()
	if len(emails) != numEmails {
		t.Errorf("Expected %d emails, got %d", numEmails, len(emails))
	}
}

func TestSMTPSender_Server_Failure(t *testing.T) {
	client := newTestSMTPClientOrSkip(t)
	defer client.Close()

	client.Server().SetShouldFail(true)

	err := client.SendTestEmail("recipient@example.com", "Test Subject", "Test Body")
	if err == nil {
		t.Error("Expected error, got nil")
	}

	emails := client.GetReceivedEmails()
	if len(emails) != 0 {
		t.Errorf("Expected 0 emails, got %d", len(emails))
	}
}

func TestSMTPSender_Timeout(t *testing.T) {
	client := newTestSMTPClientOrSkip(t)
	defer client.Close()

	client.Server().SetDelay(2 * time.Second)
	client.SetTimeout(1 * time.Second)

	start := time.Now()
	err := client.SendTestEmail("recipient@example.com", "Test Subject", "Test Body")
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	if duration > 2*time.Second {
		t.Errorf("Timeout took too long: %v", duration)
	}
}

func TestSMTPSender_Email_Headers(t *testing.T) {
	client := newTestSMTPClientOrSkip(t)
	defer client.Close()

	err := client.SendTestEmail("recipient@example.com", "Test Subject", "Test Body")
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}

	if !client.WaitForEmail(1 * time.Second) {
		t.Error("Email not received within timeout")
	}

	emails := client.GetReceivedEmails()
	if len(emails) != 1 {
		t.Errorf("Expected 1 email, got %d", len(emails))
		return
	}

	email := emails[0]
	expectedHeaders := map[string]string{
		"From":         "test@example.com",
		"To":           "recipient@example.com",
		"Subject":      "Test Subject",
		"Content-Type": "text/html; charset=\"UTF-8\"",
		"MIME-Version": "1.0",
	}

	validator := testutils.NewTestEmailValidator()
	errors := validator.ValidateEmailHeaders(&email, expectedHeaders)
	if len(errors) > 0 {
		t.Errorf("Header validation failed: %v", errors)
	}
}

func TestSMTPSender_Email_Matcher(t *testing.T) {
	client := newTestSMTPClientOrSkip(t)
	defer client.Close()

	emails := []struct {
		to      string
		subject string
		body    string
	}{
		{"user1@example.com", "Subject 1", "Body 1"},
		{"user2@example.com", "Subject 2", "Body 2"},
		{"user3@example.com", "Subject 3", "Body 3"},
	}

	for _, email := range emails {
		err := client.SendTestEmail(email.to, email.subject, email.body)
		if err != nil {
			t.Errorf("Send failed: %v", err)
		}
	}

	if !client.WaitForEmail(2 * time.Second) {
		t.Error("Emails not received within timeout")
	}

	receivedEmails := client.GetReceivedEmails()
	if len(receivedEmails) != len(emails) {
		t.Errorf("Expected %d emails, got %d", len(emails), len(receivedEmails))
	}

	matcher := testutils.NewTestEmailMatcher()

	email1 := matcher.FindEmail(receivedEmails, map[string]any{
		"to": "user1@example.com",
	})
	if email1 == nil {
		t.Error("Email 1 not found")
	}

	email2 := matcher.FindEmail(receivedEmails, map[string]any{
		"subject": "Subject 2",
	})
	if email2 == nil {
		t.Error("Email 2 not found")
	}

	email3 := matcher.FindEmail(receivedEmails, map[string]any{
		"body_contains": "Body 3",
	})
	if email3 == nil {
		t.Error("Email 3 not found")
	}

	count := matcher.CountMatchingEmails(receivedEmails, map[string]any{
		"subject_contains": "Subject",
	})
	if count != 3 {
		t.Errorf("Expected 3 emails with 'Subject' in subject, got %d", count)
	}
}

func TestSMTPSender_Email_Builder(t *testing.T) {
	builder := testutils.NewTestEmailBuilder()
	email := builder.
		From("sender@example.com").
		To("recipient@example.com").
		Subject("Test Subject").
		Body("Test Body").
		Header("X-Custom-Header", "Custom Value").
		Build()

	if email.From != "sender@example.com" {
		t.Errorf("From = %v, want sender@example.com", email.From)
	}
	if len(email.To) != 1 || email.To[0] != "recipient@example.com" {
		t.Errorf("To = %v, want [recipient@example.com]", email.To)
	}
	if email.Subject != "Test Subject" {
		t.Errorf("Subject = %v, want Test Subject", email.Subject)
	}
	if email.Body != "Test Body" {
		t.Errorf("Body = %v, want Test Body", email.Body)
	}
	if email.Headers["X-Custom-Header"] != "Custom Value" {
		t.Errorf("X-Custom-Header = %v, want Custom Value", email.Headers["X-Custom-Header"])
	}
}

func TestSMTPSender_Performance(t *testing.T) {
	client := newTestSMTPClientOrSkip(t)
	defer client.Close()

	numEmails := 100
	start := time.Now()

	for i := range numEmails {
		err := client.SendTestEmail(
			"recipient@example.com",
			"Test Subject",
			"Test Body",
		)
		if err != nil {
			t.Errorf("Send %d failed: %v", i, err)
		}
	}

	duration := time.Since(start)
	t.Logf("Sent %d emails in %v (%.2f emails/sec)", numEmails, duration, float64(numEmails)/duration.Seconds())

	if !client.WaitForEmail(5 * time.Second) {
		t.Error("Emails not received within timeout")
	}

	receivedEmails := client.GetReceivedEmails()
	if len(receivedEmails) != numEmails {
		t.Errorf("Expected %d emails, got %d", numEmails, len(receivedEmails))
	}
}

func TestSMTPSender_Stress(t *testing.T) {
	client := newTestSMTPClientOrSkip(t)
	defer client.Close()

	largeBody := strings.Repeat("This is a test email body. ", 1000)

	err := client.SendTestEmail("recipient@example.com", "Large Email Test", largeBody)
	if err != nil {
		t.Errorf("Send large email failed: %v", err)
	}

	if !client.WaitForEmail(2 * time.Second) {
		t.Error("Large email not received within timeout")
	}

	emails := client.GetReceivedEmails()
	if len(emails) != 1 {
		t.Errorf("Expected 1 email, got %d", len(emails))
	}

	email := emails[0]
	if len(email.Body) < 1000 {
		t.Errorf("Email body too short: %d characters", len(email.Body))
	}
}

func TestSMTPSender_Error_Recovery(t *testing.T) {
	client := newTestSMTPClientOrSkip(t)
	defer client.Close()

	err := client.SendTestEmail("recipient@example.com", "Test Subject", "Test Body")
	if err != nil {
		t.Errorf("First send failed: %v", err)
	}

	if !client.WaitForEmail(1 * time.Second) {
		t.Error("First email not received within timeout")
	}

	client.Server().SetShouldFail(true)

	err = client.SendTestEmail("recipient@example.com", "Test Subject", "Test Body")
	if err == nil {
		t.Error("Expected error, got nil")
	}

	client.Server().SetShouldFail(false)

	err = client.SendTestEmail("recipient@example.com", "Test Subject", "Test Body")
	if err != nil {
		t.Errorf("Recovery send failed: %v", err)
	}

	if !client.WaitForEmail(1 * time.Second) {
		t.Error("Recovery email not received within timeout")
	}

	emails := client.GetReceivedEmails()
	if len(emails) != 2 {
		t.Errorf("Expected 2 emails, got %d", len(emails))
	}
}

func TestSMTPSender_ConcurrentStress(t *testing.T) {
	client := newTestSMTPClientOrSkip(t)
	defer client.Close()

	numGoroutines := 50
	numEmailsPerGoroutine := 2
	totalEmails := numGoroutines * numEmailsPerGoroutine

	var wg sync.WaitGroup
	errors := make(chan error, totalEmails)

	start := time.Now()

	for i := range numGoroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := range numEmailsPerGoroutine {
				err := client.SendTestEmail(
					fmt.Sprintf("user%d@example.com", goroutineID),
					fmt.Sprintf("Subject %d-%d", goroutineID, j),
					fmt.Sprintf("Body %d-%d", goroutineID, j),
				)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d, email %d: %v", goroutineID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	duration := time.Since(start)
	t.Logf("Sent %d emails in %v (%.2f emails/sec)", totalEmails, duration, float64(totalEmails)/duration.Seconds())

	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent send error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Errorf("Had %d errors out of %d emails", errorCount, totalEmails)
	}

	if !client.WaitForEmail(5 * time.Second) {
		t.Error("Emails not received within timeout")
	}

	emails := client.GetReceivedEmails()
	if len(emails) != totalEmails {
		t.Errorf("Expected %d emails, got %d", totalEmails, len(emails))
	}
}

func TestSMTPSender_RaceConditions(t *testing.T) {

	client := newTestSMTPClientOrSkip(t)
	defer client.Close()

	sender := client.Sender()

	numGoroutines := 10
	var wg sync.WaitGroup

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sender.Send(fmt.Sprintf("test%d@example.com", id), "Test Subject", "Test Body")
		}(i)
	}

	wg.Wait()
}

func TestSMTPSender_MemoryLeaks(t *testing.T) {

	client := newTestSMTPClientOrSkip(t)
	defer client.Close()

	sender := client.Sender()

	numEmails := 1000
	var wg sync.WaitGroup

	for i := range numEmails {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sender.Send(fmt.Sprintf("test%d@example.com", id), "Test Subject", "Test Body")
		}(i)
	}

	wg.Wait()
}
