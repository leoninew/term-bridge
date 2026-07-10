package terminal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	termpty "gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/pty"
	"gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/storage/history"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/process"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/session"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/idgen"
	terminalproto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/terminal"
)

type SessionRuntime struct {
	registry *Registry
	session  session.Session
	pty      termpty.Session
	history  *history.Writer

	mu         sync.Mutex
	resizeMu   sync.Mutex
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
	id, err := idgen.New()
	if err != nil {
		r.mu.Unlock()
		return nil, err
	}
	client := &Client{id: id, runtime: r, queue: make(chan Outbound, r.registry.clientQueueSize)}
	attachment := AttachmentAttached
	r.registry.logger.Info("terminal attach start", "session_id", r.session.Id, "client_id", id, "current_cols", r.current.Cols, "current_rows", r.current.Rows)
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
	r.registry.logger.Info("terminal attach live", "session_id", r.session.Id, "client_id", id, "clients", len(r.clients))
	r.mu.Unlock()
	return client, nil
}

func (r *SessionRuntime) enqueueReplay(client *Client, attachment AttachmentState) error {
	r.registry.logger.Info("terminal replay enqueue start", "session_id", r.session.Id, "client_id", client.Id())
	if !client.enqueue(Outbound{Kind: OutboundText, Text: &agent.ServerControlMessage{Type: terminalproto.TypeStarted, SessionId: r.session.Id, WorkspaceId: r.session.WorkspaceId, State: string(session.StateRunning), LifecycleState: string(session.StateRunning), AttachmentState: string(attachment)}}) {
		return fmt.Errorf("client queue full")
	}
	if !client.enqueue(Outbound{Kind: OutboundText, Text: &agent.ServerControlMessage{Type: terminalproto.TypeReplayStarted}}) {
		return fmt.Errorf("client queue full")
	}
	if err := r.history.Flush(); err != nil {
		return err
	}
	historyPath := r.registry.store.HistoryPath(r.session.WorkspaceId, r.session.Id)
	data, truncated, err := r.readReplayTail(historyPath)
	if err != nil {
		return err
	}
	replayFinished := Outbound{Kind: OutboundText, Text: &agent.ServerControlMessage{Type: terminalproto.TypeReplayFinished, Truncated: &truncated}}
	if len(data) > 0 {
		r.registry.logger.Info("terminal replay enqueue history", "session_id", r.session.Id, "client_id", client.Id(), "bytes", len(data), "truncated", truncated)
		if !r.enqueueReplayChunks(client, data, replayFinished) {
			truncated = true
			r.registry.logger.Warn("terminal replay truncated by client queue", "session_id", r.session.Id, "client_id", client.Id(), "queued_bytes", client.QueuedBytes(), "queue_bytes", r.registry.clientQueueBytes)
		}
	}
	if !client.enqueue(replayFinished) {
		return fmt.Errorf("client queue full")
	}
	r.registry.logger.Info("terminal replay enqueue finish", "session_id", r.session.Id, "client_id", client.Id(), "queued_bytes", client.QueuedBytes(), "truncated", truncated)
	return nil
}

func (r *SessionRuntime) readReplayTail(historyPath string) ([]byte, bool, error) {
	file, err := os.Open(historyPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	maxBytes := r.registry.replayMaxBytes
	if maxBytes <= 0 {
		return nil, false, nil
	}
	size := info.Size()
	start := int64(0)
	truncated := false
	if size > maxBytes {
		start = size - maxBytes
		truncated = true
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, false, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, false, err
	}
	return data, truncated, nil
}

func (r *SessionRuntime) enqueueReplayChunks(client *Client, data []byte, reserved Outbound) bool {
	chunkBytes := r.registry.replayChunkBytes
	if chunkBytes <= 0 {
		chunkBytes = DefaultReplayChunkBytes
	}
	for start := 0; start < len(data); start += chunkBytes {
		end := min(start+chunkBytes, len(data))
		if !client.canEnqueue(outboundSize(reserved) + end - start) {
			return false
		}
		if !client.enqueue(Outbound{Kind: OutboundBinary, Binary: copyBytes(data[start:end])}) {
			return false
		}
	}
	return true
}

func (r *SessionRuntime) attachmentState() AttachmentState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.attachment
}

func (r *SessionRuntime) lifecycleState() session.State {
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
	r.resizeMu.Lock()
	defer r.resizeMu.Unlock()
	r.mu.Lock()
	previous := r.current
	if r.current == size {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()
	r.registry.logger.Debug("terminal pty resize", "session_id", r.session.Id, "previous_cols", previous.Cols, "previous_rows", previous.Rows, "cols", size.Cols, "rows", size.Rows)
	if err := r.pty.Resize(size); err != nil {
		r.registry.logger.Warn("terminal pty resize failed", "session_id", r.session.Id, "previous_cols", previous.Cols, "previous_rows", previous.Rows, "cols", size.Cols, "rows", size.Rows, "error_kind", apperrors.KindOf(err))
		return err
	}
	r.mu.Lock()
	r.current = size
	r.mu.Unlock()
	return nil
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
	r.broadcastText(&agent.ServerControlMessage{Type: terminalproto.TypeState, LifecycleState: string(session.StateRunning), AttachmentState: string(attachment), Reason: reason})
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
	for _, client := range clients {
		client.enqueue(Outbound{Kind: OutboundText, Text: &agent.ServerControlMessage{Type: terminalproto.TypeState, LifecycleState: string(session.StateRunning), AttachmentState: string(AttachmentDetached), Reason: reason}})
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
			if _, writeErr := r.history.Write(chunk); writeErr != nil {
				r.registry.logger.Warn("write web terminal history", "session_id", r.session.Id, "error_kind", apperrors.KindOf(writeErr))
			}
			r.publishBinary(chunk)
		}
		if err != nil {
			if !isClosedReadError(err) {
				r.registry.logger.Warn("read web terminal pty", "session_id", r.session.Id, "error_kind", apperrors.KindOf(err))
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
	var sessionUpdate *session.Session
	if r.history.Truncated() {
		sess := r.session
		sess.History.Truncated = true
		sess.UpdatedAt = endedAt
		sessionUpdate = &sess
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
	exitRecord := process.ExitRecord{SchemaVersion: 1, ExitCode: exit.Code, Reason: "user_process_exited", Forced: exit.Forced, Closed: exit.Closed, StartedAt: startedAt, EndedAt: endedAt, WaitError: waitErr}
	finalState := session.StateStopped
	if result.Err != nil && !exit.Stopped && !exit.Closed && exit.Code == 0 {
		finalState = session.StateFailed
	}
	stateRecord := session.StateRecord{SchemaVersion: session.SchemaVersion, State: finalState, Reason: "user_process_exited", UpdatedAt: endedAt}
	var saveErr error
	if sessionUpdate != nil {
		saveErr = r.registry.store.SaveSessionExitState(*sessionUpdate, exitRecord, stateRecord)
	} else {
		saveErr = r.registry.store.SaveExitState(r.session.WorkspaceId, r.session.Id, exitRecord, stateRecord)
	}
	if saveErr != nil {
		r.registry.logger.Warn("save web terminal exit state", "source", "web", "session_id", r.session.Id, "workspace_id", r.session.WorkspaceId, "cwd", r.session.LaunchCwd, "stage", "save_exit_state", "state", finalState, "error_kind", apperrors.KindOf(saveErr))
	} else {
		r.registry.logger.Info("terminal process exited", "source", "web", "session_id", r.session.Id, "workspace_id", r.session.WorkspaceId, "cwd", r.session.LaunchCwd, "exit_code", exit.Code, "state", finalState, "forced", exit.Forced, "closed", exit.Closed)
	}
	exitCode := int32(exit.Code)
	r.broadcastText(&agent.ServerControlMessage{Type: terminalproto.TypeExited, ExitCode: &exitCode, State: string(finalState), LifecycleState: string(finalState), AttachmentState: string(AttachmentDetached)})
	r.closeClients()
	r.registry.removeRuntime(r.session.Id)
	close(r.done)
}

func (r *SessionRuntime) publishBinary(chunk []byte) {
	clients := r.clientsSnapshot()
	for _, client := range clients {
		if ok := client.enqueue(Outbound{Kind: OutboundBinary, Binary: chunk}); !ok {
			r.registry.logger.Warn("terminal client queue full", "session_id", r.session.Id, "client_id", client.Id(), "queued_bytes", client.QueuedBytes(), "queue_bytes", r.registry.clientQueueBytes, "queue_messages", r.registry.clientQueueSize, "reason", "client_queue_full")
			client.Detach("client_queue_full")
		}
	}
}

func (r *SessionRuntime) broadcastText(message *agent.ServerControlMessage) {
	clients := r.clientsSnapshot()
	for _, client := range clients {
		if ok := client.enqueue(Outbound{Kind: OutboundText, Text: message}); !ok {
			r.registry.logger.Warn("terminal client queue full", "session_id", r.session.Id, "client_id", client.Id(), "queued_bytes", client.QueuedBytes(), "queue_bytes", r.registry.clientQueueBytes, "queue_messages", r.registry.clientQueueSize, "reason", "client_queue_full")
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
