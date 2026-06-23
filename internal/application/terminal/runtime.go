package terminal

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"termbridge-go/internal/domain/process"
	"termbridge-go/internal/domain/session"
	"termbridge-go/internal/infrastructure/history"
	termpty "termbridge-go/internal/infrastructure/pty"
	"termbridge-go/internal/protocol/terminal"
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
	done       chan struct{}
}

func newSessionRuntime(registry *Registry, sess session.Session, ptySession termpty.Session, historyWriter *history.Writer, initialSize process.TerminalSize) *SessionRuntime {
	return &SessionRuntime{
		registry:   registry,
		session:    sess,
		pty:        ptySession,
		history:    historyWriter,
		clients:    map[string]*Client{},
		attachment: AttachmentUnattached,
		current:    initialSize.OrDefault(),
		done:       make(chan struct{}),
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
	id, err := r.registry.ids.NewId()
	if err != nil {
		r.mu.Unlock()
		return nil, err
	}
	client := &Client{id: id, runtime: r, queue: make(chan Outbound, r.registry.clientQueueSize)}
	attachment := AttachmentAttached
	if r.registry.logger != nil {
		r.registry.logger.Info("terminal attach start", "session_id", r.session.Id, "client_id", id, "current_cols", r.current.Cols, "current_rows", r.current.Rows)
	}
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
	if r.registry.logger != nil {
		r.registry.logger.Info("terminal attach live", "session_id", r.session.Id, "client_id", id, "clients", len(r.clients))
	}
	r.mu.Unlock()
	return client, nil
}

func (r *SessionRuntime) enqueueReplay(client *Client, attachment AttachmentState) error {
	if r.registry.logger != nil {
		r.registry.logger.Info("terminal replay enqueue start", "session_id", r.session.Id, "client_id", client.Id())
	}
	if !client.enqueue(Outbound{Kind: OutboundText, Text: terminalproto.ServerMessage{Type: terminalproto.TypeStarted, SessionId: r.session.Id, WorkspaceId: r.session.WorkspaceId, WorkspaceKey: r.session.WorkspaceKey, State: string(session.StateRunning), LifecycleState: string(session.StateRunning), AttachmentState: string(attachment)}}) {
		return fmt.Errorf("client queue full")
	}
	if !client.enqueue(Outbound{Kind: OutboundText, Text: terminalproto.ServerMessage{Type: terminalproto.TypeReplayStarted}}) {
		return fmt.Errorf("client queue full")
	}
	if err := r.history.Flush(); err != nil {
		return err
	}
	historyPath := r.registry.store.HistoryPath(r.session.WorkspaceKey, r.session.Id)
	data, err := os.ReadFile(historyPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(data) > 0 {
		if r.registry.logger != nil {
			r.registry.logger.Info("terminal replay enqueue history", "session_id", r.session.Id, "client_id", client.Id(), "bytes", len(data))
		}
		if !client.enqueue(Outbound{Kind: OutboundBinary, Binary: data}) {
			return fmt.Errorf("client queue full")
		}
	}
	truncated := false
	if !client.enqueue(Outbound{Kind: OutboundText, Text: terminalproto.ServerMessage{Type: terminalproto.TypeReplayFinished, Truncated: &truncated}}) {
		return fmt.Errorf("client queue full")
	}
	if r.registry.logger != nil {
		r.registry.logger.Info("terminal replay enqueue finish", "session_id", r.session.Id, "client_id", client.Id(), "queued_bytes", client.QueuedBytes())
	}
	return nil
}

func (r *SessionRuntime) attachmentState() AttachmentState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.attachment
}

func (r *SessionRuntime) lifecycleState() session.State {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return session.StateStopping
	}
	return session.StateRunning
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
	previous := r.current
	if r.current == size {
		r.mu.Unlock()
		return nil
	}
	r.current = size
	r.mu.Unlock()
	if r.registry.logger != nil {
		r.registry.logger.Info("terminal pty resize", "session_id", r.session.Id, "previous_cols", previous.Cols, "previous_rows", previous.Rows, "cols", size.Cols, "rows", size.Rows)
	}
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
		done := r.done
		r.mu.Unlock()
		<-done
		return nil
	}
	r.closed = true
	r.stopMode = process.StopClose
	clients := r.snapshotClientsLocked()
	done := r.done
	r.mu.Unlock()
	_ = r.registry.store.SaveState(r.session.WorkspaceKey, r.session.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopping, Reason: reason, UpdatedAt: time.Now().UTC()})
	for _, client := range clients {
		client.enqueue(Outbound{Kind: OutboundText, Text: terminalproto.ServerMessage{Type: terminalproto.TypeState, LifecycleState: string(session.StateStopping), AttachmentState: string(AttachmentDetached), Reason: reason}})
	}
	if err := r.pty.Close(); err != nil {
		return err
	}
	<-done
	return nil
}

func (r *SessionRuntime) readLoop() {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.pty.Read(buf)
		if n > 0 {
			chunk := copyBytes(buf[:n])
			if _, writeErr := r.history.Write(chunk); writeErr != nil && r.registry.logger != nil {
				r.registry.logger.Warn("write web terminal history", "session_id", r.session.Id, "error", writeErr)
			}
			r.publishBinary(chunk)
		}
		if err != nil {
			if !isClosedReadError(err) && r.registry.logger != nil {
				r.registry.logger.Warn("read web terminal pty", "session_id", r.session.Id, "error", err)
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
	if err := r.registry.store.SaveExit(r.session.WorkspaceKey, r.session.Id, process.ExitRecord{SchemaVersion: 1, ExitCode: exit.Code, Reason: "user_process_exited", Forced: exit.Forced, Closed: exit.Closed, StartedAt: startedAt, EndedAt: endedAt, WaitError: waitErr}); err != nil && r.registry.logger != nil {
		r.registry.logger.Warn("save web terminal exit", "session_id", r.session.Id, "error", err)
	}
	if err := r.registry.store.SaveState(r.session.WorkspaceKey, r.session.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, Reason: "user_process_exited", UpdatedAt: endedAt}); err != nil && r.registry.logger != nil {
		r.registry.logger.Warn("save web terminal stopped state", "session_id", r.session.Id, "error", err)
	}
	r.broadcastText(terminalproto.ServerMessage{Type: terminalproto.TypeExited, ExitCode: &exit.Code, State: string(session.StateStopped), LifecycleState: string(session.StateStopped), AttachmentState: string(AttachmentDetached)})
	r.closeClients()
	r.registry.removeRuntime(r.session.Id)
	close(r.done)
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
