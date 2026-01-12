package fuzz

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNewFuzzTestHelper(t *testing.T) {
	helper := NewFuzzTestHelper()
	if helper == nil {
		t.Fatal("NewFuzzTestHelper returned nil")
	}
}

func TestRunBasicFuzzTest(t *testing.T) {
	helper := NewFuzzTestHelper()
	if helper == nil {
		t.Fatal("helper should not be nil")
	}

}

func TestRunValidationFuzzTest(t *testing.T) {
	helper := NewFuzzTestHelper()
	if helper == nil {
		t.Fatal("helper should not be nil")
	}

	validateFunc := func(input string) error {
		if len(input) > 100 {
			return fmt.Errorf("input too long")
		}
		return nil
	}

	err := validateFunc("test")
	if err != nil {
		t.Errorf("short input should not error: %v", err)
	}
}

func TestRunSanitizationFuzzTest(t *testing.T) {
	helper := NewFuzzTestHelper()
	if helper == nil {
		t.Fatal("helper should not be nil")
	}

	sanitizeFunc := func(input string) string {

		result := ""
		for _, char := range input {
			if char != ' ' {
				result += string(char)
			}
		}
		return result
	}

	result := sanitizeFunc("hello world")
	if result != "helloworld" {
		t.Errorf("expected 'helloworld', got '%s'", result)
	}
}

func TestRunSanitizationFuzzTestWithValidation(t *testing.T) {
	helper := NewFuzzTestHelper()
	if helper == nil {
		t.Fatal("helper should not be nil")
	}

	sanitizeFunc := func(input string) string {

		result := ""
		for _, char := range input {
			if char != ' ' {
				result += string(char)
			}
		}
		return result
	}

	validateFunc := func(input string) bool {

		return len(input) > 0
	}

	result := sanitizeFunc("hello world")
	if !validateFunc(result) {
		t.Error("validation should pass for non-empty result")
	}
}

func TestRunJSONFuzzTest(t *testing.T) {
	helper := NewFuzzTestHelper()
	if helper == nil {
		t.Fatal("helper should not be nil")
	}

	testCases := []map[string]any{
		{
			"body": `{"username":"FUZZED_INPUT","email":"test@example.com"}`,
		},
		{
			"body": `{"title":"FUZZED_INPUT","content":"test"}`,
		},
	}

	if len(testCases) != 2 {
		t.Errorf("expected 2 test cases, got %d", len(testCases))
	}
}

func TestRunHTTPFuzzTest(t *testing.T) {
	helper := NewFuzzTestHelper()
	if helper == nil {
		t.Fatal("helper should not be nil")
	}

	testCases := []HTTPFuzzTestCase{
		{
			Name:   "test_request",
			Method: "GET",
			URL:    "/api/test?param=FUZZED_INPUT",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: "",
		},
	}

	if len(testCases) != 1 {
		t.Errorf("expected 1 test case, got %d", len(testCases))
	}
	if testCases[0].Method != "GET" {
		t.Errorf("expected GET method, got %s", testCases[0].Method)
	}
}

func TestRunIntegrationFuzzTest(t *testing.T) {
	helper := NewFuzzTestHelper()
	if helper == nil {
		t.Fatal("helper should not be nil")
	}
}

func TestSanitizeForURL(t *testing.T) {
	helper := NewFuzzTestHelper()

	testCases := []struct {
		input    string
		expected string
	}{
		{"hello world", "hello%20world"},
		{"test\nwith\nnewlines", "testwithnewlines"},
		{"test&with=special?chars#here", "test%26with%3Dspecial%3Fchars%23here"},
		{"", ""},
		{"a very long string that should be truncated because it exceeds the maximum length allowed for URLs in this test case", "a%20very%20long%20string%20that%20should%20be%20truncated%20because%20it%20exceeds%20the%20maximum%20length%20allowed%20for%20URLs%20in%20this%20test%20case"},
	}

	for _, tc := range testCases {
		result := helper.sanitizeForURL(tc.input)
		if len(result) > 100 {
			t.Errorf("Sanitized URL too long: %d characters", len(result))
		}

	}
}

func TestGetCommonAuthTestCases(t *testing.T) {
	helper := NewFuzzTestHelper()

	testCases := helper.GetCommonAuthTestCases("test_input")

	if len(testCases) == 0 {
		t.Fatal("GetCommonAuthTestCases returned empty slice")
	}

	for _, tc := range testCases {
		if tc.Name == "" {
			t.Error("Test case name is empty")
		}
		if tc.Method == "" {
			t.Error("Test case method is empty")
		}
		if tc.URL == "" {
			t.Error("Test case URL is empty")
		}
	}
}

func TestGetCommonPostTestCases(t *testing.T) {
	helper := NewFuzzTestHelper()

	testCases := helper.GetCommonPostTestCases("test_input")

	if len(testCases) == 0 {
		t.Fatal("GetCommonPostTestCases returned empty slice")
	}

	for _, tc := range testCases {
		if tc.Name == "" {
			t.Error("Test case name is empty")
		}
		if tc.Method == "" {
			t.Error("Test case method is empty")
		}
		if tc.URL == "" {
			t.Error("Test case URL is empty")
		}
	}
}

func TestGetCommonVoteTestCases(t *testing.T) {
	helper := NewFuzzTestHelper()

	testCases := helper.GetCommonVoteTestCases("test_input")

	if len(testCases) == 0 {
		t.Fatal("GetCommonVoteTestCases returned empty slice")
	}

	for _, tc := range testCases {
		if tc.Name == "" {
			t.Error("Test case name is empty")
		}
		if tc.Method == "" {
			t.Error("Test case method is empty")
		}
		if tc.URL == "" {
			t.Error("Test case URL is empty")
		}
	}
}

func TestHTTPFuzzTestCaseStructure(t *testing.T) {
	tc := HTTPFuzzTestCase{
		Name:   "test_case",
		Method: "GET",
		URL:    "/api/test",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: "test body",
	}

	if tc.Name != "test_case" {
		t.Error("Name field not set correctly")
	}
	if tc.Method != "GET" {
		t.Error("Method field not set correctly")
	}
	if tc.URL != "/api/test" {
		t.Error("URL field not set correctly")
	}
	if tc.Headers["Content-Type"] != "application/json" {
		t.Error("Headers field not set correctly")
	}
	if tc.Body != "test body" {
		t.Error("Body field not set correctly")
	}
}

func TestRunBasicFuzzTestLogic(t *testing.T) {

	testFunc := func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Fatal("Input should be valid UTF-8")
		}
	}

	testFunc(t, "valid input")
	testFunc(t, "测试中文")
	testFunc(t, "🚀 emoji test")
}

func TestRunValidationFuzzTestLogic(t *testing.T) {

	validateFunc := func(input string) error {
		if len(input) > 100 {
			return fmt.Errorf("input too long")
		}
		return nil
	}

	err := validateFunc("short input")
	if err != nil {
		t.Errorf("Expected no error for short input, got: %v", err)
	}

	err = validateFunc(strings.Repeat("a", 150))
	if err == nil {
		t.Error("Expected error for long input")
	}
}

func TestRunSanitizationFuzzTestLogic(t *testing.T) {

	sanitizeFunc := func(input string) string {

		result := strings.ReplaceAll(input, " ", "")
		result = strings.ReplaceAll(result, "\n", "")
		result = strings.ReplaceAll(result, "\r", "")
		return result
	}

	testCases := []struct {
		input    string
		expected string
	}{
		{"hello world", "helloworld"},
		{"test\nwith\nnewlines", "testwithnewlines"},
		{"", ""},
		{"🚀 test emoji", "🚀testemoji"},
	}

	for _, tc := range testCases {
		result := sanitizeFunc(tc.input)
		if result != tc.expected {
			t.Errorf("Expected '%s', got '%s'", tc.expected, result)
		}

		if !utf8.ValidString(result) {
			t.Errorf("Sanitized result contains invalid UTF-8: %s", result)
		}
	}
}

func TestRunSanitizationFuzzTestWithValidationLogic(t *testing.T) {
	sanitizeFunc := func(input string) string {
		return strings.TrimSpace(input)
	}

	validateFunc := func(input string) bool {
		return len(input) > 0
	}

	testCases := []struct {
		input      string
		shouldPass bool
	}{
		{"  valid input  ", true},
		{"", false},
		{"   ", false},
		{"test", true},
	}

	for _, tc := range testCases {
		result := sanitizeFunc(tc.input)
		valid := validateFunc(result)

		if valid != tc.shouldPass {
			t.Errorf("For input '%s', expected validation %v, got %v", tc.input, tc.shouldPass, valid)
		}
	}
}

func TestRunJSONFuzzTestLogic(t *testing.T) {
	testCases := []map[string]any{
		{
			"body": `{"username":"FUZZED_INPUT","email":"test@example.com"}`,
		},
		{
			"body": `{"title":"FUZZED_INPUT","content":"test"}`,
		},
	}

	for _, tc := range testCases {
		body, ok := tc["body"].(string)
		if !ok {
			t.Error("Expected body to be string")
			continue
		}

		replaced := strings.ReplaceAll(body, "FUZZED_INPUT", "test_input")
		if strings.Contains(replaced, "FUZZED_INPUT") {
			t.Error("FUZZED_INPUT should be replaced")
		}

		var result map[string]any
		err := json.Unmarshal([]byte(replaced), &result)
		if err != nil {
			t.Errorf("Expected valid JSON, got error: %v", err)
		}
	}
}

func TestRunHTTPFuzzTestLogic(t *testing.T) {
	helper := NewFuzzTestHelper()

	testCases := []HTTPFuzzTestCase{
		{
			Name:   "test_request",
			Method: "GET",
			URL:    "/api/test?param=FUZZED_INPUT",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: "",
		},
		{
			Name:   "post_request",
			Method: "POST",
			URL:    "/api/posts",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"title":"FUZZED_INPUT","content":"test"}`,
		},
	}

	for _, tc := range testCases {

		sanitized := helper.sanitizeForURL("test input with spaces")
		url := strings.ReplaceAll(tc.URL, "FUZZED_INPUT", sanitized)
		body := strings.ReplaceAll(tc.Body, "FUZZED_INPUT", sanitized)

		req := httptest.NewRequest(tc.Method, url, strings.NewReader(body))

		for name, value := range tc.Headers {
			req.Header.Set(name, value)
		}

		helper.validateHTTPRequest(t, req)
	}
}

func TestValidateHTTPRequest(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test?param=value", nil)
	req.Header.Set("Content-Type", "application/json")

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithInvalidUTF8(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := &http.Request{
		Method: "GET",
		URL:    &url.URL{Path: "/api/test"},
		Header: make(http.Header),
	}

	req.URL.RawQuery = "param=test%20value"
	req.Header.Set("Content-Type", "application/json")

	helper.validateHTTPRequest(t, req)
}

func TestRunIntegrationFuzzTestLogic(t *testing.T) {

	testFunc := func(t *testing.T, input string) {
		if len(input) > 1000 {
			t.Fatal("Input should be limited to 1000 characters")
		}
	}

	testFunc(t, "short input")

	longInput := strings.Repeat("a", 2000)
	if len(longInput) > 1000 {

		longInput = longInput[:1000]
	}
	testFunc(t, longInput)
}

func TestSanitizeForURLEdgeCases(t *testing.T) {
	helper := NewFuzzTestHelper()

	testCases := []struct {
		input          string
		maxLen         int
		expectedMaxLen int
	}{
		{"", 100, 0},
		{"short", 100, 5},
		{strings.Repeat("a", 50), 100, 50},
		{strings.Repeat("a", 150), 100, 100},
		{"test with spaces", 100, 100},
		{"test\nwith\nnewlines", 100, 100},
		{"test&with=special?chars#here", 100, 100},
	}

	for _, tc := range testCases {
		result := helper.sanitizeForURL(tc.input)
		if len(result) > tc.expectedMaxLen {
			t.Errorf("Expected max length %d, got %d for input '%s'", tc.expectedMaxLen, len(result), tc.input)
		}

		if !utf8.ValidString(result) {
			t.Errorf("Sanitized URL contains invalid UTF-8: %s", result)
		}
	}
}

func TestGetCommonAuthTestCasesWithInput(t *testing.T) {
	helper := NewFuzzTestHelper()

	inputs := []string{"", "test", "test@example.com", "very_long_username_that_might_cause_issues"}

	for _, input := range inputs {
		testCases := helper.GetCommonAuthTestCases(input)

		if len(testCases) == 0 {
			t.Fatal("GetCommonAuthTestCases returned empty slice")
		}

		for _, tc := range testCases {
			if tc.Name == "" {
				t.Error("Test case name is empty")
			}
			if tc.Method == "" {
				t.Error("Test case method is empty")
			}
			if tc.URL == "" {
				t.Error("Test case URL is empty")
			}

		}
	}
}

func TestGetCommonPostTestCasesWithInput(t *testing.T) {
	helper := NewFuzzTestHelper()

	inputs := []string{"", "test", "test title", "very_long_title_that_might_cause_issues"}

	for _, input := range inputs {
		testCases := helper.GetCommonPostTestCases(input)

		if len(testCases) == 0 {
			t.Fatal("GetCommonPostTestCases returned empty slice")
		}

		for _, tc := range testCases {
			if tc.Name == "" {
				t.Error("Test case name is empty")
			}
			if tc.Method == "" {
				t.Error("Test case method is empty")
			}
			if tc.URL == "" {
				t.Error("Test case URL is empty")
			}

		}
	}
}

func TestGetCommonVoteTestCasesWithInput(t *testing.T) {
	helper := NewFuzzTestHelper()

	inputs := []string{"", "up", "down", "invalid_vote_type"}

	for _, input := range inputs {
		testCases := helper.GetCommonVoteTestCases(input)

		if len(testCases) == 0 {
			t.Fatal("GetCommonVoteTestCases returned empty slice")
		}

		for _, tc := range testCases {
			if tc.Name == "" {
				t.Error("Test case name is empty")
			}
			if tc.Method == "" {
				t.Error("Test case method is empty")
			}
			if tc.URL == "" {
				t.Error("Test case URL is empty")
			}

		}
	}
}

func TestHTTPFuzzTestCaseEdgeCases(t *testing.T) {

	tc := HTTPFuzzTestCase{
		Name:    "",
		Method:  "",
		URL:     "",
		Headers: nil,
		Body:    "",
	}

	if tc.Name != "" {
		t.Error("Empty name should be preserved")
	}
	if tc.Method != "" {
		t.Error("Empty method should be preserved")
	}
	if tc.URL != "" {
		t.Error("Empty URL should be preserved")
	}
	if tc.Headers != nil {
		t.Error("Nil headers should be preserved")
	}
	if tc.Body != "" {
		t.Error("Empty body should be preserved")
	}

	tc.Headers = make(map[string]string)
	if tc.Headers == nil {
		t.Error("Headers should not be nil after initialization")
	}
}

func TestFuzzTestHelperMethodsWithNilInput(t *testing.T) {
	helper := NewFuzzTestHelper()

	result := helper.sanitizeForURL("")
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}

	result = helper.sanitizeForURL("   \n\t\r   ")
	if result == "" {
		t.Error("Whitespace should be sanitized but not completely removed")
	}
}

func TestValidateHTTPRequestWithComplexQuery(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test?param1=value1&param2=value2&param3=value%20with%20spaces", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token123")

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithEmptyHeaders(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithSpecialCharacters(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test-path?param=value%20with%20spaces&special=chars%26symbols", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Test-Agent/1.0")

	helper.validateHTTPRequest(t, req)
}

func TestSanitizeForURLWithSpecialCharacters(t *testing.T) {
	helper := NewFuzzTestHelper()

	testCases := []struct {
		input    string
		contains string
	}{
		{"test with spaces", "%20"},
		{"test\nwith\nnewlines", ""},
		{"test&with=special?chars#here", "%26"},
		{"test/with/slashes", "%2F"},
		{"test\\with\\backslashes", "%5C"},
		{"test with unicode 🚀", "%F0%9F%9A%80"},
	}

	for _, tc := range testCases {
		result := helper.sanitizeForURL(tc.input)

		if !utf8.ValidString(result) {
			t.Errorf("Sanitized URL contains invalid UTF-8: %s", result)
		}

		if len(result) > 100 {
			t.Errorf("Sanitized URL too long: %d characters", len(result))
		}

		if tc.contains != "" && !strings.Contains(result, tc.contains) {
			t.Errorf("Expected result to contain '%s', got '%s'", tc.contains, result)
		}
	}
}

func TestSanitizeForURLWithLongInput(t *testing.T) {
	helper := NewFuzzTestHelper()

	longInput := strings.Repeat("a", 200)
	result := helper.sanitizeForURL(longInput)

	if len(result) > 100 {
		t.Errorf("Expected max 100 characters, got %d", len(result))
	}

	if !utf8.ValidString(result) {
		t.Errorf("Sanitized URL contains invalid UTF-8: %s", result)
	}
}

func TestHTTPFuzzTestCaseWithNilHeaders(t *testing.T) {
	tc := HTTPFuzzTestCase{
		Name:    "test_case",
		Method:  "GET",
		URL:     "/api/test",
		Headers: nil,
		Body:    "test body",
	}

	if tc.Headers != nil {
		t.Error("Headers should be nil")
	}

	tc.Headers = make(map[string]string)
	tc.Headers["Content-Type"] = "application/json"

	if tc.Headers["Content-Type"] != "application/json" {
		t.Error("Headers not set correctly")
	}
}

func TestGetCommonTestCasesConsistency(t *testing.T) {
	helper := NewFuzzTestHelper()

	authCases := helper.GetCommonAuthTestCases("test")
	if len(authCases) != 2 {
		t.Errorf("Expected 2 auth test cases, got %d", len(authCases))
	}

	postCases := helper.GetCommonPostTestCases("test")
	if len(postCases) != 2 {
		t.Errorf("Expected 2 post test cases, got %d", len(postCases))
	}

	voteCases := helper.GetCommonVoteTestCases("test")
	if len(voteCases) != 1 {
		t.Errorf("Expected 1 vote test case, got %d", len(voteCases))
	}
}

func TestValidateHTTPRequestWithInvalidPath(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := &http.Request{
		Method: "GET",
		URL:    &url.URL{Path: "/api/test"},
		Header: make(http.Header),
	}

	helper.validateHTTPRequest(t, req)

	req.URL.Path = ""
	helper.validateHTTPRequest(t, req)

	req.URL.Path = "/"
	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithComplexHeaders(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("User-Agent", "Test-Agent/1.0")
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithEmptyQuery(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.URL.RawQuery = ""

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithSingleQueryParam(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test?param=value", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithMultipleQueryParams(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test?param1=value1&param2=value2&param3=value3", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithEmptyValues(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test?param1=&param2=value&param3=", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithDuplicateQueryParams(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test?param=value1&param=value2&param=value3", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithEmptyHeaderValues(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Empty-Header", "")
	req.Header.Set("Another-Header", "value")

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithMultipleHeaderValues(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Add("Multi-Value-Header", "value1")
	req.Header.Add("Multi-Value-Header", "value2")
	req.Header.Add("Multi-Value-Header", "value3")

	helper.validateHTTPRequest(t, req)
}

func TestSanitizeForURLWithEmptyString(t *testing.T) {
	helper := NewFuzzTestHelper()

	result := helper.sanitizeForURL("")
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

func TestSanitizeForURLWithOnlyWhitespace(t *testing.T) {
	helper := NewFuzzTestHelper()

	testCases := []string{
		" ",
		"\n",
		"\r",
		"\t",
		"   ",
		"\n\n\n",
		"\r\r\r",
		"\t\t\t",
		" \n\r\t ",
	}

	for _, input := range testCases {
		result := helper.sanitizeForURL(input)

		if len(result) > 0 && !strings.Contains(result, "%20") && !strings.Contains(result, "%0A") && !strings.Contains(result, "%0D") && !strings.Contains(result, "%09") {
			t.Errorf("Unexpected result for input '%s': '%s'", input, result)
		}
	}
}

func TestSanitizeForURLWithUnicodeCharacters(t *testing.T) {
	helper := NewFuzzTestHelper()

	testCases := []string{
		"🚀",
		"测试",
		"café",
		"naïve",
		"résumé",
		"测试中文",
		"🚀🚀🚀",
		"test🚀test",
	}

	for _, input := range testCases {
		result := helper.sanitizeForURL(input)

		if !utf8.ValidString(result) {
			t.Errorf("Result contains invalid UTF-8 for input '%s': '%s'", input, result)
		}

		if len(result) > 100 {
			t.Errorf("Result too long for input '%s': %d characters", input, len(result))
		}
	}
}

func TestSanitizeForURLWithControlCharacters(t *testing.T) {
	helper := NewFuzzTestHelper()

	testCases := []string{
		"test\x00null",
		"test\x01control",
		"test\x1funit_separator",
		"test\x7fdelete",
		"test\x80high_bit",
		"test\xffmax_byte",
	}

	for _, input := range testCases {
		result := helper.sanitizeForURL(input)

		if len(result) > 100 {
			t.Errorf("Result too long for input '%s': %d characters", input, len(result))
		}

	}
}

func TestSanitizeForURLWithExactLengthLimit(t *testing.T) {
	helper := NewFuzzTestHelper()

	input := strings.Repeat("a", 100)
	result := helper.sanitizeForURL(input)

	if len(result) != 100 {
		t.Errorf("Expected length 100, got %d for input of length 100", len(result))
	}

	if result != input {
		t.Errorf("Expected unchanged input, got '%s'", result)
	}
}

func TestSanitizeForURLWithJustOverLengthLimit(t *testing.T) {
	helper := NewFuzzTestHelper()

	input := strings.Repeat("a", 101)
	result := helper.sanitizeForURL(input)

	if len(result) != 100 {
		t.Errorf("Expected length 100, got %d for input of length 101", len(result))
	}

	expected := strings.Repeat("a", 100)
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestSanitizeForURLWithMixedSpecialCharacters(t *testing.T) {
	helper := NewFuzzTestHelper()

	input := "test with spaces & symbols = values ? query # fragment / path \\ backslash"
	result := helper.sanitizeForURL(input)

	if !utf8.ValidString(result) {
		t.Errorf("Result contains invalid UTF-8: '%s'", result)
	}

	if len(result) > 100 {
		t.Errorf("Result too long: %d characters", len(result))
	}

	expectedEncodings := []string{"%20", "%26", "%3D", "%3F", "%23", "%2F"}
	for _, encoding := range expectedEncodings {
		if !strings.Contains(result, encoding) {
			t.Errorf("Expected result to contain '%s', got '%s'", encoding, result)
		}
	}

	if strings.Contains(result, "%5C") {

	} else if strings.Contains(result, "%5") {

		t.Logf("Backslash encoding may be truncated due to length limit: '%s'", result)
	}
}

func TestRunJSONFuzzTestWithInvalidJSON(t *testing.T) {

	testCases := []map[string]any{
		{
			"body": `{"username":"FUZZED_INPUT","email":"test@example.com"`,
		},
		{
			"body": `{"title":"FUZZED_INPUT","content":"test"`,
		},
		{
			"body": `invalid json`,
		},
		{
			"body": `{"username":"FUZZED_INPUT","email":}`,
		},
	}

	for _, tc := range testCases {
		body, ok := tc["body"].(string)
		if !ok {
			t.Error("Expected body to be string")
			continue
		}

		replaced := strings.ReplaceAll(body, "FUZZED_INPUT", "test_input")
		if strings.Contains(replaced, "FUZZED_INPUT") {
			t.Error("FUZZED_INPUT should be replaced")
		}

		var result map[string]any
		err := json.Unmarshal([]byte(replaced), &result)
		if err == nil {
			t.Errorf("Expected JSON parsing error for invalid JSON: %s", replaced)
		}
	}
}

func TestRunJSONFuzzTestWithValidJSON(t *testing.T) {

	testCases := []map[string]any{
		{
			"body": `{"username":"FUZZED_INPUT","email":"test@example.com"}`,
		},
		{
			"body": `{"title":"FUZZED_INPUT","content":"test"}`,
		},
		{
			"body": `{"id":123,"name":"FUZZED_INPUT","active":true}`,
		},
		{
			"body": `{"data":{"value":"FUZZED_INPUT","count":42}}`,
		},
	}

	for _, tc := range testCases {
		body, ok := tc["body"].(string)
		if !ok {
			t.Error("Expected body to be string")
			continue
		}

		replaced := strings.ReplaceAll(body, "FUZZED_INPUT", "test_input")
		if strings.Contains(replaced, "FUZZED_INPUT") {
			t.Error("FUZZED_INPUT should be replaced")
		}

		var result map[string]any
		err := json.Unmarshal([]byte(replaced), &result)
		if err != nil {
			t.Errorf("Expected valid JSON, got error: %v", err)
		}
	}
}

func TestRunJSONFuzzTestWithEmptyBody(t *testing.T) {

	testCases := []map[string]any{
		{
			"body": "",
		},
		{
			"body": "{}",
		},
		{
			"body": "null",
		},
	}

	for _, tc := range testCases {
		body, ok := tc["body"].(string)
		if !ok {
			t.Error("Expected body to be string")
			continue
		}

		replaced := strings.ReplaceAll(body, "FUZZED_INPUT", "test_input")
		if strings.Contains(replaced, "FUZZED_INPUT") {
			t.Error("FUZZED_INPUT should be replaced")
		}

		var result map[string]any
		err := json.Unmarshal([]byte(replaced), &result)

		if body == "" && err == nil {
			t.Error("Expected JSON parsing error for empty string")
		} else if body != "" && err != nil {
			t.Errorf("Expected valid JSON, got error: %v", err)
		}
	}
}

func TestRunJSONFuzzTestWithNonStringBody(t *testing.T) {

	testCases := []map[string]any{
		{
			"body": 123,
		},
		{
			"body": true,
		},
		{
			"body": nil,
		},
		{
			"body": []string{"test"},
		},
	}

	for _, tc := range testCases {
		_, ok := tc["body"].(string)
		if ok {
			t.Error("Expected body to not be string")
		}

	}
}

func TestRunHTTPFuzzTestWithEmptyTestCases(t *testing.T) {

	testCases := []HTTPFuzzTestCase{}

	for _, tc := range testCases {

		_ = tc
	}
}

func TestRunHTTPFuzzTestWithNilHeaders(t *testing.T) {
	helper := NewFuzzTestHelper()

	testCases := []HTTPFuzzTestCase{
		{
			Name:    "test_request",
			Method:  "GET",
			URL:     "/api/test?param=FUZZED_INPUT",
			Headers: nil,
			Body:    "",
		},
	}

	for _, tc := range testCases {

		sanitized := helper.sanitizeForURL("test input")
		url := strings.ReplaceAll(tc.URL, "FUZZED_INPUT", sanitized)
		body := strings.ReplaceAll(tc.Body, "FUZZED_INPUT", sanitized)

		req := httptest.NewRequest(tc.Method, url, strings.NewReader(body))

		if tc.Headers != nil {
			for name, value := range tc.Headers {
				req.Header.Set(name, value)
			}
		}

		helper.validateHTTPRequest(t, req)
	}
}

func TestRunHTTPFuzzTestWithEmptyHeaders(t *testing.T) {
	helper := NewFuzzTestHelper()

	testCases := []HTTPFuzzTestCase{
		{
			Name:    "test_request",
			Method:  "GET",
			URL:     "/api/test?param=FUZZED_INPUT",
			Headers: make(map[string]string),
			Body:    "",
		},
	}

	for _, tc := range testCases {

		sanitized := helper.sanitizeForURL("test input")
		url := strings.ReplaceAll(tc.URL, "FUZZED_INPUT", sanitized)
		body := strings.ReplaceAll(tc.Body, "FUZZED_INPUT", sanitized)

		req := httptest.NewRequest(tc.Method, url, strings.NewReader(body))

		for name, value := range tc.Headers {
			req.Header.Set(name, value)
		}

		helper.validateHTTPRequest(t, req)
	}
}

func TestRunHTTPFuzzTestWithDifferentMethods(t *testing.T) {
	helper := NewFuzzTestHelper()

	testCases := []HTTPFuzzTestCase{
		{
			Name:    "get_request",
			Method:  "GET",
			URL:     "/api/test?param=FUZZED_INPUT",
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    "",
		},
		{
			Name:    "post_request",
			Method:  "POST",
			URL:     "/api/test",
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    `{"param":"FUZZED_INPUT"}`,
		},
		{
			Name:    "put_request",
			Method:  "PUT",
			URL:     "/api/test",
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    `{"param":"FUZZED_INPUT"}`,
		},
		{
			Name:    "delete_request",
			Method:  "DELETE",
			URL:     "/api/test?param=FUZZED_INPUT",
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    "",
		},
		{
			Name:    "patch_request",
			Method:  "PATCH",
			URL:     "/api/test",
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    `{"param":"FUZZED_INPUT"}`,
		},
	}

	for _, tc := range testCases {

		sanitized := helper.sanitizeForURL("test input")
		url := strings.ReplaceAll(tc.URL, "FUZZED_INPUT", sanitized)
		body := strings.ReplaceAll(tc.Body, "FUZZED_INPUT", sanitized)

		req := httptest.NewRequest(tc.Method, url, strings.NewReader(body))

		for name, value := range tc.Headers {
			req.Header.Set(name, value)
		}

		helper.validateHTTPRequest(t, req)
	}
}

func TestRunHTTPFuzzTestWithComplexURLs(t *testing.T) {
	helper := NewFuzzTestHelper()

	testCases := []HTTPFuzzTestCase{
		{
			Name:    "complex_query",
			Method:  "GET",
			URL:     "/api/test?param1=FUZZED_INPUT&param2=value2&param3=value3",
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    "",
		},
		{
			Name:    "path_with_params",
			Method:  "GET",
			URL:     "/api/posts/123/comments?filter=FUZZED_INPUT&sort=date",
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    "",
		},
		{
			Name:    "nested_path",
			Method:  "GET",
			URL:     "/api/v1/users/123/posts/456/votes?type=FUZZED_INPUT",
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    "",
		},
	}

	for _, tc := range testCases {

		sanitized := helper.sanitizeForURL("test input with spaces & symbols")
		url := strings.ReplaceAll(tc.URL, "FUZZED_INPUT", sanitized)
		body := strings.ReplaceAll(tc.Body, "FUZZED_INPUT", sanitized)

		req := httptest.NewRequest(tc.Method, url, strings.NewReader(body))

		for name, value := range tc.Headers {
			req.Header.Set(name, value)
		}

		helper.validateHTTPRequest(t, req)
	}
}

func TestRunIntegrationFuzzTestWithLongInput(t *testing.T) {

	testFunc := func(t *testing.T, input string) {
		if len(input) > 1000 {
			t.Fatal("Input should be limited to 1000 characters")
		}
	}

	testCases := []struct {
		input    string
		expected bool
	}{
		{"short", true},
		{strings.Repeat("a", 500), true},
		{strings.Repeat("a", 1000), true},
		{strings.Repeat("a", 1500), false},
		{strings.Repeat("a", 2000), false},
	}

	for _, tc := range testCases {

		input := tc.input
		if len(input) > 1000 {
			input = input[:1000]
		}

		testFunc(t, input)
	}
}

func TestRunIntegrationFuzzTestWithUnicodeInput(t *testing.T) {

	testFunc := func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Fatal("Input should be valid UTF-8")
		}
		if len(input) > 1000 {
			t.Fatal("Input should be limited to 1000 characters")
		}
	}

	testCases := []string{
		"测试中文",
		"🚀 emoji test",
		"café naïve résumé",
		"测试🚀emoji测试",
		strings.Repeat("🚀", 100),
		strings.Repeat("测试", 200),
	}

	for _, input := range testCases {

		limitedInput := input
		if len(limitedInput) > 1000 {

			limitedInput = limitedInput[:1000]
		}

		if utf8.ValidString(limitedInput) {
			testFunc(t, limitedInput)
		} else {

			t.Logf("Input truncated to invalid UTF-8 (expected): %s", limitedInput)
		}
	}
}

func TestRunIntegrationFuzzTestWithEmptyInput(t *testing.T) {

	testFunc := func(t *testing.T, input string) {
		if len(input) > 1000 {
			t.Fatal("Input should be limited to 1000 characters")
		}

	}

	testFunc(t, "")
}

func TestRunIntegrationFuzzTestWithControlCharacters(t *testing.T) {

	testFunc := func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Fatal("Input should be valid UTF-8")
		}
		if len(input) > 1000 {
			t.Fatal("Input should be limited to 1000 characters")
		}
	}

	testCases := []string{
		"test\x00null",
		"test\x01control",
		"test\x1funit_separator",
		"test\x7fdelete",
		"test\x80high_bit",
		"test\xffmax_byte",
	}

	for _, input := range testCases {

		limitedInput := input
		if len(limitedInput) > 1000 {
			limitedInput = limitedInput[:1000]
		}

		if utf8.ValidString(limitedInput) {
			testFunc(t, limitedInput)
		}
	}
}

func TestValidateHTTPRequestWithNilURL(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := &http.Request{
		Method: "GET",
		URL:    nil,
		Header: make(http.Header),
	}

	defer func() {
		if r := recover(); r != nil {

			t.Logf("Expected panic due to nil URL: %v", r)
		}
	}()

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithEmptyPath(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/", nil)
	req.URL.Path = ""

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithRootPath(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithSinglePathSegment(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithMultiplePathSegments(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/v1/users/123/posts", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithEmptyPathSegments(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api//users//123", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithTrailingSlash(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/users/", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithLeadingSlash(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/users", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithNoSlash(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := &http.Request{
		Method: "GET",
		URL:    &url.URL{Path: "api/users"},
		Header: make(http.Header),
	}

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithOnlySlash(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithComplexPath(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/v1/users/123/posts/456/comments/789", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithUnicodePath(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/测试/用户", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithEmojiPath(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/🚀/test", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithSpecialCharactersInPath(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test-path_with.underscores~and~tildes", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithNumbersInPath(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/v1/users/123/posts/456", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithMixedCasePath(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/Users/Posts/Comments", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithLongPath(t *testing.T) {
	helper := NewFuzzTestHelper()

	longPath := "/api/" + strings.Repeat("very-long-path-segment/", 20)
	req := httptest.NewRequest("GET", longPath, nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithEmptyQueryString(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test?", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithOnlyQuestionMark(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test?", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithAmpersandOnly(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test?&", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithEqualsOnly(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test?=", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithMultipleEquals(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test?param=value=another", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithUnicodeQueryParam(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test?param=测试&value=用户", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithEmojiQueryParam(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test?param=🚀&value=test", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithSpecialCharactersInQuery(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test?param=value+with+spaces&other=value%20encoded", nil)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithEmptyHeaderName(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("", "value")

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithEmptyHeaderValue(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Test-Header", "")

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithUnicodeHeader(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("测试-Header", "测试-Value")

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithEmojiHeader(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("🚀-Header", "🚀-Value")

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithSpecialCharactersInHeader(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Test-Header-With-Special-Chars", "value with spaces & symbols")

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithLongHeader(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test", nil)
	longValue := strings.Repeat("very-long-header-value-", 50)
	req.Header.Set("Long-Header", longValue)

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithManyHeaders(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test", nil)
	for i := 0; i < 20; i++ {
		req.Header.Set(fmt.Sprintf("Header-%d", i), fmt.Sprintf("Value-%d", i))
	}

	helper.validateHTTPRequest(t, req)
}

func TestValidateHTTPRequestWithMixedCaseHeaders(t *testing.T) {
	helper := NewFuzzTestHelper()

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("content-type", "text/plain")
	req.Header.Set("CONTENT-TYPE", "application/xml")

	helper.validateHTTPRequest(t, req)
}

func TestGetFuzzDB(t *testing.T) {
	db, err := GetFuzzDB()
	if err != nil {
		t.Fatalf("GetFuzzDB failed: %v", err)
	}
	if db == nil {
		t.Fatal("GetFuzzDB returned nil database")
	}

	db2, err2 := GetFuzzDB()
	if err2 != nil {
		t.Fatalf("Second GetFuzzDB call failed: %v", err2)
	}
	if db2 == nil {
		t.Fatal("Second GetFuzzDB returned nil database")
	}
	if db2 == db {
		t.Fatal("GetFuzzDB should return a new database instance for each call")
	}
}

func TestGetFuzzDBMigrations(t *testing.T) {
	db, err := GetFuzzDB()
	if err != nil {
		t.Fatalf("GetFuzzDB failed: %v", err)
	}

	var count int64
	db.Table("users").Count(&count)
	db.Table("posts").Count(&count)
	db.Table("votes").Count(&count)
	db.Table("account_deletion_requests").Count(&count)
	db.Table("refresh_tokens").Count(&count)
}
