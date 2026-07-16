package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	terminalproto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/terminal"
	"gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
)

func TestTerminalRelayOutputInputAndSingleWriter(t *testing.T) {
	handler := New(testCloudConfig())
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inputCh := make(chan []byte, 1)
	agentDone := make(chan struct{})
	go func() {
		defer close(agentDone)
		runTerminalAgent(t, ctx, server.URL, inputCh)
	}()
	waitForRoute(t, handler, "dev-1")
	token := loginToken(t, handler)
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	browser, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/devices/dev-1/workspaces/ws-1/sessions/sess-1/ws", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatalf("browser Dial() error = %v", err)
	}
	defer func() { _ = browser.Close(websocket.StatusNormalClosure, "") }()
	controlType, controlData, err := browser.Read(ctx)
	if err != nil {
		t.Fatalf("browser started control Read() error = %v", err)
	}
	if controlType != websocket.MessageText {
		t.Fatalf("started frame type = %v, want text", controlType)
	}
	var started agent.ServerControlMessage
	if err := json.Unmarshal(controlData, &started); err != nil {
		t.Fatalf("Unmarshal(started) error = %v", err)
	}
	if started.GetType() != terminalproto.TypeStarted || started.GetSessionId() != "sess-1" || started.GetWorkspaceId() != "ws-1" || started.GetLifecycleState() != "running" || started.GetAttachmentState() != "attached" {
		t.Fatalf("started control type=%q session=%q workspace=%q lifecycle=%q attachment=%q", started.GetType(), started.GetSessionId(), started.GetWorkspaceId(), started.GetLifecycleState(), started.GetAttachmentState())
	}
	outputType, output, err := browser.Read(ctx)
	if err != nil {
		t.Fatalf("browser output Read() error = %v", err)
	}
	if outputType != websocket.MessageBinary {
		t.Fatalf("output frame type = %v, want binary", outputType)
	}
	wantOutput := []byte{'h', 'e', 'l', 'l', 'o', 0xff, 0xfe, 0x1b, '[', '2', 'J'}
	if !bytes.Equal(output, wantOutput) {
		t.Fatalf("terminal output = %q, want %q", output, wantOutput)
	}
	_, response, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/devices/dev-1/workspaces/ws-1/sessions/sess-1/ws", &websocket.DialOptions{HTTPHeader: header})
	if err == nil {
		t.Fatal("second writer Dial() error = nil, want conflict")
	}
	if response == nil || response.StatusCode != http.StatusConflict {
		t.Fatalf("second writer status = %#v, want 409", response)
	}
	wantInput := []byte{'i', 'n', 'p', 'u', 't', 0xff, 0x00, 0x1b, '[', 'A'}
	if err := browser.Write(ctx, websocket.MessageBinary, wantInput); err != nil {
		t.Fatalf("browser Write() error = %v", err)
	}
	select {
	case input := <-inputCh:
		if !bytes.Equal(input, wantInput) {
			t.Fatalf("input = %q, want %q", input, wantInput)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for terminal input")
	}
	cancel()
	<-agentDone
}

func runTerminalAgent(t *testing.T, ctx context.Context, serverUrl string, inputCh chan<- []byte) {
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
		if err != nil {
			continue
		}
		switch payload := frame.GetPayload().(type) {
		case *shared.TunnelFrame_TerminalAttach:
			if frame.GetRequestId() == "" || payload.TerminalAttach.GetWorkspaceId() == "" {
				return
			}
			started := &shared.TunnelFrame{StreamId: frame.GetStreamId(), Payload: &shared.TunnelFrame_TerminalControl{TerminalControl: &agent.ServerControlMessage{Type: terminalproto.TypeStarted, SessionId: "sess-1", WorkspaceId: "ws-1", LifecycleState: "running", AttachmentState: "attached"}}}
			startedData, _ := tunnel.MarshalFrame(started)
			_ = conn.Write(ctx, websocket.MessageText, startedData)
			output := &shared.TunnelFrame{StreamId: frame.GetStreamId(), Payload: &shared.TunnelFrame_TerminalOutput{TerminalOutput: &shared.TerminalOutput{Data: []byte{'h', 'e', 'l', 'l', 'o', 0xff, 0xfe, 0x1b, '[', '2', 'J'}}}}
			outputData, _ := tunnel.MarshalFrame(output)
			_ = conn.Write(ctx, websocket.MessageText, outputData)
		case *shared.TunnelFrame_TerminalInput:
			inputCh <- payload.TerminalInput.GetData()
		case *shared.TunnelFrame_Close:
			return
		}
	}
}

func TestAgentDisconnectSendsStructuredTerminalError(t *testing.T) {
	handler := New(testCloudConfig())
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inputCh := make(chan []byte, 1)
	agentDone := make(chan struct{})
	go func() {
		defer close(agentDone)
		runTerminalAgent(t, ctx, server.URL, inputCh)
	}()
	waitForRoute(t, handler, "dev-1")
	token := loginToken(t, handler)
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	browser, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/devices/dev-1/workspaces/ws-1/sessions/sess-1/ws", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatalf("browser Dial() error = %v", err)
	}
	defer func() { _ = browser.Close(websocket.StatusNormalClosure, "") }()
	if _, _, err := browser.Read(ctx); err != nil {
		t.Fatalf("initial started Read() error = %v", err)
	}
	if _, _, err := browser.Read(ctx); err != nil {
		t.Fatalf("initial output Read() error = %v", err)
	}
	cancel()
	_, data, err := browser.Read(context.Background())
	if err != nil {
		t.Fatalf("terminal error Read() error = %v", err)
	}
	var message struct {
		Type    string `json:"type"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatalf("terminal error is not JSON: %v; data=%q", err, data)
	}
	if message.Type != "error" || message.Code != "device_disconnected" || message.Message == "" {
		t.Fatalf("terminal error = %#v", message)
	}
	<-agentDone
}

func TestRouteUnavailable(t *testing.T) {
	handler := New(testCloudConfig())
	token := loginToken(t, handler)
	request := httptest.NewRequest(http.MethodGet, "/api/devices/missing/workspaces/ws-1/sessions", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := assertAPIError(t, response, http.StatusServiceUnavailable, errorCodeDeviceOffline)
	if body.Error != errorMessageDeviceOffline {
		t.Fatalf("error = %q, want %q", body.Error, errorMessageDeviceOffline)
	}
}
