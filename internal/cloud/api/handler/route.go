package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"

	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	terminalproto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/terminal"
	tunnel "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
)

const requestTimeout = 5 * time.Second

type agentRoute struct {
	deviceId string
	conn     *websocket.Conn
	writeMu  sync.Mutex
	pending  map[string]chan *shared.TunnelFrame
	terms    map[string]*terminalRelay
	mu       sync.Mutex
}

type terminalRelay struct {
	sessionId string
	browser   *websocket.Conn
	done      chan struct{}
	logger    *slog.Logger
}

func newAgentRoute(deviceId string, conn *websocket.Conn) *agentRoute {
	return &agentRoute{deviceId: deviceId, conn: conn, pending: map[string]chan *shared.TunnelFrame{}, terms: map[string]*terminalRelay{}}
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
		if responseFrame.GetError() != nil {
			return nil, errors.New(tunnel.ErrorMessage(responseFrame))
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
	defer r.mu.Unlock()
	if ch := r.pending[frame.GetStreamId()]; ch != nil && isRuntimeResponse(frame) {
		ch <- frame
		return true
	}
	if term := r.terms[frame.GetStreamId()]; term != nil {
		return term.dispatch(frame)
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

func (r *agentRoute) closeTerminals(reason string) {
	r.mu.Lock()
	terms := make([]*terminalRelay, 0, len(r.terms))
	for _, term := range r.terms {
		terms = append(terms, term)
	}
	r.terms = map[string]*terminalRelay{}
	r.mu.Unlock()
	for _, term := range terms {
		_ = writeTerminalControl(term.browser, &agent.ServerControlMessage{Type: terminalproto.TypeError, Code: "device_disconnected", Message: reason})
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
		*shared.TunnelFrame_DeleteWorkspaceResp:
		return true
	default:
		return false
	}
}

func closeOnce(ch chan struct{}) {
	defer func() { _ = recover() }()
	close(ch)
}
