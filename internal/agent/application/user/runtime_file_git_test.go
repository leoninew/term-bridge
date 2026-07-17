package application

import (
	"testing"

	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
	gitmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/git"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
)

func TestScmStatusFromModelGroupsChanges(t *testing.T) {
	status := gitmodel.StatusResult{
		State: gitmodel.StateAvailable,
		Changes: []gitmodel.Change{
			{Path: "a.go", IndexStatus: "M", AvailableLayers: []gitmodel.Layer{gitmodel.LayerStaged}},
			{Path: "b.go", WorktreeStatus: "M", AvailableLayers: []gitmodel.Layer{gitmodel.LayerUnstaged}},
			{Path: "c.go", Untracked: true, AvailableLayers: []gitmodel.Layer{gitmodel.LayerUntracked}},
		},
	}
	resp := scmStatusFromModel(status)
	if resp.GetState() != agent.ScmState_SCM_STATE_AVAILABLE {
		t.Fatalf("state = %v", resp.GetState())
	}
	if resp.GetCount() != 3 {
		t.Fatalf("count = %d, want 3", resp.GetCount())
	}
	if len(resp.GetGroups()) != 3 {
		t.Fatalf("groups = %d, want 3", len(resp.GetGroups()))
	}
}

func TestScmStatusFromModelUnavailable(t *testing.T) {
	resp := scmStatusFromModel(gitmodel.StatusResult{State: gitmodel.StateNotRepository, Message: "not a repo"})
	if resp.GetState() != agent.ScmState_SCM_STATE_NOT_REPOSITORY {
		t.Fatalf("state = %v", resp.GetState())
	}
	if resp.GetMessage() != "not a repo" {
		t.Fatalf("message = %q", resp.GetMessage())
	}
}

func TestContentAsTextRejectsBinaryAndNUL(t *testing.T) {
	if _, err := contentAsText([]byte("ok")); err != nil {
		t.Fatalf("utf8 text: %v", err)
	}
	if _, err := contentAsText([]byte{0xff, 0xfe, 0x00}); filemodel.CodeOf(err) != "file_not_text" {
		t.Fatalf("binary code = %q err=%v", filemodel.CodeOf(err), err)
	}
	if _, err := contentAsText([]byte{'a', 0, 'b'}); filemodel.CodeOf(err) != "file_not_text" {
		t.Fatalf("nul code = %q err=%v", filemodel.CodeOf(err), err)
	}
}
