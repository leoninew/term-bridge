import type { AxiosResponse } from 'axios'
import { apiClient } from '../api/client'
import { buildApiWebSocketUrl } from '../../config'
import { clampTerminalSize } from '../../protocol/terminal'
import { runtimePath, type RuntimeTarget } from '../runtimeTarget'
import type {
  CreateSessionReq,
  CreateSessionResp,
  ListResp,
  RerunSessionReq,
  SessionSummary,
  UpdateSessionReq,
  WorkspaceTreeSession,
} from '../../protocol/terminal'

type UpdateSessionOrderResp = ListResp<WorkspaceTreeSession>

export type ApiResult<T> = {
  data: T
  offline: boolean
}

function workspaceSessionPath(workspaceId: string, sessionId?: string): string {
  const base = `/workspaces/${encodeURIComponent(workspaceId)}/sessions`
  return sessionId ? `${base}/${encodeURIComponent(sessionId)}` : base
}

function offline(response: AxiosResponse): boolean {
  return String(response.headers['x-termbridge-offline'] ?? '').toLowerCase() === 'true'
}

export async function createSession(
  target: RuntimeTarget,
  workspaceId: string | null,
  request: CreateSessionReq,
): Promise<CreateSessionResp> {
  const response = await apiClient.post<CreateSessionResp>(
    runtimePath(target, workspaceId ? workspaceSessionPath(workspaceId) : '/sessions'),
    workspaceId ? { ...request, workspace_id: workspaceId } : request,
  )
  return response.data
}

export async function getSession(
  target: RuntimeTarget,
  workspaceId: string,
  sessionId: string,
): Promise<SessionSummary> {
  const response = await apiClient.get<SessionSummary>(
    runtimePath(target, workspaceSessionPath(workspaceId, sessionId)),
  )
  return response.data
}

export async function updateSession(
  target: RuntimeTarget,
  workspaceId: string,
  sessionId: string,
  request: UpdateSessionReq,
): Promise<SessionSummary> {
  const response = await apiClient.patch<SessionSummary>(
    runtimePath(target, workspaceSessionPath(workspaceId, sessionId)),
    request,
  )
  return response.data
}

export async function deleteSession(
  target: RuntimeTarget,
  workspaceId: string,
  sessionId: string,
): Promise<void> {
  await apiClient.delete(runtimePath(target, workspaceSessionPath(workspaceId, sessionId)))
}

export async function readHistory(
  target: RuntimeTarget,
  workspaceId: string,
  sessionId: string,
): Promise<ApiResult<string>> {
  const response = await apiClient.get<string>(
    runtimePath(target, `${workspaceSessionPath(workspaceId, sessionId)}/history`),
    { responseType: 'text' },
  )
  return { data: response.data, offline: offline(response) }
}

export async function closeSession(
  target: RuntimeTarget,
  workspaceId: string,
  sessionId: string,
): Promise<SessionSummary> {
  const response = await apiClient.post<SessionSummary>(
    runtimePath(target, `${workspaceSessionPath(workspaceId, sessionId)}/close`),
  )
  return response.data
}

export async function rerunSession(
  target: RuntimeTarget,
  workspaceId: string,
  sessionId: string,
  request: RerunSessionReq,
): Promise<CreateSessionResp> {
  const response = await apiClient.post<CreateSessionResp>(
    runtimePath(target, `${workspaceSessionPath(workspaceId, sessionId)}/rerun`),
    request,
  )
  return response.data
}

export async function updateSessionOrder(
  target: RuntimeTarget,
  workspaceId: string,
  sessionIds: string[],
): Promise<WorkspaceTreeSession[]> {
  const response = await apiClient.patch<UpdateSessionOrderResp>(
    runtimePath(target, `${workspaceSessionPath(workspaceId)}/order`),
    { session_ids: sessionIds },
  )
  return response.data.items
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
  return buildApiWebSocketUrl(pathWithQuery)
}
