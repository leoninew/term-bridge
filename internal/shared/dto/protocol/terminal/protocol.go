package terminalproto

import (
	"encoding/json"
	"fmt"

	terminalv1 "termbridge/internal/gen/proto/termbridge/terminal/v1"
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
	MaxCols             = 1000
	MinRows             = 1
	MaxRows             = 1000
	MaxPingNonceBytes   = 256
)

type ClientMessage = terminalv1.ClientControlMessage
type ServerMessage = terminalv1.ServerControlMessage

func DecodeClient(data []byte) (*ClientMessage, error) {
	if len(data) > MaxJSONMessageBytes {
		return nil, fmt.Errorf("control message too large: %d bytes", len(data))
	}
	message := &ClientMessage{}
	if err := json.Unmarshal(data, message); err != nil {
		return nil, fmt.Errorf("decode control message: %w", err)
	}
	if err := ValidateClient(message); err != nil {
		return nil, err
	}
	return message, nil
}

func ValidateClient(message *ClientMessage) error {
	if message == nil {
		return fmt.Errorf("missing control message")
	}
	switch message.Type {
	case TypeHello, TypeDetach:
		return nil
	case TypeResize:
		return ValidateSize(int(message.Cols), int(message.Rows))
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

func EncodeServer(message *ServerMessage) ([]byte, error) {
	if err := validateServer(message); err != nil {
		return nil, err
	}
	return json.Marshal(message)
}

func MustEncodeServer(message *ServerMessage) []byte {
	data, err := EncodeServer(message)
	if err != nil {
		panic(err)
	}
	return data
}

func validateServer(message *ServerMessage) error {
	if message == nil {
		return fmt.Errorf("missing server message")
	}
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
