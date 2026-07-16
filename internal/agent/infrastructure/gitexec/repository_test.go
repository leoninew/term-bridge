package gitexec

import (
	"bytes"
	"testing"

	gitmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/git"
)

func TestParseStatusRecognizesLayersAndRename(t *testing.T) {
	data := []byte("M  staged.txt\x00 M unstaged.txt\x00MM both.txt\x00?? new.txt\x00R  renamed.txt\x00old-name.txt\x00")

	changes, err := parseStatus(data)
	if err != nil {
		t.Fatalf("parse status: %v", err)
	}
	if len(changes) != 5 {
		t.Fatalf("change count = %d, want 5", len(changes))
	}
	assertLayers(t, changes[0], gitmodel.LayerStaged)
	assertLayers(t, changes[1], gitmodel.LayerUnstaged)
	assertLayers(t, changes[2], gitmodel.LayerStaged, gitmodel.LayerUnstaged)
	assertLayers(t, changes[3], gitmodel.LayerUntracked)
	if changes[4].Path != "renamed.txt" || changes[4].OriginalPath != "old-name.txt" {
		t.Fatalf("rename paths = %#v, want renamed.txt from old-name.txt", changes[4])
	}
}

func TestParseStatusRejectsUnsafePath(t *testing.T) {
	_, err := parseStatus([]byte(" M ../outside.txt\x00"))
	if err == nil {
		t.Fatal("unsafe status path was accepted")
	}
}

func TestDiffUsesChangedPathWhenOriginalPathIsEmpty(t *testing.T) {
	repository := &Repository{config: Config{MaxTextBytes: 1024}}
	if repository.exceedsTextLimit([]byte("text")) {
		t.Fatal("small text exceeded the configured limit")
	}
	if !repository.exceedsTextLimit(bytes.Repeat([]byte("x"), 1025)) {
		t.Fatal("oversized text did not exceed the configured limit")
	}
}

func TestRestrictToWorkspaceHidesOutsideChanges(t *testing.T) {
	changes := []gitmodel.Change{
		{Path: "workspace/inside.txt"},
		{Path: "outside.txt"},
		{Path: "workspace/renamed.txt", OriginalPath: "workspace/old-name.txt"},
		{Path: "workspace/renamed-from-outside.txt", OriginalPath: "outside-old-name.txt"},
	}

	restricted := restrictToWorkspace(changes, "workspace")
	if len(restricted) != 2 {
		t.Fatalf("workspace changes = %#v, want only contained changes", restricted)
	}
	if restricted[0].Path != "inside.txt" {
		t.Fatalf("contained path = %q, want inside.txt", restricted[0].Path)
	}
	if restricted[1].Path != "renamed.txt" || restricted[1].OriginalPath != "old-name.txt" {
		t.Fatalf("contained rename = %#v, want workspace-relative paths", restricted[1])
	}
}

func TestBoundedCaptureTracksCumulativeLimit(t *testing.T) {
	var output bytes.Buffer
	capture := boundedCapture{Writer: &output, Limit: 5}

	if _, err := capture.Write([]byte("abc")); err != nil {
		t.Fatalf("write first chunk: %v", err)
	}
	if _, err := capture.Write([]byte("def")); err != nil {
		t.Fatalf("write second chunk: %v", err)
	}
	if !capture.Overflow {
		t.Fatal("overflow = false, want true")
	}
	if output.String() != "abcde" {
		t.Fatalf("captured output = %q, want %q", output.String(), "abcde")
	}
}

func assertLayers(t *testing.T, change gitmodel.Change, want ...gitmodel.Layer) {
	t.Helper()
	if len(change.AvailableLayers) != len(want) {
		t.Fatalf("layers for %q = %v, want %v", change.Path, change.AvailableLayers, want)
	}
	for index, layer := range want {
		if change.AvailableLayers[index] != layer {
			t.Fatalf("layer %d for %q = %q, want %q", index, change.Path, change.AvailableLayers[index], layer)
		}
	}
}
