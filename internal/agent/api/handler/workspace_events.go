package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/coder/websocket"

	agentapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/user"
	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
	workspacefs "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/workspacefs"
)

func (s *Handler) bridgeWorkspaceChanges(w http.ResponseWriter, r *http.Request, runtime agentapp.RuntimeAccess, workspaceId string) error {
	subscription, err := runtime.SubscribeWorkspaceChanges(r.Context(), workspaceId)
	if err != nil {
		return err
	}
	defer func() { _ = subscription.Close() }()

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{workspacefs.Subprotocol}, OriginPatterns: s.originPatterns(r)})
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	if err := writeWorkspaceChange(conn, r.Context(), &agent.FsChangeEvent{
		WorkspaceId: workspaceId,
		Kind:        agent.FsChangeKind_FS_CHANGE_KIND_RESCAN_REQUIRED,
	}); err != nil {
		return err
	}

	for {
		select {
		case <-r.Context().Done():
			return nil
		case change, ok := <-subscription.Events():
			if !ok {
				return nil
			}
			if err := writeWorkspaceChange(conn, r.Context(), workspaceChangeEvent(workspaceId, change)); err != nil {
				return err
			}
		}
	}
}

func writeWorkspaceChange(conn *websocket.Conn, ctx context.Context, event *agent.FsChangeEvent) error {
	data, err := codec.MarshalProtoJSON(event)
	if err != nil {
		return fmt.Errorf("encode workspace change: %w", err)
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

func workspaceChangeEvent(workspaceId string, change filemodel.WorkspaceChange) *agent.FsChangeEvent {
	return &agent.FsChangeEvent{
		WorkspaceId: workspaceId,
		Sequence:    change.Sequence,
		Kind:        workspaceChangeKind(change.Kind),
		Path:        change.Path.String(),
		OldPath:     change.OldPath.String(),
	}
}

func workspaceChangeKind(kind filemodel.WorkspaceChangeKind) agent.FsChangeKind {
	switch kind {
	case filemodel.WorkspaceChangeAdded:
		return agent.FsChangeKind_FS_CHANGE_KIND_ADDED
	case filemodel.WorkspaceChangeUpdated:
		return agent.FsChangeKind_FS_CHANGE_KIND_UPDATED
	case filemodel.WorkspaceChangeDeleted:
		return agent.FsChangeKind_FS_CHANGE_KIND_DELETED
	case filemodel.WorkspaceChangeRenamed:
		return agent.FsChangeKind_FS_CHANGE_KIND_RENAMED
	default:
		return agent.FsChangeKind_FS_CHANGE_KIND_RESCAN_REQUIRED
	}
}
