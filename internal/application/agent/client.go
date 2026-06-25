package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/coder/websocket"

	terminalapp "termbridge-go/internal/application/terminal"
	"termbridge-go/internal/protocol/tunnel"
)

type Config struct {
	ConnectUrl string
	Username   string
	Password   string
	DeviceId   string
	DeviceName string
	StateDir   string
	Runtime    RuntimeAccess
	Logger     *slog.Logger
}

type Client struct {
	config Config
	device Device
	mu     sync.Mutex
	terms  map[tunnel.StreamId]chan tunnel.Frame
}

func New(config Config) *Client {
	return &Client{config: config, terms: map[tunnel.StreamId]chan tunnel.Frame{}}
}

func (c *Client) logInfo(message string, attrs ...any) {
	c.config.Logger.Info(message, attrs...)
}

func (c *Client) logWarn(message string, attrs ...any) {
	c.config.Logger.Warn(message, attrs...)
}

func (c *Client) Run(ctx context.Context) error {
	device, err := LoadOrCreateDevice(DeviceOptions{StateDir: c.config.StateDir, DeviceId: c.config.DeviceId, DeviceName: c.config.DeviceName})
	if err != nil {
		return err
	}
	c.device = device
	conn, _, err := websocket.Dial(ctx, tunnelUrl(c.config.ConnectUrl), &websocket.DialOptions{HTTPHeader: basicAuthHeader(c.config.Username, c.config.Password)})
	if err != nil {
		return err
	}
	conn.SetReadLimit(tunnel.MaxFrameBytes)
	defer conn.Close(websocket.StatusNormalClosure, "")
	go func() {
		<-ctx.Done()
		_ = conn.CloseNow()
	}()
	hello, err := tunnel.NewFrame(tunnel.ControlStreamId, tunnel.FrameHello, tunnel.HelloPayload{DeviceId: device.Id, DeviceName: device.Name, ProtocolVersion: tunnel.ProtocolVersion})
	if err != nil {
		return err
	}
	helloData, err := tunnel.Encode(hello)
	if err != nil {
		return err
	}
	if err := conn.Write(ctx, websocket.MessageText, helloData); err != nil {
		return err
	}
	_, ackData, err := conn.Read(ctx)
	if err != nil {
		return err
	}
	ack, err := tunnel.Decode(ackData)
	if err != nil {
		return err
	}
	if ack.Type != tunnel.FrameHelloAck {
		return websocket.CloseError{Code: websocket.StatusPolicyViolation, Reason: "expected hello_ack"}
	}
	return c.readLoop(ctx, conn)
}

func (c *Client) readLoop(ctx context.Context, conn *websocket.Conn) error {
	var writeMu sync.Mutex
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		frame, err := tunnel.Decode(data)
		if err != nil {
			continue
		}
		if c.dispatchTerminal(frame) {
			continue
		}
		switch frame.Type {
		case tunnel.FrameRequest:
			go c.handleRequest(ctx, conn, &writeMu, frame)
		case tunnel.FrameTerminalAttach:
			go c.handleTerminal(ctx, conn, &writeMu, frame)
		case tunnel.FramePing:
			pong, _ := tunnel.NewFrame(tunnel.ControlStreamId, tunnel.FramePong, nil)
			_ = writeTunnelFrame(ctx, conn, &writeMu, pong)
		}
	}
}

func (c *Client) handleRequest(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, frame tunnel.Frame) {
	request, err := tunnel.DecodePayload[tunnel.RequestReq](frame)
	if err != nil {
		c.logWarn("agent tunnel request decode failed", "stream_id", frame.StreamId, "error", err)
		c.writeResponse(ctx, conn, writeMu, frame.StreamId, nil, err)
		return
	}
	attrs := []any{"stream_id", frame.StreamId, "method", request.Method, "request_id", request.RequestId}
	if c.config.Runtime == nil {
		err := fmt.Errorf("runtime access unavailable")
		c.logWarn("agent tunnel request failed", append(attrs, "error", err)...)
		c.writeResponse(ctx, conn, writeMu, frame.StreamId, nil, err)
		return
	}
	result, err := c.handleRuntimeRequest(ctx, request)
	if err != nil {
		c.logWarn("agent tunnel request failed", append(attrs, "error", err)...)
	} else {
		c.logInfo("agent tunnel request handled", attrs...)
	}
	c.writeResponse(ctx, conn, writeMu, frame.StreamId, result, err)
}

func (c *Client) handleRuntimeRequest(ctx context.Context, request tunnel.RequestReq) (any, error) {
	switch request.Method {
	case "workspaces":
		return c.config.Runtime.ListWorkspaces(ctx)
	case "workspace_tree":
		return c.config.Runtime.WorkspaceTree(ctx)
	case "workspace_order":
		var params struct {
			WorkspaceIds []string `json:"workspace_ids"`
		}
		if err := decodeRequestParams(request.Params, &params); err != nil {
			return nil, err
		}
		return c.config.Runtime.UpdateWorkspaceOrder(ctx, params.WorkspaceIds)
	case "delete_workspace":
		var params struct {
			WorkspaceId string `json:"workspace_id"`
		}
		if err := decodeRequestParams(request.Params, &params); err != nil {
			return nil, err
		}
		return nil, c.config.Runtime.DeleteWorkspace(ctx, params.WorkspaceId)
	case "workspace_sessions":
		var params struct {
			WorkspaceId string `json:"workspace_id"`
		}
		if err := decodeRequestParams(request.Params, &params); err != nil {
			return nil, err
		}
		return c.config.Runtime.ListSessionsByWorkspaceId(ctx, params.WorkspaceId)
	case "create_session":
		var params terminalapp.CreateSessionReq
		if err := decodeRequestParams(request.Params, &params); err != nil {
			return nil, err
		}
		return c.config.Runtime.CreateSession(ctx, params)
	case "rerun_session":
		var params struct {
			WorkspaceId string                      `json:"workspace_id"`
			SessionId   string                      `json:"session_id"`
			Request     terminalapp.RerunSessionReq `json:"request"`
		}
		if err := decodeRequestParams(request.Params, &params); err != nil {
			return nil, err
		}
		return c.config.Runtime.RerunSession(ctx, params.WorkspaceId, params.SessionId, params.Request)
	case "get_session":
		var params struct {
			WorkspaceId string `json:"workspace_id"`
			SessionId   string `json:"session_id"`
		}
		if err := decodeRequestParams(request.Params, &params); err != nil {
			return nil, err
		}
		return c.config.Runtime.GetSession(ctx, params.WorkspaceId, params.SessionId)
	case "update_session":
		var params struct {
			WorkspaceId string                       `json:"workspace_id"`
			SessionId   string                       `json:"session_id"`
			Request     terminalapp.UpdateSessionReq `json:"request"`
		}
		if err := decodeRequestParams(request.Params, &params); err != nil {
			return nil, err
		}
		return c.config.Runtime.UpdateSession(ctx, params.WorkspaceId, params.SessionId, params.Request)
	case "delete_session":
		var params struct {
			WorkspaceId string `json:"workspace_id"`
			SessionId   string `json:"session_id"`
		}
		if err := decodeRequestParams(request.Params, &params); err != nil {
			return nil, err
		}
		return nil, c.config.Runtime.DeleteSession(ctx, params.WorkspaceId, params.SessionId)
	case "close_session":
		var params struct {
			WorkspaceId string `json:"workspace_id"`
			SessionId   string `json:"session_id"`
		}
		if err := decodeRequestParams(request.Params, &params); err != nil {
			return nil, err
		}
		return c.config.Runtime.CloseSession(ctx, params.WorkspaceId, params.SessionId)
	case "history":
		var params struct {
			WorkspaceId string `json:"workspace_id"`
			SessionId   string `json:"session_id"`
		}
		if err := decodeRequestParams(request.Params, &params); err != nil {
			return nil, err
		}
		data, err := c.config.Runtime.ReadHistory(ctx, params.WorkspaceId, params.SessionId)
		return string(data), err
	default:
		return nil, fmt.Errorf("unknown request method: %s", request.Method)
	}
}

func decodeRequestParams(params json.RawMessage, value any) error {
	if len(params) == 0 {
		params = []byte(`{}`)
	}
	return json.Unmarshal(params, value)
}

func (c *Client) writeResponse(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, streamId tunnel.StreamId, result any, err error) {
	response := tunnel.ResponseResp{OK: err == nil}
	if err != nil {
		response.Error = err.Error()
	} else {
		data, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			response.OK = false
			response.Error = marshalErr.Error()
		} else {
			response.Result = data
		}
	}
	frame, frameErr := tunnel.NewFrame(streamId, tunnel.FrameResponse, response)
	if frameErr != nil {
		return
	}
	_ = writeTunnelFrame(ctx, conn, writeMu, frame)
}

func (c *Client) handleTerminal(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, frame tunnel.Frame) {
	payload, err := tunnel.DecodePayload[tunnel.TerminalAttachReq](frame)
	if err != nil {
		c.logWarn("agent terminal attach decode failed", "stream_id", frame.StreamId, "error", err)
		_ = writeTerminalError(ctx, conn, writeMu, frame.StreamId, err.Error())
		return
	}
	attrs := []any{"stream_id", frame.StreamId, "workspace_id", payload.WorkspaceId, "session_id", payload.SessionId, "request_id", payload.RequestId}
	if c.config.Runtime == nil {
		err := fmt.Errorf("runtime access unavailable")
		c.logWarn("agent terminal attach failed", append(attrs, "error", err)...)
		_ = writeTerminalError(ctx, conn, writeMu, frame.StreamId, err.Error())
		return
	}
	stream, err := c.config.Runtime.Attach(ctx, payload.WorkspaceId, payload.SessionId)
	if err != nil {
		c.logWarn("agent terminal attach failed", append(attrs, "error", err)...)
		_ = writeTerminalError(ctx, conn, writeMu, frame.StreamId, err.Error())
		return
	}
	c.logInfo("agent terminal attached", attrs...)
	defer func() {
		stream.Detach("gateway_detached")
		c.logInfo("agent terminal detached", attrs...)
	}()
	if payload.Cols > 0 && payload.Rows > 0 {
		_ = stream.Resize(payload.Cols, payload.Rows)
	}
	inbound := make(chan tunnel.Frame, 16)
	c.addTerminal(frame.StreamId, inbound)
	defer c.removeTerminal(frame.StreamId)
	for {
		select {
		case <-ctx.Done():
			return
		case inboundFrame := <-inbound:
			switch inboundFrame.Type {
			case tunnel.FrameTerminalInput:
				input, err := tunnel.DecodePayload[tunnel.TerminalDataPayload](inboundFrame)
				if err == nil {
					_ = stream.WriteInput(input.Data)
				}
			case tunnel.FrameTerminalResize:
				resize, err := tunnel.DecodePayload[tunnel.TerminalResizePayload](inboundFrame)
				if err == nil {
					_ = stream.Resize(resize.Cols, resize.Rows)
				}
			case tunnel.FrameClose:
				return
			}
		case outbound, ok := <-stream.Outbound():
			if !ok {
				closed, _ := tunnel.NewFrame(frame.StreamId, tunnel.FrameTerminalClosed, nil)
				_ = writeTunnelFrame(ctx, conn, writeMu, closed)
				return
			}
			data := outboundData(outbound)
			output, _ := tunnel.NewFrame(frame.StreamId, tunnel.FrameTerminalOutput, tunnel.TerminalDataPayload{Data: data})
			_ = writeTunnelFrame(ctx, conn, writeMu, output)
		}
	}
}

func (c *Client) dispatchTerminal(frame tunnel.Frame) bool {
	c.mu.Lock()
	ch := c.terms[frame.StreamId]
	c.mu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case ch <- frame:
	default:
	}
	return true
}

func (c *Client) addTerminal(streamId tunnel.StreamId, ch chan tunnel.Frame) {
	c.mu.Lock()
	c.terms[streamId] = ch
	c.mu.Unlock()
}

func (c *Client) removeTerminal(streamId tunnel.StreamId) {
	c.mu.Lock()
	delete(c.terms, streamId)
	c.mu.Unlock()
}

func writeTerminalError(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, streamId tunnel.StreamId, message string) error {
	frame, err := tunnel.NewFrame(streamId, tunnel.FrameError, tunnel.ErrorPayload{Message: message})
	if err != nil {
		return err
	}
	return writeTunnelFrame(ctx, conn, writeMu, frame)
}

func writeTunnelFrame(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, frame tunnel.Frame) error {
	data, err := tunnel.Encode(frame)
	if err != nil {
		return err
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	return conn.Write(ctx, websocket.MessageText, data)
}

func outboundData(outbound terminalapp.Outbound) []byte {
	if outbound.Kind == terminalapp.OutboundBinary {
		return outbound.Binary
	}
	return []byte(outbound.Text.Message)
}

func (c *Client) Config() Config {
	return c.config
}

func (c *Client) Device() Device {
	return c.device
}

func tunnelUrl(serverUrl string) string {
	parsed, err := url.Parse(serverUrl)
	if err != nil {
		return serverUrl
	}
	if parsed.Scheme == "https" {
		parsed.Scheme = "wss"
	} else {
		parsed.Scheme = "ws"
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/agent/tunnel"
	parsed.RawQuery = ""
	return parsed.String()
}

func basicAuthHeader(username string, password string) http.Header {
	header := http.Header{}
	req, _ := http.NewRequest(http.MethodGet, "http://termbridge.local", nil)
	req.SetBasicAuth(username, password)
	header.Set("Authorization", req.Header.Get("Authorization"))
	return header
}
