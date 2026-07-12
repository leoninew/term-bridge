import { useRuntimeConfigStore } from '../../store/runtimeConfig'
import { clampTerminalSize } from '../../protocol/terminal'
import type {
  SessionSummary,
  Workspace,
  WorkspaceTreeNode,
} from '../../gen/proto/termbridge/agent/v1/workspace'
import type {
  CreateSessionReq,
  CreateSessionResp,
  RerunSessionReq,
  UpdateSessionReq,
} from '../../gen/proto/termbridge/agent/v1/session'
import type {
  CreateShortcutReq,
  Shortcut,
  UpdateShortcutOrderReq,
  UpdateShortcutReq,
} from '../../gen/proto/termbridge/agent/v1/shortcut'
import { runtimePath, type RuntimeTarget } from '../runtimeTarget'

export type ApiResult<T> = {
  data: T
  offline: boolean
}

export type ShortcutRuntimeApi = {
  listShortcuts(): Promise<Shortcut[]>
  createShortcut(request: CreateShortcutReq): Promise<Shortcut>
  updateShortcut(shortcutId: string, request: UpdateShortcutReq): Promise<Shortcut>
  updateShortcutOrder(request: UpdateShortcutOrderReq): Promise<Shortcut[]>
  deleteShortcut(shortcutId: string): Promise<void>
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
  const runtimeConfig = useRuntimeConfigStore().config
  const apiBaseUrl =
    target.mode === 'cloud' ? runtimeConfig.cloud.apiBaseUrl : runtimeConfig.local.apiBaseUrl
  return buildApiWebSocketUrl(apiBaseUrl, pathWithQuery)
}

export function workspaceSessionPath(workspaceId: string, sessionId?: string): string {
  const base = `/workspaces/${encodeURIComponent(workspaceId)}/sessions`
  return sessionId ? `${base}/${encodeURIComponent(sessionId)}` : base
}

function buildApiWebSocketUrl(apiBaseUrl: string, path: string): string {
  const url = joinBaseAndPath(apiBaseUrl, path)
  if (apiBaseUrl.startsWith('/')) {
    return url
  }
  const parsed = new URL(url)
  parsed.protocol = parsed.protocol === 'https:' ? 'wss:' : 'ws:'
  return parsed.toString()
}

function joinBaseAndPath(baseUrl: string, path: string): string {
  const normalizedBase = baseUrl.replace(/\/+$/, '')
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  return `${normalizedBase}${normalizedPath}`
}
