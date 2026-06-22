package tunnel

import (
	"encoding/json"
	"fmt"
)

const ProtocolVersion = 2

const ControlStreamID StreamID = "control"

type StreamID string

type FrameType string

const (
	FrameHello          FrameType = "hello"
	FrameHelloAck       FrameType = "hello_ack"
	FramePing           FrameType = "ping"
	FramePong           FrameType = "pong"
	FrameRequest        FrameType = "request"
	FrameResponse       FrameType = "response"
	FrameTerminalAttach FrameType = "terminal_attach"
	FrameTerminalInput  FrameType = "terminal_input"
	FrameTerminalOutput FrameType = "terminal_output"
	FrameTerminalResize FrameType = "terminal_resize"
	FrameTerminalClosed FrameType = "terminal_closed"
	FrameError          FrameType = "error"
	FrameClose          FrameType = "close"
)

type Frame struct {
	StreamID StreamID        `json:"stream_id"`
	Type     FrameType       `json:"type"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

type HelloPayload struct {
	DeviceID        string `json:"device_id"`
	DeviceName      string `json:"device_name"`
	ProtocolVersion int    `json:"protocol_version"`
}

type HelloAckPayload struct {
	ProtocolVersion int `json:"protocol_version"`
}

type ErrorPayload struct {
	Message string `json:"message"`
}

type RequestPayload struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type ResponsePayload struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type TerminalAttachPayload struct {
	SessionID string `json:"session_id"`
	Cols      int    `json:"cols,omitempty"`
	Rows      int    `json:"rows,omitempty"`
}

type TerminalDataPayload struct {
	Data []byte `json:"data"`
}

type TerminalResizePayload struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

func NewFrame(streamID StreamID, frameType FrameType, payload any) (Frame, error) {
	frame := Frame{StreamID: streamID, Type: frameType}
	if payload == nil {
		return frame, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Frame{}, fmt.Errorf("marshal tunnel payload: %w", err)
	}
	frame.Payload = data
	return frame, nil
}

func Encode(frame Frame) ([]byte, error) {
	if err := Validate(frame); err != nil {
		return nil, err
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return nil, fmt.Errorf("marshal tunnel frame: %w", err)
	}
	return data, nil
}

func Decode(data []byte) (Frame, error) {
	var frame Frame
	if err := json.Unmarshal(data, &frame); err != nil {
		return Frame{}, fmt.Errorf("decode tunnel frame: %w", err)
	}
	if err := Validate(frame); err != nil {
		return Frame{}, err
	}
	return frame, nil
}

func DecodePayload[T any](frame Frame) (T, error) {
	var payload T
	if len(frame.Payload) == 0 {
		return payload, nil
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return payload, fmt.Errorf("decode tunnel payload: %w", err)
	}
	return payload, nil
}

func Validate(frame Frame) error {
	if frame.StreamID == "" {
		return fmt.Errorf("tunnel frame missing stream_id")
	}
	if !knownFrameType(frame.Type) {
		return fmt.Errorf("unknown tunnel frame type: %s", frame.Type)
	}
	if frame.Type == FrameHello {
		payload, err := DecodePayload[HelloPayload](frame)
		if err != nil {
			return err
		}
		if payload.ProtocolVersion != ProtocolVersion {
			return fmt.Errorf("unsupported tunnel protocol version: %d", payload.ProtocolVersion)
		}
	}
	return nil
}

func knownFrameType(frameType FrameType) bool {
	switch frameType {
	case FrameHello, FrameHelloAck, FramePing, FramePong, FrameRequest, FrameResponse, FrameTerminalAttach, FrameTerminalInput, FrameTerminalOutput, FrameTerminalResize, FrameTerminalClosed, FrameError, FrameClose:
		return true
	default:
		return false
	}
}
