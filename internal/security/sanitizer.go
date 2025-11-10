package security

import (
	"fmt"
	"html"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

func SanitizeInput(input string) string {

	if !utf8.ValidString(input) {
		input = strings.ToValidUTF8(input, "")
	}

	cleaned := strings.TrimSpace(input)

	cleaned = html.EscapeString(cleaned)

	scriptRegex := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	cleaned = scriptRegex.ReplaceAllString(cleaned, "")

	jsRegex := regexp.MustCompile(`(?i)javascript:`)
	cleaned = jsRegex.ReplaceAllString(cleaned, "")

	eventRegex := regexp.MustCompile(`(?i)\son\w+\s*=\s*"[^"]*"`)
	cleaned = eventRegex.ReplaceAllString(cleaned, "")

	return cleaned
}

func SanitizeUsername(username string) string {
	cleaned := SanitizeInput(username)

	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	cleaned = reg.ReplaceAllString(cleaned, "")

	if len(cleaned) > 0 && !unicode.IsLetter(rune(cleaned[0])) && !unicode.IsDigit(rune(cleaned[0])) {
		cleaned = "user_" + cleaned
	}

	return cleaned
}

func SanitizeEmail(email string) string {
	cleaned := SanitizeInput(email)

	cleaned = strings.ToLower(cleaned)

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(cleaned) {
		return ""
	}

	return cleaned
}

func SanitizePostContent(content string) string {

	if !utf8.ValidString(content) {
		content = strings.ToValidUTF8(content, "")
	}

	cleaned := strings.TrimSpace(content)
	cleaned = html.EscapeString(cleaned)

	scriptRegex := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	cleaned = scriptRegex.ReplaceAllString(cleaned, "")

	jsRegex := regexp.MustCompile(`(?i)javascript:`)
	cleaned = jsRegex.ReplaceAllString(cleaned, "")

	eventRegex := regexp.MustCompile(`(?i)\son\w+\s*=\s*"[^"]*"`)
	cleaned = eventRegex.ReplaceAllString(cleaned, "")

	return cleaned
}

func SanitizeURL(url string) string {

	if !utf8.ValidString(url) {
		url = strings.ToValidUTF8(url, "")
	}

	cleaned := strings.TrimSpace(url)

	if !strings.HasPrefix(cleaned, "http://") && !strings.HasPrefix(cleaned, "https://") {
		return ""
	}

	privateIPRegex := regexp.MustCompile(`(?i)(localhost|127\.0\.0\.1|0\.0\.0\.0|10\.|172\.(1[6-9]|2[0-9]|3[0-1])\.|192\.168\.)`)
	if privateIPRegex.MatchString(cleaned) {
		return ""
	}

	if strings.Contains(cleaned, "169.254.169.254") {
		return ""
	}

	return cleaned
}

type InputSanitizer struct {
	MaxUsernameLength int
	MaxEmailLength    int
	MaxTitleLength    int
	MaxContentLength  int
	MaxSearchLength   int
}

func NewInputSanitizer() *InputSanitizer {
	return &InputSanitizer{
		MaxUsernameLength: 50,
		MaxEmailLength:    100,
		MaxTitleLength:    200,
		MaxContentLength:  5000,
		MaxSearchLength:   100,
	}
}

func (s *InputSanitizer) SanitizeUsernameCLI(username string) (string, error) {
	if username == "" {
		return "", fmt.Errorf("username cannot be empty")
	}

	username = strings.TrimSpace(username)

	if len(username) > s.MaxUsernameLength {
		return "", fmt.Errorf("username must be %d characters or less", s.MaxUsernameLength)
	}

	if len(username) < 3 {
		return "", fmt.Errorf("username must be at least 3 characters")
	}

	validUsernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validUsernameRegex.MatchString(username) {
		return "", fmt.Errorf("username can only contain letters, numbers, underscores, and hyphens")
	}

	reservedUsernames := []string{
		"admin", "administrator", "root", "system", "api", "www", "mail",
		"ftp", "localhost", "test", "demo", "guest", "user", "support",
		"help", "info", "contact", "about", "privacy", "terms", "login",
		"register", "signup", "signin", "logout", "profile", "settings",
		"account", "dashboard", "home", "index", "default", "null", "undefined",
	}

	lowerUsername := strings.ToLower(username)
	if slices.Contains(reservedUsernames, lowerUsername) {
		return "", fmt.Errorf("username '%s' is reserved and cannot be used", username)
	}

	if strings.Contains(username, "__") || strings.Contains(username, "--") {
		return "", fmt.Errorf("username cannot contain consecutive underscores or hyphens")
	}

	if strings.HasPrefix(username, "_") || strings.HasPrefix(username, "-") ||
		strings.HasSuffix(username, "_") || strings.HasSuffix(username, "-") {
		return "", fmt.Errorf("username cannot start or end with underscore or hyphen")
	}

	return username, nil
}

func (s *InputSanitizer) SanitizeEmailCLI(email string) (string, error) {
	if email == "" {
		return "", fmt.Errorf("email cannot be empty")
	}

	email = strings.TrimSpace(strings.ToLower(email))

	if len(email) > s.MaxEmailLength {
		return "", fmt.Errorf("email must be %d characters or less", s.MaxEmailLength)
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return "", fmt.Errorf("invalid email format")
	}

	suspiciousPatterns := []string{
		"..", "++", "--", "__", "%%",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(email, pattern) {
			return "", fmt.Errorf("email contains invalid characters")
		}
	}

	dotCount := strings.Count(email, ".")
	if dotCount > 3 {
		return "", fmt.Errorf("email contains too many dots")
	}

	return email, nil
}

func (s *InputSanitizer) SanitizePasswordCLI(password string) error {
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	if len(password) > 128 {
		return fmt.Errorf("password must be 128 characters or less")
	}

	weakPasswords := []string{
		"password", "123456", "12345678", "qwerty", "abc123",
		"password123", "admin", "letmein", "welcome", "monkey",
		"dragon", "master", "hello", "login", "princess",
	}

	lowerPassword := strings.ToLower(password)
	if slices.Contains(weakPasswords, lowerPassword) {
		return fmt.Errorf("password is too common, please choose a stronger password")
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	countBools := func(bools ...bool) int {
		count := 0
		for _, b := range bools {
			if b {
				count++
			}
		}
		return count
	}

	if countBools(hasUpper, hasLower, hasDigit, hasSpecial) < 3 {
		return fmt.Errorf("password must contain at least 3 different character types (uppercase, lowercase, digits, special characters)")
	}

	return nil
}

func (s *InputSanitizer) SanitizeSearchTerm(term string) (string, error) {
	if term == "" {
		return "", nil
	}

	if !utf8.ValidString(term) {
		term = strings.ToValidUTF8(term, "")
	}

	term = strings.TrimSpace(term)

	if len(term) > s.MaxSearchLength {
		return "", fmt.Errorf("search term must be %d characters or less", s.MaxSearchLength)
	}

	dangerousChars := []string{
		"<", ">", "\"", "'", "&", "|", ";", "`", "$", "(", ")", "{", "}",
		"[", "]", "\\", "/", "*", "?", "!", "@", "#", "%", "^", "~",
	}

	for _, char := range dangerousChars {
		term = strings.ReplaceAll(term, char, "")
	}

	term = regexp.MustCompile(`\s+`).ReplaceAllString(term, " ")

	if s.hasExcessiveRepetition(term) {
		return "", fmt.Errorf("search term contains excessive repetition")
	}

	return term, nil
}

func (s *InputSanitizer) SanitizeTitleCLI(title string) (string, error) {
	if title == "" {
		return "", fmt.Errorf("title cannot be empty")
	}

	if !utf8.ValidString(title) {
		title = strings.ToValidUTF8(title, "")
	}

	title = strings.TrimSpace(title)

	if len(title) > s.MaxTitleLength {
		return "", fmt.Errorf("title must be %d characters or less", s.MaxTitleLength)
	}

	if len(title) < 3 {
		return "", fmt.Errorf("title must be at least 3 characters")
	}

	htmlTagRegex := regexp.MustCompile(`<[^>]*>`)
	title = htmlTagRegex.ReplaceAllString(title, "")

	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")

	return title, nil
}

func (s *InputSanitizer) SanitizeContentCLI(content string) (string, error) {
	if content == "" {
		return "", fmt.Errorf("content cannot be empty")
	}

	if !utf8.ValidString(content) {
		content = strings.ToValidUTF8(content, "")
	}

	content = strings.TrimSpace(content)

	if len(content) > s.MaxContentLength {
		return "", fmt.Errorf("content must be %d characters or less", s.MaxContentLength)
	}

	if len(content) < 10 {
		return "", fmt.Errorf("content must be at least 10 characters")
	}

	dangerousTags := []string{
		"<script", "</script>", "<iframe", "</iframe>", "<object", "</object>",
		"<embed", "</embed>", "<form", "</form>", "<input", "<button",
		"<link", "<meta", "<style", "</style>",
	}

	for _, tag := range dangerousTags {
		content = regexp.MustCompile(`(?i)`+regexp.QuoteMeta(tag)).ReplaceAllString(content, "")
	}

	content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")

	return content, nil
}

func (s *InputSanitizer) hasExcessiveRepetition(text string) bool {
	if s.hasRepeatedCharacters(text, 5) {
		return true
	}

	words := strings.Fields(text)
	wordCount := make(map[string]int)
	for _, word := range words {
		wordCount[strings.ToLower(word)]++
		if wordCount[strings.ToLower(word)] > 3 {
			return true
		}
	}

	return false
}

func (s *InputSanitizer) hasRepeatedCharacters(str string, maxRepeats int) bool {
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

func (s *InputSanitizer) SanitizeID(idStr string) (uint, error) {
	if idStr == "" {
		return 0, fmt.Errorf("ID cannot be empty")
	}

	idStr = strings.TrimSpace(idStr)

	if !regexp.MustCompile(`^\d+$`).MatchString(idStr) {
		return 0, fmt.Errorf("ID must be a positive integer")
	}

	var id uint
	_, err := fmt.Sscanf(idStr, "%d", &id)
	if err != nil {
		return 0, fmt.Errorf("invalid ID format: %s", idStr)
	}

	if id == 0 {
		return 0, fmt.Errorf("ID must be greater than 0")
	}

	if id > 1000000 {
		return 0, fmt.Errorf("ID is too large")
	}

	return id, nil
}
