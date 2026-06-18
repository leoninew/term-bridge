package webserver

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogRequestsIncludesMetadata(t *testing.T) {
	entry, recorder := runLoggedRequest(t, http.MethodGet, "/api/test?x=1", "", "", 4096, 4096, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	assertLogValue(t, entry, "msg", "request completed")
	assertLogValue(t, entry, "level", "INFO")
	assertLogValue(t, entry, "method", http.MethodGet)
	assertLogValue(t, entry, "path", "/api/test")
	assertLogValue(t, entry, "uri", "/api/test?x=1")
	assertLogNumber(t, entry, "status", http.StatusCreated)
	assertLogNumber(t, entry, "bytes", len(`{"ok":true}`))
	if _, ok := entry["duration_ms"]; !ok {
		t.Fatal("expected duration_ms")
	}
	if entry["remote_addr"] == "" {
		t.Fatal("expected remote_addr")
	}
	assertLogValue(t, entry, "user_agent", "test-agent")
	assertLogValue(t, entry, "response_body", recorder.Body.String())
}

func TestLogRequestsDefaultsStatusWhenHandlerOnlyWritesBody(t *testing.T) {
	entry, _ := runLoggedRequest(t, http.MethodGet, "/api/default-status", "", "", 4096, 4096, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))

	assertLogNumber(t, entry, "status", http.StatusOK)
	assertLogNumber(t, entry, "bytes", len(`{"status":"ok"}`))
}

func TestLogRequestsRecordsWriteHeaderWithoutBody(t *testing.T) {
	entry, _ := runLoggedRequest(t, http.MethodDelete, "/api/no-content", "", "", 4096, 4096, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	assertLogNumber(t, entry, "status", http.StatusNoContent)
	assertLogNumber(t, entry, "bytes", 0)
	if _, ok := entry["response_body"]; ok {
		t.Fatalf("did not expect response_body: %+v", entry)
	}
}

func TestLogRequestsRecordsJSONRequestBodyAndRestoresIt(t *testing.T) {
	body := `{"name":"demo"}`
	var handlerBody string
	entry, _ := runLoggedRequest(t, http.MethodPost, "/api/test", "application/json", body, 4096, 4096, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		handlerBody = buf.String()
		writeTestJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	}))

	assertLogValue(t, entry, "request_body", body)
	if handlerBody != body {
		t.Fatalf("expected handler body %q, got %q", body, handlerBody)
	}
}

func TestLogRequestsSkipsNonJSONBodies(t *testing.T) {
	entry, _ := runLoggedRequest(t, http.MethodPost, "/api/test", "text/plain", "plain text", 4096, 4096, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("plain response"))
	}))

	if _, ok := entry["request_body"]; ok {
		t.Fatalf("did not expect request_body: %+v", entry)
	}
	if _, ok := entry["response_body"]; ok {
		t.Fatalf("did not expect response_body: %+v", entry)
	}
}

func TestLogRequestsRecordsJSONSuffixContentType(t *testing.T) {
	entry, _ := runLoggedRequest(t, http.MethodPost, "/api/test", "application/problem+json", `{"problem":true}`, 4096, 4096, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		_, _ = w.Write([]byte(`{"detail":"bad"}`))
	}))

	assertLogValue(t, entry, "request_body", `{"problem":true}`)
	assertLogValue(t, entry, "response_body", `{"detail":"bad"}`)
}

func TestLogRequestsTruncatesRequestAndResponseBodiesIndependently(t *testing.T) {
	requestBody := `{"value":"abcdef"}`
	responseBody := `{"value":"uvwxyz"}`
	var handlerBody string
	entry, recorder := runLoggedRequest(t, http.MethodPost, "/api/test", "application/json", requestBody, 12, 13, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		handlerBody = buf.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(responseBody))
	}))

	assertLogValue(t, entry, "request_body", requestBody[:12]+truncatedLogBodySuffix)
	assertLogValue(t, entry, "response_body", responseBody[:13]+truncatedLogBodySuffix)
	if handlerBody != requestBody {
		t.Fatalf("expected handler body %q, got %q", requestBody, handlerBody)
	}
	if recorder.Body.String() != responseBody {
		t.Fatalf("expected response body %q, got %q", responseBody, recorder.Body.String())
	}
}

func TestLoggingResponseWriterExposesOptionalInterfaces(t *testing.T) {
	writer := newLoggingResponseWriter(httptest.NewRecorder(), 4096)
	if _, ok := any(writer).(http.Flusher); !ok {
		t.Fatal("expected http.Flusher")
	}
	if _, ok := any(writer).(http.Hijacker); !ok {
		t.Fatal("expected http.Hijacker")
	}
	if _, ok := any(writer).(http.Pusher); !ok {
		t.Fatal("expected http.Pusher")
	}
	if got := writer.Unwrap(); got == nil {
		t.Fatal("expected Unwrap to return underlying writer")
	}
}

func runLoggedRequest(t *testing.T, method string, target string, contentType string, body string, requestLimit int, responseLimit int, handler http.Handler) (map[string]any, *httptest.ResponseRecorder) {
	t.Helper()
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("User-Agent", "test-agent")
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	logRequests(logger, requestLimit, responseLimit, handler).ServeHTTP(recorder, request)
	return decodeLogEntry(t, logBuffer.String()), recorder
}

func decodeLogEntry(t *testing.T, content string) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatal("expected log entry")
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("decode log entry: %v\n%s", err, content)
	}
	return entry
}

func assertLogValue(t *testing.T, entry map[string]any, key string, want string) {
	t.Helper()
	if got, ok := entry[key].(string); !ok || got != want {
		t.Fatalf("expected %s=%q, got %#v", key, want, entry[key])
	}
}

func assertLogNumber(t *testing.T, entry map[string]any, key string, want int) {
	t.Helper()
	got, ok := entry[key].(float64)
	if !ok || int(got) != want {
		t.Fatalf("expected %s=%d, got %#v", key, want, entry[key])
	}
}

func writeTestJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
