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

	"termbridge-go/internal/tunnel"
	"termbridge-go/internal/webterminal"
)

type Config struct {
	GatewayURL string
	DeviceName string
	StateDir   string
	Runtime    RuntimeAccess
	Logger     *slog.Logger
}

type Client struct {
	config Config
	device Device
	mu     sync.Mutex
	terms  map[tunnel.StreamID]chan tunnel.Frame
}

func New(config Config) *Client {
	return &Client{config: config, terms: map[tunnel.StreamID]chan tunnel.Frame{}}
}

func (c *Client) Run(ctx context.Context) error {
	device, err := LoadOrCreateDevice(DeviceOptions{StateDir: c.config.StateDir, DeviceName: c.config.DeviceName})
	if err != nil {
		return err
	}
	c.device = device
	conn, _, err := websocket.Dial(ctx, tunnelURL(c.config.GatewayURL), &websocket.DialOptions{HTTPHeader: basicAuthHeader("admin", "admin")})
	if err != nil {
		return err
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	go func() {
		<-ctx.Done()
		_ = conn.CloseNow()
	}()
	hello, err := tunnel.NewFrame(tunnel.ControlStreamID, tunnel.FrameHello, tunnel.HelloPayload{DeviceID: device.ID, DeviceName: device.Name, ProtocolVersion: tunnel.ProtocolVersion})
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
			pong, _ := tunnel.NewFrame(tunnel.ControlStreamID, tunnel.FramePong, nil)
			_ = writeTunnelFrame(ctx, conn, &writeMu, pong)
		}
	}
}

func (c *Client) handleRequest(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, frame tunnel.Frame) {
	request, err := tunnel.DecodePayload[tunnel.RequestPayload](frame)
	if err != nil {
		c.writeResponse(ctx, conn, writeMu, frame.StreamID, nil, err)
		return
	}
	if c.config.Runtime == nil {
		c.writeResponse(ctx, conn, writeMu, frame.StreamID, nil, fmt.Errorf("runtime access unavailable"))
		return
	}
	var result any
	switch request.Method {
	case "workspace_tree":
		result, err = c.config.Runtime.WorkspaceTree(ctx)
	case "sessions":
		result, err = c.config.Runtime.ListSessions(ctx)
	case "history":
		var params struct {
			SessionID string `json:"session_id"`
		}
		if len(request.Params) > 0 {
			err = json.Unmarshal(request.Params, &params)
		}
		if err == nil {
			var data []byte
			data, err = c.config.Runtime.ReadHistory(ctx, params.SessionID)
			result = string(data)
		}
	default:
		err = fmt.Errorf("unknown request method: %s", request.Method)
	}
	c.writeResponse(ctx, conn, writeMu, frame.StreamID, result, err)
}

func (c *Client) writeResponse(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, streamID tunnel.StreamID, result any, err error) {
	response := tunnel.ResponsePayload{OK: err == nil}
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
	frame, frameErr := tunnel.NewFrame(streamID, tunnel.FrameResponse, response)
	if frameErr != nil {
		return
	}
	_ = writeTunnelFrame(ctx, conn, writeMu, frame)
}

func (c *Client) handleTerminal(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, frame tunnel.Frame) {
	if c.config.Runtime == nil {
		_ = writeTerminalError(ctx, conn, writeMu, frame.StreamID, "runtime access unavailable")
		return
	}
	payload, err := tunnel.DecodePayload[tunnel.TerminalAttachPayload](frame)
	if err != nil {
		_ = writeTerminalError(ctx, conn, writeMu, frame.StreamID, err.Error())
		return
	}
	stream, err := c.config.Runtime.Attach(ctx, payload.SessionID)
	if err != nil {
		_ = writeTerminalError(ctx, conn, writeMu, frame.StreamID, err.Error())
		return
	}
	defer stream.Detach("gateway_detached")
	if payload.Cols > 0 && payload.Rows > 0 {
		_ = stream.Resize(payload.Cols, payload.Rows)
	}
	inbound := make(chan tunnel.Frame, 16)
	c.addTerminal(frame.StreamID, inbound)
	defer c.removeTerminal(frame.StreamID)
	for {
		select {
		case <-ctx.Done():
			return
		case inboundFrame := <-inbound:
			switch inboundFrame.Type {
			case tunnel.FrameTerminalInput:
				input, err := tunnel.DecodePayload[tunnel.TerminalDataPayload](inboundFrame)
				if err == nil {
					_ = stream.WriteInput([]byte(input.Data))
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
				closed, _ := tunnel.NewFrame(frame.StreamID, tunnel.FrameTerminalClosed, nil)
				_ = writeTunnelFrame(ctx, conn, writeMu, closed)
				return
			}
			data := outboundData(outbound)
			output, _ := tunnel.NewFrame(frame.StreamID, tunnel.FrameTerminalOutput, tunnel.TerminalDataPayload{Data: string(data)})
			_ = writeTunnelFrame(ctx, conn, writeMu, output)
		}
	}
}

func (c *Client) dispatchTerminal(frame tunnel.Frame) bool {
	c.mu.Lock()
	ch := c.terms[frame.StreamID]
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

func (c *Client) addTerminal(streamID tunnel.StreamID, ch chan tunnel.Frame) {
	c.mu.Lock()
	c.terms[streamID] = ch
	c.mu.Unlock()
}

func (c *Client) removeTerminal(streamID tunnel.StreamID) {
	c.mu.Lock()
	delete(c.terms, streamID)
	c.mu.Unlock()
}

func writeTerminalError(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, streamID tunnel.StreamID, message string) error {
	frame, err := tunnel.NewFrame(streamID, tunnel.FrameError, tunnel.ErrorPayload{Message: message})
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

func outboundData(outbound webterminal.Outbound) []byte {
	if outbound.Kind == webterminal.OutboundBinary {
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

func tunnelURL(gatewayURL string) string {
	parsed, err := url.Parse(gatewayURL)
	if err != nil {
		return gatewayURL
	}
	if parsed.Scheme == "https" {
		parsed.Scheme = "wss"
	} else {
		parsed.Scheme = "ws"
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/gateway/agent/tunnel"
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
