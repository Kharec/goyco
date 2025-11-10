package validation

import (
	"strings"
	"testing"

	"goyco/internal/fuzz"
)

func FuzzValidateEmail(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunValidationFuzzTest(f, ValidateEmail)
}

func FuzzValidateUsername(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunValidationFuzzTest(f, ValidateUsername)
}

func FuzzValidatePassword(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunValidationFuzzTest(f, ValidatePassword)
}

func FuzzValidateURL(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunValidationFuzzTest(f, ValidateURL)
}

func FuzzValidateTitle(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunValidationFuzzTest(f, ValidateTitle)
}

func FuzzValidateContent(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunValidationFuzzTest(f, ValidateContent)
}

func FuzzValidateSearchQuery(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunValidationFuzzTest(f, ValidateSearchQuery)
}

func FuzzSanitizeString(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunSanitizationFuzzTestWithValidation(f,
		SanitizeString,
		func(result string) bool {
			return !containsNullBytes(result)
		})
}

func containsNullBytes(s string) bool {
	return strings.Contains(s, "\x00")
}
