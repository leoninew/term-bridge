package requestlog

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const TruncatedBodySuffix = "..."

const requestIDHeader = "X-Request-ID"

var requestIDCounter atomic.Uint64

type Config struct {
	RequestBodyLimit  int
	ResponseBodyLimit int
}

func Middleware(logger *slog.Logger, config Config) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			requestID := requestIDFor(r)
			if requestID != "" {
				w.Header().Set(requestIDHeader, requestID)
			}
			requestAttrs := requestLogAttrs(r, requestID)
			requestBody, bodyErr := readRequestBodyForLog(r, config.RequestBodyLimit)

			startedAttrs := append([]any{}, requestAttrs...)
			if requestBody != "" {
				startedAttrs = append(startedAttrs, "request_body", requestBody)
			}
			if bodyErr != nil {
				startedAttrs = append(startedAttrs, "body_read_error", bodyErr.Error())
			}
			logger.Info("request started", startedAttrs...)

			responseWriter := NewLoggingResponseWriter(w, config.ResponseBodyLimit)
			next.ServeHTTP(responseWriter, r)

			completedAttrs := append([]any{}, requestAttrs...)
			completedAttrs = append(completedAttrs,
				"status", responseWriter.Status(),
				"bytes", responseWriter.BytesWritten(),
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
			if responseBody := responseWriter.Body(); responseBody != "" {
				completedAttrs = append(completedAttrs, "response_body", responseBody)
			}
			logger.Info("request completed", completedAttrs...)
		})
	}
}

func requestLogAttrs(r *http.Request, requestID string) []any {
	return []any{
		"method", r.Method,
		"path", r.URL.Path,
		"uri", r.URL.RequestURI(),
		"request_id", requestID,
		"remote_addr", r.RemoteAddr,
		"user_agent", r.UserAgent(),
	}
}

func requestIDFor(r *http.Request) string {
	requestID := strings.TrimSpace(r.Header.Get(requestIDHeader))
	if requestID != "" {
		return requestID
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatUint(requestIDCounter.Add(1), 36)
}

func readRequestBodyForLog(r *http.Request, limit int) (string, error) {
	if limit <= 0 || r.Body == nil || !isJSONContentType(r.Header.Get("Content-Type")) {
		return "", nil
	}
	body := r.Body
	loggedBytes, err := io.ReadAll(io.LimitReader(body, int64(limit)+1))
	r.Body = &prefixReadCloser{reader: io.MultiReader(bytes.NewReader(loggedBytes), body), closer: body}
	if err != nil {
		return "", err
	}
	if len(loggedBytes) == 0 {
		return "", nil
	}
	return TruncateBody(loggedBytes, limit), nil
}

func isJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = contentType
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func TruncateBody(value []byte, limit int) string {
	if limit <= 0 || len(value) == 0 {
		return ""
	}
	if len(value) <= limit {
		return string(value)
	}
	value = value[:limit]
	for len(value) > 0 && !utf8.Valid(value) {
		value = value[:len(value)-1]
	}
	return string(value) + TruncatedBodySuffix
}

type prefixReadCloser struct {
	reader io.Reader
	closer io.Closer
}

func (r *prefixReadCloser) Read(data []byte) (int, error) {
	return r.reader.Read(data)
}

func (r *prefixReadCloser) Close() error {
	return r.closer.Close()
}

type LoggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
	limit  int
	body   bytes.Buffer
}

func NewLoggingResponseWriter(w http.ResponseWriter, limit int) *LoggingResponseWriter {
	return &LoggingResponseWriter{ResponseWriter: w, limit: limit}
}

func (w *LoggingResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *LoggingResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.captureBody(data)
	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	return n, err
}

func (w *LoggingResponseWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *LoggingResponseWriter) BytesWritten() int {
	return w.bytes
}

func (w *LoggingResponseWriter) Body() string {
	if w.body.Len() == 0 {
		return ""
	}
	return TruncateBody(w.body.Bytes(), w.limit)
}

func (w *LoggingResponseWriter) Flush() {
	flusher, ok := w.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}
	flusher.Flush()
}

func (w *LoggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	if w.status == 0 {
		w.status = http.StatusSwitchingProtocols
	}
	return hijacker.Hijack()
}

func (w *LoggingResponseWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (w *LoggingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *LoggingResponseWriter) captureBody(data []byte) {
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

var _ http.Flusher = (*LoggingResponseWriter)(nil)
var _ http.Hijacker = (*LoggingResponseWriter)(nil)
var _ http.Pusher = (*LoggingResponseWriter)(nil)
var _ interface{ Unwrap() http.ResponseWriter } = (*LoggingResponseWriter)(nil)
