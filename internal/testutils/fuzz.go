package testutils

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"unicode/utf8"
)

type FuzzInputValidator struct {
	MaxInputLength int
	MinInputLength int
}

func NewFuzzInputValidator() *FuzzInputValidator {
	return &FuzzInputValidator{
		MaxInputLength: 10000,
		MinInputLength: 0,
	}
}

func (f *FuzzInputValidator) ValidateFuzzInput(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}

	if len(data) < f.MinInputLength || len(data) > f.MaxInputLength {
		return false
	}

	return true
}

func (f *FuzzInputValidator) ValidateFuzzInputStrict(data []byte) bool {
	if !f.ValidateFuzzInput(data) {
		return false
	}

	input := string(data)
	return len(strings.TrimSpace(input)) != 0
}

func ValidateUTF8String(s string) {
	if !utf8.ValidString(s) {
		panic("String contains invalid UTF-8")
	}
}

func ValidateNoNullBytes(s string) {
	if strings.Contains(s, "\x00") {
		panic("String contains null bytes")
	}
}

func ValidateNoScriptTags(s string) {
	if strings.Contains(strings.ToLower(s), "<script") {
		panic("String contains script tags")
	}
}

func ValidateNoJavascriptProtocol(s string) {
	if strings.Contains(strings.ToLower(s), "javascript:") {
		panic("String contains javascript: protocol")
	}
}

func ValidateNoDangerousChars(s string) {
	dangerousChars := []string{"<", ">", "\"", "'", "&", "|", ";", "`", "$", "(", ")", "{", "}", "[", "]", "\\", "/", "*", "?", "!", "@", "#", "%", "^", "~"}

	for _, char := range dangerousChars {
		if strings.Contains(s, char) {
			panic("String contains dangerous character: " + char)
		}
	}
}

func ValidateNoDangerousHTMLTags(s string) {
	dangerousTags := []string{
		"<script", "</script>", "<iframe", "</iframe>", "<object", "</object>",
		"<embed", "</embed>", "<form", "</form>", "<input", "<button",
		"<link", "<meta", "<style", "</style>",
	}

	for _, tag := range dangerousTags {
		if strings.Contains(strings.ToLower(s), tag) {
			panic("String contains dangerous HTML tag: " + tag)
		}
	}
}

func ValidateNoPrivateIPs(s string) {
	privateIPs := []string{
		"localhost", "127.0.0.1", "0.0.0.0", "10.", "172.", "192.168.", "169.254.169.254",
	}

	for _, ip := range privateIPs {
		if strings.Contains(strings.ToLower(s), ip) {
			panic("String contains private IP: " + ip)
		}
	}
}

func ValidateNoSQLInjectionPatterns(s string) {
	sqlPatterns := []string{
		"';", "--", "/*", "*/", "xp_", "sp_", "exec", "execute",
		"union", "select", "insert", "update", "delete", "drop",
		"create", "alter", "grant", "revoke", "truncate",
	}

	lowerS := strings.ToLower(s)
	for _, pattern := range sqlPatterns {
		if strings.Contains(lowerS, pattern) {
			panic("String contains SQL injection pattern: " + pattern)
		}
	}
}

func ValidateNoExcessiveRepetition(s string, maxRepeats int) {
	if hasRepeatedCharacters(s, maxRepeats) {
		panic("String contains excessive character repetition")
	}

	words := strings.Fields(s)
	wordCount := make(map[string]int)
	for _, word := range words {
		wordCount[strings.ToLower(word)]++
		if wordCount[strings.ToLower(word)] > 3 {
			panic("String contains excessive word repetition")
		}
	}
}

func hasRepeatedCharacters(str string, maxRepeats int) bool {
	if len(str) <= maxRepeats {
		return false
	}

	currentChar := rune(0)
	count := 0

	for _, char := range str {
		if char == currentChar {
			count++
			if count > maxRepeats {
				return true
			}
		} else {
			currentChar = char
			count = 1
		}
	}

	return false
}

type FuzzJSONParser struct{}

func NewFuzzJSONParser() *FuzzJSONParser {
	return &FuzzJSONParser{}
}

func (p *FuzzJSONParser) ParseJSON(data []byte) bool {
	var result map[string]any
	err := json.Unmarshal(data, &result)
	return err == nil
}

func (p *FuzzJSONParser) ParseJSONWithValidation(data []byte) {
	var result map[string]any
	err := json.Unmarshal(data, &result)
	if err != nil {
		return
	}

	for key, value := range result {
		ValidateUTF8String(key)
		if str, ok := value.(string); ok {
			ValidateUTF8String(str)
		}
	}
}

type FuzzHTTPRequest struct{}

func NewFuzzHTTPRequest() *FuzzHTTPRequest {
	return &FuzzHTTPRequest{}
}

func (r *FuzzHTTPRequest) CreateTestRequest(method, url string, body []byte, headers map[string]string) *http.Request {
	var reqBody bytes.Buffer
	if body != nil {
		reqBody.Write(body)
	}

	req := httptest.NewRequest(method, url, &reqBody)

	for name, value := range headers {
		req.Header.Set(name, value)
	}

	return req
}

func (r *FuzzHTTPRequest) ValidateHTTPRequest(req *http.Request) {
	pathParts := strings.SplitSeq(req.URL.Path, "/")
	for part := range pathParts {
		ValidateUTF8String(part)
	}

	for name, values := range req.URL.Query() {
		ValidateUTF8String(name)
		for _, value := range values {
			ValidateUTF8String(value)
		}
	}

	for name, values := range req.Header {
		ValidateUTF8String(name)
		for _, value := range values {
			ValidateUTF8String(value)
		}
	}
}

type FuzzSanitizer struct{}

func NewFuzzSanitizer() *FuzzSanitizer {
	return &FuzzSanitizer{}
}

func (s *FuzzSanitizer) SanitizeHTML(input string) string {
	scriptRegex := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	result := scriptRegex.ReplaceAllString(input, "")

	jsRegex := regexp.MustCompile(`(?i)javascript:`)
	result = jsRegex.ReplaceAllString(result, "")

	eventRegex := regexp.MustCompile(`(?i)\son\w+\s*=\s*"[^"]*"`)
	result = eventRegex.ReplaceAllString(result, "")

	return result
}

func (s *FuzzSanitizer) SanitizeSQL(input string) string {
	result := strings.ReplaceAll(input, "'", "''")
	result = strings.ReplaceAll(result, ";", "")
	return result
}

func (s *FuzzSanitizer) SanitizeXSS(input string) string {
	result := strings.ReplaceAll(input, "<", "&lt;")
	result = strings.ReplaceAll(result, ">", "&gt;")
	result = strings.ReplaceAll(result, "\"", "&quot;")
	result = strings.ReplaceAll(result, "'", "&#x27;")
	result = strings.ReplaceAll(result, "&", "&amp;")
	return result
}

func (s *FuzzSanitizer) SanitizeControlChars(input string) string {
	result := strings.ReplaceAll(input, "\x00", "")
	result = strings.ReplaceAll(result, "\r", "")
	result = strings.ReplaceAll(result, "\n", "")
	result = strings.ReplaceAll(result, "\t", "")
	return strings.TrimSpace(result)
}

func (s *FuzzSanitizer) ValidateSanitizedInput(input string) {
	ValidateUTF8String(input)
	ValidateNoNullBytes(input)
	ValidateNoScriptTags(input)
	ValidateNoJavascriptProtocol(input)
}

type FuzzValidationPipeline struct{}

func NewFuzzValidationPipeline() *FuzzValidationPipeline {
	return &FuzzValidationPipeline{}
}

func (p *FuzzValidationPipeline) ProcessInput(input string) string {
	result := strings.TrimSpace(input)

	if len(result) > 1000 {
		result = result[:1000]
	}

	result = strings.ReplaceAll(result, "\x00", "")
	result = strings.ReplaceAll(result, "\r", "")
	result = strings.ReplaceAll(result, "\n", "")

	return result
}

func (p *FuzzValidationPipeline) ValidateProcessedInput(input string) {
	ValidateUTF8String(input)
	ValidateNoNullBytes(input)
	ValidateNoExcessiveRepetition(input, 5)
}

type FuzzTestRunner struct{}

func NewFuzzTestRunner() *FuzzTestRunner {
	return &FuzzTestRunner{}
}

func (r *FuzzTestRunner) RunFuzzTest(data []byte, testFunc func(string)) int {
	validator := NewFuzzInputValidator()

	if !validator.ValidateFuzzInput(data) {
		return -1
	}

	input := string(data)

	testFunc(input)

	return 0
}

func (r *FuzzTestRunner) RunFuzzTestStrict(data []byte, testFunc func(string)) int {
	validator := NewFuzzInputValidator()

	if !validator.ValidateFuzzInputStrict(data) {
		return -1
	}

	input := string(data)

	testFunc(input)

	return 0
}

type CommonFuzzTestCases struct{}

func NewCommonFuzzTestCases() *CommonFuzzTestCases {
	return &CommonFuzzTestCases{}
}

func (c *CommonFuzzTestCases) GetAuthTestCases(fuzzedData string) []map[string]any {
	return []map[string]any{
		{
			"name": "auth_login",
			"body": `{"username":"` + fuzzedData + `","password":"test123"}`,
		},
		{
			"name": "auth_register",
			"body": `{"username":"` + fuzzedData + `","email":"test@example.com","password":"test123"}`,
		},
	}
}

func (c *CommonFuzzTestCases) GetPostTestCases(fuzzedData string) []map[string]any {
	return []map[string]any{
		{
			"name": "post_create",
			"body": `{"title":"` + fuzzedData + `","url":"https://example.com","content":"test"}`,
		},
		{
			"name": "post_search",
			"url":  "/api/posts/search?q=" + fuzzedData,
		},
	}
}

func (c *CommonFuzzTestCases) GetVoteTestCases(fuzzedData string) []map[string]any {
	return []map[string]any{
		{
			"name": "vote_cast",
			"body": `{"type":"` + fuzzedData + `"}`,
		},
	}
}
