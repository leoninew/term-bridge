package gitexec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	workspacefile "gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/storage/workspacefile"
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

func TestParseStatusRecognizesWorktreeRenameAndCopy(t *testing.T) {
	data := []byte(" R renamed.txt\x00old-name.txt\x00 C copied.txt\x00source.txt\x00")

	changes, err := parseStatus(data)
	if err != nil {
		t.Fatalf("parse status: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("change count = %d, want 2", len(changes))
	}
	if changes[0].Path != "renamed.txt" || changes[0].OriginalPath != "old-name.txt" {
		t.Fatalf("worktree rename = %#v", changes[0])
	}
	if changes[1].Path != "copied.txt" || changes[1].OriginalPath != "source.txt" {
		t.Fatalf("worktree copy = %#v", changes[1])
	}
}

func TestParseStatusRejectsUnsafePath(t *testing.T) {
	_, err := parseStatus([]byte(" M ../outside.txt\x00"))
	if err == nil {
		t.Fatal("unsafe status path was accepted")
	}
}

func TestStatusDistinguishesNonRepositoryAndBrokenRepository(t *testing.T) {
	repository := newTestRepository(t)

	nonRepository, err := repository.Status(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("read non-repository status: %v", err)
	}
	if nonRepository.State != gitmodel.StateNotRepository {
		t.Fatalf("non-repository state = %q, want %q", nonRepository.State, gitmodel.StateNotRepository)
	}

	repositoryRoot := initializeGitRepository(t)
	writeTestFile(t, filepath.Join(repositoryRoot, ".git", "config"), "[core\n")
	brokenRepository, err := repository.Status(context.Background(), repositoryRoot)
	if err != nil {
		t.Fatalf("read broken repository status: %v", err)
	}
	if brokenRepository.State != gitmodel.StateUnavailable {
		t.Fatalf("broken repository state = %q, want %q", brokenRepository.State, gitmodel.StateUnavailable)
	}
}

func TestStatusReturnsUnavailableForMalformedGlobalGitConfig(t *testing.T) {
	globalConfigDirectory := t.TempDir()
	writeTestFile(t, filepath.Join(globalConfigDirectory, ".gitconfig"), "[core\n")
	repository := newTestRepository(t)
	repository.environ = func() []string {
		return environmentWith(os.Environ(), "HOME", globalConfigDirectory, "USERPROFILE", globalConfigDirectory)
	}

	status, err := repository.Status(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("read status with malformed global Git config: %v", err)
	}
	if status.State != gitmodel.StateUnavailable {
		t.Fatalf("status state = %q, want %q", status.State, gitmodel.StateUnavailable)
	}
}

func TestWorkTreeStateReturnsUnavailableForExecutionFailure(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	repository := newTestRepository(t)
	repository.config.Executable = testGitFailureExecutable(t)
	repository.lookPath = func(string) (string, error) { return repository.config.Executable, nil }

	state := repository.workTreeState(context.Background(), repositoryRoot)
	if state != gitmodel.StateUnavailable {
		t.Fatalf("work tree state = %q, want %q", state, gitmodel.StateUnavailable)
	}
}

func TestGitEnvironmentRemovesInheritedGitOverrides(t *testing.T) {
	environment := gitEnvironment([]string{
		"PATH=/bin",
		"GIT_DIR=elsewhere",
		"GIT_WORK_TREE=elsewhere",
		"GIT_INDEX_FILE=elsewhere",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=core.fsmonitor",
		"GIT_CONFIG_VALUE_0=true",
		"git_dir=lowercase-elsewhere",
	})

	if hasEnvironmentKey(environment, "GIT_DIR") || hasEnvironmentKey(environment, "GIT_WORK_TREE") || hasEnvironmentKey(environment, "GIT_INDEX_FILE") || hasEnvironmentKey(environment, "GIT_CONFIG_COUNT") || hasEnvironmentKey(environment, "git_dir") {
		t.Fatalf("Git overrides survived sanitization: %v", environment)
	}
	if !hasEnvironmentValue(environment, "GIT_OPTIONAL_LOCKS=0") || !hasEnvironmentValue(environment, "GIT_PAGER=cat") {
		t.Fatalf("required Git safety settings missing: %v", environment)
	}
}

func TestStatusScopesNestedWorkspaceAndIgnoresInheritedGitDirectory(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	workspaceRoot := filepath.Join(repositoryRoot, "workspace")
	writeTestFile(t, filepath.Join(workspaceRoot, "inside.txt"), "base\n")
	writeTestFile(t, filepath.Join(repositoryRoot, "outside.txt"), "base\n")
	runGit(t, repositoryRoot, "add", ".")
	runGit(t, repositoryRoot, "-c", "user.name=TermBridge Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	writeTestFile(t, filepath.Join(workspaceRoot, "inside.txt"), "workspace change\n")
	writeTestFile(t, filepath.Join(repositoryRoot, "outside.txt"), "outside change\n")

	repository := newTestRepository(t)
	gitDirectoryKey := "GIT_DIR"
	if os.PathSeparator == '\\' {
		gitDirectoryKey = "git_dir"
	}
	repository.environ = func() []string {
		return append(os.Environ(), gitDirectoryKey+"="+filepath.Join(t.TempDir(), "redirected"))
	}
	status, err := repository.Status(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("read workspace Git status: %v", err)
	}
	if status.State != gitmodel.StateAvailable {
		t.Fatalf("status state = %q, want %q", status.State, gitmodel.StateAvailable)
	}
	if len(status.Changes) != 1 || status.Changes[0].Path != "inside.txt" {
		t.Fatalf("workspace changes = %#v, want only inside.txt", status.Changes)
	}
	assertLayers(t, status.Changes[0], gitmodel.LayerUnstaged)
}

func TestDiffReturnsUnmergedForConflictedChange(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "base\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	runGit(t, repositoryRoot, "-c", "user.name=TermBridge Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")

	runGit(t, repositoryRoot, "checkout", "-b", "conflict-branch")
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "branch change\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	runGit(t, repositoryRoot, "-c", "user.name=TermBridge Test", "-c", "user.email=test@example.com", "commit", "-m", "branch change")
	runGit(t, repositoryRoot, "checkout", "-")
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "main change\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	runGit(t, repositoryRoot, "-c", "user.name=TermBridge Test", "-c", "user.email=test@example.com", "commit", "-m", "main change")

	command := exec.Command(requireGit(t), "merge", "conflict-branch")
	command.Dir = repositoryRoot
	command.Env = gitEnvironment(os.Environ())
	if err := command.Run(); err == nil {
		t.Fatal("merge unexpectedly completed without a conflict")
	}

	repository := newTestRepository(t)
	diff, err := repository.Diff(context.Background(), repositoryRoot, gitmodel.DiffRequest{Path: "tracked.txt", Layer: gitmodel.LayerUnstaged})
	if err != nil {
		t.Fatalf("read conflicted diff: %v", err)
	}
	if diff.State != gitmodel.StateUnmerged {
		t.Fatalf("diff state = %q, want %q", diff.State, gitmodel.StateUnmerged)
	}
}

func TestDiffReturnsSubmoduleForStagedGitlink(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "base\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	runGit(t, repositoryRoot, "-c", "user.name=TermBridge Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	commitId := strings.TrimSpace(runGitOutput(t, repositoryRoot, "rev-parse", "HEAD"))
	runGit(t, repositoryRoot, "update-index", "--add", "--cacheinfo", "160000,"+commitId+",linked-module")

	repository := newTestRepository(t)
	diff, err := repository.Diff(context.Background(), repositoryRoot, gitmodel.DiffRequest{
		Path:  "linked-module",
		Layer: gitmodel.LayerStaged,
	})
	if err != nil {
		t.Fatalf("read staged gitlink diff: %v", err)
	}
	if diff.State != gitmodel.StateSubmodule {
		t.Fatalf("diff state = %q, want %q", diff.State, gitmodel.StateSubmodule)
	}
}

func TestDiffUsesRenameSourceForWorktreeLayer(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	originalText := "first line\nsecond line\nthird line\nfourth line\nfifth line\n"
	modifiedText := "first line\nsecond line\nthird line\nfourth line\nrenamed fifth line\n"
	writeTestFile(t, filepath.Join(repositoryRoot, "old-name.txt"), originalText)
	runGit(t, repositoryRoot, "add", "old-name.txt")
	runGit(t, repositoryRoot, "-c", "user.name=TermBridge Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	writeTestFile(t, filepath.Join(repositoryRoot, "new-name.txt"), modifiedText)

	repository := newTestRepository(t)
	diff, err := repository.diff(
		context.Background(),
		repositoryRoot,
		gitmodel.Change{Path: "new-name.txt", OriginalPath: "old-name.txt", WorktreeStatus: "R"},
		gitmodel.LayerUnstaged,
		"",
	)
	if err != nil {
		t.Fatalf("read worktree rename diff: %v", err)
	}
	if diff.State != gitmodel.StateAvailable || diff.OriginalText != originalText || diff.ModifiedText != modifiedText {
		t.Fatalf("worktree rename diff = %#v, want original to modified content", diff)
	}
}

func TestStatusAndDiffPreserveStagedAndUnstagedLayers(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "base\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	runGit(t, repositoryRoot, "-c", "user.name=TermBridge Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "index\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "worktree\n")
	writeTestFile(t, filepath.Join(repositoryRoot, "untracked.txt"), "new\n")

	repository := newTestRepository(t)
	status, err := repository.Status(context.Background(), repositoryRoot)
	if err != nil {
		t.Fatalf("read Git status: %v", err)
	}
	if status.State != gitmodel.StateAvailable {
		t.Fatalf("status state = %q, want %q", status.State, gitmodel.StateAvailable)
	}
	tracked := changeForPath(t, status.Changes, "tracked.txt")
	assertLayers(t, tracked, gitmodel.LayerStaged, gitmodel.LayerUnstaged)
	staged, err := repository.Diff(context.Background(), repositoryRoot, gitmodel.DiffRequest{Path: "tracked.txt", Layer: gitmodel.LayerStaged})
	if err != nil {
		t.Fatalf("read staged diff: %v", err)
	}
	if staged.State != gitmodel.StateAvailable || staged.OriginalText != "base\n" || staged.ModifiedText != "index\n" {
		t.Fatalf("staged diff = %#v, want base to index", staged)
	}
	unstaged, err := repository.Diff(context.Background(), repositoryRoot, gitmodel.DiffRequest{Path: "tracked.txt", Layer: gitmodel.LayerUnstaged})
	if err != nil {
		t.Fatalf("read unstaged diff: %v", err)
	}
	if unstaged.State != gitmodel.StateAvailable || unstaged.OriginalText != "index\n" || unstaged.ModifiedText != "worktree\n" {
		t.Fatalf("unstaged diff = %#v, want index to worktree", unstaged)
	}
	untracked := changeForPath(t, status.Changes, "untracked.txt")
	assertLayers(t, untracked, gitmodel.LayerUntracked)
}

func TestDiffUsesStagedRenameDestinationForUnstagedLayer(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	indexText := "first line\nsecond line\nthird line\nfourth line\nfifth line\n"
	worktreeText := "first line\nsecond line\nthird line\nfourth line\nworktree fifth line\n"
	writeTestFile(t, filepath.Join(repositoryRoot, "old-name.txt"), indexText)
	runGit(t, repositoryRoot, "add", "old-name.txt")
	runGit(t, repositoryRoot, "-c", "user.name=TermBridge Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	runGit(t, repositoryRoot, "mv", "old-name.txt", "new-name.txt")
	writeTestFile(t, filepath.Join(repositoryRoot, "new-name.txt"), worktreeText)

	repository := newTestRepository(t)
	status, err := repository.Status(context.Background(), repositoryRoot)
	if err != nil {
		t.Fatalf("read staged rename status: %v", err)
	}
	change := changeForPath(t, status.Changes, "new-name.txt")
	if change.IndexStatus != "R" || change.OriginalPath != "old-name.txt" || change.WorktreeStatus != "M" {
		t.Fatalf("staged rename status = %#v, want staged rename and unstaged edit", change)
	}
	diff, err := repository.Diff(context.Background(), repositoryRoot, gitmodel.DiffRequest{Path: "new-name.txt", Layer: gitmodel.LayerUnstaged})
	if err != nil {
		t.Fatalf("read staged rename worktree diff: %v", err)
	}
	if diff.State != gitmodel.StateAvailable || diff.OriginalText != indexText || diff.ModifiedText != worktreeText {
		t.Fatalf("staged rename worktree diff = %#v, want index destination to worktree content", diff)
	}
}

func TestDiffReturnsLayerUnavailableForChangedTrackedFile(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "base\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	runGit(t, repositoryRoot, "-c", "user.name=TermBridge Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "changed\n")

	repository := newTestRepository(t)
	diff, err := repository.Diff(context.Background(), repositoryRoot, gitmodel.DiffRequest{Path: "tracked.txt", Layer: gitmodel.LayerUntracked})
	if err != nil {
		t.Fatalf("read unavailable diff: %v", err)
	}
	if diff.State != gitmodel.StateLayerUnavailable {
		t.Fatalf("diff state = %q, want %q", diff.State, gitmodel.StateLayerUnavailable)
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

func TestMutatePathStagesUnstagesRestoresAndDeletesOneScopedFile(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "base\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	commitTestChanges(t, repositoryRoot, "initial")
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "index\n")

	repository := newTestRepository(t)
	staged, err := repository.MutatePath(context.Background(), repositoryRoot, gitmodel.PathMutationRequest{
		Path: "tracked.txt", Layer: gitmodel.LayerUnstaged, Mutation: gitmodel.MutationStage,
	})
	if err != nil {
		t.Fatalf("stage tracked file: %v", err)
	}
	if staged.State != gitmodel.OperationStateAvailable {
		t.Fatalf("stage state = %q, want available", staged.State)
	}
	assertLayers(t, changeForPath(t, staged.Status.Changes, "tracked.txt"), gitmodel.LayerStaged)

	unstaged, err := repository.MutatePath(context.Background(), repositoryRoot, gitmodel.PathMutationRequest{
		Path: "tracked.txt", Layer: gitmodel.LayerStaged, Mutation: gitmodel.MutationUnstage,
	})
	if err != nil {
		t.Fatalf("unstage tracked file: %v", err)
	}
	if unstaged.State != gitmodel.OperationStateAvailable {
		t.Fatalf("unstage state = %q, want available", unstaged.State)
	}
	assertLayers(t, changeForPath(t, unstaged.Status.Changes, "tracked.txt"), gitmodel.LayerUnstaged)

	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "index\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "worktree\n")
	restored, err := repository.MutatePath(context.Background(), repositoryRoot, gitmodel.PathMutationRequest{
		Path: "tracked.txt", Layer: gitmodel.LayerUnstaged, Mutation: gitmodel.MutationRestoreUnstaged,
	})
	if err != nil {
		t.Fatalf("restore tracked worktree file: %v", err)
	}
	if restored.State != gitmodel.OperationStateAvailable {
		t.Fatalf("restore state = %q, want available", restored.State)
	}
	contents, err := os.ReadFile(filepath.Join(repositoryRoot, "tracked.txt"))
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if string(contents) != "index\n" {
		t.Fatalf("restored file = %q, want index content", contents)
	}

	writeTestFile(t, filepath.Join(repositoryRoot, "untracked.txt"), "new\n")
	deleted, err := repository.MutatePath(context.Background(), repositoryRoot, gitmodel.PathMutationRequest{
		Path: "untracked.txt", Layer: gitmodel.LayerUntracked, Mutation: gitmodel.MutationDeleteUntracked,
	})
	if err != nil {
		t.Fatalf("delete untracked file: %v", err)
	}
	if deleted.State != gitmodel.OperationStateAvailable {
		t.Fatalf("delete state = %q, want available", deleted.State)
	}
	if _, err := os.Stat(filepath.Join(repositoryRoot, "untracked.txt")); !os.IsNotExist(err) {
		t.Fatalf("untracked file still exists or had an unexpected stat error: %v", err)
	}
}

func TestMutatePathUnstagesNewlyStagedFile(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "base\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	commitTestChanges(t, repositoryRoot, "initial")
	writeTestFile(t, filepath.Join(repositoryRoot, "new.txt"), "new\n")
	runGit(t, repositoryRoot, "add", "new.txt")

	repository := newTestRepository(t)
	result, err := repository.MutatePath(context.Background(), repositoryRoot, gitmodel.PathMutationRequest{
		Path: "new.txt", Layer: gitmodel.LayerStaged, Mutation: gitmodel.MutationUnstage,
	})
	if err != nil {
		t.Fatalf("unstage newly staged file: %v", err)
	}
	if result.State != gitmodel.OperationStateAvailable {
		t.Fatalf("unstage state = %q, want available", result.State)
	}
	assertLayers(t, changeForPath(t, result.Status.Changes, "new.txt"), gitmodel.LayerUntracked)
}

func TestCommitSummaryAndLocalBranchOperations(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "base\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	commitTestChanges(t, repositoryRoot, "initial")
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "committed\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")

	repository := newTestRepository(t)
	committed, err := repository.Commit(context.Background(), repositoryRoot, gitmodel.CommitRequest{Message: "update tracked file\n"})
	if err != nil {
		t.Fatalf("commit staged file: %v", err)
	}
	if committed.State != gitmodel.OperationStateAvailable {
		t.Fatalf("commit state = %q, want available", committed.State)
	}
	if len(committed.Summary.History) != 2 || committed.Summary.History[0].Subject != "update tracked file" {
		t.Fatalf("commit history = %#v, want newest commit metadata", committed.Summary.History)
	}

	created, err := repository.CreateBranch(context.Background(), repositoryRoot, gitmodel.BranchRequest{Name: "feature/review"})
	if err != nil {
		t.Fatalf("create local branch: %v", err)
	}
	if created.State != gitmodel.OperationStateAvailable || !slices.Contains(created.Summary.LocalBranches, "feature/review") {
		t.Fatalf("branch create result = %#v, want created local branch", created)
	}

	switched, err := repository.SwitchBranch(context.Background(), repositoryRoot, gitmodel.BranchRequest{Name: "feature/review"})
	if err != nil {
		t.Fatalf("switch local branch: %v", err)
	}
	if switched.State != gitmodel.OperationStateAvailable || switched.Summary.CurrentBranch != "feature/review" {
		t.Fatalf("branch switch result = %#v, want feature/review current", switched)
	}

	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "dirty\n")
	blocked, err := repository.SwitchBranch(context.Background(), repositoryRoot, gitmodel.BranchRequest{Name: "master"})
	if err != nil {
		t.Fatalf("switch dirty repository: %v", err)
	}
	if blocked.State != gitmodel.OperationStateCleanWorktreeRequired {
		t.Fatalf("dirty switch state = %q, want clean worktree required", blocked.State)
	}
}

func TestMutatePathRejectsUnmergedChangesWithoutTouchingTheWorktree(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "base\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	commitTestChanges(t, repositoryRoot, "initial")
	runGit(t, repositoryRoot, "checkout", "-b", "conflict-branch")
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "branch\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	commitTestChanges(t, repositoryRoot, "branch change")
	runGit(t, repositoryRoot, "checkout", "-")
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "main\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	commitTestChanges(t, repositoryRoot, "main change")
	command := exec.Command(requireGit(t), "merge", "conflict-branch")
	command.Dir = repositoryRoot
	command.Env = gitEnvironment(os.Environ())
	if err := command.Run(); err == nil {
		t.Fatal("merge unexpectedly completed without a conflict")
	}

	repository := newTestRepository(t)
	result, err := repository.MutatePath(context.Background(), repositoryRoot, gitmodel.PathMutationRequest{
		Path: "tracked.txt", Layer: gitmodel.LayerUnstaged, Mutation: gitmodel.MutationRestoreUnstaged,
	})
	if err != nil {
		t.Fatalf("restore unmerged path: %v", err)
	}
	if result.State != gitmodel.OperationStateConflict {
		t.Fatalf("unmerged mutation state = %q, want %q", result.State, gitmodel.OperationStateConflict)
	}
	contents, err := os.ReadFile(filepath.Join(repositoryRoot, "tracked.txt"))
	if err != nil {
		t.Fatalf("read conflicted file: %v", err)
	}
	if !strings.Contains(string(contents), "<<<<<<<") {
		t.Fatalf("conflicted file content = %q, want conflict markers retained", contents)
	}
}

func TestCommitFailureOmitsPreCommitStatus(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "base\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	commitTestChanges(t, repositoryRoot, "initial")
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "staged\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	writeTestFile(t, filepath.Join(repositoryRoot, ".git", "hooks", "pre-commit"), "#!/bin/sh\nexit 1\n")
	if err := os.Chmod(filepath.Join(repositoryRoot, ".git", "hooks", "pre-commit"), 0o700); err != nil {
		t.Fatalf("make pre-commit hook executable: %v", err)
	}

	repository := newTestRepository(t)
	result, err := repository.Commit(context.Background(), repositoryRoot, gitmodel.CommitRequest{Message: "must fail"})
	if err != nil {
		t.Fatalf("commit with failing hook: %v", err)
	}
	if result.State != gitmodel.OperationStateCommitFailed {
		t.Fatalf("commit failure state = %q, want %q", result.State, gitmodel.OperationStateCommitFailed)
	}
	if result.Status.State != "" || result.Status.Changes != nil || result.Status.Message != "" {
		t.Fatalf("commit failure status = %#v, want no pre-command snapshot", result.Status)
	}
}

func TestCommitAndBranchOperationsRejectInvalidInput(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	writeTestFile(t, filepath.Join(repositoryRoot, "tracked.txt"), "base\n")
	runGit(t, repositoryRoot, "add", "tracked.txt")
	commitTestChanges(t, repositoryRoot, "initial")
	repository := newTestRepository(t)

	emptyCommit, err := repository.Commit(context.Background(), repositoryRoot, gitmodel.CommitRequest{Message: "   "})
	if err != nil {
		t.Fatalf("commit empty message: %v", err)
	}
	if emptyCommit.State != gitmodel.OperationStateInvalidCommitMessage {
		t.Fatalf("empty commit state = %q, want %q", emptyCommit.State, gitmodel.OperationStateInvalidCommitMessage)
	}
	oversizedCommit, err := repository.Commit(context.Background(), repositoryRoot, gitmodel.CommitRequest{Message: strings.Repeat("a", gitmodel.MaxCommitMessageBytes+1)})
	if err != nil {
		t.Fatalf("commit oversized message: %v", err)
	}
	if oversizedCommit.State != gitmodel.OperationStateInvalidCommitMessage {
		t.Fatalf("oversized commit state = %q, want %q", oversizedCommit.State, gitmodel.OperationStateInvalidCommitMessage)
	}
	invalidBranch, err := repository.CreateBranch(context.Background(), repositoryRoot, gitmodel.BranchRequest{Name: "bad..branch"})
	if err != nil {
		t.Fatalf("create invalid branch: %v", err)
	}
	if invalidBranch.State != gitmodel.OperationStateInvalidBranchName {
		t.Fatalf("invalid branch state = %q, want %q", invalidBranch.State, gitmodel.OperationStateInvalidBranchName)
	}
	missingBranch, err := repository.SwitchBranch(context.Background(), repositoryRoot, gitmodel.BranchRequest{Name: "missing"})
	if err != nil {
		t.Fatalf("switch missing branch: %v", err)
	}
	if missingBranch.State != gitmodel.OperationStateBranchNotFound {
		t.Fatalf("missing branch state = %q, want %q", missingBranch.State, gitmodel.OperationStateBranchNotFound)
	}
}

func TestCreateBranchRejectsUnbornRepository(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	repository := newTestRepository(t)

	result, err := repository.CreateBranch(context.Background(), repositoryRoot, gitmodel.BranchRequest{Name: "feature/review"})
	if err != nil {
		t.Fatalf("create branch in unborn repository: %v", err)
	}
	if result.State != gitmodel.OperationStateUnavailable {
		t.Fatalf("unborn repository branch state = %q, want %q", result.State, gitmodel.OperationStateUnavailable)
	}
}

func TestCommitRejectsStagedChangesOutsideNestedWorkspace(t *testing.T) {
	repositoryRoot := initializeGitRepository(t)
	workspaceRoot := filepath.Join(repositoryRoot, "workspace")
	writeTestFile(t, filepath.Join(workspaceRoot, "inside.txt"), "base\n")
	writeTestFile(t, filepath.Join(repositoryRoot, "outside.txt"), "base\n")
	runGit(t, repositoryRoot, "add", ".")
	commitTestChanges(t, repositoryRoot, "initial")
	writeTestFile(t, filepath.Join(workspaceRoot, "inside.txt"), "inside\n")
	writeTestFile(t, filepath.Join(repositoryRoot, "outside.txt"), "outside\n")
	runGit(t, repositoryRoot, "add", ".")

	repository := newTestRepository(t)
	result, err := repository.Commit(context.Background(), workspaceRoot, gitmodel.CommitRequest{Message: "workspace update"})
	if err != nil {
		t.Fatalf("commit nested workspace: %v", err)
	}
	if result.State != gitmodel.OperationStateConflict {
		t.Fatalf("nested commit state = %q, want conflict for outside staged changes", result.State)
	}
	if summary := strings.TrimSpace(runGitOutput(t, repositoryRoot, "log", "-1", "--format=%s")); summary != "initial" {
		t.Fatalf("latest commit = %q, want initial because scope guard must not commit outside change", summary)
	}
}

func commitTestChanges(t *testing.T, directory string, message string) {
	t.Helper()
	runGit(t, directory, "-c", "user.name=TermBridge Test", "-c", "user.email=test@example.com", "commit", "-m", message)
}

func initializeGitRepository(t *testing.T) string {
	t.Helper()
	requireGit(t)
	directory := t.TempDir()
	runGit(t, directory, "init")
	return directory
}

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	files, err := workspacefile.New(workspacefile.Config{
		MaxTextBytes:              1024 * 1024,
		MaxDirectoryEntries:       128,
		MaxRecursiveDeleteEntries: 128,
	})
	if err != nil {
		t.Fatalf("create workspace file store: %v", err)
	}
	repository := New(Config{Executable: requireGit(t), MaxStdoutBytes: 1024 * 1024, MaxStderrBytes: 64 * 1024, MaxTextBytes: 1024 * 1024}, files)
	return repository
}

func requireGit(t *testing.T) string {
	t.Helper()
	executable, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("Git executable unavailable: %v", err)
	}
	return executable
}

func runGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	if _, err := runGitOutputResult(t, directory, args...); err != nil {
		t.Fatal(err)
	}
}

func runGitOutput(t *testing.T, directory string, args ...string) string {
	t.Helper()
	output, err := runGitOutputResult(t, directory, args...)
	if err != nil {
		t.Fatal(err)
	}
	return output
}

func runGitOutputResult(t *testing.T, directory string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command(requireGit(t), args...)
	command.Dir = directory
	command.Env = gitEnvironment(os.Environ())
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w; output=%s", strings.Join(args, " "), err, output)
	}
	return string(output), nil
}

func testGitFailureExecutable(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	if os.PathSeparator == '\\' {
		path := filepath.Join(directory, "git-failure.cmd")
		if err := os.WriteFile(path, []byte("@exit /b 17\r\n"), 0o600); err != nil {
			t.Fatalf("write Git failure executable: %v", err)
		}
		return path
	}
	path := filepath.Join(directory, "git-failure")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 17\n"), 0o700); err != nil {
		t.Fatalf("write Git failure executable: %v", err)
	}
	return path
}

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create test directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

func changeForPath(t *testing.T, changes []gitmodel.Change, path string) gitmodel.Change {
	t.Helper()
	for _, change := range changes {
		if change.Path == path {
			return change
		}
	}
	t.Fatalf("Git status does not contain %q: %#v", path, changes)
	return gitmodel.Change{}
}

func environmentWith(base []string, values ...string) []string {
	environment := append([]string(nil), base...)
	for index := 0; index < len(values); index += 2 {
		key, value := values[index], values[index+1]
		for entryIndex := len(environment) - 1; entryIndex >= 0; entryIndex-- {
			entryKey, _, found := strings.Cut(environment[entryIndex], "=")
			if found && strings.EqualFold(entryKey, key) {
				environment = append(environment[:entryIndex], environment[entryIndex+1:]...)
			}
		}
		environment = append(environment, key+"="+value)
	}
	return environment
}

func hasEnvironmentKey(environment []string, want string) bool {
	for _, entry := range environment {
		key, _, found := strings.Cut(entry, "=")
		if found && key == want {
			return true
		}
	}
	return false
}

func hasEnvironmentValue(environment []string, want string) bool {
	return slices.Contains(environment, want)
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
