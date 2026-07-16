package api

import (
	"net/http"

	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
)

func (s *Handler) handleWorkspaceFileRoute(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, deviceId string, workspaceId string, parts []string) {
	if len(parts) == 1 && parts[0] == "tree" && r.Method == http.MethodGet {
		s.handleJSONRuntime(w, r, endpoint, deviceId, "list_files", &agent.ListFilesReq{WorkspaceId: workspaceId, Path: r.URL.Query().Get("path")}, "")
		return
	}
	if len(parts) == 1 && parts[0] == "content" {
		switch r.Method {
		case http.MethodGet:
			s.handleJSONRuntime(w, r, endpoint, deviceId, "read_file", &agent.ReadFileReq{WorkspaceId: workspaceId, Path: r.URL.Query().Get("path")}, "")
		case http.MethodPut:
			request := &agent.WriteFileReq{}
			if !s.decodeJSONRequest(w, r, request) {
				return
			}
			request.WorkspaceId = workspaceId
			s.handleJSONRuntime(w, r, endpoint, deviceId, "write_file", request, "")
		default:
			s.methodNotAllowed(w, r, http.MethodGet, http.MethodPut)
		}
		return
	}
	if len(parts) == 0 && r.Method == http.MethodPost {
		request := &agent.CreateFileReq{}
		if !s.decodeJSONRequest(w, r, request) {
			return
		}
		request.WorkspaceId = workspaceId
		s.handleJSONRuntimeWithStatus(w, r, endpoint, deviceId, "create_file", request, "", http.StatusCreated)
		return
	}
	s.writeNotFound(w, r)
}

func (s *Handler) handleWorkspaceDirectoryRoute(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, deviceId string, workspaceId string, parts []string) {
	if len(parts) != 0 || r.Method != http.MethodPost {
		s.writeNotFound(w, r)
		return
	}
	request := &agent.CreateDirectoryReq{}
	if !s.decodeJSONRequest(w, r, request) {
		return
	}
	request.WorkspaceId = workspaceId
	s.handleJSONRuntimeWithStatus(w, r, endpoint, deviceId, "create_directory", request, "", http.StatusCreated)
}

func (s *Handler) handleWorkspaceEntryRenameRoute(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, deviceId string, workspaceId string, parts []string) {
	if len(parts) != 0 || r.Method != http.MethodPost {
		s.writeNotFound(w, r)
		return
	}
	request := &agent.RenameEntryReq{}
	if !s.decodeJSONRequest(w, r, request) {
		return
	}
	request.WorkspaceId = workspaceId
	s.handleJSONRuntime(w, r, endpoint, deviceId, "rename_entry", request, "")
}

func (s *Handler) handleWorkspaceEntryMoveRoute(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, deviceId string, workspaceId string, parts []string) {
	if len(parts) != 0 || r.Method != http.MethodPost {
		s.writeNotFound(w, r)
		return
	}
	request := &agent.MoveEntryReq{}
	if !s.decodeJSONRequest(w, r, request) {
		return
	}
	request.WorkspaceId = workspaceId
	s.handleJSONRuntime(w, r, endpoint, deviceId, "move_entry", request, "")
}

func (s *Handler) handleWorkspaceEntryRoute(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, deviceId string, workspaceId string, parts []string) {
	if len(parts) != 0 || r.Method != http.MethodDelete {
		s.writeNotFound(w, r)
		return
	}
	request := &agent.DeleteEntryReq{WorkspaceId: workspaceId, Path: r.URL.Query().Get("path"), ExpectedRevision: r.URL.Query().Get("expected_revision"), ExpectedParentRevision: r.URL.Query().Get("expected_parent_revision"), Recursive: r.URL.Query().Get("recursive") == "true"}
	s.handleJSONRuntime(w, r, endpoint, deviceId, "delete_entry", request, "")
}

func (s *Handler) handleWorkspaceGitRoute(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, deviceId string, workspaceId string, parts []string) {
	if len(parts) != 1 || r.Method != http.MethodGet {
		s.writeNotFound(w, r)
		return
	}
	switch parts[0] {
	case "status":
		s.handleJSONRuntime(w, r, endpoint, deviceId, "git_status", &agent.GitStatusReq{WorkspaceId: workspaceId}, "")
	case "diff":
		layer, ok := gitLayerQuery(r.URL.Query().Get("layer"))
		if !ok {
			s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, errorMessageBadRequest, nil)
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "git_diff", &agent.GitDiffReq{WorkspaceId: workspaceId, Path: r.URL.Query().Get("path"), Layer: layer}, "")
	default:
		s.writeNotFound(w, r)
	}
}

func gitLayerQuery(value string) (agent.GitLayer, bool) {
	switch value {
	case "staged":
		return agent.GitLayer_GIT_LAYER_STAGED, true
	case "unstaged":
		return agent.GitLayer_GIT_LAYER_UNSTAGED, true
	case "untracked":
		return agent.GitLayer_GIT_LAYER_UNTRACKED, true
	default:
		return agent.GitLayer_GIT_LAYER_UNSPECIFIED, false
	}
}
