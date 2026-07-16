package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
	terminalproto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/terminal"
	tunnel "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

const requestIdHeader = "X-Request-ID"

const (
	errorCodeBadRequest         = "bad_request"
	errorCodeUnauthorized       = "unauthorized"
	errorCodeNotFound           = "not_found"
	errorCodeMethodNotAllowed   = "method_not_allowed"
	errorCodeDeviceOffline      = "device_offline"
	errorCodeUpstream           = "upstream_error"
	errorCodeConflict           = "conflict"
	errorCodeInternal           = "internal_error"
	errorCodeServiceUnavailable = "service_unavailable"
)

const (
	errorMessageBadRequest       = "Request body is invalid."
	errorMessageUnauthorized     = "Authentication is required."
	errorMessageNotFound         = "Resource was not found."
	errorMessageMethodNotAllowed = "Method is not allowed."
	errorMessageDeviceOffline    = "Device is offline."
	errorMessageUpstream         = "Upstream request failed."
	errorMessageUpstreamInvalid  = "Upstream response is invalid."
	errorMessageConflict         = "Session already has an active writer."
	errorMessageInternal         = "An unexpected error occurred."
)

var generatedRequestCounter atomic.Uint64

func (h *Handler) requestIdFor(w http.ResponseWriter, r *http.Request) string {
	requestId := strings.TrimSpace(r.Header.Get(requestIdHeader))
	if requestId == "" {
		requestId = strings.TrimSpace(w.Header().Get(requestIdHeader))
	}
	if requestId == "" {
		requestId = newRequestId()
	}
	w.Header().Set(requestIdHeader, requestId)
	return requestId
}

func (h *Handler) writeAPIError(w http.ResponseWriter, r *http.Request, status int, code string, safeError string, cause error) {
	h.writeAPIErrorWithDetails(w, r, status, code, safeError, cause, fileErrorDetails(cause))
}

func (h *Handler) writeAPIErrorWithDetails(w http.ResponseWriter, r *http.Request, status int, code string, safeError string, cause error, details *structpb.Struct) {
	requestId := h.requestIdFor(w, r)
	h.logAPIError(r, status, code, requestId, cause)
	body := &shared.ErrorResp{Code: code, Error: h.responseErrorText(safeError, cause), RequestId: requestId, Details: details}
	data, err := codec.MarshalProtoJSON(body)
	if err != nil {
		h.logAPIError(r, http.StatusInternalServerError, errorCodeInternal, requestId, err)
		data, _ = json.Marshal(shared.ErrorResp{Code: errorCodeInternal, Error: errorMessageInternal, RequestId: requestId})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func fileErrorDetails(err error) *structpb.Struct {
	var fileErr *filemodel.Error
	if !errors.As(err, &fileErr) || fileErr.Code != "revision_conflict" || fileErr.Entry == nil {
		return nil
	}
	entry := fileErr.Entry
	details, err := structpb.NewStruct(map[string]any{
		"type": "revision_conflict",
		"current_entry": map[string]any{
			"path":     entry.Path.String(),
			"name":     entry.Name,
			"kind":     string(entry.Kind),
			"size":     entry.Size,
			"revision": entry.Revision,
		},
	})
	if err != nil {
		return nil
	}
	return details
}

func (h *Handler) writeRuntimeError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := runtimeErrorResponse(err)
	h.writeAPIError(w, r, status, code, message, err)
}

func runtimeErrorResponse(err error) (int, string, string) {
	var remote *tunnel.RemoteError
	if errors.As(err, &remote) && remote.Response != nil {
		return fileErrorResponse(remote.Response.GetCode(), remote.Response.GetError())
	}
	var fileErr *filemodel.Error
	if errors.As(err, &fileErr) {
		return fileErrorResponse(fileErr.Code, fileErr.Message)
	}
	return http.StatusBadGateway, errorCodeUpstream, errorMessageUpstream
}

func fileErrorResponse(code string, message string) (int, string, string) {
	if message == "" {
		message = errorMessageInternal
	}
	switch code {
	case "invalid_file_path", "invalid_git_layer", "invalid_operation", "root_protected":
		return http.StatusBadRequest, code, message
	case "workspace_not_found", "file_not_found":
		return http.StatusNotFound, code, message
	case "revision_conflict", "already_exists", "directory_not_empty":
		return http.StatusConflict, code, message
	case "workspace_root_unavailable", "file_not_text", "file_too_large", "type_mismatch", "symlink_unsupported":
		return http.StatusUnprocessableEntity, code, message
	default:
		return http.StatusBadGateway, errorCodeUpstream, errorMessageUpstream
	}
}

func (h *Handler) writeNotFound(w http.ResponseWriter, r *http.Request) {
	h.writeAPIError(w, r, http.StatusNotFound, errorCodeNotFound, errorMessageNotFound, nil)
}

func (h *Handler) methodNotAllowed(w http.ResponseWriter, r *http.Request, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	h.writeAPIError(w, r, http.StatusMethodNotAllowed, errorCodeMethodNotAllowed, errorMessageMethodNotAllowed, nil)
}

func (h *Handler) decodeJSONRequest(w http.ResponseWriter, r *http.Request, message proto.Message) bool {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, terminalproto.MaxJSONMessageBytes))
	if err != nil {
		h.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, errorMessageBadRequest, err)
		return false
	}
	if err := codec.UnmarshalProtoJSON(data, message); err != nil {
		h.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, errorMessageBadRequest, err)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, message proto.Message) {
	data, err := codec.MarshalProtoJSON(message)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func (h *Handler) responseErrorText(safeError string, cause error) string {
	if !h.config.DebugErrors || cause == nil {
		return safeError
	}
	causeText := strings.TrimSpace(cause.Error())
	if causeText == "" || containsSensitiveTerm(causeText) {
		return safeError
	}
	return causeText
}

func (h *Handler) logAPIError(r *http.Request, status int, code string, requestId string, cause error) {
	attrs := []any{
		"request_id", requestId,
		"method", r.Method,
		"path", r.URL.Path,
		"query", redactQuery(r.URL.Query()),
		"status", status,
		"code", code,
	}
	if cause != nil {
		attrs = append(attrs, "cause", cause.Error())
	}
	h.config.Logger.Warn("agent api error", attrs...)
}

func newRequestId() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err == nil {
		return "req_" + hex.EncodeToString(data[:])
	}
	return "req_" + strconv.FormatInt(time.Now().UnixNano(), 36) + "_" + strconv.FormatUint(generatedRequestCounter.Add(1), 36)
}

func containsSensitiveTerm(value string) bool {
	lower := strings.ToLower(value)
	terms := []string{"authorization", "code", "cookie", "credential", "password", "secret", "token", "connection string", "apikey", "api_key"}
	for _, term := range terms {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func redactQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	redacted := url.Values{}
	for key, value := range values {
		copied := append([]string(nil), value...)
		if containsSensitiveTerm(key) {
			for index := range copied {
				copied[index] = "[REDACTED]"
			}
		}
		redacted[key] = copied
	}
	return redacted.Encode()
}
