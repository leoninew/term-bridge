import { buildApiWebSocketUrl } from '../../config'
import { clampTerminalSize } from '../../protocol/terminal'
import type {
  CreateSessionReq,
  CreateSessionResp,
  RerunSessionReq,
  SessionSummary,
  UpdateSessionReq,
  Workspace,
  WorkspaceTreeNode,
} from '../../gen/proto/termbridge/runtime/v1/runtime'
import { runtimeApiTarget, runtimePath, type RuntimeTarget } from '../runtimeTarget'

export type ApiResult<T> = {
  data: T
  offline: boolean
}

export type SessionRuntimeApi = {
  createSession(workspaceId: string | null, request: CreateSessionReq): Promise<CreateSessionResp>
  getSession(workspaceId: string, sessionId: string): Promise<SessionSummary>
  updateSession(
    workspaceId: string,
    sessionId: string,
    request: UpdateSessionReq,
  ): Promise<SessionSummary>
  deleteSession(workspaceId: string, sessionId: string): Promise<void>
  readHistory(workspaceId: string, sessionId: string): Promise<ApiResult<string>>
  closeSession(workspaceId: string, sessionId: string): Promise<SessionSummary>
  rerunSession(
    workspaceId: string,
    sessionId: string,
    request: RerunSessionReq,
  ): Promise<CreateSessionResp>
  updateSessionOrder(workspaceId: string, sessionIds: string[]): Promise<SessionSummary[]>
  listWorkspaceTree(): Promise<ApiResult<WorkspaceTreeNode[]>>
  updateWorkspaceOrder(workspaceIds: string[]): Promise<Workspace[]>
  deleteWorkspace(workspaceId: string): Promise<void>
}

export function terminalWsUrl(
  target: RuntimeTarget,
  workspaceId: string,
  sessionId: string,
  token?: string,
  size?: { cols: number; rows: number } | null,
): string {
  const path = runtimePath(target, `${workspaceSessionPath(workspaceId, sessionId)}/ws`)
  const params = new URLSearchParams()
  if (token) {
    params.set('token', token)
  }
  if (size) {
    const clamped = clampTerminalSize(size)
    params.set('cols', String(clamped.cols))
    params.set('rows', String(clamped.rows))
  }
  const query = params.toString()
  const pathWithQuery = query ? `${path}?${query}` : path
  return buildApiWebSocketUrl(pathWithQuery, runtimeApiTarget(target))
}

export function workspaceSessionPath(workspaceId: string, sessionId?: string): string {
  const base = `/workspaces/${encodeURIComponent(workspaceId)}/sessions`
  return sessionId ? `${base}/${encodeURIComponent(sessionId)}` : base
}
