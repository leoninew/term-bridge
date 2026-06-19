package terminalproto

import (
	"encoding/json"
	"fmt"
)

const (
	Subprotocol = "termbridge.terminal.v1"

	TypeHello  = "hello"
	TypeResize = "resize"
	TypeDetach = "detach"
	TypePing   = "ping"

	TypeStarted        = "started"
	TypeReplayStarted  = "replay_started"
	TypeReplayFinished = "replay_finished"
	TypeState          = "state"
	TypeExited         = "exited"
	TypeError          = "error"
	TypePong           = "pong"

	MaxJSONMessageBytes = 16 * 1024
	MaxBinaryFrameBytes = 1024 * 1024
	MinCols             = 1
	MaxCols             = 10000
	MinRows             = 1
	MaxRows             = 10000
	MaxPingNonceBytes   = 256
)

type ClientMessage struct {
	Type  string `json:"type"`
	Cols  int    `json:"cols,omitempty"`
	Rows  int    `json:"rows,omitempty"`
	Nonce string `json:"nonce,omitempty"`
}

type ServerMessage struct {
	Type            string `json:"type"`
	SessionId       string `json:"session_id,omitempty"`
	WorkspaceId     string `json:"workspace_id,omitempty"`
	WorkspaceKey    string `json:"workspace_key,omitempty"`
	State           string `json:"state,omitempty"`
	LifecycleState  string `json:"lifecycle_state,omitempty"`
	AttachmentState string `json:"attachment_state,omitempty"`
	Reason          string `json:"reason,omitempty"`
	ExitCode        *int   `json:"exit_code,omitempty"`
	Code            string `json:"code,omitempty"`
	Message         string `json:"message,omitempty"`
	Nonce           string `json:"nonce,omitempty"`
	Truncated       *bool  `json:"truncated,omitempty"`
}

func DecodeClient(data []byte) (ClientMessage, error) {
	if len(data) > MaxJSONMessageBytes {
		return ClientMessage{}, fmt.Errorf("control message too large: %d bytes", len(data))
	}
	var message ClientMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return ClientMessage{}, fmt.Errorf("decode control message: %w", err)
	}
	if err := ValidateClient(message); err != nil {
		return ClientMessage{}, err
	}
	return message, nil
}

func ValidateClient(message ClientMessage) error {
	switch message.Type {
	case TypeHello, TypeDetach:
		return nil
	case TypeResize:
		return ValidateSize(message.Cols, message.Rows)
	case TypePing:
		if len([]byte(message.Nonce)) > MaxPingNonceBytes {
			return fmt.Errorf("ping nonce too large")
		}
		return nil
	default:
		return fmt.Errorf("unknown control message type %q", message.Type)
	}
}

func ValidateSize(cols int, rows int) error {
	if cols < MinCols || cols > MaxCols {
		return fmt.Errorf("cols out of range: %d", cols)
	}
	if rows < MinRows || rows > MaxRows {
		return fmt.Errorf("rows out of range: %d", rows)
	}
	return nil
}

func EncodeServer(message ServerMessage) ([]byte, error) {
	if err := validateServer(message); err != nil {
		return nil, err
	}
	return json.Marshal(message)
}

func MustEncodeServer(message ServerMessage) []byte {
	data, err := EncodeServer(message)
	if err != nil {
		panic(err)
	}
	return data
}

func validateServer(message ServerMessage) error {
	switch message.Type {
	case TypeStarted:
		if message.SessionId == "" || message.WorkspaceId == "" || message.State == "" && message.LifecycleState == "" {
			return fmt.Errorf("invalid started message")
		}
		return nil
	case TypeReplayStarted, TypeReplayFinished:
		return nil
	case TypeState:
		if message.LifecycleState == "" && message.State == "" {
			return fmt.Errorf("invalid state message")
		}
		return nil
	case TypeExited:
		if message.ExitCode == nil || message.State == "" && message.LifecycleState == "" {
			return fmt.Errorf("invalid exited message")
		}
		return nil
	case TypeError:
		if message.Code == "" || message.Message == "" {
			return fmt.Errorf("invalid error message")
		}
		return nil
	case TypePong:
		if len([]byte(message.Nonce)) > MaxPingNonceBytes {
			return fmt.Errorf("pong nonce too large")
		}
		return nil
	default:
		return fmt.Errorf("unknown server message type %q", message.Type)
	}
}
