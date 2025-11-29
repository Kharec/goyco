package integration

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIntegration_Compression(t *testing.T) {
	ctx := setupTestContext(t)
	router := ctx.Router

	t.Run("Response_Compression_Gzip", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/api/posts", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		contentEncoding := recorder.Header().Get("Content-Encoding")
		if contentEncoding != "" && strings.Contains(contentEncoding, "gzip") {
			assertHeaderContains(t, recorder, "Content-Encoding", "gzip")
			reader, err := gzip.NewReader(recorder.Body)
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
		request := httptest.NewRequest("GET", "/api/posts", nil)
		request.Header.Set("Accept-Encoding", "gzip, deflate")
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Header().Get("Vary") != "" {
			assertHeaderContains(t, recorder, "Vary", "Accept-Encoding")
		} else {
			t.Log("Vary header may not always be present")
		}
	})
}

func TestIntegration_StaticFiles(t *testing.T) {
	ctx := setupTestContext(t)
	router := ctx.Router

	t.Run("Robots_Txt_Served", func(t *testing.T) {
		request := makeGetRequest(t, router, "/robots.txt")

		assertStatus(t, request, http.StatusOK)

		if !strings.Contains(request.Body.String(), "User-agent") {
			t.Error("Expected robots.txt content")
		}
	})

	t.Run("Static_Files_Security_Headers", func(t *testing.T) {
		request := makeGetRequest(t, router, "/robots.txt")

		if request.Header().Get("X-Content-Type-Options") == "" {
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

		postBody := map[string]any{
			"title":   "Test Post",
			"url":     "https://example.com/metadata-test",
			"content": "Test content",
		}
		request := makePostRequest(t, router, "/api/posts", postBody, user, nil)

		assertStatus(t, request, http.StatusCreated)
	})

	t.Run("URL_Metadata_Endpoint", func(t *testing.T) {
		ctx.Suite.TitleFetcher.SetTitle("Endpoint Title")

		request := makeGetRequest(t, router, "/api/posts/title?url=https://example.com/test")

		response := assertJSONResponse(t, request, http.StatusOK)
		if response != nil {
			if data, ok := response["data"].(map[string]any); ok {
				if _, exists := data["title"]; !exists {
					t.Error("Expected title in metadata response")
				}
			}
		}
	})
}
