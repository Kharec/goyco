package security

import (
	"testing"

	"goyco/internal/fuzz"
	"goyco/internal/testutils"
)

func FuzzSanitizeInput(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunSanitizationFuzzTest(f, func(input string) string {
		result := SanitizeInput(input)
		testutils.ValidateNoScriptTags(result)
		testutils.ValidateNoJavascriptProtocol(result)
		return result
	})
}

func FuzzSanitizeUsername(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunSanitizationFuzzTest(f, func(input string) string {
		result := SanitizeUsername(input)
		testutils.ValidateNoDangerousChars(result)
		return result
	})
}

func FuzzSanitizeEmail(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		result := SanitizeEmail(input)
		if result != "" {
			testutils.ValidateUTF8String(result)
		}
	})
}

func FuzzSanitizePostContent(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunSanitizationFuzzTest(f, func(input string) string {
		result := SanitizePostContent(input)
		testutils.ValidateNoScriptTags(result)
		testutils.ValidateNoJavascriptProtocol(result)
		return result
	})
}

func FuzzSanitizeURL(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		result := SanitizeURL(input)
		if result != "" {
			testutils.ValidateUTF8String(result)
			testutils.ValidateNoPrivateIPs(result)
		}
	})
}

func FuzzInputSanitizerUsernameCLI(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		sanitizer := NewInputSanitizer()
		result, err := sanitizer.SanitizeUsernameCLI(input)
		if err == nil {
			testutils.ValidateUTF8String(result)
			testutils.ValidateNoDangerousChars(result)
		}
	})
}

func FuzzInputSanitizerEmailCLI(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		sanitizer := NewInputSanitizer()
		result, err := sanitizer.SanitizeEmailCLI(input)
		if err == nil {
			testutils.ValidateUTF8String(result)
		}
	})
}

func FuzzInputSanitizerPasswordCLI(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		sanitizer := NewInputSanitizer()
		_ = sanitizer.SanitizePasswordCLI(input)
	})
}

func FuzzInputSanitizerSearchTerm(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		sanitizer := NewInputSanitizer()
		result, err := sanitizer.SanitizeSearchTerm(input)
		if err == nil {
			testutils.ValidateUTF8String(result)
			testutils.ValidateNoDangerousChars(result)
		}
	})
}

func FuzzInputSanitizerTitleCLI(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		sanitizer := NewInputSanitizer()
		result, err := sanitizer.SanitizeTitleCLI(input)
		if err == nil {
			testutils.ValidateUTF8String(result)
		}
	})
}

func FuzzInputSanitizerContentCLI(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		sanitizer := NewInputSanitizer()
		result, err := sanitizer.SanitizeContentCLI(input)
		if err == nil {
			testutils.ValidateUTF8String(result)
			testutils.ValidateNoDangerousHTMLTags(result)
		}
	})
}

func FuzzInputSanitizerID(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		sanitizer := NewInputSanitizer()
		result, err := sanitizer.SanitizeID(input)
		if err == nil {
			if result == 0 {
				t.Fatal("SanitizeID returned 0 for valid input")
			}
			if result > 1000000 {
				t.Fatal("SanitizeID returned ID larger than expected limit")
			}
		}
	})
}
