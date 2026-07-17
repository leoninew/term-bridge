package api

import (
	"testing"

	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
)

func TestLocalRuntimeRequestFrameBuildsFileGitFrames(t *testing.T) {
	cases := []struct {
		method string
		params any
		assert func(*testing.T, *shared.TunnelFrame)
	}{
		{method: "fs_stat", params: &agent.FsStatReq{WorkspaceId: "workspace-1", Path: "directory"}, assert: func(t *testing.T, frame *shared.TunnelFrame) {
			if request := frame.GetFsStatReq(); request == nil || request.GetPath() != "directory" {
				t.Fatalf("fs stat request = %#v", request)
			}
		}},
		{method: "fs_read_directory", params: &agent.FsReadDirectoryReq{WorkspaceId: "workspace-1", Path: "directory"}, assert: func(t *testing.T, frame *shared.TunnelFrame) {
			if request := frame.GetFsReadDirectoryReq(); request == nil || request.GetPath() != "directory" {
				t.Fatalf("fs read directory request = %#v", request)
			}
		}},
		{method: "fs_read_file", params: &agent.FsReadFileReq{WorkspaceId: "workspace-1", Path: "notes.txt"}, assert: func(t *testing.T, frame *shared.TunnelFrame) {
			if request := frame.GetFsReadFileReq(); request == nil || request.GetPath() != "notes.txt" {
				t.Fatalf("fs read file request = %#v", request)
			}
		}},
		{method: "fs_write_file", params: &agent.FsWriteFileReq{WorkspaceId: "workspace-1", Path: "notes.txt", Content: []byte("updated"), Create: true, Overwrite: true}, assert: func(t *testing.T, frame *shared.TunnelFrame) {
			if request := frame.GetFsWriteFileReq(); request == nil || string(request.GetContent()) != "updated" {
				t.Fatalf("fs write file request = %#v", request)
			}
		}},
		{method: "fs_create_directory", params: &agent.FsCreateDirectoryReq{WorkspaceId: "workspace-1", Path: "dir"}, assert: func(t *testing.T, frame *shared.TunnelFrame) {
			if request := frame.GetFsCreateDirectoryReq(); request == nil || request.GetPath() != "dir" {
				t.Fatalf("fs create directory request = %#v", request)
			}
		}},
		{method: "fs_delete", params: &agent.FsDeleteReq{WorkspaceId: "workspace-1", Path: "notes.txt", Recursive: true}, assert: func(t *testing.T, frame *shared.TunnelFrame) {
			if request := frame.GetFsDeleteReq(); request == nil || !request.GetRecursive() {
				t.Fatalf("fs delete request = %#v", request)
			}
		}},
		{method: "fs_rename", params: &agent.FsRenameReq{WorkspaceId: "workspace-1", OldPath: "a.txt", NewPath: "b.txt", Overwrite: true}, assert: func(t *testing.T, frame *shared.TunnelFrame) {
			if request := frame.GetFsRenameReq(); request == nil || request.GetNewPath() != "b.txt" {
				t.Fatalf("fs rename request = %#v", request)
			}
		}},
		{method: "scm_status", params: &agent.ScmStatusReq{WorkspaceId: "workspace-1"}, assert: func(t *testing.T, frame *shared.TunnelFrame) {
			if request := frame.GetScmStatusReq(); request == nil || request.GetWorkspaceId() != "workspace-1" {
				t.Fatalf("scm status request = %#v", request)
			}
		}},
		{method: "scm_original_content", params: &agent.ScmOriginalContentReq{WorkspaceId: "workspace-1", Path: "notes.txt", GroupId: "changes"}, assert: func(t *testing.T, frame *shared.TunnelFrame) {
			if request := frame.GetScmOriginalContentReq(); request == nil || request.GetGroupId() != "changes" {
				t.Fatalf("scm original content request = %#v", request)
			}
		}},
		{method: "scm_execute", params: &agent.ScmExecuteReq{WorkspaceId: "workspace-1", Command: agent.ScmCommand_SCM_COMMAND_STAGE, Path: "notes.txt", GroupId: "changes"}, assert: func(t *testing.T, frame *shared.TunnelFrame) {
			if request := frame.GetScmExecuteReq(); request == nil || request.GetCommand() != agent.ScmCommand_SCM_COMMAND_STAGE {
				t.Fatalf("scm execute request = %#v", request)
			}
		}},
		{method: "scm_repository", params: &agent.ScmRepositoryReq{WorkspaceId: "workspace-1"}, assert: func(t *testing.T, frame *shared.TunnelFrame) {
			if request := frame.GetScmRepositoryReq(); request == nil || request.GetWorkspaceId() != "workspace-1" {
				t.Fatalf("scm repository request = %#v", request)
			}
		}},
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
			method: "update_shortcut_order",
			params: &agent.UpdateShortcutOrderReq{ShortcutIds: []string{"shortcut-2", "shortcut-1"}},
			assert: func(t *testing.T, frame *shared.TunnelFrame) {
				if got := frame.GetUpdateShortcutOrderReq().GetShortcutIds(); len(got) != 2 || got[0] != "shortcut-2" || got[1] != "shortcut-1" {
					t.Fatalf("shortcut order request = %v, want [shortcut-2 shortcut-1]", got)
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
