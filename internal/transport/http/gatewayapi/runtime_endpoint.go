package gatewayapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	agentapp "termbridge-go/internal/application/agent"
	terminalapp "termbridge-go/internal/application/terminal"
	terminalproto "termbridge-go/internal/protocol/terminal"
	"termbridge-go/internal/protocol/tunnel"
)

type runtimeEndpoint interface {
	JSON(ctx context.Context, method string, params any, requestId string) (json.RawMessage, error)
	History(ctx context.Context, workspaceId string, sessionId string, requestId string) (text string, offline bool, err error)
	Attach(w http.ResponseWriter, r *http.Request, workspaceId string, sessionId string) error
	Available() bool
}

type localRuntimeEndpoint struct {
	runtime agentapp.RuntimeAccess
	handler *Handler
}

func (e localRuntimeEndpoint) Available() bool {
	return e.runtime != nil
}

func (e localRuntimeEndpoint) JSON(ctx context.Context, method string, params any, requestId string) (json.RawMessage, error) {
	paramsData, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	result, err := agentapp.HandleRuntimeRequest(ctx, e.runtime, tunnel.RequestReq{Method: method, Params: paramsData, RequestId: requestId})
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func (e localRuntimeEndpoint) History(ctx context.Context, workspaceId string, sessionId string, _ string) (string, bool, error) {
	data, err := e.runtime.ReadHistory(ctx, workspaceId, sessionId)
	return string(data), false, err
}

func (e localRuntimeEndpoint) Attach(w http.ResponseWriter, r *http.Request, workspaceId string, sessionId string) error {
	return e.handler.bridgeTerminalStream(w, r, e.runtime, workspaceId, sessionId)
}

type tunnelRuntimeEndpoint struct {
	route    *agentRoute
	deviceId string
	handler  *Handler
}

func (e tunnelRuntimeEndpoint) Available() bool {
	return e.route != nil
}

func (e tunnelRuntimeEndpoint) JSON(ctx context.Context, method string, params any, requestId string) (json.RawMessage, error) {
	return e.route.request(ctx, method, params, requestId)
}

func (e tunnelRuntimeEndpoint) History(ctx context.Context, workspaceId string, sessionId string, requestId string) (string, bool, error) {
	result, err := e.route.request(ctx, "history", terminalapp.WorkspaceSessionReq{WorkspaceId: workspaceId, SessionId: sessionId}, requestId)
	if err != nil {
		return "", false, err
	}
	var text string
	if err := json.Unmarshal(result, &text); err != nil {
		return "", false, err
	}
	return text, false, nil
}

func (e tunnelRuntimeEndpoint) Attach(w http.ResponseWriter, r *http.Request, workspaceId string, sessionId string) error {
	e.handler.handleTerminalWS(w, r, e.route, workspaceId, sessionId)
	return nil
}

func (s *Handler) bridgeTerminalStream(w http.ResponseWriter, r *http.Request, runtime agentapp.RuntimeAccess, workspaceId string, sessionId string) error {
	writerKey := sessionScopeKey(workspaceId, sessionId)
	s.writerMu.Lock()
	if _, exists := s.writers[writerKey]; exists {
		s.writerMu.Unlock()
		s.writeAPIError(w, r, http.StatusConflict, errorCodeConflict, errorMessageConflict, nil)
		return nil
	}
	streamId := tunnel.StreamId("local-term-" + fmt.Sprint(time.Now().UnixNano()))
	s.writers[writerKey] = streamId
	s.writerMu.Unlock()
	defer func() {
		s.writerMu.Lock()
		delete(s.writers, writerKey)
		s.writerMu.Unlock()
	}()

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{terminalproto.Subprotocol}, OriginPatterns: s.originPatterns(r)})
	if err != nil {
		return err
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	cols, rows, hasAttachSize, sizeErr := terminalAttachSizeFromQuery(r)
	if sizeErr != nil {
		s.config.Logger.Warn("terminal attach size invalid", "workspace_id", workspaceId, "session_id", sessionId, "error", sizeErr)
		_ = writeTerminalControl(conn, terminalproto.ServerMessage{Type: terminalproto.TypeError, Code: "bad_control", Message: sizeErr.Error()})
		return nil
	}

	stream, err := runtime.Attach(r.Context(), workspaceId, sessionId)
	if err != nil {
		return err
	}
	defer stream.Detach("gateway_detached")
	if hasAttachSize {
		if err := stream.Resize(cols, rows); err != nil {
			return err
		}
	}

	done := make(chan struct{})
	var closeDone sync.Once
	stop := func() { closeDone.Do(func() { close(done) }) }

	go func() {
		defer stop()
		for {
			messageType, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			switch messageType {
			case websocket.MessageText:
				message, err := terminalproto.DecodeClient(data)
				if err != nil {
					_ = writeTerminalControl(conn, terminalproto.ServerMessage{Type: terminalproto.TypeError, Code: "bad_control", Message: err.Error()})
					continue
				}
				switch message.Type {
				case terminalproto.TypeHello:
					continue
				case terminalproto.TypeResize:
					if err := stream.Resize(message.Cols, message.Rows); err != nil {
						s.config.Logger.Warn("terminal resize failed", "workspace_id", workspaceId, "session_id", sessionId, "cols", message.Cols, "rows", message.Rows, "error", err)
					}
				case terminalproto.TypeDetach:
					stream.Detach("browser_detached")
					return
				case terminalproto.TypePing:
					_ = writeTerminalControl(conn, terminalproto.ServerMessage{Type: terminalproto.TypePong, Nonce: message.Nonce})
				}
			case websocket.MessageBinary:
				if err := stream.WriteInput(data); err != nil {
					s.config.Logger.Warn("terminal input write failed", "workspace_id", workspaceId, "session_id", sessionId, "bytes", len(data), "error", err)
				}
			}
		}
	}()

	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-done:
			return nil
		case outbound, ok := <-stream.Outbound():
			if !ok {
				_ = conn.Close(websocket.StatusNormalClosure, "terminal closed")
				return nil
			}
			if outbound.Kind == terminalapp.OutboundBinary {
				if err := conn.Write(r.Context(), websocket.MessageBinary, outbound.Binary); err != nil {
					return err
				}
				continue
			}
			data, err := terminalproto.EncodeServer(outbound.Text)
			if err != nil {
				return err
			}
			if err := conn.Write(r.Context(), websocket.MessageText, data); err != nil {
				return err
			}
		}
	}
}
