package testutils

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func AssertHTTPStatus(t *testing.T, rr *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if rr.Code != expected {
		t.Errorf("Expected status %d, got %d. Body: %s", expected, rr.Code, rr.Body.String())
	}
}

func AssertJSONResponse(t *testing.T, rr *httptest.ResponseRecorder, expected any) {
	t.Helper()
	var actual any
	if err := json.NewDecoder(rr.Body).Decode(&actual); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}
	assert.Equal(t, expected, actual)
}

func AssertJSONField(t *testing.T, rr *httptest.ResponseRecorder, fieldPath string, expected any) {
	t.Helper()
	var response map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	actual := getNestedField(response, fieldPath)
	assert.Equal(t, expected, actual)
}

func AssertJSONContains(t *testing.T, rr *httptest.ResponseRecorder, expectedFields map[string]any) {
	t.Helper()
	var response map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	for field, expectedValue := range expectedFields {
		actual := getNestedField(response, field)
		assert.Equal(t, expectedValue, actual, "Field %s mismatch", field)
	}
}

func AssertErrorResponse(t *testing.T, rr *httptest.ResponseRecorder, expectedStatus int, expectedError string) {
	t.Helper()
	AssertHTTPStatus(t, rr, expectedStatus)

	var response map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if errorMsg, ok := response["error"].(string); ok {
		assert.Contains(t, errorMsg, expectedError)
	} else {
		t.Errorf("Expected error message in response, got: %v", response)
	}
}

func AssertSuccessResponse(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	AssertHTTPStatus(t, rr, 200)

	var response map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if success, ok := response["success"].(bool); ok {
		assert.True(t, success, "Expected success: true")
	}
}

func AssertHeader(t *testing.T, rr *httptest.ResponseRecorder, headerName, expectedValue string) {
	t.Helper()
	actual := rr.Header().Get(headerName)
	assert.Equal(t, expectedValue, actual, "Header %s mismatch", headerName)
}

func AssertHeaderContains(t *testing.T, rr *httptest.ResponseRecorder, headerName, expectedValue string) {
	t.Helper()
	actual := rr.Header().Get(headerName)
	assert.Contains(t, actual, expectedValue, "Header %s should contain %s", headerName, expectedValue)
}

func AssertWithinTimeRange(t *testing.T, actual, expected time.Time, tolerance time.Duration) {
	t.Helper()
	diff := actual.Sub(expected)
	if diff < -tolerance || diff > tolerance {
		t.Errorf("Time %v is not within %v of expected %v", actual, tolerance, expected)
	}
}

func getNestedField(data map[string]any, path string) any {
	keys := splitPath(path)
	current := data

	for i, key := range keys {
		if i == len(keys)-1 {
			return current[key]
		}

		if next, ok := current[key].(map[string]any); ok {
			current = next
		} else {
			return nil
		}
	}

	return nil
}

func splitPath(path string) []string {
	var keys []string
	var current string

	for _, char := range path {
		if char == '.' {
			keys = append(keys, current)
			current = ""
		} else {
			current += string(char)
		}
	}

	if current != "" {
		keys = append(keys, current)
	}

	return keys
}
