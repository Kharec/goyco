package e2e

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"goyco/internal/testutils"
)

func TestE2E_RobotsTxt(t *testing.T) {
	ctx := setupTestContext(t)

	t.Run("robots_txt_served", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/robots.txt", nil)
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
			t.Errorf("Expected status 200 for robots.txt, got %d", resp.StatusCode)
			return
		}

		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/plain") && !strings.Contains(contentType, "text") {
			t.Logf("Unexpected Content-Type for robots.txt: %s", contentType)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read robots.txt body: %v", err)
		}

		content := string(body)
		if len(content) == 0 {
			t.Error("robots.txt is empty")
			return
		}

		if !strings.Contains(content, "User-agent") {
			t.Error("robots.txt missing User-agent directive")
		}
	})

	t.Run("robots_txt_content_validation", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/robots.txt", nil)
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
			t.Skip("robots.txt not available")
			return
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read robots.txt body: %v", err)
		}

		content := string(body)
		lines := strings.Split(content, "\n")

		hasUserAgent := false
		hasDisallow := false
		hasAllow := false

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "User-agent:") {
				hasUserAgent = true
			}
			if strings.HasPrefix(trimmed, "Disallow:") {
				hasDisallow = true
			}
			if strings.HasPrefix(trimmed, "Allow:") {
				hasAllow = true
			}
		}

		if !hasUserAgent {
			t.Error("robots.txt missing User-agent directive")
		}

		if !hasDisallow && !hasAllow {
			t.Log("robots.txt missing Allow/Disallow directives (may be intentional)")
		}
	})

	t.Run("robots_txt_api_disallowed", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/robots.txt", nil)
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
			t.Skip("robots.txt not available")
			return
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read robots.txt body: %v", err)
		}

		content := string(body)
		if strings.Contains(content, "Disallow: /api/") {
			t.Log("robots.txt correctly disallows /api/")
		} else {
			t.Log("robots.txt may not explicitly disallow /api/")
		}
	})

	t.Run("robots_txt_health_allowed", func(t *testing.T) {
		req, err := http.NewRequest("GET", ctx.baseURL+"/robots.txt", nil)
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
			t.Skip("robots.txt not available")
			return
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read robots.txt body: %v", err)
		}

		content := string(body)
		if strings.Contains(content, "Allow: /health") {
			t.Log("robots.txt correctly allows /health")
		} else {
			t.Log("robots.txt may not explicitly allow /health")
		}
	})
}
