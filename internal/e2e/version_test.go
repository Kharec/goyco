package e2e

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"

	"goyco/internal/testutils"
)

func TestE2E_VersionEndpoint(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("version_in_api_info", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/api", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 for /api endpoint, got %d", resp.StatusCode)
			return
		}

		var apiInfo map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&apiInfo); err != nil {
			t.Fatalf("Failed to decode API info response: %v", err)
		}

		data, ok := apiInfo["data"].(map[string]any)
		if !ok {
			t.Fatalf("API info data is not a map")
		}

		version, ok := data["version"].(string)
		if !ok {
			t.Error("Version field missing or not a string in API info")
			return
		}

		if version == "" {
			t.Error("Version is empty")
		}

		versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+`)
		if !versionPattern.MatchString(version) {
			t.Errorf("Version format invalid, expected semantic version (x.y.z), got: %s", version)
		}
	})

	t.Run("version_in_health_endpoint", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/health", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 for /health endpoint, got %d", resp.StatusCode)
			return
		}

		var healthInfo map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&healthInfo); err != nil {
			t.Fatalf("Failed to decode health response: %v", err)
		}

		data, ok := healthInfo["data"].(map[string]any)
		if !ok {
			t.Fatalf("Health data is not a map")
		}

		version, ok := data["version"].(string)
		if !ok {
			t.Error("Version field missing or not a string in health info")
			return
		}

		if version == "" {
			t.Error("Version is empty")
		}

		versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+`)
		if !versionPattern.MatchString(version) {
			t.Errorf("Version format invalid, expected semantic version (x.y.z), got: %s", version)
		}
	})

	t.Run("version_consistency", func(t *testing.T) {
		apiReq, err := http.NewRequest("GET", ctx.baseURL+"/api", nil)
		if err != nil {
			t.Fatalf("Failed to create API request: %v", err)
		}
		testutils.WithStandardHeaders(apiReq)

		apiResp, err := ctx.client.Do(apiReq)
		if err != nil {
			t.Fatalf("API request failed: %v", err)
		}
		defer apiResp.Body.Close()

		healthReq, err := http.NewRequest("GET", ctx.baseURL+"/health", nil)
		if err != nil {
			t.Fatalf("Failed to create health request: %v", err)
		}
		testutils.WithStandardHeaders(healthReq)

		healthResp, err := ctx.client.Do(healthReq)
		if err != nil {
			t.Fatalf("Health request failed: %v", err)
		}
		defer healthResp.Body.Close()

		if apiResp.StatusCode != http.StatusOK || healthResp.StatusCode != http.StatusOK {
			t.Skip("One or both endpoints unavailable")
			return
		}

		var apiInfo map[string]any
		if err := json.NewDecoder(apiResp.Body).Decode(&apiInfo); err != nil {
			t.Fatalf("Failed to decode API info: %v", err)
		}

		var healthInfo map[string]any
		if err := json.NewDecoder(healthResp.Body).Decode(&healthInfo); err != nil {
			t.Fatalf("Failed to decode health info: %v", err)
		}

		apiData, _ := apiInfo["data"].(map[string]any)
		healthData, _ := healthInfo["data"].(map[string]any)

		apiVersion, apiOk := apiData["version"].(string)
		healthVersion, healthOk := healthData["version"].(string)

		if apiOk && healthOk && apiVersion != healthVersion {
			t.Errorf("Version mismatch: /api has %s, /health has %s", apiVersion, healthVersion)
		}
	})

	t.Run("version_format_validation", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/api", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Skip("API endpoint unavailable")
			return
		}

		var apiInfo map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&apiInfo); err != nil {
			t.Fatalf("Failed to decode API info: %v", err)
		}

		data, ok := apiInfo["data"].(map[string]any)
		if !ok {
			return
		}

		version, ok := data["version"].(string)
		if !ok || version == "" {
			return
		}

		semverPattern := regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9-]+)?(\+[a-zA-Z0-9-]+)?$`)
		if !semverPattern.MatchString(version) {
			t.Logf("Version '%s' does not strictly follow semantic versioning (acceptable)", version)
		}
	})
}
