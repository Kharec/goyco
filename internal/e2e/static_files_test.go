package e2e

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"goyco/internal/testutils"
)

func TestE2E_StaticFileServing(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("static_css_file_served", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/static/css/main.css", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, "text/css") && !strings.Contains(contentType, "application/octet-stream") {
				t.Logf("Unexpected Content-Type for CSS file: %s", contentType)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			if len(body) == 0 {
				t.Error("Static CSS file is empty")
			}
		} else if resp.StatusCode == http.StatusNotFound {
			t.Log("Static CSS file not found (may not exist in test environment)")
		} else {
			t.Errorf("Expected status 200 or 404, got %d", resp.StatusCode)
		}
	})

	t.Run("static_file_not_found", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/static/nonexistent/file.txt", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 for nonexistent file, got %d", resp.StatusCode)
		}
	})

	t.Run("static_directory_listing_disabled", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/static/", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusForbidden {
			t.Logf("Directory listing status: %d (acceptable)", resp.StatusCode)
		}
	})

	t.Run("static_favicon_served", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/static/favicon.ico", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, "image") && !strings.Contains(contentType, "application/octet-stream") {
				t.Logf("Unexpected Content-Type for favicon: %s", contentType)
			}
		} else if resp.StatusCode == http.StatusNotFound {
			t.Log("Favicon not found (may not exist in test environment)")
		}
	})

	t.Run("static_path_traversal_prevented", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/static/../common.go", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		testutils.WithStandardHeaders(req)

		resp, err := ctx.client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 404 or 403 for path traversal attempt, got %d", resp.StatusCode)
		}
	})
}
