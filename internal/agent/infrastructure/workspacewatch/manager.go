package workspacewatch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"

	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
)

type Config struct {
	SubscriberQueueSize int
}

func (c Config) Validate() error {
	if c.SubscriberQueueSize < 1 {
		return fmt.Errorf("subscriber queue size must be positive")
	}
	return nil
}

type watcher interface {
	Add(path string) error
	Remove(path string) error
	Close() error
	Events() <-chan fsnotify.Event
	Errors() <-chan error
}

type watcherFactory func() (watcher, error)

type Manager struct {
	ctx      context.Context
	cancel   context.CancelFunc
	config   Config
	newWatch watcherFactory

	mu     sync.Mutex
	roots  map[string]*rootWatcher
	closed bool
}

func New(ctx context.Context, config Config) (*Manager, error) {
	return newManager(ctx, config, func() (watcher, error) {
		created, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, err
		}
		return fsnotifyAdapter{Watcher: created}, nil
	})
}

type fsnotifyAdapter struct {
	*fsnotify.Watcher
}

func (w fsnotifyAdapter) Events() <-chan fsnotify.Event { return w.Watcher.Events }

func (w fsnotifyAdapter) Errors() <-chan error { return w.Watcher.Errors }

func newManager(ctx context.Context, config Config, newWatch watcherFactory) (*Manager, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if newWatch == nil {
		return nil, errors.New("watcher factory is required")
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	managerCtx, cancel := context.WithCancel(ctx)
	return &Manager{
		ctx:      managerCtx,
		cancel:   cancel,
		config:   config,
		newWatch: newWatch,
		roots:    map[string]*rootWatcher{},
	}, nil
}

func (m *Manager) Subscribe(ctx context.Context, workspaceRoot string) (filemodel.WorkspaceChangeSubscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := normalizeRoot(workspaceRoot)
	if err != nil {
		return nil, err
	}

	for {
		m.mu.Lock()
		if m.closed {
			m.mu.Unlock()
			return nil, errors.New("workspace watcher manager is closed")
		}
		state := m.roots[root]
		if state == nil {
			state, err = m.startRootLocked(root)
			if err != nil {
				m.mu.Unlock()
				return nil, err
			}
		}
		m.mu.Unlock()

		if subscription, active := state.subscribe(ctx); active {
			return subscription, nil
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
}

func (m *Manager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	m.cancel()
	roots := make([]*rootWatcher, 0, len(m.roots))
	for _, state := range m.roots {
		roots = append(roots, state)
	}
	m.roots = map[string]*rootWatcher{}
	m.mu.Unlock()

	for _, state := range roots {
		state.close()
	}
	return nil
}

func (m *Manager) startRootLocked(root string) (*rootWatcher, error) {
	created, err := m.newWatch()
	if err != nil {
		return nil, fmt.Errorf("create workspace watcher: %w", err)
	}
	state := &rootWatcher{
		manager:     m,
		root:        root,
		watcher:     created,
		subscribers: map[uint64]*subscription{},
	}
	if err := state.addDirectoryTree(root); err != nil {
		_ = created.Close()
		return nil, err
	}
	m.roots[root] = state
	go state.run()
	return state, nil
}

func (m *Manager) removeRoot(state *rootWatcher) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeRootLocked(state)
}

func (m *Manager) removeRootLocked(state *rootWatcher) {
	if m.roots[state.root] == state {
		delete(m.roots, state.root)
	}
}

type rootWatcher struct {
	manager *Manager
	root    string
	watcher watcher

	mu          sync.Mutex
	nextId      uint64
	sequence    uint64
	subscribers map[uint64]*subscription
	closed      bool
	closeOnce   sync.Once
}

func (r *rootWatcher) subscribe(ctx context.Context) (*subscription, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, false
	}
	r.nextId++
	subscriber := &subscription{
		root:     r,
		id:       r.nextId,
		events:   make(chan filemodel.WorkspaceChange, r.manager.config.SubscriberQueueSize),
		finished: make(chan struct{}),
	}
	r.subscribers[subscriber.id] = subscriber
	go func() {
		select {
		case <-ctx.Done():
			_ = subscriber.Close()
		case <-subscriber.finished:
		}
	}()
	return subscriber, true
}

func (r *rootWatcher) run() {
	defer r.close()
	for {
		select {
		case <-r.manager.ctx.Done():
			return
		case event, ok := <-r.watcher.Events():
			if !ok {
				r.publishRescanAndClose()
				return
			}
			r.handleEvent(event)
		case _, ok := <-r.watcher.Errors():
			if !ok {
				r.publishRescanAndClose()
				return
			}
			r.publishRescanAndClose()
			return
		}
	}
}

func (r *rootWatcher) handleEvent(event fsnotify.Event) {
	if isTermBridgeTemporaryFile(event.Name) {
		return
	}
	path, err := workspacePath(r.root, event.Name)
	if err != nil {
		r.publishRescanAndClose()
		return
	}
	if event.Has(fsnotify.Rename) {
		r.publishRescanAndClose()
		return
	}
	if event.Has(fsnotify.Create) {
		info, statErr := os.Lstat(event.Name)
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			r.publishRescanAndClose()
			return
		}
		if statErr == nil && info.IsDir() {
			if err := r.addDirectoryTree(event.Name); err != nil {
				r.publishRescanAndClose()
				return
			}
		}
		r.publish(filemodel.WorkspaceChange{Kind: filemodel.WorkspaceChangeAdded, Path: path})
		return
	}
	if event.Has(fsnotify.Remove) {
		r.publish(filemodel.WorkspaceChange{Kind: filemodel.WorkspaceChangeDeleted, Path: path})
		return
	}
	if event.Has(fsnotify.Write) || event.Has(fsnotify.Chmod) {
		r.publish(filemodel.WorkspaceChange{Kind: filemodel.WorkspaceChangeUpdated, Path: path})
	}
}

func (r *rootWatcher) addDirectoryTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		if err := r.watcher.Add(path); err != nil {
			return fmt.Errorf("watch workspace directory %q: %w", path, err)
		}
		return nil
	})
}

func (r *rootWatcher) publish(change filemodel.WorkspaceChange) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.sequence++
	change.Sequence = r.sequence
	for _, subscriber := range r.subscribers {
		select {
		case subscriber.events <- change:
		default:
			r.publishRescanAndCloseLocked()
			return
		}
	}
}

func (r *rootWatcher) publishRescanAndClose() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.publishRescanAndCloseLocked()
}

func (r *rootWatcher) publishRescanAndCloseLocked() {
	if r.closed {
		return
	}
	r.sequence++
	change := filemodel.WorkspaceChange{Kind: filemodel.WorkspaceChangeRescanRequired, Sequence: r.sequence}
	for _, subscriber := range r.subscribers {
		for {
			select {
			case <-subscriber.events:
			default:
				goto drained
			}
		}
	drained:
		subscriber.events <- change
	}
	r.closed = true
	for id, subscriber := range r.subscribers {
		delete(r.subscribers, id)
		subscriber.finishLocked()
	}
	go func() { _ = r.watcher.Close() }()
	go r.manager.removeRoot(r)
}

func (r *rootWatcher) removeSubscriber(subscriber *subscription) {
	r.mu.Lock()
	if _, exists := r.subscribers[subscriber.id]; !exists {
		r.mu.Unlock()
		return
	}
	delete(r.subscribers, subscriber.id)
	subscriber.finishLocked()
	empty := len(r.subscribers) == 0
	if empty {
		r.closed = true
	}
	r.mu.Unlock()
	if empty {
		_ = r.watcher.Close()
		r.manager.removeRoot(r)
	}
}

func (r *rootWatcher) close() {
	r.closeOnce.Do(func() {
		r.mu.Lock()
		if !r.closed {
			r.closed = true
			for id, subscriber := range r.subscribers {
				delete(r.subscribers, id)
				subscriber.finishLocked()
			}
		}
		r.mu.Unlock()
		_ = r.watcher.Close()
		r.manager.removeRoot(r)
	})
}

type subscription struct {
	root *rootWatcher
	id   uint64

	events     chan filemodel.WorkspaceChange
	finished   chan struct{}
	closeOnce  sync.Once
	finishOnce sync.Once
}

func (s *subscription) Events() <-chan filemodel.WorkspaceChange { return s.events }

func (s *subscription) Close() error {
	s.closeOnce.Do(func() {
		s.root.removeSubscriber(s)
	})
	return nil
}

func (s *subscription) finishLocked() {
	s.finishOnce.Do(func() {
		close(s.events)
		close(s.finished)
	})
}

func normalizeRoot(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", errors.New("workspace root is required")
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", fmt.Errorf("inspect workspace root: %w", err)
	}
	if !info.IsDir() || isUnsafeFileMode(info.Mode()) {
		return "", errors.New("workspace root must be a directory")
	}
	return filepath.Clean(absolute), nil
}

func workspacePath(root string, value string) (filemodel.RelativePath, error) {
	relative, err := filepath.Rel(root, value)
	if err != nil || relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("watch event is outside workspace root")
	}
	return filemodel.ParseRelativePath(filepath.ToSlash(relative), false)
}

func isTermBridgeTemporaryFile(value string) bool {
	base := filepath.Base(value)
	return strings.Contains(base, ".termbridge-") && strings.HasSuffix(base, ".tmp")
}

func isUnsafeFileMode(mode fs.FileMode) bool {
	return mode&os.ModeSymlink != 0 || mode&os.ModeIrregular != 0 || mode&os.ModeDevice != 0 || mode&os.ModeNamedPipe != 0 || mode&os.ModeSocket != 0
}

var _ filemodel.WorkspaceChangeSource = (*Manager)(nil)
var _ filemodel.WorkspaceChangeSubscription = (*subscription)(nil)
