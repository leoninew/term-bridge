package tunnel

import (
	"encoding/json"
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	commonv1 "termbridge-go/internal/gen/proto/termbridge/common/v1"
	tunnelv1 "termbridge-go/internal/gen/proto/termbridge/tunnel/v1"
	"termbridge-go/internal/shared/codec"
)

const ProtocolVersion = 2

const MaxFrameBytes = 32 * 1024 * 1024

const ControlStreamID = "control"

func MarshalFrame(frame *tunnelv1.TunnelFrame) ([]byte, error) {
	if err := ValidateFrame(frame); err != nil {
		return nil, err
	}
	return codec.MarshalProtoJSON(frame)
}

func UnmarshalFrame(data []byte) (*tunnelv1.TunnelFrame, error) {
	var frame tunnelv1.TunnelFrame
	if err := codec.UnmarshalProtoJSON(data, &frame); err != nil {
		return nil, err
	}
	if err := ValidateFrame(&frame); err != nil {
		return nil, err
	}
	return &frame, nil
}

func ValidateFrame(frame *tunnelv1.TunnelFrame) error {
	if frame == nil {
		return fmt.Errorf("tunnel frame is nil")
	}
	if frame.GetStreamId() == "" {
		return fmt.Errorf("tunnel frame missing stream_id")
	}
	if frame.GetPayload() == nil {
		return fmt.Errorf("tunnel frame missing payload")
	}
	if hello := frame.GetHello(); hello != nil && hello.GetProtocolVersion() != ProtocolVersion {
		return fmt.Errorf("unsupported tunnel protocol version: %d", hello.GetProtocolVersion())
	}
	return nil
}

func ErrorFrame(streamId string, requestId string, code string, message string) *tunnelv1.TunnelFrame {
	return &tunnelv1.TunnelFrame{
		StreamId:  streamId,
		RequestId: requestId,
		Payload: &tunnelv1.TunnelFrame_Error{Error: &commonv1.ErrorResp{
			Code:      code,
			Error:     message,
			RequestId: requestId,
		}},
	}
}

func ErrorMessage(frame *tunnelv1.TunnelFrame) string {
	if frame == nil || frame.GetError() == nil || frame.GetError().GetError() == "" {
		return "tunnel frame error"
	}
	return frame.GetError().GetError()
}

func ResponseJSON(frame *tunnelv1.TunnelFrame) (json.RawMessage, error) {
	if frame == nil {
		return nil, fmt.Errorf("tunnel response is nil")
	}
	if errResp := frame.GetError(); errResp != nil {
		return nil, errors.New(errResp.GetError())
	}
	message := responseMessage(frame)
	if message == nil {
		return nil, fmt.Errorf("tunnel response has unsupported payload %T", frame.GetPayload())
	}
	data, err := codec.MarshalProtoJSON(message)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func responseMessage(frame *tunnelv1.TunnelFrame) proto.Message {
	switch payload := frame.GetPayload().(type) {
	case *tunnelv1.TunnelFrame_ListWorkspacesResp:
		return payload.ListWorkspacesResp
	case *tunnelv1.TunnelFrame_WorkspaceTreeResp:
		return payload.WorkspaceTreeResp
	case *tunnelv1.TunnelFrame_WorkspaceSessionsResp:
		return payload.WorkspaceSessionsResp
	case *tunnelv1.TunnelFrame_CreateSessionResp:
		return payload.CreateSessionResp
	case *tunnelv1.TunnelFrame_GetSessionResp:
		return payload.GetSessionResp.GetSession()
	case *tunnelv1.TunnelFrame_RerunSessionResp:
		return payload.RerunSessionResp
	case *tunnelv1.TunnelFrame_UpdateSessionResp:
		return payload.UpdateSessionResp.GetSession()
	case *tunnelv1.TunnelFrame_CloseSessionResp:
		return payload.CloseSessionResp.GetSession()
	case *tunnelv1.TunnelFrame_DeleteSessionResp:
		return payload.DeleteSessionResp
	case *tunnelv1.TunnelFrame_ReadHistoryResp:
		return payload.ReadHistoryResp
	case *tunnelv1.TunnelFrame_UpdateWorkspaceOrderResp:
		return payload.UpdateWorkspaceOrderResp
	case *tunnelv1.TunnelFrame_UpdateSessionOrderResp:
		return payload.UpdateSessionOrderResp
	case *tunnelv1.TunnelFrame_DeleteWorkspaceResp:
		return payload.DeleteWorkspaceResp
	default:
		return nil
	}
}
