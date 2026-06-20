package webterminal

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"termbridge-go/internal/history"
	"termbridge-go/internal/process"
	termpty "termbridge-go/internal/pty"
	"termbridge-go/internal/session"
	"termbridge-go/internal/terminalproto"
)

type SessionRuntime struct {
	registry *Registry
	session  session.Session
	pty      termpty.Session
	history  *history.Writer

	mu         sync.Mutex
	clients    map[string]*Client
	attachment AttachmentState
	closed     bool
	stopMode   process.StopMode
	current    process.TerminalSize
}

func newSessionRuntime(registry *Registry, sess session.Session, ptySession termpty.Session, historyWriter *history.Writer) *SessionRuntime {
	return &SessionRuntime{
		registry:   registry,
		session:    sess,
		pty:        ptySession,
		history:    historyWriter,
		clients:    map[string]*Client{},
		attachment: AttachmentUnattached,
		current:    process.DefaultTerminalSize(),
	}
}

func (r *SessionRuntime) start() {
	go r.readLoop()
	go r.waitLoop()
}

func (r *SessionRuntime) attach() (*Client, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, fmt.Errorf("session is closed")
	}
	id, err := r.registry.ids.NewID()
	if err != nil {
		r.mu.Unlock()
		return nil, err
	}
	client := &Client{id: id, runtime: r, queue: make(chan Outbound, r.registry.clientQueueSize)}
	attachment := AttachmentAttached
	r.mu.Unlock()

	if err := r.enqueueReplay(client, attachment); err != nil {
		client.closeQueue()
		return nil, err
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		client.closeQueue()
		return nil, fmt.Errorf("session is closed")
	}
	r.clients[id] = client
	r.attachment = attachment
	r.mu.Unlock()
	return client, nil
}

func (r *SessionRuntime) enqueueReplay(client *Client, attachment AttachmentState) error {
	if !client.enqueue(Outbound{Kind: OutboundText, Text: terminalproto.ServerMessage{Type: terminalproto.TypeStarted, SessionId: r.session.ID, WorkspaceId: r.session.WorkspaceId, WorkspaceKey: r.session.WorkspaceKey, State: string(session.StateRunning), LifecycleState: string(session.StateRunning), AttachmentState: string(attachment)}}) {
		return fmt.Errorf("client queue full")
	}
	if !client.enqueue(Outbound{Kind: OutboundText, Text: terminalproto.ServerMessage{Type: terminalproto.TypeReplayStarted}}) {
		return fmt.Errorf("client queue full")
	}
	if err := r.history.Flush(); err != nil {
		return err
	}
	historyPath := r.registry.store.HistoryPath(r.session.WorkspaceKey, r.session.ID)
	data, err := os.ReadFile(historyPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(data) > 0 {
		if !client.enqueue(Outbound{Kind: OutboundBinary, Binary: data}) {
			return fmt.Errorf("client queue full")
		}
	}
	truncated := false
	if !client.enqueue(Outbound{Kind: OutboundText, Text: terminalproto.ServerMessage{Type: terminalproto.TypeReplayFinished, Truncated: &truncated}}) {
		return fmt.Errorf("client queue full")
	}
	return nil
}

func (r *SessionRuntime) attachmentState() AttachmentState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.attachment
}

func (r *SessionRuntime) writeInput(data []byte) error {
	if len(data) > terminalproto.MaxBinaryFrameBytes {
		return fmt.Errorf("binary frame too large: %d bytes", len(data))
	}
	_, err := r.pty.Write(data)
	return err
}

func (r *SessionRuntime) resize(cols int, rows int) error {
	if err := terminalproto.ValidateSize(cols, rows); err != nil {
		return err
	}
	size := process.TerminalSize{Cols: cols, Rows: rows}
	r.mu.Lock()
	if r.current == size {
		r.mu.Unlock()
		return nil
	}
	r.current = size
	r.mu.Unlock()
	return r.pty.Resize(size)
}

func (r *SessionRuntime) detachClient(id string, reason string) {
	r.mu.Lock()
	client := r.clients[id]
	if client != nil {
		delete(r.clients, id)
	}
	if len(r.clients) == 0 && !r.closed {
		r.attachment = AttachmentDetached
	}
	attachment := r.attachment
	r.mu.Unlock()
	if client != nil {
		client.closeQueue()
	}
	r.broadcastText(terminalproto.ServerMessage{Type: terminalproto.TypeState, LifecycleState: string(session.StateRunning), AttachmentState: string(attachment), Reason: reason})
}

func (r *SessionRuntime) closeSession(reason string) error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	r.stopMode = process.StopClose
	clients := r.snapshotClientsLocked()
	r.mu.Unlock()
	_ = r.registry.store.SaveState(r.session.WorkspaceKey, r.session.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopping, Reason: reason, UpdatedAt: time.Now().UTC()})
	for _, client := range clients {
		client.enqueue(Outbound{Kind: OutboundText, Text: terminalproto.ServerMessage{Type: terminalproto.TypeState, LifecycleState: string(session.StateStopping), AttachmentState: string(AttachmentDetached), Reason: reason}})
	}
	return r.pty.Close()
}

func (r *SessionRuntime) readLoop() {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.pty.Read(buf)
		if n > 0 {
			chunk := copyBytes(buf[:n])
			if _, writeErr := r.history.Write(chunk); writeErr != nil && r.registry.logger != nil {
				r.registry.logger.Warn("write web terminal history", "session_id", r.session.ID, "error", writeErr)
			}
			r.publishBinary(chunk)
		}
		if err != nil {
			if !isClosedReadError(err) && r.registry.logger != nil {
				r.registry.logger.Warn("read web terminal pty", "session_id", r.session.ID, "error", err)
			}
			return
		}
	}
}

func (r *SessionRuntime) waitLoop() {
	result := r.pty.Wait()
	endedAt := time.Now().UTC()
	r.mu.Lock()
	mode := r.stopMode
	r.closed = true
	r.attachment = AttachmentDetached
	r.mu.Unlock()
	exit := process.InterpretExit(result.Err, result.ExitCode, mode)
	_ = r.history.Close()
	if r.history.Truncated() {
		sess := r.session
		sess.History.Truncated = true
		sess.UpdatedAt = endedAt
		_ = r.registry.store.SaveSession(sess)
	}
	startedAt := r.session.CreatedAt
	if reporter, ok := r.pty.(termpty.ProcessReporter); ok {
		if record := reporter.ProcessInfo(); !record.StartedAt.IsZero() {
			startedAt = record.StartedAt
		}
	}
	waitErr := ""
	if exit.WaitErr != nil {
		waitErr = exit.WaitErr.Error()
	}
	_ = r.registry.store.SaveExit(r.session.WorkspaceKey, r.session.ID, process.ExitRecord{SchemaVersion: 1, ExitCode: exit.Code, Reason: "user_process_exited", Forced: exit.Forced, Closed: exit.Closed, StartedAt: startedAt, EndedAt: endedAt, WaitError: waitErr})
	_ = r.registry.store.SaveState(r.session.WorkspaceKey, r.session.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, Reason: "user_process_exited", UpdatedAt: endedAt})
	r.broadcastText(terminalproto.ServerMessage{Type: terminalproto.TypeExited, ExitCode: &exit.Code, State: string(session.StateStopped), LifecycleState: string(session.StateStopped), AttachmentState: string(AttachmentDetached)})
	r.closeClients()
	r.registry.removeRuntime(r.session.ID)
}

func (r *SessionRuntime) publishBinary(chunk []byte) {
	clients := r.clientsSnapshot()
	for _, client := range clients {
		if ok := client.enqueue(Outbound{Kind: OutboundBinary, Binary: chunk}); !ok {
			client.Detach("client_queue_full")
		}
	}
}

func (r *SessionRuntime) broadcastText(message terminalproto.ServerMessage) {
	clients := r.clientsSnapshot()
	for _, client := range clients {
		if ok := client.enqueue(Outbound{Kind: OutboundText, Text: message}); !ok {
			client.Detach("client_queue_full")
		}
	}
}

func (r *SessionRuntime) clientsSnapshot() []*Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotClientsLocked()
}

func (r *SessionRuntime) snapshotClientsLocked() []*Client {
	clients := make([]*Client, 0, len(r.clients))
	for _, client := range r.clients {
		clients = append(clients, client)
	}
	return clients
}

func (r *SessionRuntime) closeClients() {
	r.mu.Lock()
	clients := r.snapshotClientsLocked()
	r.clients = map[string]*Client{}
	r.mu.Unlock()
	for _, client := range clients {
		client.once.Do(func() { client.closeQueue() })
	}
}
