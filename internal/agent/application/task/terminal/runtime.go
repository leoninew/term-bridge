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

var errNotController = errors.New(terminalproto.ErrorCodeNotController)

type SessionRuntime struct {
	registry *Registry
	session  session.Session
	pty      termpty.Session
	history  *history.Writer

	mu sync.Mutex
	// controlMu serializes controller identity changes with controller-gated PTY ops.
	// Lock order: controlMu -> resizeMu -> mu. Never hold controlMu across client queue waits.
	controlMu          sync.Mutex
	resizeMu           sync.Mutex
	clients            map[string]*Client
	clientOrder        []string
	controllerClientId string
	attachment         AttachmentState
	closed             bool
	stopMode           process.StopMode
	current            process.TerminalSize
	done               chan struct{}
}

func newSessionRuntime(registry *Registry, sess session.Session, ptySession termpty.Session, historyWriter *history.Writer, initialSize process.TerminalSize) *SessionRuntime {
	return &SessionRuntime{
		registry:    registry,
		session:     sess,
		pty:         ptySession,
		history:     historyWriter,
		clients:     map[string]*Client{},
		clientOrder: nil,
		attachment:  AttachmentUnattached,
		current:     initialSize.OrDefault(),
		done:        make(chan struct{}),
	}
}

func (r *SessionRuntime) start() {
	go r.readLoop()
	go r.waitLoop()
}

func (r *SessionRuntime) attach() (*Client, error) {
	r.controlMu.Lock()
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		r.controlMu.Unlock()
		return nil, session.Closed()
	}
	id, err := idgen.New()
	if err != nil {
		r.mu.Unlock()
		r.controlMu.Unlock()
		return nil, err
	}
	client := &Client{id: id, runtime: r, queue: make(chan Outbound, r.registry.clientQueueSize)}
	attachment := AttachmentAttached
	// Reserve membership + controller under lock so concurrent attaches elect exactly one controller.
	r.clients[id] = client
	r.clientOrder = append(r.clientOrder, id)
	controlRole := terminalproto.ControlRoleObserver
	if r.controllerClientId == "" {
		r.controllerClientId = id
		controlRole = terminalproto.ControlRoleController
	}
	r.attachment = attachment
	r.registry.logger.Info("terminal attach start", "session_id", r.session.Id, "client_id", id, "control_role", controlRole, "current_cols", r.current.Cols, "current_rows", r.current.Rows)
	r.mu.Unlock()
	r.controlMu.Unlock()

	if err := r.enqueueReplay(client, attachment, controlRole); err != nil {
		r.detachClient(id, "attach_replay_failed")
		return nil, err
	}

	r.registry.logger.Info("terminal attach live", "session_id", r.session.Id, "client_id", id, "control_role", controlRole, "clients", len(r.clientsSnapshot()))
	return client, nil
}

func (r *SessionRuntime) enqueueReplay(client *Client, attachment AttachmentState, controlRole string) error {
	r.registry.logger.Debug("terminal replay enqueue start", "session_id", r.session.Id, "client_id", client.Id(), "control_role", controlRole)
	if !client.enqueue(Outbound{Kind: OutboundText, Text: &agent.ServerControlMessage{Type: terminalproto.TypeStarted, SessionId: r.session.Id, WorkspaceId: r.session.WorkspaceId, State: string(session.StateRunning), LifecycleState: string(session.StateRunning), AttachmentState: string(attachment), ControlRole: controlRole}}) {
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
		r.registry.logger.Debug("terminal replay enqueue history", "session_id", r.session.Id, "client_id", client.Id(), "bytes", len(data), "truncated", truncated)
		if !r.enqueueReplayChunks(client, data, replayFinished) {
			truncated = true
			r.registry.logger.Warn("terminal replay truncated by client queue", "session_id", r.session.Id, "client_id", client.Id(), "queued_bytes", client.QueuedBytes(), "queue_bytes", r.registry.clientQueueBytes)
		}
	}
	if !client.enqueue(replayFinished) {
		return fmt.Errorf("client queue full")
	}
	r.registry.logger.Debug("terminal replay enqueue finish", "session_id", r.session.Id, "client_id", client.Id(), "queued_bytes", client.QueuedBytes(), "truncated", truncated)
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
	r.controlMu.Lock()
	r.mu.Lock()
	client := r.clients[id]
	wasController := r.controllerClientId == id
	if client != nil {
		delete(r.clients, id)
	}
	r.clientOrder = removeClientOrder(r.clientOrder, id)
	var promoted *Client
	if wasController {
		r.controllerClientId = ""
		if len(r.clientOrder) > 0 {
			nextId := r.clientOrder[0]
			r.controllerClientId = nextId
			promoted = r.clients[nextId]
		}
	}
	if len(r.clients) == 0 && !r.closed {
		r.attachment = AttachmentDetached
		r.controllerClientId = ""
	}
	attachment := r.attachment
	controllerId := r.controllerClientId
	r.mu.Unlock()
	r.controlMu.Unlock()
	if client != nil {
		client.closeQueue()
	}
	if promoted != nil {
		r.registry.logger.Info("terminal control auto granted", "session_id", r.session.Id, "client_id", promoted.Id(), "reason", reason)
		r.enqueueControlRole(promoted, attachment, terminalproto.ControlRoleController, terminalproto.ReasonControlAutoGranted)
		return
	}
	if controllerId == "" {
		r.broadcastText(&agent.ServerControlMessage{Type: terminalproto.TypeState, LifecycleState: string(session.StateRunning), AttachmentState: string(attachment), Reason: reason})
	}
}

func removeClientOrder(order []string, id string) []string {
	if len(order) == 0 {
		return nil
	}
	out := make([]string, 0, len(order))
	for _, item := range order {
		if item != id {
			out = append(out, item)
		}
	}
	return out
}

func (r *SessionRuntime) isControllerLocked(id string) bool {
	return r.controllerClientId == id
}

func (r *SessionRuntime) takeControl(id string) {
	r.controlMu.Lock()
	r.mu.Lock()
	client := r.clients[id]
	if client == nil || r.closed {
		r.mu.Unlock()
		r.controlMu.Unlock()
		return
	}
	oldId := r.controllerClientId
	if oldId == id {
		attachment := r.attachment
		r.mu.Unlock()
		r.controlMu.Unlock()
		r.enqueueControlRole(client, attachment, terminalproto.ControlRoleController, terminalproto.ReasonControlGranted)
		return
	}
	r.controllerClientId = id
	var oldClient *Client
	if oldId != "" {
		oldClient = r.clients[oldId]
	}
	attachment := r.attachment
	r.mu.Unlock()
	r.controlMu.Unlock()

	r.registry.logger.Info("terminal control transferred", "session_id", r.session.Id, "from_client_id", oldId, "to_client_id", id)
	// Role events are best-effort UI sync; queue pressure must not re-elect controllers.
	r.enqueueControlRole(client, attachment, terminalproto.ControlRoleController, terminalproto.ReasonControlGranted)
	if oldClient != nil {
		r.enqueueControlRole(oldClient, attachment, terminalproto.ControlRoleObserver, terminalproto.ReasonControlLost)
	}
}

func (r *SessionRuntime) writeInputFrom(id string, data []byte) error {
	r.controlMu.Lock()
	defer r.controlMu.Unlock()
	r.mu.Lock()
	if !r.isControllerLocked(id) {
		r.mu.Unlock()
		return errNotController
	}
	r.mu.Unlock()
	return r.writeInput(data)
}

func (r *SessionRuntime) resizeFrom(id string, cols int, rows int) error {
	r.controlMu.Lock()
	defer r.controlMu.Unlock()
	r.mu.Lock()
	if !r.isControllerLocked(id) {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()
	return r.resize(cols, rows)
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
	mode := r.stopMode
	clients := r.snapshotClientsLocked()
	done := r.done
	r.mu.Unlock()
	r.registry.logger.Info(
		"terminal process close requested",
		"source", "web",
		"session_id", r.session.Id,
		"workspace_id", r.session.WorkspaceId,
		"reason", reason,
		"stop_mode", mode.String(),
		"clients", len(clients),
	)
	for _, client := range clients {
		client.enqueue(Outbound{Kind: OutboundText, Text: &agent.ServerControlMessage{Type: terminalproto.TypeState, LifecycleState: string(session.StateRunning), AttachmentState: string(AttachmentDetached), Reason: reason}})
	}
	if err := r.pty.Close(); err != nil {
		r.registry.logger.Warn("terminal process close failed", "source", "web", "session_id", r.session.Id, "workspace_id", r.session.WorkspaceId, "reason", reason, "error_kind", apperrors.KindOf(err))
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
	startedAt := r.session.CreatedAt
	if reporter, ok := r.pty.(termpty.ProcessReporter); ok {
		if record := reporter.ProcessInfo(); !record.StartedAt.IsZero() {
			startedAt = record.StartedAt
		}
	}
	r.registry.logger.Info(
		"terminal pty wait completed",
		"source", "web",
		"session_id", r.session.Id,
		"workspace_id", r.session.WorkspaceId,
		"raw_exit_code", result.ExitCode,
		"raw_wait_error", result.Err,
		"raw_wait_error_kind", apperrors.KindOf(result.Err),
		"stop_mode", mode.String(),
		"elapsed", endedAt.Sub(startedAt),
	)
	exit := process.InterpretExit(result.Err, result.ExitCode, mode)
	_ = r.history.Close()
	var sessionUpdate *session.Session
	if r.history.Truncated() {
		sess := r.session
		sess.History.Truncated = true
		sess.UpdatedAt = endedAt
		sessionUpdate = &sess
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
		r.registry.logger.Info(
			"terminal process exited",
			"source", "web",
			"session_id", r.session.Id,
			"workspace_id", r.session.WorkspaceId,
			"cwd", r.session.LaunchCwd,
			"raw_exit_code", result.ExitCode,
			"exit_code", exit.Code,
			"state", finalState,
			"stop_mode", mode.String(),
			"forced", exit.Forced,
			"closed", exit.Closed,
			"elapsed", endedAt.Sub(startedAt),
		)
	}
	exitCode := int32(exit.Code)
	r.broadcastText(&agent.ServerControlMessage{Type: terminalproto.TypeExited, ExitCode: &exitCode, State: string(finalState), LifecycleState: string(finalState), AttachmentState: string(AttachmentDetached)})
	r.closeClients()
	r.registry.removeRuntime(r.session.Id)
	close(r.done)
}

// enqueueControlRole best-effort delivers a control-role state event for browser UI sync.
// Queue full or closed client: log and drop. Must not detach or re-elect controllers.
func (r *SessionRuntime) enqueueControlRole(client *Client, attachment AttachmentState, role string, reason string) {
	if client == nil {
		return
	}
	ok := client.enqueue(Outbound{Kind: OutboundText, Text: &agent.ServerControlMessage{
		Type:            terminalproto.TypeState,
		LifecycleState:  string(session.StateRunning),
		AttachmentState: string(attachment),
		ControlRole:     role,
		Reason:          reason,
	}})
	if ok {
		return
	}
	r.registry.logger.Warn(
		"terminal control role notify dropped",
		"session_id", r.session.Id,
		"client_id", client.Id(),
		"control_role", role,
		"notify_reason", reason,
		"queued_bytes", client.QueuedBytes(),
		"queue_bytes", r.registry.clientQueueBytes,
		"queue_messages", r.registry.clientQueueSize,
	)
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

func IsNotController(err error) bool {
	return err != nil && errors.Is(err, errNotController)
}
