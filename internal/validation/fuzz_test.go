package validation

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzValidateEmail(f *testing.F) {
	runValidationFuzzTest(f, ValidateEmail)
}

func FuzzValidateUsername(f *testing.F) {
	runValidationFuzzTest(f, ValidateUsername)
}

func FuzzValidatePassword(f *testing.F) {
	runValidationFuzzTest(f, ValidatePassword)
}

func FuzzValidateURL(f *testing.F) {
	runValidationFuzzTest(f, ValidateURL)
}

func FuzzValidateTitle(f *testing.F) {
	runValidationFuzzTest(f, ValidateTitle)
}

func FuzzValidateContent(f *testing.F) {
	runValidationFuzzTest(f, ValidateContent)
}

func FuzzValidateSearchQuery(f *testing.F) {
	runValidationFuzzTest(f, ValidateSearchQuery)
}

func FuzzSanitizeString(f *testing.F) {
	f.Add("test input")
	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			return
		}
		result := SanitizeString(input)
		if !utf8.ValidString(result) {
			t.Fatal("Sanitized result contains invalid UTF-8")
		}
		if containsNullBytes(result) {
			t.Fatal("Sanitized result contains null bytes")
		}
	})
}

func runValidationFuzzTest(f *testing.F, validateFunc func(string) error) {
	f.Add("test input")
	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			return
		}
		err := validateFunc(input)
		_ = err
	})
}

func containsNullBytes(s string) bool {
	return strings.Contains(s, "\x00")
}
