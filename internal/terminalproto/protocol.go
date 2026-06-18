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
	TypeClose  = "close"
	TypePing   = "ping"

	TypeStarted = "started"
	TypeState   = "state"
	TypeExited  = "exited"
	TypeError   = "error"
	TypePong    = "pong"

	MaxJSONMessageBytes = 16 * 1024
	MaxBinaryFrameBytes = 1024 * 1024
	MinCols             = 1
	MaxCols             = 500
	MinRows             = 1
	MaxRows             = 200
	MaxPingNonceBytes   = 256
)

type ClientMessage struct {
	Type    string `json:"type"`
	LastSeq *int64 `json:"last_seq,omitempty"`
	Cols    int    `json:"cols,omitempty"`
	Rows    int    `json:"rows,omitempty"`
	Nonce   string `json:"nonce,omitempty"`
}

type ServerMessage struct {
	Type            string `json:"type"`
	SessionID       string `json:"session_id,omitempty"`
	WorkspaceID     string `json:"workspace_id,omitempty"`
	WorkspaceKey    string `json:"workspace_key,omitempty"`
	State           string `json:"state,omitempty"`
	LifecycleState  string `json:"lifecycle_state,omitempty"`
	AttachmentState string `json:"attachment_state,omitempty"`
	Reason          string `json:"reason,omitempty"`
	ExitCode        *int   `json:"exit_code,omitempty"`
	Code            string `json:"code,omitempty"`
	Message         string `json:"message,omitempty"`
	Nonce           string `json:"nonce,omitempty"`
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
	case TypeHello, TypeDetach, TypeClose:
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
	case TypeStarted, TypeState, TypeExited, TypeError, TypePong:
		return nil
	default:
		return fmt.Errorf("unknown server message type %q", message.Type)
	}
}
