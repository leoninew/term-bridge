package api

import (
	"testing"

	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
)

func TestLocalRuntimeRequestFrameBuildsShortcutFrames(t *testing.T) {
	name := "updated shell"
	cases := []struct {
		method string
		params any
		assert func(*testing.T, *shared.TunnelFrame)
	}{
		{
			method: "list_shortcuts",
			assert: func(t *testing.T, frame *shared.TunnelFrame) {
				if frame.GetListShortcutsReq() == nil {
					t.Fatalf("frame payload = %T, want list shortcuts request", frame.GetPayload())
				}
			},
		},
		{
			method: "create_shortcut",
			params: &agent.CreateShortcutReq{Name: "shell", Command: `cmd /c "echo hello"`},
			assert: func(t *testing.T, frame *shared.TunnelFrame) {
				if frame.GetCreateShortcutReq().GetCommand() != `cmd /c "echo hello"` {
					t.Fatalf("create command = %q, want raw command", frame.GetCreateShortcutReq().GetCommand())
				}
			},
		},
		{
			method: "update_shortcut",
			params: &agent.UpdateShortcutRequest{ShortcutId: "shortcut-1", Request: &agent.UpdateShortcutReq{Name: &name}},
			assert: func(t *testing.T, frame *shared.TunnelFrame) {
				if frame.GetUpdateShortcutReq().GetShortcutId() != "shortcut-1" || frame.GetUpdateShortcutReq().GetRequest().GetName() != name {
					t.Fatalf("update request = %#v, want target id and patch", frame.GetUpdateShortcutReq())
				}
			},
		},
		{
			method: "delete_shortcut",
			params: &agent.DeleteShortcutReq{ShortcutId: "shortcut-1"},
			assert: func(t *testing.T, frame *shared.TunnelFrame) {
				if frame.GetDeleteShortcutReq().GetShortcutId() != "shortcut-1" {
					t.Fatalf("delete request = %#v, want target id", frame.GetDeleteShortcutReq())
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			frame, err := localRuntimeRequestFrame(tc.method, tc.params, "request-1")
			if err != nil {
				t.Fatalf("localRuntimeRequestFrame() error = %v", err)
			}
			if frame.GetRequestId() != "request-1" {
				t.Fatalf("RequestId = %q, want request-1", frame.GetRequestId())
			}
			tc.assert(t, frame)
		})
	}
}
