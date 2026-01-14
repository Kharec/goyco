package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"goyco/internal/fuzz"
)

func FuzzJSONParsing(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	testCases := []map[string]any{
		{
			"name": "auth_login",
			"body": `{"username":"FUZZED_INPUT","password":"test"}`,
		},
		{
			"name": "auth_register",
			"body": `{"username":"FUZZED_INPUT","email":"test@example.com","password":"test123"}`,
		},
		{
			"name": "post_create",
			"body": `{"title":"FUZZED_INPUT","url":"https://example.com","content":"test"}`,
		},
		{
			"name": "vote_cast",
			"body": `{"type":"FUZZED_INPUT"}`,
		},
	}
	helper.RunJSONFuzzTest(f, testCases)
}

func FuzzURLParsing(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		var sanitized strings.Builder
		sanitized.Grow(len(input))
		sanitizedLen := 0
		for _, char := range input {
			if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
				(char >= '0' && char <= '9') || char == '-' || char == '_' {
				sanitized.WriteRune(char)
				sanitizedLen++
				if sanitizedLen >= 20 {
					break
				}
			}
		}

		if sanitizedLen == 0 {
			return
		}

		url := "/api/posts/" + sanitized.String()
		req := httptest.NewRequest("GET", url, nil)

		pathParts := strings.Split(req.URL.Path, "/")
		if len(pathParts) >= 4 {
			idStr := pathParts[3]
			_ = idStr
		}
	})
}

func FuzzQueryParameters(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			return
		}

		var sanitized strings.Builder
		sanitized.Grow(len(input))
		sanitizedLen := 0
		for _, char := range input {
			if char >= 32 && char <= 126 {
				switch char {
				case ' ', '\n', '\r', '\t':

					continue
				case '&':
					sanitized.WriteString("%26")
					sanitizedLen += 3
				case '=':
					sanitized.WriteString("%3D")
					sanitizedLen += 3
				case '?':
					sanitized.WriteString("%3F")
					sanitizedLen += 3
				case '#':
					sanitized.WriteString("%23")
					sanitizedLen += 3
				case '/':
					sanitized.WriteString("%2F")
					sanitizedLen += 3
				case '\\':
					sanitized.WriteString("%5C")
					sanitizedLen += 3
				default:
					sanitized.WriteRune(char)
					sanitizedLen++
				}
				if sanitizedLen >= 100 {
					break
				}
			}
		}

		if sanitizedLen == 0 {
			return
		}

		query := "?q=" + sanitized.String() + "&limit=10&offset=0"
		req := httptest.NewRequest("GET", "/api/posts/search"+query, nil)

		q := req.URL.Query().Get("q")
		limit := req.URL.Query().Get("limit")
		offset := req.URL.Query().Get("offset")

		if !utf8.ValidString(q) {

			return
		}
		_ = limit
		_ = offset
	})
}

func FuzzHTTPHeaders(f *testing.F) {
	helper := fuzz.NewFuzzTestHelper()
	helper.RunBasicFuzzTest(f, func(t *testing.T, input string) {
		req := httptest.NewRequest("GET", "/api/test", nil)

		req.Header.Set("Authorization", "Bearer "+input)
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("User-Agent", input)
		req.Header.Set("X-Forwarded-For", input)

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
	})
}
