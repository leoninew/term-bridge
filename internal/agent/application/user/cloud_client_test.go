package application

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	terminalapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/task/terminal"
	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
	gitmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/git"
	sessionmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/session"
	shortcutmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/shortcut"
	workspacemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/workspace"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	tunnel "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestHandleRuntimeRequestDispatchesShortcutCRUD(t *testing.T) {
	lastUsedAt := time.Date(2026, time.July, 14, 12, 30, 0, 0, time.UTC)
	runtime := &shortcutRuntime{value: &agent.Shortcut{Id: "shortcut-1", Name: "shell", Command: `cmd /c "echo hello"`, LastUsedAt: timestamppb.New(lastUsedAt)}}
	description := "launch shell"
	updateName := "updated shell"
	cases := []struct {
		name    string
		request *shared.TunnelFrame
		assert  func(*testing.T, *shared.TunnelFrame)
	}{
		{
			name:    "list",
			request: &shared.TunnelFrame{StreamId: "list", RequestId: "request-1", Payload: &shared.TunnelFrame_ListShortcutsReq{ListShortcutsReq: &agent.ListShortcutsReq{}}},
			assert: func(t *testing.T, response *shared.TunnelFrame) {
				items := response.GetListShortcutsResp().GetItems()
				if len(items) != 1 || items[0].GetCommand() != `cmd /c "echo hello"` || items[0].GetLastUsedAt() == nil || !items[0].GetLastUsedAt().AsTime().Equal(lastUsedAt) {
					t.Fatalf("list response = %#v, want raw shortcut command", response)
				}
			},
		},
		{
			name:    "create",
			request: &shared.TunnelFrame{StreamId: "create", RequestId: "request-2", Payload: &shared.TunnelFrame_CreateShortcutReq{CreateShortcutReq: &agent.CreateShortcutReq{Name: "created", Command: "cmd", Description: &description}}},
			assert: func(t *testing.T, response *shared.TunnelFrame) {
				if response.GetCreateShortcutResp().GetShortcut().GetName() != "created" {
					t.Fatalf("create response = %#v", response)
				}
			},
		},
		{
			name:    "update",
			request: &shared.TunnelFrame{StreamId: "update", RequestId: "request-3", Payload: &shared.TunnelFrame_UpdateShortcutReq{UpdateShortcutReq: &agent.UpdateShortcutRequest{ShortcutId: "shortcut-1", Request: &agent.UpdateShortcutReq{Name: &updateName}}}},
			assert: func(t *testing.T, response *shared.TunnelFrame) {
				if response.GetUpdateShortcutResp().GetShortcut().GetName() != updateName {
					t.Fatalf("update response = %#v", response)
				}
			},
		},
		{
			name:    "update order",
			request: &shared.TunnelFrame{StreamId: "order", RequestId: "request-4", Payload: &shared.TunnelFrame_UpdateShortcutOrderReq{UpdateShortcutOrderReq: &agent.UpdateShortcutOrderReq{ShortcutIds: []string{"shortcut-2", "shortcut-1"}}}},
			assert: func(t *testing.T, response *shared.TunnelFrame) {
				items := response.GetUpdateShortcutOrderResp().GetItems()
				if len(items) != 2 || items[0].GetId() != "shortcut-2" || items[1].GetId() != "shortcut-1" {
					t.Fatalf("order response = %#v, want canonical shortcut order", response)
				}
			},
		},
		{
			name:    "delete",
			request: &shared.TunnelFrame{StreamId: "delete", RequestId: "request-5", Payload: &shared.TunnelFrame_DeleteShortcutReq{DeleteShortcutReq: &agent.DeleteShortcutReq{ShortcutId: "shortcut-1"}}},
			assert: func(t *testing.T, response *shared.TunnelFrame) {
				if response.GetDeleteShortcutResp() == nil {
					t.Fatalf("delete response = %#v", response)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := HandleRuntimeRequest(context.Background(), runtime, tc.request)
			if err != nil {
				t.Fatalf("HandleRuntimeRequest() error = %v", err)
			}
			if response.GetStreamId() != tc.request.GetStreamId() || response.GetRequestId() != tc.request.GetRequestId() {
				t.Fatalf("response correlation = %#v, want request correlation", response)
			}
			tc.assert(t, response)
			if _, err := tunnel.ResponseJSON(response); err != nil {
				t.Fatalf("ResponseJSON() error = %v", err)
			}
		})
	}
	if runtime.updatedId != "shortcut-1" || runtime.deletedId != "shortcut-1" {
		t.Fatalf("shortcut target ids = updated %q / deleted %q", runtime.updatedId, runtime.deletedId)
	}
	if len(runtime.updatedOrderIds) != 2 || runtime.updatedOrderIds[0] != "shortcut-2" || runtime.updatedOrderIds[1] != "shortcut-1" {
		t.Fatalf("shortcut order ids = %v, want [shortcut-2 shortcut-1]", runtime.updatedOrderIds)
	}
}

type shortcutRuntime struct {
	RuntimeAccess
	value           *agent.Shortcut
	updatedId       string
	updatedOrderIds []string
	deletedId       string
}

func (r shortcutRuntime) ListShortcuts(context.Context) ([]*agent.Shortcut, error) {
	return []*agent.Shortcut{r.value}, nil
}

func (r shortcutRuntime) CreateShortcut(_ context.Context, request *agent.CreateShortcutReq) (*agent.Shortcut, error) {
	return &agent.Shortcut{Id: "shortcut-created", Name: request.GetName(), Command: request.GetCommand(), Description: request.Description}, nil
}

func (r *shortcutRuntime) UpdateShortcut(_ context.Context, shortcutId string, request *agent.UpdateShortcutReq) (*agent.Shortcut, error) {
	r.updatedId = shortcutId
	r.value.Name = request.GetName()
	return r.value, nil
}

func (r *shortcutRuntime) UpdateShortcutOrder(_ context.Context, shortcutIds []string) ([]*agent.Shortcut, error) {
	r.updatedOrderIds = append([]string(nil), shortcutIds...)
	items := make([]*agent.Shortcut, 0, len(shortcutIds))
	for _, shortcutId := range shortcutIds {
		items = append(items, &agent.Shortcut{Id: shortcutId})
	}
	return items, nil
}

func (r *shortcutRuntime) DeleteShortcut(_ context.Context, shortcutId string) error {
	r.deletedId = shortcutId
	return nil
}

func TestTerminalOutboundFramePreservesControlPayload(t *testing.T) {
	started := &agent.ServerControlMessage{
		Type:            "started",
		SessionId:       "session-1",
		WorkspaceId:     "workspace-1",
		LifecycleState:  "running",
		AttachmentState: "attached",
	}
	controlFrame, err := terminalOutboundFrame("stream-1", terminalapp.Outbound{Kind: terminalapp.OutboundText, Text: started})
	if err != nil {
		t.Fatalf("terminalOutboundFrame() error = %v", err)
	}
	if controlFrame.GetTerminalOutput() != nil {
		t.Fatalf("control frame output = %#v, want no binary output", controlFrame.GetTerminalOutput())
	}
	control := controlFrame.GetTerminalControl()
	if control == nil || control.GetSessionId() != started.GetSessionId() || control.GetWorkspaceId() != started.GetWorkspaceId() || control.GetLifecycleState() != started.GetLifecycleState() || control.GetAttachmentState() != started.GetAttachmentState() {
		t.Fatalf("control frame = %#v, want complete started control", control)
	}

	output := []byte{0x1b, '[', '2', 'J'}
	outputFrame, err := terminalOutboundFrame("stream-1", terminalapp.Outbound{Kind: terminalapp.OutboundBinary, Binary: output})
	if err != nil {
		t.Fatalf("terminalOutboundFrame() error = %v", err)
	}
	if outputFrame.GetTerminalControl() != nil {
		t.Fatalf("binary frame control = %#v, want no terminal control", outputFrame.GetTerminalControl())
	}
	if got := outputFrame.GetTerminalOutput().GetData(); string(got) != string(output) {
		t.Fatalf("binary frame data = %v, want %v", got, output)
	}
}

func TestTerminalOutboundFrameRejectsNilControl(t *testing.T) {
	_, err := terminalOutboundFrame("stream-1", terminalapp.Outbound{Kind: terminalapp.OutboundText})
	if err == nil {
		t.Fatal("terminalOutboundFrame() error = nil, want nil control rejection")
	}
}

func TestTunnelUrlUsesConfiguredAPIBasePath(t *testing.T) {
	for _, tc := range []struct {
		name string
		base string
		want string
	}{
		{name: "HTTPS API base", base: "https://cloud.example.test/api", want: "wss://cloud.example.test/api/agent/tunnel"},
		{name: "HTTP API base with trailing slash", base: "http://cloud.example.test/api/", want: "ws://cloud.example.test/api/agent/tunnel"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tunnelUrl(tc.base); got != tc.want {
				t.Fatalf("tunnelUrl(%q) = %q, want %q", tc.base, got, tc.want)
			}
		})
	}
}

func TestRuntimeErrorFramePreservesFileBusinessMessage(t *testing.T) {
	err := filemodel.NewError("directory_not_empty", "The directory is not empty.")
	frame := runtimeErrorFrame("stream-1", "request-1", err)
	response := frame.GetError()
	if response == nil {
		t.Fatal("expected error payload")
	}
	if response.GetCode() != "directory_not_empty" {
		t.Fatalf("code = %q, want directory_not_empty", response.GetCode())
	}
	if response.GetError() != "The directory is not empty." {
		t.Fatalf("error = %q, want business message", response.GetError())
	}
	if response.GetRequestId() != "request-1" || frame.GetStreamId() != "stream-1" {
		t.Fatalf("correlation = stream=%q request=%q", frame.GetStreamId(), response.GetRequestId())
	}
}

func TestRuntimeErrorFrameKeepsGenericMessageForUntypedErrors(t *testing.T) {
	frame := runtimeErrorFrame("stream-2", "request-2", fmt.Errorf("open /secret/path: access denied"))
	response := frame.GetError()
	if response == nil {
		t.Fatal("expected error payload")
	}
	if response.GetCode() != "runtime_error" {
		t.Fatalf("code = %q, want runtime_error", response.GetCode())
	}
	if response.GetError() != "Runtime request failed." {
		t.Fatalf("error = %q, want generic message", response.GetError())
	}
	if strings.Contains(response.GetError(), "/secret/path") {
		t.Fatalf("internal path leaked: %q", response.GetError())
	}
}

func TestRuntimeErrorFramePreservesRevisionConflictDetails(t *testing.T) {
	err := filemodel.Conflict(&filemodel.Entry{Path: filemodel.RelativePath("notes.txt"), Name: "notes.txt", Kind: filemodel.EntryKindFile, Size: 12, Revision: "rev-1"})
	frame := runtimeErrorFrame("stream-3", "request-3", err)
	response := frame.GetError()
	if response == nil || response.GetCode() != "revision_conflict" {
		t.Fatalf("response = %#v", response)
	}
	if response.GetError() != "The workspace entry changed." {
		t.Fatalf("error = %q", response.GetError())
	}
	if response.GetDetails() == nil {
		t.Fatal("expected revision conflict details")
	}
}

func TestRuntimeErrorFramePreservesSessionBusinessMessage(t *testing.T) {
	frame := runtimeErrorFrame("stream-session", "request-session", sessionmodel.NotAttachable())
	response := frame.GetError()
	if response.GetCode() != sessionmodel.CodeNotAttachable || response.GetError() != "Session is not attachable." {
		t.Fatalf("response = %#v", response)
	}
}

func TestRuntimeErrorFramePreservesGitBusinessMessage(t *testing.T) {
	frame := runtimeErrorFrame("stream-git", "request-git", gitmodel.InvalidOperation())
	response := frame.GetError()
	if response.GetCode() != gitmodel.CodeInvalidOperation || response.GetError() != "The Git operation is invalid." {
		t.Fatalf("response = %#v", response)
	}
}

func TestRuntimeErrorFramePreservesSessionP1Catalog(t *testing.T) {
	frame := runtimeErrorFrame("stream-p1", "request-p1", sessionmodel.CannotDeleteRunning())
	response := frame.GetError()
	if response.GetCode() != sessionmodel.CodeCannotDeleteRunning || response.GetError() != "A running session cannot be deleted." {
		t.Fatalf("response = %#v", response)
	}
}

func TestRuntimeErrorFramePreservesWorkspaceAndShortcutCatalog(t *testing.T) {
	frame := runtimeErrorFrame("stream-ws", "request-ws", workspacemodel.CannotDeleteWithRunningSessions())
	response := frame.GetError()
	if response.GetCode() != workspacemodel.CodeCannotDeleteWithRunningSessions {
		t.Fatalf("workspace response = %#v", response)
	}
	frame = runtimeErrorFrame("stream-sc", "request-sc", shortcutmodel.Disabled())
	response = frame.GetError()
	if response.GetCode() != shortcutmodel.CodeDisabled || response.GetError() != "Shortcut is disabled." {
		t.Fatalf("shortcut response = %#v", response)
	}
}
