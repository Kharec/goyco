package validation

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var EmailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
var UsernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,50}$`)
var URLRegex = regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)

func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return &ValidationError{Field: "email", Message: "Email is required"}
	}
	if len(email) > 254 {
		return &ValidationError{Field: "email", Message: "Email is too long"}
	}
	if !EmailRegex.MatchString(email) {
		return &ValidationError{Field: "email", Message: "Invalid email format"}
	}
	return nil
}

func ValidateUsername(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return &ValidationError{Field: "username", Message: "Username is required"}
	}
	if len(username) < 3 {
		return &ValidationError{Field: "username", Message: "Username must be at least 3 characters"}
	}
	if len(username) > 50 {
		return &ValidationError{Field: "username", Message: "Username must be at most 50 characters"}
	}
	if !UsernameRegex.MatchString(username) {
		return &ValidationError{Field: "username", Message: "Username can only contain letters, numbers, underscores, and hyphens"}
	}
	return nil
}

func ValidatePassword(password string) error {
	if password == "" {
		return &ValidationError{Field: "password", Message: "Password is required"}
	}
	if len(password) < 8 {
		return &ValidationError{Field: "password", Message: "Password must be at least 8 characters long"}
	}
	if len(password) > 128 {
		return &ValidationError{Field: "password", Message: "Password must be no more than 128 characters long"}
	}

	hasLetter := false
	for _, char := range password {
		if unicode.IsLetter(char) {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return &ValidationError{Field: "password", Message: "Password must contain at least one letter"}
	}

	hasNumber := false
	for _, char := range password {
		if unicode.IsNumber(char) {
			hasNumber = true
			break
		}
	}
	if !hasNumber {
		return &ValidationError{Field: "password", Message: "Password must contain at least one number"}
	}

	hasSpecial := false
	specialChars := "!@#$%^&*()_+-=[]{}|\\:\";'<>?,./~`"
	for _, char := range password {
		if strings.ContainsRune(specialChars, char) {
			hasSpecial = true
			break
		}
	}
	if !hasSpecial {
		return &ValidationError{Field: "password", Message: "Password must contain at least one special character"}
	}

	return nil
}

func ValidateTitle(title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return &ValidationError{Field: "title", Message: "Title is required"}
	}
	if len(title) < 3 {
		return &ValidationError{Field: "title", Message: "Title must be at least 3 characters"}
	}
	if len(title) > 200 {
		return &ValidationError{Field: "title", Message: "Title must be at most 200 characters"}
	}
	return nil
}

func ValidateContent(content string) error {
	content = strings.TrimSpace(content)
	if len(content) > 10000 {
		return &ValidationError{Field: "content", Message: "Content must be at most 10000 characters"}
	}
	return nil
}

func ValidateURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return &ValidationError{Field: "url", Message: "URL is required"}
	}
	if len(url) > 2048 {
		return &ValidationError{Field: "url", Message: "URL is too long"}
	}
	if !URLRegex.MatchString(url) {
		return &ValidationError{Field: "url", Message: "Invalid URL format. Only HTTP and HTTPS URLs are allowed"}
	}
	return nil
}

func ValidateSearchQuery(query string) error {
	query = strings.TrimSpace(query)
	if len(query) > 100 {
		return &ValidationError{Field: "query", Message: "Search query is too long"}
	}
	return nil
}

func SanitizeString(input string) string {
	input = strings.ReplaceAll(input, "\x00", "")
	input = strings.ReplaceAll(input, "\r", "")
	input = strings.ReplaceAll(input, "\n", "")
	input = strings.ReplaceAll(input, "\t", "")
	return strings.TrimSpace(input)
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

type StructValidationError struct {
	Errors []FieldValidationError
}

func (e *StructValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	messages := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		messages[i] = err.Message
	}
	return strings.Join(messages, "; ")
}

type FieldValidationError struct {
	Field   string
	Tag     string
	Param   string
	Message string
}

func getFieldDisplayName(field reflect.StructField) string {
	jsonTag := field.Tag.Get("json")
	if jsonTag != "" && jsonTag != "-" {
		if idx := strings.Index(jsonTag, ","); idx != -1 {
			jsonTag = jsonTag[:idx]
		}
		if jsonTag != "" {
			return jsonTag
		}
	}
	return camelCaseToWords(field.Name)
}

func camelCaseToWords(s string) string {
	if s == "" {
		return s
	}
	var result strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune(' ')
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}

func ValidateStruct(s interface{}) error {
	if s == nil {
		return nil
	}

	val := reflect.ValueOf(s)
	typ := reflect.TypeOf(s)

	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
		typ = typ.Elem()
	}

	if val.Kind() != reflect.Struct {
		return fmt.Errorf("ValidateStruct: expected struct, got %s", val.Kind())
	}

	var errors []FieldValidationError

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !fieldVal.CanInterface() {
			continue
		}

		validateTag := field.Tag.Get("validate")
		if validateTag == "" {
			continue
		}

		tags := strings.Split(validateTag, ",")
		omitempty := false

		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}

			if tag == "omitempty" {
				omitempty = true
				continue
			}

			var tagName, param string
			if idx := strings.Index(tag, "="); idx != -1 {
				tagName = tag[:idx]
				param = tag[idx+1:]
			} else {
				tagName = tag
			}

			displayName := getFieldDisplayName(field)
			if err := validateField(displayName, fieldVal, tagName, param, omitempty); err != nil {
				errors = append(errors, FieldValidationError{
					Field:   displayName,
					Tag:     tagName,
					Param:   param,
					Message: err.Message,
				})
				if tagName == "required" {
					break
				}
			}
		}
	}

	if len(errors) > 0 {
		return &StructValidationError{Errors: errors}
	}

	return nil
}

func validateField(fieldName string, fieldVal reflect.Value, tagName, param string, omitempty bool) *ValidationError {
	if omitempty && isEmptyValue(fieldVal) {
		return nil
	}

	switch tagName {
	case "required":
		if isEmptyValue(fieldVal) {
			return &ValidationError{Field: fieldName, Message: fieldName + " is required"}
		}
	case "min":
		if err := validateMin(fieldName, fieldVal, param); err != nil {
			return err
		}
	case "max":
		if err := validateMax(fieldName, fieldVal, param); err != nil {
			return err
		}
	case "email":
		if err := validateEmailTag(fieldName, fieldVal); err != nil {
			return err
		}
	case "url":
		if err := validateURLTag(fieldName, fieldVal); err != nil {
			return err
		}
	case "oneof":
		if err := validateOneOf(fieldName, fieldVal, param); err != nil {
			return err
		}
	default:
		return &ValidationError{Field: fieldName, Message: fieldName + " is invalid"}
	}

	return nil
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return strings.TrimSpace(v.String()) == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Bool:
		return false
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map:
		return v.IsNil()
	default:
		return false
	}
}

func validateMin(fieldName string, v reflect.Value, param string) *ValidationError {
	min, err := strconv.Atoi(param)
	if err != nil {
		return &ValidationError{Field: fieldName, Message: fieldName + " has invalid min parameter"}
	}

	switch v.Kind() {
	case reflect.String:
		s := strings.TrimSpace(v.String())
		if len([]rune(s)) < min {
			return &ValidationError{Field: fieldName, Message: fmt.Sprintf("%s must be at least %d characters", fieldName, min)}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Int() < int64(min) {
			return &ValidationError{Field: fieldName, Message: fmt.Sprintf("%s must be at least %d", fieldName, min)}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if min < 0 {
			return &ValidationError{Field: fieldName, Message: fieldName + " has invalid min parameter (must be non-negative)"}
		}
		if v.Uint() < uint64(min) {
			return &ValidationError{Field: fieldName, Message: fmt.Sprintf("%s must be at least %d", fieldName, min)}
		}
	}

	return nil
}

func validateMax(fieldName string, v reflect.Value, param string) *ValidationError {
	max, err := strconv.Atoi(param)
	if err != nil {
		return &ValidationError{Field: fieldName, Message: fieldName + " has invalid max parameter"}
	}

	switch v.Kind() {
	case reflect.String:
		if len([]rune(v.String())) > max {
			return &ValidationError{Field: fieldName, Message: fmt.Sprintf("%s must be at most %d characters", fieldName, max)}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Int() > int64(max) {
			return &ValidationError{Field: fieldName, Message: fmt.Sprintf("%s must be at most %d", fieldName, max)}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if max < 0 {
			return &ValidationError{Field: fieldName, Message: fieldName + " has invalid max parameter (must be non-negative)"}
		}
		if v.Uint() > uint64(max) {
			return &ValidationError{Field: fieldName, Message: fmt.Sprintf("%s must be at most %d", fieldName, max)}
		}
	}

	return nil
}

func validateEmailTag(fieldName string, v reflect.Value) *ValidationError {
	if v.Kind() != reflect.String {
		return nil
	}

	email := strings.TrimSpace(v.String())
	if email == "" {
		return nil
	}

	if !EmailRegex.MatchString(email) {
		return &ValidationError{Field: fieldName, Message: fieldName + " must be a valid email address"}
	}

	return nil
}

func validateURLTag(fieldName string, v reflect.Value) *ValidationError {
	if v.Kind() != reflect.String {
		return nil
	}

	url := strings.TrimSpace(v.String())
	if url == "" {
		return nil
	}

	if len(url) > 2048 {
		return &ValidationError{Field: fieldName, Message: fieldName + " must be at most 2048 characters"}
	}

	if !URLRegex.MatchString(url) {
		return &ValidationError{Field: fieldName, Message: fieldName + " must be a valid URL"}
	}

	return nil
}

func validateOneOf(fieldName string, v reflect.Value, param string) *ValidationError {
	if v.Kind() != reflect.String {
		return nil
	}

	value := v.String()
	allowedValues := strings.Split(param, " ")
	allowedMap := make(map[string]bool, len(allowedValues))

	for _, allowed := range allowedValues {
		allowedMap[allowed] = true
	}

	if !allowedMap[value] {
		return &ValidationError{Field: fieldName, Message: fieldName + " must be one of: " + param}
	}

	return nil
}
