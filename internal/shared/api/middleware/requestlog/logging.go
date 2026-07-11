package requestlog

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	sharedconfig "gitee.com/leoninew/TermBridge-go/internal/shared/infrastructure/config"
)

const TruncatedBodySuffix = "..."

const requestIdHeader = "X-Request-ID"

const assetsPathPrefix = "/assets/"

var requestIdCounter atomic.Uint64

func Middleware(logger *slog.Logger, config sharedconfig.LogHTTPConfig) func(http.Handler) http.Handler {
	if logger == nil {
		panic("request logger is required")
	}
	assetExtensions := assetExtensionSet(config.SkipAssetExtensions)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			requestId := requestIdFor(r)
			if requestId != "" {
				w.Header().Set(requestIdHeader, requestId)
			}
			requestAttrs := requestLogAttrs(r, requestId)
			requestBody, bodyErr := "", error(nil)
			if !isSensitiveCloudAuthRequest(r.URL.Path) {
				requestBody, bodyErr = readRequestBodyForLog(r, config.RequestBodyLimit)
			}

			startedAttrs := append([]any{}, requestAttrs...)
			if requestBody != "" {
				startedAttrs = append(startedAttrs, "request_body", requestBody)
			}
			if bodyErr != nil {
				startedAttrs = append(startedAttrs, "body_read_error", bodyErr.Error())
			}
			deferStartedLog := config.SkipAssetEnabled && isSkippableAssetPath(r.URL.Path, assetExtensions)
			if !deferStartedLog {
				logger.Info("request started", startedAttrs...)
			}

			responseWriter := NewLoggingResponseWriter(w, config.ResponseBodyLimit)
			next.ServeHTTP(responseWriter, r)

			okStatus := responseWriter.Status() == http.StatusOK || responseWriter.Status() == http.StatusNotModified
			if deferStartedLog && okStatus {
				return
			}
			if deferStartedLog {
				logger.Info("request started", startedAttrs...)
			}

			completedAttrs := append([]any{}, requestAttrs...)
			completedAttrs = append(completedAttrs,
				"status", responseWriter.Status(),
				"bytes", responseWriter.BytesWritten(),
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
			if responseBody := responseWriter.Body(); responseBody != "" && !isSensitiveCloudAuthResponse(r.URL.Path) {
				completedAttrs = append(completedAttrs, "response_body", responseBody)
			}
			logger.Info("request completed", completedAttrs...)
		})
	}
}

func isSensitiveCloudAuthRequest(requestPath string) bool {
	return requestPath == "/cloud-api/auth/login" || requestPath == "/cloud-api/auth/register"
}

func isSensitiveCloudAuthResponse(requestPath string) bool {
	return requestPath == "/cloud-api/auth/login/csrf"
}

func isSkippableAssetPath(requestPath string, assetExtensions map[string]struct{}) bool {
	if len(assetExtensions) == 0 {
		return false
	}
	requestPath = path.Clean("/" + requestPath)
	if !strings.HasPrefix(requestPath, assetsPathPrefix) {
		return false
	}
	_, ok := assetExtensions[strings.ToLower(path.Ext(requestPath))]
	return ok
}

func assetExtensionSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		out[value] = struct{}{}
	}
	return out
}

func requestLogAttrs(r *http.Request, requestId string) []any {
	return []any{
		"method", r.Method,
		"path", r.URL.Path,
		"query", r.URL.RawQuery,
		"request_id", requestId,
		"remote_addr", r.RemoteAddr,
		"user_agent", r.UserAgent(),
	}
}

func requestIdFor(r *http.Request) string {
	requestId := strings.TrimSpace(r.Header.Get(requestIdHeader))
	if requestId != "" {
		return requestId
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatUint(requestIdCounter.Add(1), 36)
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
