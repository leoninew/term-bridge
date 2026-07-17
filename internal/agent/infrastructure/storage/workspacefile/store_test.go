package workspacefile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
)

func TestRootResolveKeepsLogicalPathWithinWorkspace(t *testing.T) {
	rootPath := t.TempDir()
	opened, err := openRoot(rootPath)
	if err != nil {
		t.Fatalf("open root: %v", err)
	}
	defer func() { _ = opened.Close() }()

	resolved, err := opened.resolve("directory/notes.txt")
	if err != nil {
		t.Fatalf("resolve nested path: %v", err)
	}
	if resolved != filepath.Join(rootPath, "directory", "notes.txt") {
		t.Fatalf("resolved path = %q, want workspace child", resolved)
	}

	for _, path := range []filemodel.RelativePath{"../outside.txt", "/outside.txt", "C:/outside.txt", "directory\\notes.txt"} {
		if _, err := opened.resolve(path); filemodel.CodeOf(err) != "invalid_file_path" {
			t.Fatalf("resolve %q error code = %q, want invalid_file_path; err=%v", path, filemodel.CodeOf(err), err)
		}
	}
}

func TestStoreCreatesWritesAndDeletesWorkspaceEntry(t *testing.T) {
	rootPath := t.TempDir()
	store := testStore(t)
	root := entryAt(t, rootPath, "")

	created, err := store.CreateFile(context.Background(), rootPath, filemodel.CreateFileRequest{Path: "notes.txt", Text: "first", ExpectedParentRevision: root.Revision})
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if created.DestinationEntry == nil || created.DestinationEntry.Path != "notes.txt" {
		t.Fatalf("created entry = %#v, want notes.txt", created.DestinationEntry)
	}

	read, err := store.Read(context.Background(), rootPath, "notes.txt")
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if read.Text != "first" {
		t.Fatalf("read text = %q, want %q", read.Text, "first")
	}

	updated, err := store.Write(context.Background(), rootPath, filemodel.WriteRequest{Path: "notes.txt", Text: "second", ExpectedRevision: read.Entry.Revision})
	if err != nil {
		t.Fatalf("write file: %v", err)
	}
	if updated.DestinationEntry == nil || updated.DestinationEntry.Revision == read.Entry.Revision {
		t.Fatalf("updated entry = %#v, want a new revision", updated.DestinationEntry)
	}

	parent := entryAt(t, rootPath, "")
	if _, err := store.Delete(context.Background(), rootPath, filemodel.DeleteRequest{Path: "notes.txt", ExpectedRevision: updated.DestinationEntry.Revision, ExpectedParentRevision: parent.Revision}); err != nil {
		t.Fatalf("delete file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(rootPath, "notes.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted file stat error = %v, want not found", err)
	}
}

func TestStoreRevisionChangesWhenContentsChangeWithoutMetadataChange(t *testing.T) {
	rootPath := t.TempDir()
	filePath := filepath.Join(rootPath, "notes.txt")
	if err := os.WriteFile(filePath, []byte("first"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	before := entryAt(t, rootPath, "notes.txt")

	if err := os.WriteFile(filePath, []byte("other"), 0o600); err != nil {
		t.Fatalf("replace content: %v", err)
	}
	if err := os.Chtimes(filePath, before.ModifiedAt, before.ModifiedAt); err != nil {
		t.Fatalf("restore timestamp: %v", err)
	}
	after := entryAt(t, rootPath, "notes.txt")

	if after.Size != before.Size || !after.ModifiedAt.Equal(before.ModifiedAt) {
		t.Fatalf("replacement metadata = size:%d mtime:%v, want preserved size:%d mtime:%v", after.Size, after.ModifiedAt, before.Size, before.ModifiedAt)
	}
	if after.Revision == before.Revision {
		t.Fatalf("revision = %q after equal-metadata content replacement, want a new revision", after.Revision)
	}
}

func TestStoreRejectsStaleWrite(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootPath, "notes.txt"), []byte("first"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	store := testStore(t)
	entry := entryAt(t, rootPath, "notes.txt")

	_, err := store.Write(context.Background(), rootPath, filemodel.WriteRequest{Path: "notes.txt", Text: "second", ExpectedRevision: "stale"})
	if filemodel.CodeOf(err) != "revision_conflict" {
		t.Fatalf("write error code = %q, want revision_conflict; err=%v", filemodel.CodeOf(err), err)
	}
	read, err := store.Read(context.Background(), rootPath, "notes.txt")
	if err != nil {
		t.Fatalf("read after conflict: %v", err)
	}
	if read.Entry.Revision != entry.Revision || read.Text != "first" {
		t.Fatalf("file after conflict = %#v, want unchanged first revision", read)
	}
}

func TestStoreRequiresRecursiveIntentForNonEmptyDirectory(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.Mkdir(filepath.Join(rootPath, "directory"), 0o700); err != nil {
		t.Fatalf("create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootPath, "directory", "notes.txt"), []byte("text"), 0o600); err != nil {
		t.Fatalf("seed nested file: %v", err)
	}
	store := testStore(t)
	directory := entryAt(t, rootPath, "directory")
	root := entryAt(t, rootPath, "")

	_, err := store.Delete(context.Background(), rootPath, filemodel.DeleteRequest{Path: "directory", ExpectedRevision: directory.Revision, ExpectedParentRevision: root.Revision})
	if filemodel.CodeOf(err) != "directory_not_empty" {
		t.Fatalf("delete error code = %q, want directory_not_empty; err=%v", filemodel.CodeOf(err), err)
	}
	if _, err := os.Stat(filepath.Join(rootPath, "directory", "notes.txt")); err != nil {
		t.Fatalf("nested file after refused deletion: %v", err)
	}

	directory = entryAt(t, rootPath, "directory")
	root = entryAt(t, rootPath, "")
	deleted, err := store.Delete(context.Background(), rootPath, filemodel.DeleteRequest{Path: "directory", ExpectedRevision: directory.Revision, ExpectedParentRevision: root.Revision, Recursive: true})
	if err != nil {
		t.Fatalf("delete directory recursively: %v", err)
	}
	if deleted.AffectedCount != 2 {
		t.Fatalf("deleted entry count = %d, want 2", deleted.AffectedCount)
	}
	if _, err := os.Stat(filepath.Join(rootPath, "directory")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted directory stat error = %v, want not found", err)
	}
}

func TestStoreRejectsSymlinkEntry(t *testing.T) {
	rootPath := t.TempDir()
	outsidePath := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o600); err != nil {
		t.Fatalf("seed outside file: %v", err)
	}
	linkPath := filepath.Join(rootPath, "link.txt")
	if err := os.Symlink(outsidePath, linkPath); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	store := testStore(t)

	_, err := store.Read(context.Background(), rootPath, "link.txt")
	if filemodel.CodeOf(err) != "symlink_unsupported" {
		t.Fatalf("read symlink error code = %q, want symlink_unsupported; err=%v", filemodel.CodeOf(err), err)
	}
}

func TestStoreRejectsSymlinkIntermediateDirectory(t *testing.T) {
	rootPath := t.TempDir()
	outsidePath := t.TempDir()
	if err := os.WriteFile(filepath.Join(outsidePath, "outside.txt"), []byte("outside"), 0o600); err != nil {
		t.Fatalf("seed outside file: %v", err)
	}
	if err := os.Symlink(outsidePath, filepath.Join(rootPath, "linked")); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	store := testStore(t)

	_, err := store.Read(context.Background(), rootPath, "linked/outside.txt")
	if filemodel.CodeOf(err) != "symlink_unsupported" {
		t.Fatalf("read through symlink error code = %q, want symlink_unsupported; err=%v", filemodel.CodeOf(err), err)
	}
}

func TestStoreMoveOverwriteReplacesDestination(t *testing.T) {
	rootPath := t.TempDir()
	store := testStore(t)
	if err := os.WriteFile(filepath.Join(rootPath, "a.txt"), []byte("source"), 0o600); err != nil {
		t.Fatalf("seed a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootPath, "b.txt"), []byte("dest"), 0o600); err != nil {
		t.Fatalf("seed b: %v", err)
	}

	_, err := store.Move(context.Background(), rootPath, filemodel.MoveRequest{
		SourcePath:      "a.txt",
		DestinationPath: "b.txt",
		Overwrite:       false,
	})
	if filemodel.CodeOf(err) != "already_exists" {
		t.Fatalf("move without overwrite code = %q, want already_exists; err=%v", filemodel.CodeOf(err), err)
	}

	moved, err := store.Move(context.Background(), rootPath, filemodel.MoveRequest{
		SourcePath:      "a.txt",
		DestinationPath: "b.txt",
		Overwrite:       true,
	})
	if err != nil {
		t.Fatalf("move with overwrite: %v", err)
	}
	if moved.DestinationEntry == nil || moved.DestinationEntry.Path != "b.txt" {
		t.Fatalf("moved entry = %#v", moved.DestinationEntry)
	}
	if _, err := os.Stat(filepath.Join(rootPath, "a.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source should be gone: %v", err)
	}
	read, err := store.Read(context.Background(), rootPath, "b.txt")
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if read.Text != "source" {
		t.Fatalf("destination text = %q, want source", read.Text)
	}
}

func TestStoreStatIsMetadataOnly(t *testing.T) {
	rootPath := t.TempDir()
	store := testStore(t)
	if err := os.WriteFile(filepath.Join(rootPath, "index.html"), []byte("<html></html>"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(rootPath, "web"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fileEntry, err := store.Stat(context.Background(), rootPath, "index.html")
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if fileEntry.Kind != filemodel.EntryKindFile || fileEntry.Size != int64(len("<html></html>")) {
		t.Fatalf("file entry = %#v", fileEntry)
	}
	dirEntry, err := store.Stat(context.Background(), rootPath, "web")
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if dirEntry.Kind != filemodel.EntryKindDirectory {
		t.Fatalf("dir entry = %#v", dirEntry)
	}
	rootEntry, err := store.Stat(context.Background(), rootPath, "")
	if err != nil {
		t.Fatalf("stat root: %v", err)
	}
	if rootEntry.Kind != filemodel.EntryKindDirectory {
		t.Fatalf("root entry = %#v", rootEntry)
	}
}

func TestStoreReadBinaryPNG(t *testing.T) {
	rootPath := t.TempDir()
	store := testStore(t)
	// Minimal PNG-like binary with non-UTF8 + NUL
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0xff, 0xfe}
	if err := os.WriteFile(filepath.Join(rootPath, "sketch.png"), png, 0o600); err != nil {
		t.Fatalf("write png: %v", err)
	}
	read, err := store.Read(context.Background(), rootPath, "sketch.png")
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if read.Text != string(png) {
		t.Fatalf("binary content mismatch len=%d want=%d", len(read.Text), len(png))
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := New(Config{MaxTextBytes: 1024, MaxDirectoryEntries: 100, MaxRecursiveDeleteEntries: 100})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	return store
}

func entryAt(t *testing.T, rootPath string, path filemodel.RelativePath) filemodel.Entry {
	t.Helper()
	root, err := openRoot(rootPath)
	if err != nil {
		t.Fatalf("open root: %v", err)
	}
	defer func() { _ = root.Close() }()
	entry, err := root.entry(path)
	if err != nil {
		t.Fatalf("read entry %q: %v", path, err)
	}
	return entry
}
