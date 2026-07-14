package application

import (
	"context"
	"testing"
	"time"

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

func TestTunnelUrlUsesCanonicalAPIPathWhenBaseHasNoPath(t *testing.T) {
	for _, tc := range []struct {
		name string
		base string
		want string
	}{
		{name: "HTTPS base", base: "https://cloud.example.test", want: "wss://cloud.example.test/api/agent/tunnel"},
		{name: "HTTP root", base: "http://cloud.example.test/", want: "ws://cloud.example.test/api/agent/tunnel"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tunnelUrl(tc.base); got != tc.want {
				t.Fatalf("tunnelUrl(%q) = %q, want %q", tc.base, got, tc.want)
			}
		})
	}
}
