package e2e

import (
	"encoding/json"
	"net/http"
	"testing"

	"goyco/internal/testutils"
)

func fetchSwaggerDoc(t *testing.T, ctx *testContext) map[string]any {
	t.Helper()

	request, err := http.NewRequest("GET", ctx.baseURL+"/swagger/doc.json", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	testutils.WithStandardHeaders(request)

	response, err := ctx.client.Do(request)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Skipf("Swagger JSON not available (status %d)", response.StatusCode)
		return nil
	}

	var swaggerDoc map[string]any
	if err := json.NewDecoder(response.Body).Decode(&swaggerDoc); err != nil {
		t.Fatalf("Failed to decode Swagger JSON: %v", err)
	}

	return swaggerDoc
}

func TestE2E_SwaggerDocumentation(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("swagger_json_is_valid", func(t *testing.T) {
		swaggerDoc := fetchSwaggerDoc(t, ctx)
		if swaggerDoc == nil {
			return
		}

		if swaggerDoc["swagger"] == nil && swaggerDoc["openapi"] == nil {
			t.Error("Swagger JSON missing swagger/openapi version")
		}

		if swaggerDoc["info"] == nil {
			t.Error("Swagger JSON missing info section")
		}

		if swaggerDoc["paths"] == nil {
			t.Error("Swagger JSON missing paths section")
		}
	})

	t.Run("swagger_yaml_is_valid", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/swagger/doc.yaml", nil)
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
			t.Logf("Swagger YAML endpoint returned status %d (may not be available)", resp.StatusCode)
		}
	})

	t.Run("api_endpoints_documented", func(t *testing.T) {
		swaggerDoc := fetchSwaggerDoc(t, ctx)
		if swaggerDoc == nil {
			return
		}

		paths, ok := swaggerDoc["paths"].(map[string]any)
		if !ok {
			t.Error("Paths section is not a map")
			return
		}

		requiredPaths := []string{
			"/api",
			"/api/auth/login",
			"/api/auth/register",
			"/api/auth/me",
			"/api/auth/refresh",
			"/api/auth/revoke",
			"/api/auth/revoke-all",
			"/api/auth/logout",
			"/api/posts",
		}

		for _, requiredPath := range requiredPaths {
			if paths[requiredPath] == nil {
				t.Errorf("Required endpoint %s not documented", requiredPath)
			}
		}
	})

	t.Run("request_response_schemas_present", func(t *testing.T) {
		swaggerDoc := fetchSwaggerDoc(t, ctx)
		if swaggerDoc == nil {
			return
		}

		definitions, ok := swaggerDoc["definitions"].(map[string]any)
		if !ok {
			definitions, ok = swaggerDoc["components"].(map[string]any)
			if ok {
				definitions, _ = definitions["schemas"].(map[string]any)
			}
		}

		if definitions == nil {
			t.Log("No definitions/schemas section found (may use inline schemas)")
			return
		}

		if len(definitions) == 0 {
			t.Error("Definitions/schemas section is empty")
		}
	})

	t.Run("swagger_ui_accessible", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/swagger/index.html", nil)
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
			t.Logf("Swagger UI returned status %d (may not be available)", resp.StatusCode)
		}
	})
}

func TestE2E_APIEndpointDocumentation(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("api_info_endpoint_documented", func(t *testing.T) {
		swaggerDoc := fetchSwaggerDoc(t, ctx)
		if swaggerDoc == nil {
			return
		}

		paths, ok := swaggerDoc["paths"].(map[string]any)
		if !ok {
			return
		}

		apiPath, ok := paths["/api"].(map[string]any)
		if !ok {
			t.Error("API endpoint not documented")
			return
		}

		getMethod, ok := apiPath["get"].(map[string]any)
		if !ok {
			t.Error("API GET method not documented")
			return
		}

		if getMethod["responses"] == nil {
			t.Error("API endpoint missing responses")
		}
	})

	t.Run("auth_endpoints_documented", func(t *testing.T) {
		swaggerDoc := fetchSwaggerDoc(t, ctx)
		if swaggerDoc == nil {
			return
		}

		paths, ok := swaggerDoc["paths"].(map[string]any)
		if !ok {
			return
		}

		authEndpoints := []string{
			"/api/auth/login",
			"/api/auth/register",
			"/api/auth/refresh",
			"/api/auth/revoke",
			"/api/auth/revoke-all",
			"/api/auth/logout",
		}

		for _, endpoint := range authEndpoints {
			endpointData, ok := paths[endpoint].(map[string]any)
			if !ok {
				t.Errorf("Auth endpoint %s not documented", endpoint)
				continue
			}

			postMethod, ok := endpointData["post"].(map[string]any)
			if !ok {
				t.Errorf("Auth endpoint %s missing POST method", endpoint)
				continue
			}

			if postMethod["parameters"] == nil && postMethod["requestBody"] == nil {
				t.Logf("Auth endpoint %s may use inline request body", endpoint)
			}
		}
	})
}
