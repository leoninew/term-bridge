package webserver

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"
)

const truncatedLogBodySuffix = "..."

func logRequests(logger *slog.Logger, requestBodyLimit int, responseBodyLimit int, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		requestBody, requestBodyErr := readRequestBodyForLog(r, requestBodyLimit)
		responseWriter := newLoggingResponseWriter(w, responseBodyLimit)

		next.ServeHTTP(responseWriter, r)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"uri", r.URL.RequestURI(),
			"status", responseWriter.Status(),
			"bytes", responseWriter.BytesWritten(),
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		}
		if requestBody != "" {
			attrs = append(attrs, "request_body", requestBody)
		}
		if responseBody := responseWriter.Body(); responseBody != "" {
			attrs = append(attrs, "response_body", responseBody)
		}
		if requestBodyErr != nil {
			attrs = append(attrs, "request_body_read_error", requestBodyErr.Error())
		}

		logger.Info("request completed", attrs...)
	})
}

func readRequestBodyForLog(r *http.Request, limit int) (string, error) {
	if limit <= 0 || r.Body == nil || !isJSONContentType(r.Header.Get("Content-Type")) {
		return "", nil
	}
	prefix, err := io.ReadAll(io.LimitReader(r.Body, int64(limit+1)))
	r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(prefix), r.Body))
	if len(prefix) == 0 {
		return "", err
	}
	return truncateLogBody(prefix, limit), err
}

func isJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = contentType
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func truncateLogBody(data []byte, limit int) string {
	if limit <= 0 || len(data) == 0 {
		return ""
	}
	if len(data) <= limit {
		return string(bytes.ToValidUTF8(data, nil))
	}
	return string(bytes.ToValidUTF8(data[:limit], nil)) + truncatedLogBodySuffix
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
	limit  int
	body   bytes.Buffer
}

func newLoggingResponseWriter(w http.ResponseWriter, limit int) *loggingResponseWriter {
	return &loggingResponseWriter{ResponseWriter: w, limit: limit}
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.captureBody(data)
	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	return n, err
}

func (w *loggingResponseWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *loggingResponseWriter) BytesWritten() int {
	return w.bytes
}

func (w *loggingResponseWriter) Body() string {
	if w.body.Len() == 0 {
		return ""
	}
	return truncateLogBody(w.body.Bytes(), w.limit)
}

func (w *loggingResponseWriter) Flush() {
	flusher, ok := w.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}
	flusher.Flush()
}

func (w *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	if w.status == 0 {
		w.status = http.StatusSwitchingProtocols
	}
	return hijacker.Hijack()
}

func (w *loggingResponseWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (w *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *loggingResponseWriter) captureBody(data []byte) {
	if w.limit <= 0 || !isJSONContentType(w.Header().Get("Content-Type")) || len(data) == 0 {
		return
	}
	max := w.limit + 1
	if w.body.Len() >= max {
		return
	}
	remaining := max - w.body.Len()
	if len(data) > remaining {
		data = data[:remaining]
	}
	_, _ = w.body.Write(data)
}

var _ http.Flusher = (*loggingResponseWriter)(nil)
var _ http.Hijacker = (*loggingResponseWriter)(nil)
var _ http.Pusher = (*loggingResponseWriter)(nil)
var _ interface{ Unwrap() http.ResponseWriter } = (*loggingResponseWriter)(nil)
