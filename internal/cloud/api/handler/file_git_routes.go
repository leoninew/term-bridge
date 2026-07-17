package api

import (
	"net/http"

	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
)

// Routes aligned with vscode FileSystemProvider / SCM Provider.
// /api/workspaces/{id}/fs/* and /api/workspaces/{id}/scm/*

func (s *Handler) handleWorkspaceFsRoute(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, deviceId string, workspaceId string, parts []string) {
	if len(parts) != 1 {
		s.writeNotFound(w, r)
		return
	}
	switch parts[0] {
	case "stat":
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w, r, http.MethodGet)
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "fs_stat", &agent.FsStatReq{WorkspaceId: workspaceId, Path: r.URL.Query().Get("path")}, "")
	case "readDirectory":
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w, r, http.MethodGet)
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "fs_read_directory", &agent.FsReadDirectoryReq{WorkspaceId: workspaceId, Path: r.URL.Query().Get("path")}, "")
	case "readFile":
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w, r, http.MethodGet)
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "fs_read_file", &agent.FsReadFileReq{WorkspaceId: workspaceId, Path: r.URL.Query().Get("path")}, "")
	case "writeFile":
		if r.Method != http.MethodPut {
			s.methodNotAllowed(w, r, http.MethodPut)
			return
		}
		request := &agent.FsWriteFileReq{}
		if !s.decodeJSONRequest(w, r, request) {
			return
		}
		request.WorkspaceId = workspaceId
		s.handleJSONRuntime(w, r, endpoint, deviceId, "fs_write_file", request, "")
	case "createDirectory":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w, r, http.MethodPost)
			return
		}
		request := &agent.FsCreateDirectoryReq{}
		if !s.decodeJSONRequest(w, r, request) {
			return
		}
		request.WorkspaceId = workspaceId
		s.handleJSONRuntimeWithStatus(w, r, endpoint, deviceId, "fs_create_directory", request, "", http.StatusCreated)
	case "delete":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w, r, http.MethodPost)
			return
		}
		request := &agent.FsDeleteReq{}
		if !s.decodeJSONRequest(w, r, request) {
			return
		}
		request.WorkspaceId = workspaceId
		s.handleJSONRuntime(w, r, endpoint, deviceId, "fs_delete", request, "")
	case "rename":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w, r, http.MethodPost)
			return
		}
		request := &agent.FsRenameReq{}
		if !s.decodeJSONRequest(w, r, request) {
			return
		}
		request.WorkspaceId = workspaceId
		s.handleJSONRuntime(w, r, endpoint, deviceId, "fs_rename", request, "")
	default:
		s.writeNotFound(w, r)
	}
}

func (s *Handler) handleWorkspaceScmRoute(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, deviceId string, workspaceId string, parts []string) {
	if len(parts) != 1 {
		s.writeNotFound(w, r)
		return
	}
	switch parts[0] {
	case "status":
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w, r, http.MethodGet)
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "scm_status", &agent.ScmStatusReq{WorkspaceId: workspaceId}, "")
	case "originalContent":
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w, r, http.MethodGet)
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "scm_original_content", &agent.ScmOriginalContentReq{
			WorkspaceId: workspaceId,
			Path:        r.URL.Query().Get("path"),
			GroupId:     r.URL.Query().Get("groupId"),
		}, "")
	case "execute":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w, r, http.MethodPost)
			return
		}
		request := &agent.ScmExecuteReq{}
		if !s.decodeJSONRequest(w, r, request) {
			return
		}
		request.WorkspaceId = workspaceId
		s.handleJSONRuntime(w, r, endpoint, deviceId, "scm_execute", request, "")
	case "repository":
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w, r, http.MethodGet)
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "scm_repository", &agent.ScmRepositoryReq{WorkspaceId: workspaceId}, "")
	default:
		s.writeNotFound(w, r)
	}
}

func isFileGitDeviceRoute(parts []string) bool {
	return len(parts) >= 4 && parts[1] == "workspaces" && (parts[3] == "fs" || parts[3] == "scm")
}

func (s *Handler) authorizeFileGitDevice(w http.ResponseWriter, r *http.Request, deviceId string) bool {
	claims, ok := s.claimsFromRequest(r)
	if !ok || s.config.DeviceRepository == nil {
		s.writeNotFound(w, r)
		return false
	}
	owns, err := s.config.DeviceRepository.UserOwnsDevice(r.Context(), claims.Sub, deviceId)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
		return false
	}
	if !owns {
		s.writeNotFound(w, r)
		return false
	}
	return true
}
