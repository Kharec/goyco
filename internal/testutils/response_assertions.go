package testutils

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func AssertStatusCode(t TestingT, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		var bodyPreview string
		if resp.Body != nil {
			bodyBytes := make([]byte, 512)
			n, _ := resp.Body.Read(bodyBytes)
			bodyPreview = string(bodyBytes[:n])
			if seeker, ok := resp.Body.(io.Seeker); ok {
				seeker.Seek(0, io.SeekStart)
			}
		}
		t.Errorf("Expected status code %d, got %d. Response preview: %s", expected, resp.StatusCode, bodyPreview)
	}
}

func AssertStatusCodeFatal(t TestingT, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		var bodyPreview string
		if resp.Body != nil {
			bodyBytes := make([]byte, 512)
			n, _ := resp.Body.Read(bodyBytes)
			bodyPreview = string(bodyBytes[:n])
		}
		t.Fatalf("Expected status code %d, got %d. Response preview: %s", expected, resp.StatusCode, bodyPreview)
	}
}

func AssertE2EJSONResponse(t TestingT, resp *http.Response) (*APIResponse, error) {
	t.Helper()
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}
	var apiResp APIResponse
	if err := json.NewDecoder(reader).Decode(&apiResp); err != nil {
		if resp.Body != nil {
			bodyBytes := make([]byte, 1024)
			n, _ := resp.Body.Read(bodyBytes)
			return nil, fmt.Errorf("failed to decode JSON response: %w. Body preview: %s", err, string(bodyBytes[:n]))
		}
		return nil, fmt.Errorf("failed to decode JSON response: %w", err)
	}
	return &apiResp, nil
}

func AssertE2ESuccessResponse(t TestingT, resp *http.Response, expectedStatus int) {
	t.Helper()
	AssertStatusCode(t, resp, expectedStatus)
	apiResp, err := AssertE2EJSONResponse(t, resp)
	if err != nil {
		t.Errorf("Failed to decode JSON response: %v", err)
		return
	}
	if !apiResp.Success {
		t.Errorf("Expected response to indicate success (success: true), got success: false. Message: %s", apiResp.Message)
		if apiResp.Data != nil {
			t.Errorf("Response data: %v", apiResp.Data)
		}
	}
}

func AssertE2ESuccessResponseFatal(t TestingT, resp *http.Response, expectedStatus int) {
	t.Helper()
	if resp.StatusCode != expectedStatus {
		var bodyPreview string
		if resp.Body != nil {
			bodyBytes := make([]byte, 512)
			n, _ := resp.Body.Read(bodyBytes)
			bodyPreview = string(bodyBytes[:n])
		}
		t.Fatalf("Expected status code %d, got %d. Response preview: %s", expectedStatus, resp.StatusCode, bodyPreview)
	}
	apiResp, err := AssertE2EJSONResponse(t, resp)
	if err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}
	if !apiResp.Success {
		t.Fatalf("Expected response to indicate success (success: true), got success: false. Message: %s", apiResp.Message)
	}
}

func AssertE2EErrorResponse(t TestingT, resp *http.Response, expectedStatus int, errorPattern string) {
	t.Helper()
	if resp.StatusCode < 400 {
		t.Errorf("Expected error status code (4xx or 5xx), got %d", resp.StatusCode)
	}
	if expectedStatus > 0 && resp.StatusCode != expectedStatus {
		t.Errorf("Expected error status code %d, got %d", expectedStatus, resp.StatusCode)
	}
	apiResp, err := AssertE2EJSONResponse(t, resp)
	if err != nil {
		return
	}
	if apiResp.Success {
		t.Errorf("Expected error response (success: false), got success: true")
	}
	if errorPattern != "" {
		var errorMsg string
		if errorField, ok := getErrorField(apiResp); ok {
			errorMsg = errorField
		} else if apiResp.Message != "" {
			errorMsg = apiResp.Message
		}
		if errorMsg == "" {
			t.Errorf("Expected error message containing '%s', but no error message found in response", errorPattern)
		} else if !strings.Contains(strings.ToLower(errorMsg), strings.ToLower(errorPattern)) {
			t.Errorf("Expected error message to contain '%s', got: %s", errorPattern, errorMsg)
		}
	}
}

func AssertE2EErrorResponseFatal(t TestingT, resp *http.Response, expectedStatus int, errorPattern string) {
	t.Helper()
	if expectedStatus > 0 && resp.StatusCode != expectedStatus {
		var bodyPreview string
		if resp.Body != nil {
			bodyBytes := make([]byte, 512)
			n, _ := resp.Body.Read(bodyBytes)
			bodyPreview = string(bodyBytes[:n])
		}
		t.Fatalf("Expected error status code %d, got %d. Response preview: %s", expectedStatus, resp.StatusCode, bodyPreview)
	}
	apiResp, err := AssertE2EJSONResponse(t, resp)
	if err != nil {
		return
	}
	if apiResp.Success {
		t.Fatalf("Expected error response (success: false), got success: true")
	}
	if errorPattern != "" {
		var errorMsg string
		if errorField, ok := getErrorField(apiResp); ok {
			errorMsg = errorField
		} else if apiResp.Message != "" {
			errorMsg = apiResp.Message
		}
		if errorMsg == "" {
			t.Fatalf("Expected error message containing '%s', but no error message found in response", errorPattern)
		} else if !strings.Contains(strings.ToLower(errorMsg), strings.ToLower(errorPattern)) {
			t.Fatalf("Expected error message to contain '%s', got: %s", errorPattern, errorMsg)
		}
	}
}

func getErrorField(resp *APIResponse) (string, bool) {
	if resp == nil {
		return "", false
	}
	if dataMap, ok := resp.Data.(map[string]any); ok {
		if errorVal, ok := dataMap["error"].(string); ok {
			return errorVal, true
		}
	}
	if resp.Message != "" {
		return resp.Message, true
	}
	return "", false
}

func ReadResponseBody(t TestingT, resp *http.Response) (string, error) {
	t.Helper()
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}
	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	return string(bodyBytes), nil
}
