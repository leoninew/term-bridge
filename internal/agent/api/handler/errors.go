package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
	terminalproto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/terminal"

	"google.golang.org/protobuf/proto"
)

const requestIdHeader = "X-Request-ID"

const (
	errorCodeBadRequest       = "bad_request"
	errorCodeUnauthorized     = "unauthorized"
	errorCodeNotFound         = "not_found"
	errorCodeMethodNotAllowed = "method_not_allowed"
	errorCodeDeviceOffline    = "device_offline"
	errorCodeUpstream         = "upstream_error"
	errorCodeConflict         = "conflict"
	errorCodeInternal         = "internal_error"
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
	requestId := h.requestIdFor(w, r)
	h.logAPIError(r, status, code, requestId, cause)
	body := &shared.ErrorResp{Code: code, Error: h.responseErrorText(safeError, cause), RequestId: requestId}
	data, err := codec.MarshalProtoJSON(body)
	if err != nil {
		h.logAPIError(r, http.StatusInternalServerError, errorCodeInternal, requestId, err)
		data, _ = json.Marshal(shared.ErrorResp{Code: errorCodeInternal, Error: errorMessageInternal, RequestId: requestId})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func (h *Handler) writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	h.writeAPIError(w, r, http.StatusUnauthorized, errorCodeUnauthorized, errorMessageUnauthorized, nil)
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
