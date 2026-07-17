package workspacewatch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
)

func TestManagerSharesRootWatchAndPublishesScopedChanges(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "nested"))
	factory := &fakeWatcherFactory{}
	manager, err := newManager(context.Background(), Config{SubscriberQueueSize: 4}, factory.New)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer func() { _ = manager.Close() }()

	first, err := manager.Subscribe(context.Background(), root)
	if err != nil {
		t.Fatalf("subscribe first: %v", err)
	}
	second, err := manager.Subscribe(context.Background(), filepath.Join(root, "."))
	if err != nil {
		t.Fatalf("subscribe second: %v", err)
	}

	watcher := factory.Only(t)
	watcher.RequireAdded(t, root, filepath.Join(root, "nested"))
	watcher.events <- fsnotify.Event{Name: filepath.Join(root, "nested", "notes.txt"), Op: fsnotify.Write}

	firstChange := requireChange(t, first.Events())
	secondChange := requireChange(t, second.Events())
	assertChange(t, firstChange, filemodel.WorkspaceChangeUpdated, "nested/notes.txt", "", 1)
	assertChange(t, secondChange, filemodel.WorkspaceChangeUpdated, "nested/notes.txt", "", 1)

	if err := first.Close(); err != nil {
		t.Fatalf("close first subscription: %v", err)
	}
	watcher.RequireOpen(t)
	if err := second.Close(); err != nil {
		t.Fatalf("close second subscription: %v", err)
	}
	watcher.RequireClosed(t)
}

func TestManagerPublishesPhysicalFileChanges(t *testing.T) {
	root := t.TempDir()
	manager, err := New(context.Background(), Config{SubscriberQueueSize: 4})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer func() { _ = manager.Close() }()

	subscription, err := manager.Subscribe(context.Background(), root)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "external.txt"), []byte("changed outside the API"), 0o600); err != nil {
		t.Fatalf("write physical file: %v", err)
	}

	change := requireChange(t, subscription.Events())
	if change.Path != "external.txt" || (change.Kind != filemodel.WorkspaceChangeAdded && change.Kind != filemodel.WorkspaceChangeUpdated) {
		t.Fatalf("physical change = %#v, want add or update for external.txt", change)
	}
}

func TestManagerSignalsRescanAndClosesOverflowedSubscription(t *testing.T) {
	root := t.TempDir()
	factory := &fakeWatcherFactory{}
	manager, err := newManager(context.Background(), Config{SubscriberQueueSize: 1}, factory.New)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer func() { _ = manager.Close() }()

	subscription, err := manager.Subscribe(context.Background(), root)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	watcher := factory.Only(t)
	watcher.events <- fsnotify.Event{Name: filepath.Join(root, "first.txt"), Op: fsnotify.Write}
	waitForQueuedChange(t, subscription)
	watcher.events <- fsnotify.Event{Name: filepath.Join(root, "second.txt"), Op: fsnotify.Write}
	waitForSubscriptionFinish(t, subscription)

	change := requireChange(t, subscription.Events())
	assertChange(t, change, filemodel.WorkspaceChangeRescanRequired, "", "", 3)
	assertClosed(t, subscription.Events())
}

func TestManagerSignalsRescanForAmbiguousRename(t *testing.T) {
	root := t.TempDir()
	factory := &fakeWatcherFactory{}
	manager, err := newManager(context.Background(), Config{SubscriberQueueSize: 2}, factory.New)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer func() { _ = manager.Close() }()

	subscription, err := manager.Subscribe(context.Background(), root)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	watcher := factory.Only(t)
	watcher.events <- fsnotify.Event{Name: filepath.Join(root, "before.txt"), Op: fsnotify.Rename}

	change := requireChange(t, subscription.Events())
	assertChange(t, change, filemodel.WorkspaceChangeRescanRequired, "", "", 1)
	assertClosed(t, subscription.Events())
}

func TestManagerAddsNewDirectoryTreeWithoutFollowingSymlinks(t *testing.T) {
	root := t.TempDir()
	createdDirectory := filepath.Join(root, "created")
	mustMkdirAll(t, filepath.Join(createdDirectory, "nested"))
	outside := t.TempDir()
	linkPath := filepath.Join(createdDirectory, "linked")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	factory := &fakeWatcherFactory{}
	manager, err := newManager(context.Background(), Config{SubscriberQueueSize: 2}, factory.New)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer func() { _ = manager.Close() }()

	subscription, err := manager.Subscribe(context.Background(), root)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	watcher := factory.Only(t)
	watcher.events <- fsnotify.Event{Name: createdDirectory, Op: fsnotify.Create}

	change := requireChange(t, subscription.Events())
	assertChange(t, change, filemodel.WorkspaceChangeAdded, "created", "", 1)
	watcher.RequireAdded(t, root, createdDirectory, filepath.Join(createdDirectory, "nested"))
	watcher.RequireNotAdded(t, linkPath)
}

func TestManagerSignalsRescanWhenEventLeavesWorkspace(t *testing.T) {
	root := t.TempDir()
	factory := &fakeWatcherFactory{}
	manager, err := newManager(context.Background(), Config{SubscriberQueueSize: 2}, factory.New)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer func() { _ = manager.Close() }()

	subscription, err := manager.Subscribe(context.Background(), root)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	watcher := factory.Only(t)
	watcher.events <- fsnotify.Event{Name: filepath.Join(filepath.Dir(root), "outside.txt"), Op: fsnotify.Write}

	change := requireChange(t, subscription.Events())
	assertChange(t, change, filemodel.WorkspaceChangeRescanRequired, "", "", 1)
	assertClosed(t, subscription.Events())
}

func TestManagerStopsRootWatchWhenContextEnds(t *testing.T) {
	root := t.TempDir()
	factory := &fakeWatcherFactory{}
	manager, err := newManager(context.Background(), Config{SubscriberQueueSize: 2}, factory.New)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer func() { _ = manager.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	subscription, err := manager.Subscribe(ctx, root)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	watcher := factory.Only(t)
	cancel()

	assertClosed(t, subscription.Events())
	watcher.RequireClosed(t)
}

func TestManagerRejectsInvalidSubscriberQueueSize(t *testing.T) {
	_, err := New(context.Background(), Config{})
	if err == nil {
		t.Fatal("new manager error = nil, want invalid queue size")
	}
}

type fakeWatcherFactory struct {
	mu       sync.Mutex
	watchers []*fakeWatcher
}

func (f *fakeWatcherFactory) New() (watcher, error) {
	created := &fakeWatcher{
		events: make(chan fsnotify.Event, 16),
		errors: make(chan error, 2),
		added:  map[string]struct{}{},
	}
	f.mu.Lock()
	f.watchers = append(f.watchers, created)
	f.mu.Unlock()
	return created, nil
}

func (f *fakeWatcherFactory) Only(t *testing.T) *fakeWatcher {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.watchers) != 1 {
		t.Fatalf("watcher count = %d, want 1", len(f.watchers))
	}
	return f.watchers[0]
}

type fakeWatcher struct {
	events chan fsnotify.Event
	errors chan error

	mu     sync.Mutex
	added  map[string]struct{}
	closed bool
}

func (w *fakeWatcher) Add(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return errors.New("watcher is closed")
	}
	w.added[filepath.Clean(path)] = struct{}{}
	return nil
}

func (w *fakeWatcher) Remove(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.added, filepath.Clean(path))
	return nil
}

func (w *fakeWatcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	close(w.events)
	close(w.errors)
	return nil
}

func (w *fakeWatcher) Events() <-chan fsnotify.Event { return w.events }

func (w *fakeWatcher) Errors() <-chan error { return w.errors }

func (w *fakeWatcher) RequireAdded(t *testing.T, paths ...string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		w.mu.Lock()
		matched := true
		for _, path := range paths {
			if _, ok := w.added[filepath.Clean(path)]; !ok {
				matched = false
				break
			}
		}
		w.mu.Unlock()
		if matched {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("watched directories = %#v, want %#v", w.added, paths)
		}
		time.Sleep(time.Millisecond)
	}
}

func (w *fakeWatcher) RequireNotAdded(t *testing.T, path string) {
	t.Helper()
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.added[filepath.Clean(path)]; ok {
		t.Fatalf("unexpected symlink watch %q", path)
	}
}

func (w *fakeWatcher) RequireOpen(t *testing.T) {
	t.Helper()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		t.Fatal("watcher closed while another subscription remains")
	}
}

func (w *fakeWatcher) RequireClosed(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		w.mu.Lock()
		closed := w.closed
		w.mu.Unlock()
		if closed {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("watcher remained open")
		}
		time.Sleep(time.Millisecond)
	}
}

func waitForQueuedChange(t *testing.T, value filemodel.WorkspaceChangeSubscription) {
	t.Helper()
	concrete, ok := value.(*subscription)
	if !ok {
		t.Fatalf("subscription type = %T, want workspacewatch subscription", value)
	}
	deadline := time.Now().Add(time.Second)
	for {
		if len(concrete.events) > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for queued workspace change")
		}
		time.Sleep(time.Millisecond)
	}
}

func waitForSubscriptionFinish(t *testing.T, value filemodel.WorkspaceChangeSubscription) {
	t.Helper()
	concrete, ok := value.(*subscription)
	if !ok {
		t.Fatalf("subscription type = %T, want workspacewatch subscription", value)
	}
	select {
	case <-concrete.finished:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscription finish")
	}
}

func requireChange(t *testing.T, events <-chan filemodel.WorkspaceChange) filemodel.WorkspaceChange {
	t.Helper()
	select {
	case change, ok := <-events:
		if !ok {
			t.Fatal("changes closed before expected event")
		}
		return change
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for workspace change")
		return filemodel.WorkspaceChange{}
	}
}

func assertChange(t *testing.T, actual filemodel.WorkspaceChange, kind filemodel.WorkspaceChangeKind, path filemodel.RelativePath, oldPath filemodel.RelativePath, sequence uint64) {
	t.Helper()
	if actual.Kind != kind || actual.Path != path || actual.OldPath != oldPath || actual.Sequence != sequence {
		t.Fatalf("workspace change = %#v, want kind=%q path=%q old_path=%q sequence=%d", actual, kind, path, oldPath, sequence)
	}
}

func assertClosed(t *testing.T, events <-chan filemodel.WorkspaceChange) {
	t.Helper()
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("changes remained open after rescan")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscription close")
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("create directory tree %q: %v", path, err)
	}
}
