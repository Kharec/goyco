package fuzz

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"
)

type FuzzTestHelper struct{}

func NewFuzzTestHelper() *FuzzTestHelper {
	return &FuzzTestHelper{}
}

func (h *FuzzTestHelper) RunBasicFuzzTest(f *testing.F, testFunc func(t *testing.T, input string)) {
	f.Add("test input")
	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			return
		}
		testFunc(t, input)
	})
}

func (h *FuzzTestHelper) RunValidationFuzzTest(f *testing.F, validateFunc func(string) error) {
	h.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		err := validateFunc(input)
		_ = err
	})
}

func (h *FuzzTestHelper) RunSanitizationFuzzTest(f *testing.F, sanitizeFunc func(string) string) {
	h.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		result := sanitizeFunc(input)
		if !utf8.ValidString(result) {
			t.Fatal("Sanitized result contains invalid UTF-8")
		}
	})
}

func (h *FuzzTestHelper) RunSanitizationFuzzTestWithValidation(f *testing.F, sanitizeFunc func(string) string, validateFunc func(string) bool) {
	h.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		result := sanitizeFunc(input)
		if !utf8.ValidString(result) {
			t.Fatal("Sanitized result contains invalid UTF-8")
		}
		if validateFunc != nil {
			if !validateFunc(result) {
				t.Fatal("Sanitized result failed validation")
			}
		}
	})
}

func (h *FuzzTestHelper) RunJSONFuzzTest(f *testing.F, testCases []map[string]any) {
	h.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		for _, tc := range testCases {
			body, ok := tc["body"].(string)
			if !ok {
				continue
			}
			encoded, err := json.Marshal(input)
			if err != nil {
				return
			}
			encodedStr := string(encoded)
			body = strings.ReplaceAll(body, "FUZZED_INPUT", encodedStr)

			var result map[string]any
			err = json.Unmarshal([]byte(body), &result)
			if err != nil {
				return
			}
		}
	})
}

func (h *FuzzTestHelper) RunHTTPFuzzTest(f *testing.F, testCases []HTTPFuzzTestCase) {
	h.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		for _, tc := range testCases {

			sanitized := h.sanitizeForURL(input)

			url := strings.ReplaceAll(tc.URL, "FUZZED_INPUT", sanitized)
			body := strings.ReplaceAll(tc.Body, "FUZZED_INPUT", sanitized)

			req := httptest.NewRequest(tc.Method, url, bytes.NewBufferString(body))

			for name, value := range tc.Headers {
				req.Header.Set(name, value)
			}

			h.validateHTTPRequest(t, req)
		}
	})
}

func (h *FuzzTestHelper) sanitizeForURL(input string) string {
	sanitized := strings.ReplaceAll(input, "\n", "")
	sanitized = strings.ReplaceAll(sanitized, "\r", "")
	sanitized = strings.ReplaceAll(sanitized, "\t", "")
	sanitized = url.QueryEscape(sanitized)
	sanitized = strings.ReplaceAll(sanitized, "+", "%20")

	if len(sanitized) > 100 {
		sanitized = sanitized[:100]
	}

	return sanitized
}

type HTTPFuzzTestCase struct {
	Name    string
	Method  string
	URL     string
	Headers map[string]string
	Body    string
}

func (h *FuzzTestHelper) validateHTTPRequest(t *testing.T, req *http.Request) {
	pathParts := strings.SplitSeq(req.URL.Path, "/")
	for part := range pathParts {
		if !utf8.ValidString(part) {
			t.Fatal("Path contains invalid UTF-8")
		}
	}

	for name, values := range req.URL.Query() {
		if !utf8.ValidString(name) {
			t.Fatal("Query parameter name contains invalid UTF-8")
		}
		for _, value := range values {
			if !utf8.ValidString(value) {
				t.Fatal("Query parameter value contains invalid UTF-8")
			}
		}
	}

	for name, values := range req.Header {
		if !utf8.ValidString(name) {
			t.Fatal("Header name contains invalid UTF-8")
		}
		for _, value := range values {
			if !utf8.ValidString(value) {
				t.Fatal("Header value contains invalid UTF-8")
			}
		}
	}
}

func (h *FuzzTestHelper) RunIntegrationFuzzTest(f *testing.F, testFunc func(t *testing.T, input string)) {
	h.RunBasicFuzzTest(f, func(t *testing.T, input string) {

		if len(input) > 1000 {
			input = input[:1000]
		}

		testFunc(t, input)
	})
}

func (h *FuzzTestHelper) GetCommonAuthTestCases(input string) []HTTPFuzzTestCase {
	return []HTTPFuzzTestCase{
		{
			Name:   "auth_register",
			Method: "POST",
			URL:    "/api/auth/register",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"username":"FUZZED_INPUT","email":"test@example.com","password":"test123"}`,
		},
		{
			Name:   "auth_login",
			Method: "POST",
			URL:    "/api/auth/login",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"username":"FUZZED_INPUT","password":"test123"}`,
		},
	}
}

func (h *FuzzTestHelper) GetCommonPostTestCases(input string) []HTTPFuzzTestCase {
	return []HTTPFuzzTestCase{
		{
			Name:   "post_create",
			Method: "POST",
			URL:    "/api/posts",
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer FUZZED_INPUT",
			},
			Body: `{"title":"FUZZED_INPUT","url":"https://example.com","content":"test"}`,
		},
		{
			Name:   "post_search",
			Method: "GET",
			URL:    "/api/posts/search?q=FUZZED_INPUT",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}
}

func (h *FuzzTestHelper) GetCommonVoteTestCases(input string) []HTTPFuzzTestCase {
	return []HTTPFuzzTestCase{
		{
			Name:   "vote_cast",
			Method: "POST",
			URL:    "/api/posts/1/vote",
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer FUZZED_INPUT",
			},
			Body: `{"type":"FUZZED_INPUT"}`,
		},
	}
}
