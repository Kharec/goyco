package services

import (
	"fmt"
	"html"
	"os"
	"strings"
	"testing"
	"time"

	"goyco/internal/config"
	"goyco/internal/testutils"
)

func TestEmailService_IntegrationWithRealSMTP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	sender := testutils.GetSMTPSenderFromEnv(t)

	recipient := os.Getenv("SMTP_TEST_RECIPIENT")
	if strings.TrimSpace(recipient) == "" {
		recipient = sender.From
	}

	config := testutils.NewEmailTestConfig("https://example.com")
	service, err := NewEmailService(config, sender)
	if err != nil {
		t.Skipf("Skipping SMTP integration test: failed to create email service: %v", err)
	}

	user := testutils.NewEmailTestUser("integrationuser", recipient)
	token := "integration-test-token"

	t.Run("VerificationEmail_RealSMTP", func(t *testing.T) {
		err := service.SendVerificationEmail(user, token)
		if err != nil {
			t.Errorf("SendVerificationEmail failed: %v", err)
		}

		body := service.GenerateVerificationEmailBody(user.Username, fmt.Sprintf("%s/confirm?token=%s", config.App.BaseURL, token))
		if !strings.Contains(body, "https://example.com/confirm?token=integration-test-token") {
			t.Errorf("Expected body to contain verification URL")
		}
		if !strings.Contains(body, "<!DOCTYPE html>") {
			t.Errorf("Expected HTML content in email body")
		}
	})

	t.Run("PasswordResetEmail_RealSMTP", func(t *testing.T) {
		err := service.SendPasswordResetEmail(user, token)
		if err != nil {
			t.Errorf("SendPasswordResetEmail failed: %v", err)
		}

		body := service.GeneratePasswordResetEmailBody(user.Username, fmt.Sprintf("%s/reset-password?token=%s", config.App.BaseURL, token))
		if !strings.Contains(body, "https://example.com/reset-password?token=integration-test-token") {
			t.Errorf("Expected body to contain reset URL")
		}
	})

	t.Run("AccountDeletionEmail_RealSMTP", func(t *testing.T) {
		err := service.SendAccountDeletionEmail(user, token)
		if err != nil {
			t.Errorf("SendAccountDeletionEmail failed: %v", err)
		}

		body := service.GenerateAccountDeletionEmailBody(user.Username, fmt.Sprintf("%s/settings/delete/confirm?token=%s", config.App.BaseURL, token))
		if !strings.Contains(body, "https://example.com/settings/delete/confirm?token=integration-test-token") {
			t.Errorf("Expected body to contain deletion confirmation URL")
		}
	})
}

func TestEmailService_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	config := testutils.NewEmailTestConfig("https://example.com")
	service, err := NewEmailService(config, &testutils.MockEmailSender{})
	if err != nil {
		t.Fatalf("Failed to create email service: %v", err)
	}

	user := testutils.NewEmailTestUser("perfuser", "perf@example.com")

	service.GenerateVerificationEmailBody(user.Username, "https://example.com/confirm?token=test")

	start := time.Now()
	iterations := 1000
	for i := 0; i < iterations; i++ {
		service.GenerateVerificationEmailBody(user.Username, "https://example.com/confirm?token=test")
	}
	duration := time.Since(start)

	maxDuration := 500 * time.Millisecond
	if duration > maxDuration {
		t.Errorf("HTML generation took too long: %v (expected < %v for %d iterations, %.2fms per template)",
			duration, maxDuration, iterations, float64(duration.Nanoseconds())/float64(iterations)/1e6)
	}

	t.Logf("Generated %d HTML emails in %v (%.2fms per template)",
		iterations, duration, float64(duration.Nanoseconds())/float64(iterations)/1e6)
}

func TestEmailService_EdgeCases(t *testing.T) {
	config := testutils.NewEmailTestConfig("https://example.com")
	service, err := NewEmailService(config, &testutils.MockEmailSender{})
	if err != nil {
		t.Fatalf("Failed to create email service: %v", err)
	}

	t.Run("EmptyUsername", func(t *testing.T) {
		body := service.GenerateVerificationEmailBody("", "https://example.com/confirm?token=test")
		if !strings.Contains(body, "Hello ,") {
			t.Error("Expected empty username to be handled gracefully")
		}
	})

	t.Run("VeryLongUsername", func(t *testing.T) {
		longUsername := strings.Repeat("a", 1000)
		body := service.GenerateVerificationEmailBody(longUsername, "https://example.com/confirm?token=test")
		if !strings.Contains(body, longUsername) {
			t.Error("Expected long username to be included in email")
		}
	})

	t.Run("SpecialCharactersInUsername", func(t *testing.T) {
		specialUsername := "user@domain.com & <script>alert('xss')</script>"
		body := service.GenerateVerificationEmailBody(specialUsername, "https://example.com/confirm?token=test")
		escapedUsername := html.EscapeString(specialUsername)
		if !strings.Contains(body, escapedUsername) {
			t.Errorf("Expected escaped username %q to be included", escapedUsername)
		}
	})

	t.Run("EmptyToken", func(t *testing.T) {
		body := service.GenerateVerificationEmailBody("testuser", "https://example.com/confirm?token=")
		if !strings.Contains(body, "https://example.com/confirm?token=") {
			t.Error("Expected empty token to be handled")
		}
	})

	t.Run("VeryLongToken", func(t *testing.T) {
		longToken := strings.Repeat("a", 1000)
		url := fmt.Sprintf("https://example.com/confirm?token=%s", longToken)
		body := service.GenerateVerificationEmailBody("testuser", url)
		if !strings.Contains(body, url) {
			t.Error("Expected long token to be included in email")
		}
	})
}

func TestNewEmailService(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		sender      EmailSender
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid configuration",
			config:      testutils.NewEmailTestConfig("https://example.com"),
			sender:      &testutils.MockEmailSender{},
			expectError: false,
		},
		{
			name:        "Empty base URL",
			config:      testutils.NewEmailTestConfig(""),
			sender:      &testutils.MockEmailSender{},
			expectError: true,
			errorMsg:    "APP_BASE_URL is required",
		},
		{
			name:        "Whitespace base URL",
			config:      testutils.NewEmailTestConfig("   "),
			sender:      &testutils.MockEmailSender{},
			expectError: true,
			errorMsg:    "APP_BASE_URL is required",
		},
		{
			name:        "Base URL with trailing slash",
			config:      testutils.NewEmailTestConfig("https://example.com/"),
			sender:      &testutils.MockEmailSender{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewEmailService(tt.config, tt.sender)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if service == nil {
				t.Error("Expected service, got nil")
				return
			}

			expectedBaseURL := strings.TrimRight(strings.TrimSpace(tt.config.App.BaseURL), "/")
			if service.baseURL != expectedBaseURL {
				t.Errorf("Expected baseURL '%s', got '%s'", expectedBaseURL, service.baseURL)
			}
		})
	}
}

func TestEmailService_DynamicTitle(t *testing.T) {
	const (
		placeholderTitle = "My Custom Site"
		customTitle      = "Custom Community"
	)

	cfg := &config.Config{
		App: config.AppConfig{
			Title:   customTitle,
			BaseURL: "https://example.com",
		},
	}

	sender := &testutils.MockEmailSender{}
	service, err := NewEmailService(cfg, sender)
	if err != nil {
		t.Fatalf("Failed to create email service: %v", err)
	}

	body := service.GenerateVerificationEmailBody("testuser", "https://example.com/confirm?token=abc123")

	if !strings.Contains(body, customTitle) {
		t.Error("Expected email body to contain custom site title")
	}

	if strings.Contains(body, placeholderTitle) {
		t.Errorf("Expected email body to not contain placeholder title %q", placeholderTitle)
	}

	if strings.Contains(body, "The Goyco Team") {
		t.Error("Expected email body to not contain default team name when custom title is set")
	}

	cfgDefault := &config.Config{
		App: config.AppConfig{
			Title:   "Goyco",
			BaseURL: "https://example.com",
		},
	}

	serviceDefault, err := NewEmailService(cfgDefault, sender)
	if err != nil {
		t.Fatalf("Failed to create email service: %v", err)
	}

	bodyDefault := serviceDefault.GenerateVerificationEmailBody("testuser", "https://example.com/confirm?token=abc123")

	if !strings.Contains(bodyDefault, "Goyco") {
		t.Error("Expected email body to contain default site title")
	}
}
