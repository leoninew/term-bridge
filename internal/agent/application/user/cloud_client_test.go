package application

import (
	"context"
	"testing"

	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	tunnel "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
)

func TestHandleRuntimeRequestDispatchesShortcutCRUD(t *testing.T) {
	runtime := &shortcutRuntime{value: &agent.Shortcut{Id: "shortcut-1", Name: "shell", Command: `cmd /c "echo hello"`}}
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
				if len(items) != 1 || items[0].GetCommand() != `cmd /c "echo hello"` {
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
			name:    "delete",
			request: &shared.TunnelFrame{StreamId: "delete", RequestId: "request-4", Payload: &shared.TunnelFrame_DeleteShortcutReq{DeleteShortcutReq: &agent.DeleteShortcutReq{ShortcutId: "shortcut-1"}}},
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
}

type shortcutRuntime struct {
	RuntimeAccess
	value     *agent.Shortcut
	updatedId string
	deletedId string
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

func (r *shortcutRuntime) DeleteShortcut(_ context.Context, shortcutId string) error {
	r.deletedId = shortcutId
	return nil
}
