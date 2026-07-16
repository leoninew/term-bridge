package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBrowserAPIRelay(t *testing.T) {
	handler := New(testCloudConfig())
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	agentDone := make(chan struct{})
	go func() {
		defer close(agentDone)
		runFakeAgent(t, ctx, server.URL, func(frame *shared.TunnelFrame) *shared.TunnelFrame {
			if frame.GetRequestId() != "req_test_relay" {
				return tunnel.ErrorFrame(frame.GetStreamId(), frame.GetRequestId(), "bad_request", "missing request id")
			}
			switch payload := frame.GetPayload().(type) {
			case *shared.TunnelFrame_WorkspaceTreeReq:
				return responseFrame(frame, &shared.TunnelFrame_WorkspaceTreeResp{WorkspaceTreeResp: &agent.WorkspaceTreeResp{Items: []*agent.WorkspaceTreeNode{{Id: "ws-1", Name: "Workspace"}}}})
			case *shared.TunnelFrame_WorkspaceSessionsReq:
				return responseFrame(frame, &shared.TunnelFrame_WorkspaceSessionsResp{WorkspaceSessionsResp: &agent.WorkspaceSessionsResp{Items: []*agent.SessionSummary{{Id: "sess-1", WorkspaceId: payload.WorkspaceSessionsReq.GetWorkspaceId(), Name: "Session"}}}})
			case *shared.TunnelFrame_ReadHistoryReq:
				return responseFrame(frame, &shared.TunnelFrame_ReadHistoryResp{ReadHistoryResp: &agent.ReadHistoryResp{Text: strings.Repeat("h", 64*1024)}})
			case *shared.TunnelFrame_ListShortcutsReq:
				description := "launch shell"
				return responseFrame(frame, &shared.TunnelFrame_ListShortcutsResp{ListShortcutsResp: &agent.ListShortcutsResp{Items: []*agent.Shortcut{{Id: "shortcut-1", Name: "Shell", Command: `cmd /c "echo hello" && dir`, Description: &description, LastUsedAt: timestamppb.New(time.Date(2026, time.July, 14, 12, 30, 0, 0, time.UTC))}}}})
			case *shared.TunnelFrame_CreateShortcutReq:
				return responseFrame(frame, &shared.TunnelFrame_CreateShortcutResp{CreateShortcutResp: &agent.CreateShortcutResp{Shortcut: &agent.Shortcut{Id: "shortcut-created", Name: payload.CreateShortcutReq.GetName(), Command: payload.CreateShortcutReq.GetCommand(), Description: payload.CreateShortcutReq.Description}}})
			case *shared.TunnelFrame_UpdateShortcutReq:
				return responseFrame(frame, &shared.TunnelFrame_UpdateShortcutResp{UpdateShortcutResp: &agent.UpdateShortcutResp{Shortcut: &agent.Shortcut{Id: payload.UpdateShortcutReq.GetShortcutId(), Name: payload.UpdateShortcutReq.GetRequest().GetName(), Command: payload.UpdateShortcutReq.GetRequest().GetCommand(), Description: payload.UpdateShortcutReq.GetRequest().Description}}})
			case *shared.TunnelFrame_UpdateShortcutOrderReq:
				items := make([]*agent.Shortcut, 0, len(payload.UpdateShortcutOrderReq.GetShortcutIds()))
				for _, shortcutId := range payload.UpdateShortcutOrderReq.GetShortcutIds() {
					items = append(items, &agent.Shortcut{Id: shortcutId})
				}
				return responseFrame(frame, &shared.TunnelFrame_UpdateShortcutOrderResp{UpdateShortcutOrderResp: &agent.UpdateShortcutOrderResp{Items: items}})
			case *shared.TunnelFrame_DeleteShortcutReq:
				return responseFrame(frame, &shared.TunnelFrame_DeleteShortcutResp{DeleteShortcutResp: &agent.DeleteShortcutResp{}})
			default:
				return tunnel.ErrorFrame(frame.GetStreamId(), frame.GetRequestId(), "bad_request", "unknown payload")
			}
		})
	}()
	waitForRoute(t, handler, "dev-1")
	token := loginToken(t, handler)
	for _, tc := range []struct {
		path string
		want string
	}{
		{"/api/devices/dev-1/workspaces/tree", "Workspace"},
		{"/api/devices/dev-1/workspaces/ws-1/sessions", "sess-1"},
		{"/api/devices/dev-1/workspaces/ws-1/sessions/sess-1/history", strings.Repeat("h", 64*1024)},
		{"/api/devices/dev-1/shortcuts", "shortcut-1"},
	} {
		request := httptest.NewRequest(http.MethodGet, tc.path, nil)
		request.Header.Set(requestIdHeader, "req_test_relay")
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d; body=%s", tc.path, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), tc.want) {
			t.Fatalf("%s body = %s, want %q", tc.path, response.Body.String(), tc.want)
		}
	}
	for _, tc := range []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{http.MethodGet, "/api/devices/dev-1/shortcuts", "", http.StatusOK, "2026-07-14T12:30:00Z"},
		{http.MethodPost, "/api/devices/dev-1/shortcuts", `{"name":"Created shell","command":"cmd /c \"echo created\""}`, http.StatusCreated, "Created shell"},
		{http.MethodPatch, "/api/devices/dev-1/shortcuts/shortcut-1", `{"name":"Updated shell"}`, http.StatusOK, "Updated shell"},
		{http.MethodPatch, "/api/devices/dev-1/shortcuts/order", `{"shortcut_ids":["shortcut-2","shortcut-1"]}`, http.StatusOK, "shortcut-2"},
		{http.MethodDelete, "/api/devices/dev-1/shortcuts/shortcut-1", "", http.StatusNoContent, ""},
	} {
		request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		request.Header.Set(requestIdHeader, "req_test_relay")
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("%s %s status = %d, want %d; body=%s", tc.method, tc.path, response.Code, tc.status, response.Body.String())
		}
		if tc.want != "" && !strings.Contains(response.Body.String(), tc.want) {
			t.Fatalf("%s %s body = %s, want %q", tc.method, tc.path, response.Body.String(), tc.want)
		}
	}
	cancel()
	<-agentDone
}

func loginToken(t *testing.T, handler *Handler) string {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(testLoginRequestBody(t, handler, "admin", "admin"))))
	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d; body=%s", response.Code, response.Body.String())
	}
	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &tokenResp); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	return tokenResp.AccessToken
}

func waitForRoute(t *testing.T, handler *Handler, deviceId string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if handler.routeFor(deviceId) != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("route %s not registered", deviceId)
}

func runFakeAgent(t *testing.T, ctx context.Context, serverUrl string, respond func(*shared.TunnelFrame) *shared.TunnelFrame) {
	t.Helper()
	conn, _, err := websocket.Dial(ctx, "ws"+serverUrl[len("http"):]+"/api/agent/tunnel", &websocket.DialOptions{HTTPHeader: signedTestTunnelHeader(t)})
	if err != nil {
		t.Errorf("Dial() error = %v", err)
		return
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	hello := &shared.TunnelFrame{StreamId: tunnel.ControlStreamID, Payload: &shared.TunnelFrame_Hello{Hello: &shared.Hello{DeviceId: "dev-1", DeviceName: "local", ProtocolVersion: tunnel.ProtocolVersion}}}
	helloData, _ := tunnel.MarshalFrame(hello)
	if err := conn.Write(ctx, websocket.MessageText, helloData); err != nil {
		t.Errorf("hello Write() error = %v", err)
		return
	}
	if _, _, err := conn.Read(ctx); err != nil {
		t.Errorf("ack Read() error = %v", err)
		return
	}
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		frame, err := tunnel.UnmarshalFrame(data)
		if err != nil || !isRuntimeRequestFrame(frame) {
			continue
		}
		responseData, _ := tunnel.MarshalFrame(respond(frame))
		_ = conn.Write(ctx, websocket.MessageText, responseData)
	}
}

func responseFrame(request *shared.TunnelFrame, payload any) *shared.TunnelFrame {
	frame := &shared.TunnelFrame{StreamId: request.GetStreamId(), RequestId: request.GetRequestId()}
	switch value := payload.(type) {
	case *shared.TunnelFrame_WorkspaceTreeResp:
		frame.Payload = value
	case *shared.TunnelFrame_WorkspaceSessionsResp:
		frame.Payload = value
	case *shared.TunnelFrame_ReadHistoryResp:
		frame.Payload = value
	case *shared.TunnelFrame_ListShortcutsResp:
		frame.Payload = value
	case *shared.TunnelFrame_CreateShortcutResp:
		frame.Payload = value
	case *shared.TunnelFrame_UpdateShortcutResp:
		frame.Payload = value
	case *shared.TunnelFrame_UpdateShortcutOrderResp:
		frame.Payload = value
	case *shared.TunnelFrame_DeleteShortcutResp:
		frame.Payload = value
	}
	return frame
}

func isRuntimeRequestFrame(frame *shared.TunnelFrame) bool {
	switch frame.GetPayload().(type) {
	case *shared.TunnelFrame_WorkspaceTreeReq,
		*shared.TunnelFrame_WorkspaceSessionsReq,
		*shared.TunnelFrame_ReadHistoryReq,
		*shared.TunnelFrame_ListShortcutsReq,
		*shared.TunnelFrame_CreateShortcutReq,
		*shared.TunnelFrame_UpdateShortcutReq,
		*shared.TunnelFrame_UpdateShortcutOrderReq,
		*shared.TunnelFrame_DeleteShortcutReq:
		return true
	default:
		return false
	}
}
