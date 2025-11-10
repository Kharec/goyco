package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggingRecordsStatusAndLogs(t *testing.T) {
	originalOutput := log.Writer()
	defer log.SetOutput(originalOutput)

	var buf bytes.Buffer
	log.SetOutput(&buf)

	handler := Logging(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/logging-test", nil)
	request.Header.Set("User-Agent", "test-agent")

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", recorder.Result().StatusCode)
	}

	logLine := buf.String()
	if !strings.Contains(logLine, "GET /logging-test 201") {
		t.Fatalf("expected log line to contain method, path and status, got %q", logLine)
	}

	if !strings.Contains(logLine, "test-agent") {
		t.Fatalf("expected log line to contain user agent, got %q", logLine)
	}
}

func TestResponseWriterWriteHeaderStoresStatus(t *testing.T) {
	recorder := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: recorder, statusCode: http.StatusOK}

	wrapped.WriteHeader(http.StatusAccepted)

	if wrapped.statusCode != http.StatusAccepted {
		t.Fatalf("expected stored status 202, got %d", wrapped.statusCode)
	}

	if recorder.Result().StatusCode != http.StatusAccepted {
		t.Fatalf("expected underlying writer to receive 202, got %d", recorder.Result().StatusCode)
	}
}
