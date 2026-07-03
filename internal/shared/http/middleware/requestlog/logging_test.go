package requestlog

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

const testBodyMaxBytes = 32

func TestMiddlewareIncludesMetadata(t *testing.T) {
	entries, recorder := runLoggedRequest(t, http.MethodGet, "/api/test?x=1", "", "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	started, completed := assertStartedAndCompleted(t, entries)

	assertLogValue(t, started, "level", "INFO")
	assertLogValue(t, started, "method", http.MethodGet)
	assertLogValue(t, started, "path", "/api/test")
	assertLogValue(t, started, "query", "x=1")
	if started["request_id"] == "" {
		t.Fatal("expected started request_id")
	}
	if started["remote_addr"] == "" {
		t.Fatal("expected started remote_addr")
	}
	assertLogValue(t, started, "user_agent", "test-agent")
	assertLogMissing(t, started, "status")
	assertLogMissing(t, started, "bytes")
	assertLogMissing(t, started, "duration_ms")
	assertLogMissing(t, started, "request_body")
	assertLogMissing(t, started, "response_body")

	assertLogValue(t, completed, "level", "INFO")
	assertLogValue(t, completed, "method", http.MethodGet)
	assertLogValue(t, completed, "path", "/api/test")
	assertLogValue(t, completed, "query", "x=1")
	assertLogNumber(t, completed, "status", http.StatusCreated)
	assertLogNumber(t, completed, "bytes", len(`{"ok":true}`))
	if _, ok := completed["duration_ms"]; !ok {
		t.Fatal("expected duration_ms")
	}
	if completed["request_id"] == "" {
		t.Fatal("expected completed request_id")
	}
	if completed["remote_addr"] == "" {
		t.Fatal("expected completed remote_addr")
	}
	assertLogValue(t, completed, "user_agent", "test-agent")
	assertLogValue(t, completed, "response_body", recorder.Body.String())
}

func TestMiddlewareDefaultsStatusWhenHandlerOnlyWritesBody(t *testing.T) {
	entries, _ := runLoggedRequest(t, http.MethodGet, "/api/default-status", "", "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	_, completed := assertStartedAndCompleted(t, entries)

	assertLogNumber(t, completed, "status", http.StatusOK)
	assertLogNumber(t, completed, "bytes", len(`{"status":"ok"}`))
}

func TestMiddlewareRecordsWriteHeaderWithoutBody(t *testing.T) {
	entries, _ := runLoggedRequest(t, http.MethodDelete, "/api/no-content", "", "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	_, completed := assertStartedAndCompleted(t, entries)

	assertLogNumber(t, completed, "status", http.StatusNoContent)
	assertLogNumber(t, completed, "bytes", 0)
	assertLogMissing(t, completed, "response_body")
}

func TestMiddlewareRecordsJSONRequestBodyAndRestoresIt(t *testing.T) {
	body := `{"name":"demo"}`
	var handlerBody string
	entries, _ := runLoggedRequest(t, http.MethodPost, "/api/test", "application/json", body, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		handlerBody = buf.String()
		writeTestJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	}))
	started, completed := assertStartedAndCompleted(t, entries)

	assertLogValue(t, started, "request_body", body)
	assertLogMissing(t, completed, "request_body")
	if handlerBody != body {
		t.Fatalf("expected handler body %q, got %q", body, handlerBody)
	}
}

func TestMiddlewareSkipsNonJSONBodies(t *testing.T) {
	entries, _ := runLoggedRequest(t, http.MethodPost, "/api/test", "text/plain", "plain text", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("plain response"))
	}))
	started, completed := assertStartedAndCompleted(t, entries)

	assertLogMissing(t, started, "request_body")
	assertLogMissing(t, started, "response_body")
	assertLogMissing(t, completed, "request_body")
	assertLogMissing(t, completed, "response_body")
}

func TestMiddlewareRecordsJSONSuffixContentType(t *testing.T) {
	entries, _ := runLoggedRequest(t, http.MethodPost, "/api/test", "application/problem+json", `{"problem":true}`, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		_, _ = w.Write([]byte(`{"detail":"bad"}`))
	}))
	started, completed := assertStartedAndCompleted(t, entries)

	assertLogValue(t, started, "request_body", `{"problem":true}`)
	assertLogValue(t, completed, "response_body", `{"detail":"bad"}`)
}

func TestMiddlewareTruncatesRequestAndResponseBodiesIndependently(t *testing.T) {
	requestBody := `{"value":"` + strings.Repeat("好", testBodyMaxBytes) + `"}`
	responseBody := `{"value":"` + strings.Repeat("坏", testBodyMaxBytes) + `"}`
	var handlerBody string
	entries, recorder := runLoggedRequest(t, http.MethodPost, "/api/test", "application/json", requestBody, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		handlerBody = buf.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(responseBody))
	}))
	started, completed := assertStartedAndCompleted(t, entries)

	assertTruncatedBody(t, started["request_body"])
	assertTruncatedBody(t, completed["response_body"])
	if handlerBody != requestBody {
		t.Fatalf("expected handler body %q, got %q", requestBody, handlerBody)
	}
	if recorder.Body.String() != responseBody {
		t.Fatalf("expected response body %q, got %q", responseBody, recorder.Body.String())
	}
}

func TestLoggingResponseWriterExposesOptionalInterfaces(t *testing.T) {
	writer := NewLoggingResponseWriter(httptest.NewRecorder(), testBodyMaxBytes)
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

func runLoggedRequest(t *testing.T, method string, target string, contentType string, body string, handler http.Handler) ([]map[string]any, *httptest.ResponseRecorder) {
	t.Helper()
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("User-Agent", "test-agent")
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	Middleware(logger, Config{RequestBodyLimit: testBodyMaxBytes, ResponseBodyLimit: testBodyMaxBytes})(handler).ServeHTTP(recorder, request)
	return decodeLogEntries(t, logBuffer.String()), recorder
}

func decodeLogEntries(t *testing.T, content string) []map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatal("expected log entry")
	}
	entries := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode log entry: %v\n%s", err, content)
		}
		entries = append(entries, entry)
	}
	return entries
}

func assertStartedAndCompleted(t *testing.T, entries []map[string]any) (map[string]any, map[string]any) {
	t.Helper()
	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d: %+v", len(entries), entries)
	}
	started := entries[0]
	completed := entries[1]
	assertLogValue(t, started, "msg", "request started")
	assertLogValue(t, completed, "msg", "request completed")
	if started["request_id"] != completed["request_id"] {
		t.Fatalf("expected matching request_id, got started=%#v completed=%#v", started["request_id"], completed["request_id"])
	}
	return started, completed
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

func assertLogMissing(t *testing.T, entry map[string]any, key string) {
	t.Helper()
	if _, ok := entry[key]; ok {
		t.Fatalf("did not expect %s: %+v", key, entry)
	}
}

func assertTruncatedBody(t *testing.T, body any) {
	t.Helper()
	value, ok := body.(string)
	if !ok {
		t.Fatalf("expected string body, got %#v", body)
	}
	if !strings.HasSuffix(value, TruncatedBodySuffix) {
		t.Fatalf("expected truncated body, got %q", value)
	}
	if !utf8.ValidString(value) {
		t.Fatalf("expected valid utf-8 body, got %q", value)
	}
	withoutSuffix := strings.TrimSuffix(value, TruncatedBodySuffix)
	if len([]byte(withoutSuffix)) > testBodyMaxBytes {
		t.Fatalf("expected body within %d bytes, got %d", testBodyMaxBytes, len([]byte(withoutSuffix)))
	}
}

func writeTestJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
