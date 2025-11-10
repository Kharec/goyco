package security

import (
	"testing"
)

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic text",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "script tag removal",
			input:    "<script>alert('xss')</script>Hello",
			expected: "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;Hello",
		},
		{
			name:     "javascript protocol removal",
			input:    "javascript:alert('xss')",
			expected: "alert(&#39;xss&#39;)",
		},
		{
			name:     "event handler removal",
			input:    "<img src='x' onerror='alert(1)'>",
			expected: "&lt;img src=&#39;x&#39; onerror=&#39;alert(1)&#39;&gt;",
		},
		{
			name:     "mixed content",
			input:    "Hello <script>alert('xss')</script> World",
			expected: "Hello &lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt; World",
		},
		{
			name:     "whitespace trimming",
			input:    "  Hello World  ",
			expected: "Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeInput(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeInput(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeUsername(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid username",
			input:    "testuser",
			expected: "testuser",
		},
		{
			name:     "username with special chars",
			input:    "test_user-123",
			expected: "test_user-123",
		},
		{
			name:     "username with invalid chars",
			input:    "test@user#123",
			expected: "testuser123",
		},
		{
			name:     "username starting with number",
			input:    "123test",
			expected: "123test",
		},
		{
			name:     "username starting with special char",
			input:    "@testuser",
			expected: "testuser",
		},
		{
			name:     "empty username",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeUsername(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeUsername(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid email",
			input:    "test@example.com",
			expected: "test@example.com",
		},
		{
			name:     "email with uppercase",
			input:    "TEST@EXAMPLE.COM",
			expected: "test@example.com",
		},
		{
			name:     "invalid email",
			input:    "not-an-email",
			expected: "",
		},
		{
			name:     "email with script",
			input:    "test<script>@example.com",
			expected: "",
		},
		{
			name:     "empty email",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeEmail(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeEmail(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid https url",
			input:    "https://example.com",
			expected: "https://example.com",
		},
		{
			name:     "valid http url",
			input:    "http://example.com",
			expected: "http://example.com",
		},
		{
			name:     "invalid protocol",
			input:    "ftp://example.com",
			expected: "",
		},
		{
			name:     "localhost blocked",
			input:    "http://localhost:8080",
			expected: "",
		},
		{
			name:     "private ip blocked",
			input:    "http://192.168.1.1",
			expected: "",
		},
		{
			name:     "aws metadata blocked",
			input:    "http://169.254.169.254/latest/meta-data",
			expected: "",
		},
		{
			name:     "empty url",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeURL(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeURL(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestInputSanitizer_SanitizePasswordCLI(t *testing.T) {
	sanitizer := &InputSanitizer{}

	tests := []struct {
		name        string
		password    string
		expectError bool
		description string
	}{
		{
			name:        "valid_password_with_all_types",
			password:    "Password123!",
			expectError: false,
			description: "Password with uppercase, lowercase, numbers, and special chars should be valid",
		},
		{
			name:        "valid_password_with_underscore",
			password:    "Password123_",
			expectError: false,
			description: "Password with underscore should be valid",
		},
		{
			name:        "valid_password_with_hyphen",
			password:    "Password123-",
			expectError: false,
			description: "Password with hyphen should be valid",
		},
		{
			name:        "valid_password_minimum_length",
			password:    "Pass123!",
			expectError: false,
			description: "Password with exactly 8 characters should be valid",
		},
		{
			name:        "valid_password_maximum_length",
			password:    createValidCLIPassword(128),
			expectError: false,
			description: "Password with exactly 128 characters should be valid",
		},

		{
			name:        "empty_password",
			password:    "",
			expectError: true,
			description: "Empty password should be rejected",
		},

		{
			name:        "password_too_short",
			password:    "Pass1!",
			expectError: true,
			description: "Password shorter than 8 characters should be rejected",
		},
		{
			name:        "password_too_long",
			password:    string(make([]byte, 129)),
			expectError: true,
			description: "Password longer than 128 characters should be rejected",
		},

		{
			name:        "common_weak_password",
			password:    "password",
			expectError: true,
			description: "Common weak password should be rejected",
		},
		{
			name:        "common_weak_password_123456",
			password:    "123456",
			expectError: true,
			description: "Common weak password 123456 should be rejected",
		},
		{
			name:        "common_weak_password_12345678",
			password:    "12345678",
			expectError: true,
			description: "Common weak password 12345678 should be rejected",
		},
		{
			name:        "common_weak_password_qwerty",
			password:    "qwerty",
			expectError: true,
			description: "Common weak password qwerty should be rejected",
		},
		{
			name:        "common_weak_password_abc123",
			password:    "abc123",
			expectError: true,
			description: "Common weak password abc123 should be rejected",
		},
		{
			name:        "common_weak_password_password123",
			password:    "password123",
			expectError: true,
			description: "Common weak password password123 should be rejected",
		},
		{
			name:        "common_weak_password_admin",
			password:    "admin",
			expectError: true,
			description: "Common weak password admin should be rejected",
		},
		{
			name:        "common_weak_password_letmein",
			password:    "letmein",
			expectError: true,
			description: "Common weak password letmein should be rejected",
		},
		{
			name:        "common_weak_password_welcome",
			password:    "welcome",
			expectError: true,
			description: "Common weak password welcome should be rejected",
		},
		{
			name:        "common_weak_password_monkey",
			password:    "monkey",
			expectError: true,
			description: "Common weak password monkey should be rejected",
		},
		{
			name:        "common_weak_password_dragon",
			password:    "dragon",
			expectError: true,
			description: "Common weak password dragon should be rejected",
		},
		{
			name:        "common_weak_password_master",
			password:    "master",
			expectError: true,
			description: "Common weak password master should be rejected",
		},
		{
			name:        "common_weak_password_hello",
			password:    "hello",
			expectError: true,
			description: "Common weak password hello should be rejected",
		},
		{
			name:        "common_weak_password_login",
			password:    "login",
			expectError: true,
			description: "Common weak password login should be rejected",
		},
		{
			name:        "common_weak_password_princess",
			password:    "princess",
			expectError: true,
			description: "Common weak password princess should be rejected",
		},

		{
			name:        "password_only_uppercase",
			password:    "PASSWORD",
			expectError: true,
			description: "Password with only uppercase letters should be rejected",
		},
		{
			name:        "password_only_lowercase",
			password:    "password",
			expectError: true,
			description: "Password with only lowercase letters should be rejected",
		},
		{
			name:        "password_only_numbers",
			password:    "12345678",
			expectError: true,
			description: "Password with only numbers should be rejected",
		},
		{
			name:        "password_only_special_chars",
			password:    "!@#$%^&*",
			expectError: true,
			description: "Password with only special characters should be rejected",
		},
		{
			name:        "password_uppercase_and_lowercase_only",
			password:    "Password",
			expectError: true,
			description: "Password with only uppercase and lowercase should be rejected",
		},
		{
			name:        "password_uppercase_and_numbers_only",
			password:    "PASSWORD123",
			expectError: true,
			description: "Password with only uppercase and numbers should be rejected",
		},
		{
			name:        "password_lowercase_and_numbers_only",
			password:    "password123",
			expectError: true,
			description: "Password with only lowercase and numbers should be rejected",
		},
		{
			name:        "password_uppercase_and_special_only",
			password:    "PASSWORD!",
			expectError: true,
			description: "Password with only uppercase and special chars should be rejected",
		},
		{
			name:        "password_lowercase_and_special_only",
			password:    "password!",
			expectError: true,
			description: "Password with only lowercase and special chars should be rejected",
		},
		{
			name:        "password_numbers_and_special_only",
			password:    "12345678!",
			expectError: true,
			description: "Password with only numbers and special chars should be rejected",
		},

		{
			name:        "valid_password_3_types_upper_lower_digit",
			password:    "Password123!",
			expectError: false,
			description: "Password with uppercase, lowercase, digits, and special chars should be valid",
		},
		{
			name:        "valid_password_3_types_upper_lower_special",
			password:    "Password!",
			expectError: false,
			description: "Password with uppercase, lowercase, and special chars should be valid",
		},
		{
			name:        "valid_password_3_types_upper_digit_special",
			password:    "PASSWORD123!",
			expectError: false,
			description: "Password with uppercase, digits, and special chars should be valid",
		},
		{
			name:        "valid_password_3_types_lower_digit_special",
			password:    "password123!",
			expectError: false,
			description: "Password with lowercase, digits, and special chars should be valid",
		},

		{
			name:        "common_weak_password_uppercase",
			password:    "PASSWORD",
			expectError: true,
			description: "Common weak password in uppercase should be rejected",
		},
		{
			name:        "common_weak_password_mixed_case",
			password:    "PassWord",
			expectError: true,
			description: "Common weak password in mixed case should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sanitizer.SanitizePasswordCLI(tt.password)

			if tt.expectError && err == nil {
				t.Errorf("SanitizePasswordCLI(%q) expected error, got nil. %s", tt.password, tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("SanitizePasswordCLI(%q) unexpected error: %v. %s", tt.password, err, tt.description)
			}
		})
	}
}

func TestInputSanitizer_SanitizePasswordCLI_Unicode(t *testing.T) {
	sanitizer := &InputSanitizer{}

	tests := []struct {
		name        string
		password    string
		expectError bool
		description string
	}{
		{
			name:        "unicode_letters_valid",
			password:    "Pássw0rd123!",
			expectError: false,
			description: "Password with Unicode letters should be valid",
		},
		{
			name:        "unicode_numbers_valid",
			password:    "Password１２３!",
			expectError: false,
			description: "Password with Unicode numbers should be valid",
		},
		{
			name:        "unicode_special_chars_valid",
			password:    "Password123！",
			expectError: false,
			description: "Password with Unicode special characters should be valid",
		},
		{
			name:        "mixed_unicode_valid",
			password:    "Pássw0rd１２３！",
			expectError: false,
			description: "Password with mixed Unicode characters should be valid",
		},
		{
			name:        "unicode_only_letters",
			password:    "Pásswórd",
			expectError: true,
			description: "Password with only Unicode letters should be rejected",
		},
		{
			name:        "unicode_only_numbers",
			password:    "１２３４５６７８",
			expectError: true,
			description: "Password with only Unicode numbers should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sanitizer.SanitizePasswordCLI(tt.password)

			if tt.expectError && err == nil {
				t.Errorf("SanitizePasswordCLI(%q) expected error, got nil. %s", tt.password, tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("SanitizePasswordCLI(%q) unexpected error: %v. %s", tt.password, err, tt.description)
			}
		})
	}
}

func TestInputSanitizer_SanitizePasswordCLI_Boundary(t *testing.T) {
	sanitizer := &InputSanitizer{}

	tests := []struct {
		name        string
		password    string
		expectError bool
		description string
	}{
		{
			name:        "exactly_8_chars_valid",
			password:    "Pass123!",
			expectError: false,
			description: "Password with exactly 8 characters should be valid",
		},
		{
			name:        "exactly_128_chars_valid",
			password:    createValidCLIPassword(128),
			expectError: false,
			description: "Password with exactly 128 characters should be valid",
		},
		{
			name:        "7_chars_invalid",
			password:    "Pass12!",
			expectError: true,
			description: "Password with 7 characters should be rejected",
		},
		{
			name:        "129_chars_invalid",
			password:    string(make([]byte, 129)),
			expectError: true,
			description: "Password with 129 characters should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testPassword := tt.password
			if len(testPassword) >= 8 && len(testPassword) <= 128 {
				testPassword = createValidCLIPassword(len(tt.password))
			}

			err := sanitizer.SanitizePasswordCLI(testPassword)

			if tt.expectError && err == nil {
				t.Errorf("SanitizePasswordCLI(%q) expected error, got nil. %s", testPassword, tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("SanitizePasswordCLI(%q) unexpected error: %v. %s", testPassword, err, tt.description)
			}
		})
	}
}

func createValidCLIPassword(length int) string {
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
