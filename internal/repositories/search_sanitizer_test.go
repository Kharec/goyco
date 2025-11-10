package repositories

import (
	"testing"
)

func TestSearchSanitizer_SanitizeSearchQuery(t *testing.T) {
	sanitizer := NewSearchSanitizer()

	tests := []struct {
		name           string
		query          string
		expectedResult string
		expectError    bool
		errorType      string
	}{
		{
			name:           "normal_search",
			query:          "golang programming",
			expectedResult: "golang programming",
			expectError:    false,
		},
		{
			name:           "empty_query",
			query:          "",
			expectedResult: "",
			expectError:    false,
		},
		{
			name:           "whitespace_only",
			query:          "   ",
			expectedResult: "",
			expectError:    false,
		},
		{
			name:           "normalize_whitespace",
			query:          "golang   programming   tutorial",
			expectedResult: "golang programming tutorial",
			expectError:    false,
		},
		{
			name:           "remove_excessive_wildcards",
			query:          "test***wildcard",
			expectedResult: "test**wildcard",
			expectError:    false,
		},
		{
			name:           "remove_excessive_dots",
			query:          "test...dots",
			expectedResult: "test..dots",
			expectError:    false,
		},
		{
			name:           "query_too_long",
			query:          "a very long search query that exceeds the maximum length limit and should be rejected by the sanitizer",
			expectedResult: "",
			expectError:    true,
			errorType:      "QueryTooLong",
		},
		{
			name:           "too_many_special_chars",
			query:          "test@#$%^&*()_+{}|:<>?[]\\;'\",./",
			expectedResult: "",
			expectError:    true,
			errorType:      "TooManySpecialChars",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sanitizer.SanitizeSearchQuery(tt.query)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}

				if searchErr, ok := err.(*SearchError); ok {
					if searchErr.Type != tt.errorType {
						t.Errorf("Expected error type %s, got %s", tt.errorType, searchErr.Type)
					}
				} else {
					t.Errorf("Expected SearchError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}

				if result != tt.expectedResult {
					t.Errorf("Expected result %q, got %q", tt.expectedResult, result)
				}
			}
		})
	}
}

func TestSearchSanitizer_ValidateSearchQuery(t *testing.T) {
	sanitizer := NewSearchSanitizer()

	tests := []struct {
		name        string
		query       string
		expectError bool
		errorType   string
	}{
		{
			name:        "valid_query",
			query:       "golang programming",
			expectError: false,
		},
		{
			name:        "empty_query",
			query:       "",
			expectError: false,
		},
		{
			name:        "sql_injection_attempt",
			query:       "'; DROP TABLE users; --",
			expectError: true,
			errorType:   "InvalidQuery",
		},
		{
			name:        "union_attack",
			query:       "test UNION SELECT * FROM users",
			expectError: true,
			errorType:   "InvalidQuery",
		},
		{
			name:        "excessive_repetition",
			query:       "test test test test test",
			expectError: true,
			errorType:   "DoSPattern",
		},
		{
			name:        "repeated_characters",
			query:       "aaaaaaaaaaaa",
			expectError: true,
			errorType:   "DoSPattern",
		},
		{
			name:        "valid_with_special_chars",
			query:       "golang-programming_tutorial",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sanitizer.ValidateSearchQuery(tt.query)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}

				if searchErr, ok := err.(*SearchError); ok {
					if searchErr.Type != tt.errorType {
						t.Errorf("Expected error type %s, got %s", tt.errorType, searchErr.Type)
					}
				} else {
					t.Errorf("Expected SearchError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSearchSanitizer_EdgeCases(t *testing.T) {
	sanitizer := NewSearchSanitizer()

	t.Run("unicode_characters", func(t *testing.T) {
		query := "golang программирование 🚀"
		result, err := sanitizer.SanitizeSearchQuery(query)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if result != query {
			t.Errorf("Expected %q, got %q", query, result)
		}
	})

	t.Run("mixed_case_sql_injection", func(t *testing.T) {
		query := "test UnIoN SeLeCt * FrOm users"
		err := sanitizer.ValidateSearchQuery(query)
		if err == nil {
			t.Errorf("Expected error for SQL injection attempt")
		}
	})

	t.Run("normalize_whitespace_complex", func(t *testing.T) {
		query := "  \t\n  golang   \t\n  programming  \t\n  "
		result, err := sanitizer.SanitizeSearchQuery(query)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		expected := "golang programming"
		if result != expected {
			t.Errorf("Expected %q, got %q", expected, result)
		}
	})
}
