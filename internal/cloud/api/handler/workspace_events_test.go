package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
	"gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
	workspacefs "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/workspacefs"
)

func TestCloudWorkspaceWatchRelaysSubscribedAndChanges(t *testing.T) {
	handler := New(testCloudConfig())
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	subscribe := make(chan *shared.TunnelFrame, 1)
	closed := make(chan *shared.TunnelFrame, 1)
	agentDone := make(chan struct{})
	go func() {
		defer close(agentDone)
		runWorkspaceWatchAgent(t, ctx, server.URL, subscribe, closed)
	}()
	waitForRoute(t, handler, "0f490dee643b01b06e0ea84c253a9005")

	browserCtx, browserCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer browserCancel()
	browser, _, err := websocket.Dial(
		browserCtx,
		"ws"+server.URL[len("http"):]+"/api/devices/0f490dee643b01b06e0ea84c253a9005/workspaces/ws-1/fs/events",
		&websocket.DialOptions{
			HTTPHeader:   http.Header{"Authorization": []string{"Bearer " + loginToken(t, handler)}},
			Subprotocols: []string{workspacefs.Subprotocol},
		},
	)
	if err != nil {
		t.Fatalf("browser Dial() error = %v", err)
	}
	defer func() { _ = browser.Close(websocket.StatusNormalClosure, "") }()

	var subscribeFrame *shared.TunnelFrame
	select {
	case subscribeFrame = <-subscribe:
	case <-browserCtx.Done():
		t.Fatal("timed out waiting for workspace watch subscription")
	}
	if subscribeFrame.GetFsWatchSubscribeReq().GetWorkspaceId() != "ws-1" {
		t.Fatalf("subscribe workspace_id = %q, want ws-1", subscribeFrame.GetFsWatchSubscribeReq().GetWorkspaceId())
	}

	var subscribed agent.FsWatchSubscribed
	readWorkspaceWatchMessage(t, browserCtx, browser, &subscribed)
	if subscribed.GetWorkspaceId() != "ws-1" {
		t.Fatalf("subscribed workspace_id = %q, want ws-1", subscribed.GetWorkspaceId())
	}
	var initialRescan agent.FsChangeEvent
	readWorkspaceWatchMessage(t, browserCtx, browser, &initialRescan)
	if initialRescan.GetWorkspaceId() != "ws-1" || initialRescan.GetKind() != agent.FsChangeKind_FS_CHANGE_KIND_RESCAN_REQUIRED {
		t.Fatalf("initial change = %+v, want ws-1 rescan_required", &initialRescan)
	}
	var change agent.FsChangeEvent
	readWorkspaceWatchMessage(t, browserCtx, browser, &change)
	if change.GetWorkspaceId() != "ws-1" || change.GetSequence() != 7 || change.GetKind() != agent.FsChangeKind_FS_CHANGE_KIND_UPDATED || change.GetPath() != "README.md" {
		t.Fatalf("change = %+v, want ws-1 updated README.md at sequence 7", &change)
	}

	if err := browser.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatalf("browser Close() error = %v", err)
	}
	select {
	case closeFrame := <-closed:
		if closeFrame.GetStreamId() != subscribeFrame.GetStreamId() || closeFrame.GetClose().GetReason() != "browser_disconnected" {
			t.Fatalf("close frame stream=%q reason=%q", closeFrame.GetStreamId(), closeFrame.GetClose().GetReason())
		}
	case <-browserCtx.Done():
		t.Fatal("timed out waiting for browser disconnect frame")
	}
	cancel()
	<-agentDone
}

func runWorkspaceWatchAgent(t *testing.T, ctx context.Context, serverUrl string, subscribe chan<- *shared.TunnelFrame, closed chan<- *shared.TunnelFrame) {
	t.Helper()
	conn, _, err := websocket.Dial(ctx, "ws"+serverUrl[len("http"):]+"/api/agent/tunnel", &websocket.DialOptions{HTTPHeader: signedTestTunnelHeader(t)})
	if err != nil {
		t.Errorf("Dial() error = %v", err)
		return
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	hello := &shared.TunnelFrame{
		StreamId: tunnel.ControlStreamID,
		Payload: &shared.TunnelFrame_Hello{Hello: &shared.Hello{
			DeviceId:        "0f490dee643b01b06e0ea84c253a9005",
			DeviceName:      "local",
			ProtocolVersion: tunnel.ProtocolVersion,
		}},
	}
	writeTunnelTestFrame(t, ctx, conn, hello)
	if _, _, err := conn.Read(ctx); err != nil {
		t.Errorf("Read(hello ack) error = %v", err)
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
		switch frame.GetPayload().(type) {
		case *shared.TunnelFrame_FsWatchSubscribeReq:
			subscribe <- frame
			writeTunnelTestFrame(t, ctx, conn, &shared.TunnelFrame{
				StreamId: frame.GetStreamId(),
				Payload: &shared.TunnelFrame_FsWatchSubscribed{
					FsWatchSubscribed: &agent.FsWatchSubscribed{WorkspaceId: "ws-1"},
				},
			})
			writeTunnelTestFrame(t, ctx, conn, &shared.TunnelFrame{
				StreamId: frame.GetStreamId(),
				Payload: &shared.TunnelFrame_FsChangeEvent{
					FsChangeEvent: &agent.FsChangeEvent{
						WorkspaceId: "ws-1",
						Kind:        agent.FsChangeKind_FS_CHANGE_KIND_RESCAN_REQUIRED,
					},
				},
			})
			writeTunnelTestFrame(t, ctx, conn, &shared.TunnelFrame{
				StreamId: frame.GetStreamId(),
				Payload: &shared.TunnelFrame_FsChangeEvent{
					FsChangeEvent: &agent.FsChangeEvent{
						WorkspaceId: "ws-1",
						Sequence:    7,
						Kind:        agent.FsChangeKind_FS_CHANGE_KIND_UPDATED,
						Path:        "README.md",
					},
				},
			})
		case *shared.TunnelFrame_Close:
			closed <- frame
			return
		}
	}
}

func writeTunnelTestFrame(t *testing.T, ctx context.Context, conn *websocket.Conn, frame *shared.TunnelFrame) {
	t.Helper()
	data, err := tunnel.MarshalFrame(frame)
	if err != nil {
		t.Errorf("MarshalFrame() error = %v", err)
		return
	}
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Errorf("Write() error = %v", err)
	}
}

func readWorkspaceWatchMessage(t *testing.T, ctx context.Context, conn *websocket.Conn, target proto.Message) {
	t.Helper()
	messageType, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("browser Read() error = %v", err)
	}
	if messageType != websocket.MessageText {
		t.Fatalf("browser message type = %v, want text", messageType)
	}
	if err := codec.UnmarshalProtoJSON(data, target); err != nil {
		t.Fatalf("decode workspace watch message: %v; body=%s", err, data)
	}
}
