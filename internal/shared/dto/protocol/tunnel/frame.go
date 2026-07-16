package tunnel

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/proto"

	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
)

const ProtocolVersion = 4

const MaxFrameBytes = 32 * 1024 * 1024

const ControlStreamID = "control"

func MarshalFrame(frame *shared.TunnelFrame) ([]byte, error) {
	if err := ValidateFrame(frame); err != nil {
		return nil, err
	}
	return codec.MarshalProtoJSON(frame)
}

func UnmarshalFrame(data []byte) (*shared.TunnelFrame, error) {
	var frame shared.TunnelFrame
	if err := codec.UnmarshalProtoJSON(data, &frame); err != nil {
		return nil, err
	}
	if err := ValidateFrame(&frame); err != nil {
		return nil, err
	}
	return &frame, nil
}

func ValidateFrame(frame *shared.TunnelFrame) error {
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

func ErrorFrame(streamId string, requestId string, code string, message string) *shared.TunnelFrame {
	return &shared.TunnelFrame{
		StreamId:  streamId,
		RequestId: requestId,
		Payload: &shared.TunnelFrame_Error{Error: &shared.ErrorResp{
			Code:      code,
			Error:     message,
			RequestId: requestId,
		}},
	}
}

func ErrorMessage(frame *shared.TunnelFrame) string {
	if frame == nil || frame.GetError() == nil || frame.GetError().GetError() == "" {
		return "tunnel frame error"
	}
	return frame.GetError().GetError()
}

type RemoteError struct {
	Response *shared.ErrorResp
}

func (e *RemoteError) Error() string {
	if e == nil || e.Response == nil || e.Response.GetError() == "" {
		return "tunnel frame error"
	}
	return e.Response.GetError()
}

func RemoteErrorFromFrame(frame *shared.TunnelFrame) (*RemoteError, bool) {
	if frame == nil || frame.GetError() == nil {
		return nil, false
	}
	return &RemoteError{Response: frame.GetError()}, true
}

func ResponseJSON(frame *shared.TunnelFrame) (json.RawMessage, error) {
	if frame == nil {
		return nil, fmt.Errorf("tunnel response is nil")
	}
	if remoteErr, ok := RemoteErrorFromFrame(frame); ok {
		return nil, remoteErr
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

func responseMessage(frame *shared.TunnelFrame) proto.Message {
	switch payload := frame.GetPayload().(type) {
	case *shared.TunnelFrame_ListWorkspacesResp:
		return payload.ListWorkspacesResp
	case *shared.TunnelFrame_WorkspaceTreeResp:
		return payload.WorkspaceTreeResp
	case *shared.TunnelFrame_WorkspaceSessionsResp:
		return payload.WorkspaceSessionsResp
	case *shared.TunnelFrame_CreateSessionResp:
		return payload.CreateSessionResp
	case *shared.TunnelFrame_GetSessionResp:
		return payload.GetSessionResp.GetSession()
	case *shared.TunnelFrame_RerunSessionResp:
		return payload.RerunSessionResp
	case *shared.TunnelFrame_UpdateSessionResp:
		return payload.UpdateSessionResp.GetSession()
	case *shared.TunnelFrame_CloseSessionResp:
		return payload.CloseSessionResp.GetSession()
	case *shared.TunnelFrame_DeleteSessionResp:
		return payload.DeleteSessionResp
	case *shared.TunnelFrame_ReadHistoryResp:
		return payload.ReadHistoryResp
	case *shared.TunnelFrame_UpdateWorkspaceOrderResp:
		return payload.UpdateWorkspaceOrderResp
	case *shared.TunnelFrame_UpdateSessionOrderResp:
		return payload.UpdateSessionOrderResp
	case *shared.TunnelFrame_DeleteWorkspaceResp:
		return payload.DeleteWorkspaceResp
	case *shared.TunnelFrame_ListShortcutsResp:
		return payload.ListShortcutsResp
	case *shared.TunnelFrame_CreateShortcutResp:
		return payload.CreateShortcutResp.GetShortcut()
	case *shared.TunnelFrame_UpdateShortcutResp:
		return payload.UpdateShortcutResp.GetShortcut()
	case *shared.TunnelFrame_DeleteShortcutResp:
		return payload.DeleteShortcutResp
	case *shared.TunnelFrame_UpdateShortcutOrderResp:
		return payload.UpdateShortcutOrderResp
	case *shared.TunnelFrame_ListFilesResp:
		return payload.ListFilesResp
	case *shared.TunnelFrame_ReadFileResp:
		return payload.ReadFileResp
	case *shared.TunnelFrame_CreateFileResp:
		return payload.CreateFileResp
	case *shared.TunnelFrame_CreateDirectoryResp:
		return payload.CreateDirectoryResp
	case *shared.TunnelFrame_WriteFileResp:
		return payload.WriteFileResp
	case *shared.TunnelFrame_RenameEntryResp:
		return payload.RenameEntryResp
	case *shared.TunnelFrame_MoveEntryResp:
		return payload.MoveEntryResp
	case *shared.TunnelFrame_DeleteEntryResp:
		return payload.DeleteEntryResp
	case *shared.TunnelFrame_GitStatusResp:
		return payload.GitStatusResp
	case *shared.TunnelFrame_GitDiffResp:
		return payload.GitDiffResp
	default:
		return nil
	}
}
