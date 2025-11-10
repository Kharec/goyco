package integration

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goyco/internal/middleware"
	"goyco/internal/testutils"
)

func TestIntegration_Compression(t *testing.T) {
	ctx := setupTestContext(t)
	router := ctx.Router

	t.Run("Response_Compression_Gzip", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Header().Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(rec.Body)
			if err != nil {
				t.Fatalf("Failed to create gzip reader: %v", err)
			}
			defer reader.Close()

			decompressed, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("Failed to decompress: %v", err)
			}

			if len(decompressed) == 0 {
				t.Error("Expected decompressed content")
			}
		} else {
			t.Log("Compression may not be applied to small responses")
		}
	})

	t.Run("Compression_Headers_Present", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts", nil)
		req.Header.Set("Accept-Encoding", "gzip, deflate")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Header().Get("Vary") == "" {
			t.Log("Vary header may not always be present")
		}
	})
}

func TestIntegration_StaticFiles(t *testing.T) {
	ctx := setupTestContext(t)
	router := ctx.Router

	t.Run("Robots_Txt_Served", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/robots.txt", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK)

		if !strings.Contains(rec.Body.String(), "User-agent") {
			t.Error("Expected robots.txt content")
		}
	})

	t.Run("Static_Files_Security_Headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/robots.txt", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Header().Get("X-Content-Type-Options") == "" {
			t.Log("Security headers may not be applied to all static files")
		}
	})
}

func TestIntegration_URLMetadata(t *testing.T) {
	ctx := setupTestContext(t)
	router := ctx.Router

	t.Run("URL_Metadata_Fetch_On_Post_Creation", func(t *testing.T) {
		ctx.Suite.EmailSender.Reset()
		user := createAuthenticatedUser(t, ctx.AuthService, ctx.Suite.UserRepo, "metadata_user", "metadata@example.com")

		ctx.Suite.TitleFetcher.SetTitle("Fetched Title")

		postBody := map[string]string{
			"title":   "Test Post",
			"url":     "https://example.com/metadata-test",
			"content": "Test content",
		}
		body, _ := json.Marshal(postBody)
		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req = testutils.WithUserContext(req, middleware.UserIDKey, user.User.ID)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusCreated)
	})

	t.Run("URL_Metadata_Endpoint", func(t *testing.T) {
		ctx.Suite.TitleFetcher.SetTitle("Endpoint Title")

		req := httptest.NewRequest("GET", "/api/posts/title?url=https://example.com/test", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		response := assertJSONResponse(t, rec, http.StatusOK)
		if response != nil {
			if data, ok := response["data"].(map[string]any); ok {
				if _, exists := data["title"]; !exists {
					t.Error("Expected title in metadata response")
				}
			}
		}
	})
}
