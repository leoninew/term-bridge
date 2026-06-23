package gatewayapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	agentapp "termbridge-go/internal/application/agent"
	terminalapp "termbridge-go/internal/application/terminal"
	"termbridge-go/internal/domain/session"
)

func TestGatewayAgentTerminalAttachE2E(t *testing.T) {
	gateway := New(Config{})
	server := httptest.NewServer(gateway)
	defer server.Close()

	runtime := newFakeRuntimeAccess()
	stateDir := t.TempDir()
	device := writeGatewayE2EDevice(t, stateDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	client := agentapp.New(agentapp.Config{ServerURL: server.URL, DeviceName: device.Name, StateDir: stateDir, Runtime: runtime})
	go func() {
		errCh <- client.Run(ctx)
	}()
	waitForRoute(t, gateway, device.ID)

	cookie := loginCookie(t, gateway)
	header := http.Header{}
	header.Set("Cookie", cookie.Name+"="+cookie.Value)
	browser, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/gateway/devices/"+device.ID+"/sessions/sess-1/ws", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatalf("browser Dial() error = %v", err)
	}
	defer browser.Close(websocket.StatusNormalClosure, "")

	stream := runtime.waitAttach(t, "sess-1")
	stream.sendBinary("POMELO_M6_ATTACH_READY\r\n")
	messageType, output, err := browser.Read(ctx)
	if err != nil {
		t.Fatalf("browser Read() error = %v", err)
	}
	if messageType != websocket.MessageBinary || !strings.Contains(string(output), "POMELO_M6_ATTACH_READY") {
		t.Fatalf("terminal output type=%v data=%q, want attach ready marker", messageType, output)
	}

	if err := browser.Write(ctx, websocket.MessageText, []byte(`{"type":"hello"}`)); err != nil {
		t.Fatalf("hello Write() error = %v", err)
	}
	if err := browser.Write(ctx, websocket.MessageText, []byte(`{"type":"resize","cols":132,"rows":35}`)); err != nil {
		t.Fatalf("resize Write() error = %v", err)
	}
	stream.waitResize(t, 132, 35)

	if err := browser.Write(ctx, websocket.MessageBinary, []byte("echo POMELO_M6_ATTACH_INPUT\r\n")); err != nil {
		t.Fatalf("input Write() error = %v", err)
	}
	stream.waitInput(t, "POMELO_M6_ATTACH_INPUT")
	stream.sendBinary("POMELO_M6_ATTACH_INPUT\r\n")
	_, echo, err := browser.Read(ctx)
	if err != nil {
		t.Fatalf("browser echo Read() error = %v", err)
	}
	if !strings.Contains(string(echo), "POMELO_M6_ATTACH_INPUT") {
		t.Fatalf("echo output = %q, want input marker", echo)
	}

	if err := browser.Write(ctx, websocket.MessageText, []byte(`{"type":"detach"}`)); err != nil {
		t.Fatalf("detach Write() error = %v", err)
	}
	stream.waitDetach(t, "gateway_detached")

	cancel()
	select {
	case err := <-errCh:
		if err != nil && !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "closed network connection") {
			t.Fatalf("client Run() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for agent client to stop")
	}
}

func writeGatewayE2EDevice(t *testing.T, stateDir string) agentapp.Device {
	t.Helper()
	device := agentapp.Device{ID: "gateway-e2e-device", Name: "gateway-e2e", CreatedAt: time.Now().UTC()}
	data, err := json.MarshalIndent(device, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, agentapp.DeviceFileName), append(data, '\n'), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return device
}

type fakeRuntimeAccess struct {
	mu      sync.Mutex
	streams map[string]*fakeTerminalStream
	attach  chan attachedStream
}

type attachedStream struct {
	sessionID string
	stream    *fakeTerminalStream
}

func newFakeRuntimeAccess() *fakeRuntimeAccess {
	return &fakeRuntimeAccess{streams: map[string]*fakeTerminalStream{}, attach: make(chan attachedStream, 1)}
}

func (r *fakeRuntimeAccess) WorkspaceTree(context.Context) ([]terminalapp.WorkspaceTreeNode, error) {
	return []terminalapp.WorkspaceTreeNode{{ID: "ws-1", Key: "ws-1", Name: "Workspace", Path: ".", Children: []terminalapp.WorkspaceSessionSummary{{ID: "sess-1", Name: "Session", Command: "fake-tui", LifecycleState: session.StateRunning}}}}, nil
}

func (r *fakeRuntimeAccess) ListSessions(context.Context) ([]terminalapp.SessionSummary, error) {
	return []terminalapp.SessionSummary{{ID: "sess-1", Name: "Session", Command: "fake-tui", LifecycleState: session.StateRunning}}, nil
}

func (r *fakeRuntimeAccess) ReadHistory(context.Context, string) ([]byte, error) {
	return []byte("POMELO_M6_ATTACH_READY\n"), nil
}

func (r *fakeRuntimeAccess) Attach(_ context.Context, sessionID string) (agentapp.TerminalStream, error) {
	stream := newFakeTerminalStream()
	r.mu.Lock()
	r.streams[sessionID] = stream
	r.mu.Unlock()
	r.attach <- attachedStream{sessionID: sessionID, stream: stream}
	return stream, nil
}

func (r *fakeRuntimeAccess) waitAttach(t *testing.T, sessionID string) *fakeTerminalStream {
	t.Helper()
	select {
	case attached := <-r.attach:
		if attached.sessionID != sessionID {
			t.Fatalf("attached session = %q, want %q", attached.sessionID, sessionID)
		}
		return attached.stream
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for attach %s", sessionID)
		return nil
	}
}

type fakeTerminalStream struct {
	outbound chan terminalapp.Outbound
	input    chan []byte
	resize   chan terminalResize
	detach   chan string
}

type terminalResize struct {
	cols int
	rows int
}

func newFakeTerminalStream() *fakeTerminalStream {
	return &fakeTerminalStream{outbound: make(chan terminalapp.Outbound, 8), input: make(chan []byte, 8), resize: make(chan terminalResize, 8), detach: make(chan string, 1)}
}

func (s *fakeTerminalStream) Outbound() <-chan terminalapp.Outbound {
	return s.outbound
}

func (s *fakeTerminalStream) WriteInput(data []byte) error {
	s.input <- append([]byte(nil), data...)
	return nil
}

func (s *fakeTerminalStream) Resize(cols int, rows int) error {
	s.resize <- terminalResize{cols: cols, rows: rows}
	return nil
}

func (s *fakeTerminalStream) Detach(reason string) {
	s.detach <- reason
	close(s.outbound)
}

func (s *fakeTerminalStream) sendBinary(value string) {
	s.outbound <- terminalapp.Outbound{Kind: terminalapp.OutboundBinary, Binary: []byte(value)}
}

func (s *fakeTerminalStream) waitInput(t *testing.T, want string) {
	t.Helper()
	select {
	case input := <-s.input:
		if !strings.Contains(string(input), want) {
			t.Fatalf("input = %q, want marker %q", input, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for input %q", want)
	}
}

func (s *fakeTerminalStream) waitResize(t *testing.T, cols int, rows int) {
	t.Helper()
	select {
	case resize := <-s.resize:
		if resize.cols != cols || resize.rows != rows {
			t.Fatalf("resize = %dx%d, want %dx%d", resize.cols, resize.rows, cols, rows)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for resize %dx%d", cols, rows)
	}
}

func (s *fakeTerminalStream) waitDetach(t *testing.T, want string) {
	t.Helper()
	select {
	case reason := <-s.detach:
		if reason != want {
			t.Fatalf("detach reason = %q, want %q", reason, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for detach %q", want)
	}
}

var _ agentapp.RuntimeAccess = (*fakeRuntimeAccess)(nil)
var _ agentapp.TerminalStream = (*fakeTerminalStream)(nil)
