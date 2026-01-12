package validation

import (
	"strings"
	"testing"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "test@example.com", false},
		{"valid email with subdomain", "user@mail.example.com", false},
		{"empty email", "", true},
		{"invalid format", "invalid-email", true},
		{"missing @", "testexample.com", true},
		{"missing domain", "test@", true},
		{"multiple @ symbols", "test@@example.com", true},
		{"consecutive dots", "test..user@example.com", false},
		{"dot at start", ".test@example.com", false},
		{"dot at end", "test.@example.com", false},
		{"too long", "a" + strings.Repeat("x", 250) + "@example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{"valid username", "testuser", false},
		{"valid with underscore", "test_user", false},
		{"valid with hyphen", "test-user", false},
		{"valid with numbers", "test123", false},
		{"empty username", "", true},
		{"too short", "ab", true},
		{"too long", "a" + strings.Repeat("x", 50), true},
		{"invalid characters", "test@user", true},
		{"spaces", "test user", true},
		{"exactly 3 chars", "abc", false},
		{"exactly 20 chars", strings.Repeat("a", 20), false},
		{"exactly 50 chars", strings.Repeat("a", 50), false},
		{"exactly 51 chars", strings.Repeat("a", 51), true},
		{"starts with number", "123user", false},
		{"starts with underscore", "_user", false},
		{"starts with hyphen", "-user", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUsername() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"valid password", "Password123!", false},
		{"valid with special chars", "Pass@123", false},
		{"valid with underscore", "Password123_", false},
		{"valid with hyphen", "Password123-", false},
		{"valid with tilde", "Password123~", false},
		{"valid with backtick", "Password123`", false},
		{"valid with pipe", "Password123|", false},
		{"valid with brackets", "Password123[]", false},
		{"valid with braces", "Password123{}", false},
		{"valid with colon", "Password123:", false},
		{"valid with semicolon", "Password123;", false},
		{"valid with quotes", "Password123\"'", false},
		{"valid with angle brackets", "Password123<>", false},
		{"valid with comma and period", "Password123,.", false},
		{"valid with question mark and slash", "Password123?/", false},
		{"empty password", "", true},
		{"too short", "Pass1!", true},
		{"exactly 7 chars", "Pass12!", true},
		{"exactly 8 chars", "Pass123!", false},
		{"exactly 128 chars", createValidPassword(128), false},
		{"exactly 129 chars", createValidPassword(129), true},
		{"no letters", "12345678!", true},
		{"no numbers", "Password!", true},
		{"no special chars", "Password123", true},
		{"too long", strings.Repeat("a", 130), true},
		{"unicode letters", "Pássw0rd123!", false},
		{"unicode numbers", "Password１２３!", false},
		{"unicode special", "Password123!", false},
		{"mixed unicode", "Pássw0rd１２３!", false},
		{"only unicode letters", "Pásswórd", true},
		{"only unicode numbers", "１２３４５６７８", true},
		{"only unicode special", "！＠＃＄％＾＆＊", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTitle(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		wantErr bool
	}{
		{"valid title", "Test Post Title", false},
		{"empty title", "", true},
		{"too short", "ab", true},
		{"too long", strings.Repeat("a", 201), true},
		{"whitespace only", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTitle(tt.title)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTitle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"valid content", "This is a valid content", false},
		{"empty content", "", false},
		{"short content", "ab", false},
		{"whitespace only", "    ", false},
		{"exactly max length", strings.Repeat("a", 10000), false},
		{"over max length", strings.Repeat("a", 10001), true},
		{"very long content", strings.Repeat("a", 20000), true},
		{"content with newlines", "Line 1\nLine 2\nLine 3", false},
		{"content with special chars", "Content with special chars: !@#$%^&*()", false},
		{"unicode content", "Content with unicode: 你好世界", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContent(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateContent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid HTTP URL", "http://example.com", false},
		{"valid HTTPS URL", "https://example.com", false},
		{"valid with path", "https://example.com/path", false},
		{"valid with query params", "https://example.com/search?q=test", false},
		{"valid with fragment", "https://example.com/page#section", false},
		{"valid with port", "https://example.com:8080/path", false},
		{"empty URL", "", true},
		{"invalid protocol", "ftp://example.com", true},
		{"no protocol", "example.com", true},
		{"too long", "https://example.com/" + strings.Repeat("a", 2040), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSearchQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{"valid query", "search term", false},
		{"empty query", "", false},
		{"short query", "a", false},
		{"whitespace only", "   ", false},
		{"exactly max length", strings.Repeat("a", 100), false},
		{"over max length", strings.Repeat("a", 101), true},
		{"very long query", strings.Repeat("a", 200), true},
		{"query with special chars", "search with !@#$%^&*()", false},
		{"unicode query", "search with unicode: 你好世界", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSearchQuery(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSearchQuery() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal string", "hello world", "hello world"},
		{"with newlines", "hello\nworld", "helloworld"},
		{"with tabs", "hello\tworld", "helloworld"},
		{"with carriage returns", "hello\rworld", "helloworld"},
		{"with null bytes", "hello\x00world", "helloworld"},
		{"with spaces", "  hello world  ", "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeString(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeString() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateStruct(t *testing.T) {
	type TestStruct struct {
		Username string `json:"username" validate:"required,min=3,max=20"`
		Email    string `json:"email" validate:"required,email"`
		Age      int    `json:"age" validate:"min=18,max=120"`
		URL      string `json:"url" validate:"url"`
		Status   string `json:"status" validate:"oneof=active inactive pending"`
		Optional string `json:"optional" validate:"omitempty,min=1"`
	}

	t.Run("valid struct", func(t *testing.T) {
		s := TestStruct{
			Username: "testuser",
			Email:    "test@example.com",
			Age:      25,
			URL:      "https://example.com",
			Status:   "active",
		}
		err := ValidateStruct(s)
		if err != nil {
			t.Errorf("ValidateStruct() error = %v, want nil", err)
		}
	})

	t.Run("missing required field", func(t *testing.T) {
		s := TestStruct{
			Email:  "test@example.com",
			Age:    25,
			URL:    "https://example.com",
			Status: "active",
		}
		err := ValidateStruct(s)
		if err == nil {
			t.Error("ValidateStruct() expected error, got nil")
		}
		if structErr, ok := err.(*StructValidationError); ok {
			if len(structErr.Errors) == 0 {
				t.Error("Expected validation errors, got none")
			}
			if structErr.Errors[0].Message != "username is required" {
				t.Errorf("Expected JSON tag name in error, got %q", structErr.Errors[0].Message)
			}
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		s := TestStruct{
			Username: "testuser",
			Email:    "invalid-email",
			Age:      25,
			URL:      "https://example.com",
			Status:   "active",
		}
		err := ValidateStruct(s)
		if err == nil {
			t.Error("ValidateStruct() expected error, got nil")
		}
	})

	t.Run("invalid min", func(t *testing.T) {
		s := TestStruct{
			Username: "ab",
			Email:    "test@example.com",
			Age:      25,
			URL:      "https://example.com",
			Status:   "active",
		}
		err := ValidateStruct(s)
		if err == nil {
			t.Error("ValidateStruct() expected error, got nil")
		}
	})

	t.Run("whitespace required field", func(t *testing.T) {
		s := TestStruct{
			Username: "   ",
			Email:    "test@example.com",
			Age:      25,
			URL:      "https://example.com",
			Status:   "active",
		}
		err := ValidateStruct(s)
		if err == nil {
			t.Error("ValidateStruct() expected error, got nil")
		}
	})

	t.Run("invalid max", func(t *testing.T) {
		s := TestStruct{
			Username: strings.Repeat("a", 21),
			Email:    "test@example.com",
			Age:      25,
			URL:      "https://example.com",
			Status:   "active",
		}
		err := ValidateStruct(s)
		if err == nil {
			t.Error("ValidateStruct() expected error, got nil")
		}
	})

	t.Run("invalid URL", func(t *testing.T) {
		s := TestStruct{
			Username: "testuser",
			Email:    "test@example.com",
			Age:      25,
			URL:      "invalid-url",
			Status:   "active",
		}
		err := ValidateStruct(s)
		if err == nil {
			t.Error("ValidateStruct() expected error, got nil")
		}
	})

	t.Run("invalid oneof", func(t *testing.T) {
		s := TestStruct{
			Username: "testuser",
			Email:    "test@example.com",
			Age:      25,
			URL:      "https://example.com",
			Status:   "invalid",
		}
		err := ValidateStruct(s)
		if err == nil {
			t.Error("ValidateStruct() expected error, got nil")
		}
	})

	t.Run("omitempty with empty value", func(t *testing.T) {
		s := TestStruct{
			Username: "testuser",
			Email:    "test@example.com",
			Age:      25,
			URL:      "https://example.com",
			Status:   "active",
			Optional: "",
		}
		err := ValidateStruct(s)
		if err != nil {
			t.Errorf("ValidateStruct() error = %v, want nil (empty optional field should be allowed)", err)
		}
	})

	t.Run("omitempty with valid value", func(t *testing.T) {
		s := TestStruct{
			Username: "testuser",
			Email:    "test@example.com",
			Age:      25,
			URL:      "https://example.com",
			Status:   "active",
			Optional: "valid",
		}
		err := ValidateStruct(s)
		if err != nil {
			t.Errorf("ValidateStruct() error = %v, want nil", err)
		}
	})

	t.Run("pointer to struct", func(t *testing.T) {
		s := &TestStruct{
			Username: "testuser",
			Email:    "test@example.com",
			Age:      25,
			URL:      "https://example.com",
			Status:   "active",
		}
		err := ValidateStruct(s)
		if err != nil {
			t.Errorf("ValidateStruct() error = %v, want nil", err)
		}
	})

	t.Run("nil pointer", func(t *testing.T) {
		var s *TestStruct = nil
		err := ValidateStruct(s)
		if err != nil {
			t.Errorf("ValidateStruct() error = %v, want nil (nil should be allowed)", err)
		}
	})
}

func createValidPassword(length int) string {
	if length < 8 {
		return "Pass123!"
	}

	password := make([]byte, length)

	for i := range length {
		switch i % 4 {
		case 0:
			password[i] = 'P'
		case 1:
			password[i] = 'a'
		case 2:
			password[i] = '1'
		case 3:
			password[i] = '!'
		}
	}

	return string(password)
}
