package terminalproto

import (
	"fmt"

	terminal "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
)

const (
	Subprotocol = "termbridge.terminal"

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

	ErrorCodeBadControl    = "bad_control"
	ErrorMessageBadControl = "Terminal control message is invalid."

	// Device disconnect reasons observed by Cloud and mapped to browser-safe terminal errors.
	DisconnectReasonDeviceDisconnected = "device disconnected"
	DisconnectReasonDeviceReconnected  = "device reconnected"
	DisconnectReasonDeviceDeleted      = "device deleted"

	ErrorCodeDeviceDisconnected    = "device_disconnected"
	ErrorMessageDeviceDisconnected = "Device disconnected."
	ErrorMessageDeviceReconnected  = "Device reconnected."
	ErrorMessageDeviceDeleted      = "Device was removed."

	MaxJSONMessageBytes = 16 * 1024
	MaxBinaryFrameBytes = 1024 * 1024
	MinCols             = 1
	MaxCols             = 1000
	MinRows             = 1
	MaxRows             = 1000
	MaxPingNonceBytes   = 256
)

func DecodeClient(data []byte) (*terminal.ClientControlMessage, error) {
	if len(data) > MaxJSONMessageBytes {
		return nil, fmt.Errorf("control message too large: %d bytes", len(data))
	}
	message := &terminal.ClientControlMessage{}
	if err := codec.UnmarshalProtoJSON(data, message); err != nil {
		return nil, fmt.Errorf("decode control message: %w", err)
	}
	if err := ValidateClient(message); err != nil {
		return nil, err
	}
	return message, nil
}

func ValidateClient(message *terminal.ClientControlMessage) error {
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

func EncodeServer(message *terminal.ServerControlMessage) ([]byte, error) {
	if err := validateServer(message); err != nil {
		return nil, err
	}
	return codec.MarshalProtoJSON(message)
}

func MustEncodeServer(message *terminal.ServerControlMessage) []byte {
	data, err := EncodeServer(message)
	if err != nil {
		panic(err)
	}
	return data
}

func validateServer(message *terminal.ServerControlMessage) error {
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

// BrowserDisconnectError maps an internal disconnect reason to a browser-safe terminal error.
func BrowserDisconnectError(reason string) (code string, message string) {
	switch reason {
	case DisconnectReasonDeviceReconnected:
		return ErrorCodeDeviceDisconnected, ErrorMessageDeviceReconnected
	case DisconnectReasonDeviceDeleted:
		return ErrorCodeDeviceDisconnected, ErrorMessageDeviceDeleted
	default:
		return ErrorCodeDeviceDisconnected, ErrorMessageDeviceDisconnected
	}
}
