package api

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"

	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
	terminalproto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/terminal"
	tunnel "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
	sharedconfig "gitee.com/leoninew/TermBridge-go/internal/shared/infrastructure/config"
)

const (
	requestTransportAllowance = 5 * time.Second
	requestTimeout            = sharedconfig.MaxGitCommandTimeout + requestTransportAllowance
)

type agentRoute struct {
	deviceId string
	conn     *websocket.Conn
	writeMu  sync.Mutex
	pending  map[string]chan *shared.TunnelFrame
	terms    map[string]*terminalRelay
	watches  map[string]*workspaceWatchRelay
	mu       sync.Mutex
}

type terminalRelay struct {
	sessionId string
	browser   *websocket.Conn
	done      chan struct{}
	logger    *slog.Logger
}

type workspaceWatchRelay struct {
	workspaceId string
	browser     *websocket.Conn
	done        chan struct{}
	logger      *slog.Logger
	writeMu     sync.Mutex
}

func newAgentRoute(deviceId string, conn *websocket.Conn) *agentRoute {
	return &agentRoute{
		deviceId: deviceId,
		conn:     conn,
		pending:  map[string]chan *shared.TunnelFrame{},
		terms:    map[string]*terminalRelay{},
		watches:  map[string]*workspaceWatchRelay{},
	}
}

func (r *agentRoute) request(ctx context.Context, frame *shared.TunnelFrame) (*shared.TunnelFrame, error) {
	if frame.GetStreamId() == "" {
		frame.StreamId = fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	ch := make(chan *shared.TunnelFrame, 1)
	r.mu.Lock()
	r.pending[frame.GetStreamId()] = ch
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.pending, frame.GetStreamId())
		r.mu.Unlock()
	}()
	if err := r.writeFrame(ctx, frame); err != nil {
		return nil, err
	}
	waitCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	select {
	case <-waitCtx.Done():
		return nil, waitCtx.Err()
	case responseFrame := <-ch:
		if remoteErr, ok := tunnel.RemoteErrorFromFrame(responseFrame); ok {
			return nil, remoteErr
		}
		return responseFrame, nil
	}
}

func (r *agentRoute) writeFrame(ctx context.Context, frame *shared.TunnelFrame) error {
	data, err := tunnel.MarshalFrame(frame)
	if err != nil {
		return err
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	return r.conn.Write(ctx, websocket.MessageText, data)
}

func (r *agentRoute) dispatch(frame *shared.TunnelFrame) bool {
	r.mu.Lock()
	if ch := r.pending[frame.GetStreamId()]; ch != nil && isRuntimeResponse(frame) {
		r.mu.Unlock()
		select {
		case ch <- frame:
		default:
		}
		return true
	}
	term := r.terms[frame.GetStreamId()]
	watch := r.watches[frame.GetStreamId()]
	r.mu.Unlock()
	if term != nil {
		return term.dispatch(frame)
	}
	if watch != nil {
		return watch.dispatch(frame)
	}
	return false
}

func (r *agentRoute) addTerminal(streamId string, term *terminalRelay) {
	r.mu.Lock()
	r.terms[streamId] = term
	r.mu.Unlock()
}

func (r *agentRoute) removeTerminal(streamId string) {
	r.mu.Lock()
	delete(r.terms, streamId)
	r.mu.Unlock()
}

func (r *agentRoute) addWorkspaceWatch(streamId string, watch *workspaceWatchRelay) {
	r.mu.Lock()
	r.watches[streamId] = watch
	r.mu.Unlock()
}

func (r *agentRoute) removeWorkspaceWatch(streamId string) {
	r.mu.Lock()
	delete(r.watches, streamId)
	r.mu.Unlock()
}

func (r *agentRoute) closeWorkspaceWatches(reason string) {
	r.mu.Lock()
	watches := make([]*workspaceWatchRelay, 0, len(r.watches))
	for _, watch := range r.watches {
		watches = append(watches, watch)
	}
	r.watches = map[string]*workspaceWatchRelay{}
	r.mu.Unlock()
	for _, watch := range watches {
		watch.writeMu.Lock()
		_ = watch.browser.Close(websocket.StatusGoingAway, reason)
		watch.writeMu.Unlock()
		closeOnce(watch.done)
	}
}

func (r *agentRoute) closeTerminals(reason string) {
	r.mu.Lock()
	terms := make([]*terminalRelay, 0, len(r.terms))
	for _, term := range r.terms {
		terms = append(terms, term)
	}
	r.terms = map[string]*terminalRelay{}
	r.mu.Unlock()
	for _, term := range terms {
		code, message := terminalproto.BrowserDisconnectError(reason)
		_ = writeTerminalControl(term.browser, &agent.ServerControlMessage{Type: terminalproto.TypeError, Code: code, Message: message})
		_ = term.browser.Close(websocket.StatusGoingAway, reason)
		closeOnce(term.done)
	}
}

func (t *terminalRelay) dispatch(frame *shared.TunnelFrame) bool {
	switch payload := frame.GetPayload().(type) {
	case *shared.TunnelFrame_TerminalOutput:
		data := payload.TerminalOutput.GetData()
		if err := t.browser.Write(context.Background(), websocket.MessageBinary, data); err != nil && t.logger != nil {
			t.logger.Warn("terminal websocket output write failed", "session_id", t.sessionId, "stream_id", frame.GetStreamId(), "error", err)
		}
		return true
	case *shared.TunnelFrame_TerminalControl:
		if err := writeTerminalControl(t.browser, payload.TerminalControl); err != nil {
			if t.logger != nil {
				t.logger.Warn("terminal websocket control write failed", "session_id", t.sessionId, "stream_id", frame.GetStreamId(), "error", err)
			}
			_ = t.browser.Close(websocket.StatusInternalError, "terminal control relay failed")
			closeOnce(t.done)
		}
		return true
	case *shared.TunnelFrame_Error:
		message := tunnel.ErrorMessage(frame)
		if t.logger != nil {
			t.logger.Warn("terminal stream error", "session_id", t.sessionId, "stream_id", frame.GetStreamId(), "message", message)
		}
		_ = writeTerminalControl(t.browser, &agent.ServerControlMessage{Type: terminalproto.TypeError, Code: "terminal_stream_error", Message: message})
		_ = t.browser.Close(websocket.StatusNormalClosure, "terminal closed")
		closeOnce(t.done)
		return true
	case *shared.TunnelFrame_TerminalClosed, *shared.TunnelFrame_Close:
		_ = t.browser.Close(websocket.StatusNormalClosure, "terminal closed")
		closeOnce(t.done)
		return true
	default:
		return false
	}
}

func workspaceWatchJSON(frame *shared.TunnelFrame) ([]byte, error) {
	if subscribed := frame.GetFsWatchSubscribed(); subscribed != nil {
		return codec.MarshalProtoJSON(subscribed)
	}
	if event := frame.GetFsChangeEvent(); event != nil {
		return codec.MarshalProtoJSON(event)
	}
	return nil, fmt.Errorf("unsupported workspace watch payload %T", frame.GetPayload())
}

func (w *workspaceWatchRelay) writeBrowser(data []byte) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	return w.browser.Write(context.Background(), websocket.MessageText, data)
}

func (w *workspaceWatchRelay) dispatch(frame *shared.TunnelFrame) bool {
	switch frame.GetPayload().(type) {
	case *shared.TunnelFrame_FsWatchSubscribed, *shared.TunnelFrame_FsChangeEvent:
		data, err := workspaceWatchJSON(frame)
		if err != nil {
			if w.logger != nil {
				w.logger.Warn("workspace watch frame encode failed", "stream_id", frame.GetStreamId(), "error", err)
			}
			w.writeMu.Lock()
			_ = w.browser.Close(websocket.StatusInternalError, "workspace watch relay failed")
			w.writeMu.Unlock()
			closeOnce(w.done)
			return true
		}
		if err := w.writeBrowser(data); err != nil {
			if w.logger != nil {
				w.logger.Warn("workspace watch browser write failed", "stream_id", frame.GetStreamId(), "error", err)
			}
			w.writeMu.Lock()
			_ = w.browser.Close(websocket.StatusInternalError, "workspace watch relay failed")
			w.writeMu.Unlock()
			closeOnce(w.done)
		}
		return true
	case *shared.TunnelFrame_Error, *shared.TunnelFrame_Close:
		w.writeMu.Lock()
		_ = w.browser.Close(websocket.StatusGoingAway, "workspace watch closed")
		w.writeMu.Unlock()
		closeOnce(w.done)
		return true
	default:
		return false
	}
}

func isRuntimeResponse(frame *shared.TunnelFrame) bool {
	if frame.GetError() != nil {
		return true
	}
	switch frame.GetPayload().(type) {
	case *shared.TunnelFrame_ListWorkspacesResp,
		*shared.TunnelFrame_WorkspaceTreeResp,
		*shared.TunnelFrame_WorkspaceSessionsResp,
		*shared.TunnelFrame_CreateSessionResp,
		*shared.TunnelFrame_GetSessionResp,
		*shared.TunnelFrame_RerunSessionResp,
		*shared.TunnelFrame_UpdateSessionResp,
		*shared.TunnelFrame_CloseSessionResp,
		*shared.TunnelFrame_DeleteSessionResp,
		*shared.TunnelFrame_ReadHistoryResp,
		*shared.TunnelFrame_UpdateWorkspaceOrderResp,
		*shared.TunnelFrame_UpdateSessionOrderResp,
		*shared.TunnelFrame_DeleteWorkspaceResp,
		*shared.TunnelFrame_ListShortcutsResp,
		*shared.TunnelFrame_CreateShortcutResp,
		*shared.TunnelFrame_UpdateShortcutResp,
		*shared.TunnelFrame_UpdateShortcutOrderResp,
		*shared.TunnelFrame_DeleteShortcutResp,
		*shared.TunnelFrame_FsStatResp,
		*shared.TunnelFrame_FsReadDirectoryResp,
		*shared.TunnelFrame_FsReadFileResp,
		*shared.TunnelFrame_FsWriteFileResp,
		*shared.TunnelFrame_FsCreateDirectoryResp,
		*shared.TunnelFrame_FsDeleteResp,
		*shared.TunnelFrame_FsRenameResp,
		*shared.TunnelFrame_ScmStatusResp,
		*shared.TunnelFrame_ScmOriginalContentResp,
		*shared.TunnelFrame_ScmExecuteResp,
		*shared.TunnelFrame_ScmRepositoryResp:
		return true
	default:
		return false
	}
}

func closeOnce(ch chan struct{}) {
	defer func() { _ = recover() }()
	close(ch)
}
