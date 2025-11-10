package repositories

import (
	"regexp"
	"strings"
	"unicode"
)

type SearchSanitizer struct {
	MaxQueryLength  int
	MaxSpecialChars int
}

func NewSearchSanitizer() *SearchSanitizer {
	return &SearchSanitizer{
		MaxQueryLength:  100,
		MaxSpecialChars: 10,
	}
}

func (s *SearchSanitizer) SanitizeSearchQuery(query string) (string, error) {
	query = strings.TrimSpace(query)

	if query == "" {
		return "", nil
	}

	if len(query) > s.MaxQueryLength {
		return "", &SearchError{
			Type:    "QueryTooLong",
			Message: "Search query exceeds maximum length",
		}
	}

	specialCharCount := 0
	for _, char := range query {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && !unicode.IsSpace(char) {
			specialCharCount++
		}
	}

	if specialCharCount > s.MaxSpecialChars {
		return "", &SearchError{
			Type:    "TooManySpecialChars",
			Message: "Search query contains too many special characters",
		}
	}

	query = s.removeDangerousPatterns(query)

	query = s.normalizeWhitespace(query)

	if len(query) > s.MaxQueryLength {
		query = query[:s.MaxQueryLength]
	}

	return query, nil
}

func (s *SearchSanitizer) removeDangerousPatterns(query string) string {
	query = regexp.MustCompile(`\*{3,}`).ReplaceAllString(query, "**")
	query = regexp.MustCompile(`%{3,}`).ReplaceAllString(query, "%%")

	query = regexp.MustCompile(`\.{3,}`).ReplaceAllString(query, "..")

	query = regexp.MustCompile(`\?{3,}`).ReplaceAllString(query, "??")

	query = regexp.MustCompile(`\+{3,}`).ReplaceAllString(query, "++")

	query = regexp.MustCompile(`\{[^}]*\{[^}]*\}`).ReplaceAllString(query, "")

	query = regexp.MustCompile(`\[[^\]]*\[[^\]]*\]`).ReplaceAllString(query, "")

	query = regexp.MustCompile(`\([^)]*\([^)]*\)`).ReplaceAllString(query, "")

	return query
}

func (s *SearchSanitizer) normalizeWhitespace(query string) string {
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")

	query = strings.TrimSpace(query)

	return query
}

func (s *SearchSanitizer) ValidateSearchQuery(query string) error {
	if len(query) == 0 {
		return nil
	}

	sqlInjectionPatterns := []string{
		"';", "--", "/*", "*/", "xp_", "sp_", "exec", "execute",
		"union", "select", "insert", "update", "delete", "drop",
		"create", "alter", "grant", "revoke", "truncate",
	}

	lowerQuery := strings.ToLower(query)
	for _, pattern := range sqlInjectionPatterns {
		if strings.Contains(lowerQuery, pattern) {
			return &SearchError{
				Type:    "InvalidQuery",
				Message: "Search query contains potentially dangerous patterns",
			}
		}
	}

	if s.hasExcessiveRepetition(query) {
		return &SearchError{
			Type:    "DoSPattern",
			Message: "Search query contains patterns that could cause denial of service",
		}
	}

	return nil
}

func (s *SearchSanitizer) hasExcessiveRepetition(query string) bool {
	if s.hasRepeatedCharacters(query, 5) {
		return true
	}

	words := strings.Fields(query)
	wordCount := make(map[string]int)
	for _, word := range words {
		wordCount[strings.ToLower(word)]++
		if wordCount[strings.ToLower(word)] > 3 {
			return true
		}
	}

	return false
}

func (s *SearchSanitizer) hasRepeatedCharacters(str string, maxRepeats int) bool {
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

type SearchError struct {
	Type    string
	Message string
}

func (e *SearchError) Error() string {
	return e.Message
}
