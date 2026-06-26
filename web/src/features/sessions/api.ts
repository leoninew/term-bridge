import type { AxiosResponse } from 'axios'
import { apiClient } from '../api/client'
import { clampTerminalSize } from '../../protocol/terminal'
import type {
  CreateSessionReq,
  CreateSessionResp,
  RerunSessionReq,
  SessionSummary,
  UpdateSessionReq,
  WorkspaceTreeSession,
} from '../../protocol/terminal'

export type ApiResult<T> = {
  data: T
  offline: boolean
}

function devicePath(deviceId: string, path: string): string {
  return `/api/devices/${encodeURIComponent(deviceId)}${path}`
}

function workspaceSessionPath(workspaceId: string, sessionId?: string): string {
  const base = `/workspaces/${encodeURIComponent(workspaceId)}/sessions`
  return sessionId ? `${base}/${encodeURIComponent(sessionId)}` : base
}

function offline(response: AxiosResponse): boolean {
  return String(response.headers['x-termbridge-offline'] ?? '').toLowerCase() === 'true'
}

export async function createSession(
  deviceId: string,
  workspaceId: string | null,
  request: CreateSessionReq,
): Promise<CreateSessionResp> {
  const response = await apiClient.post<CreateSessionResp>(
    devicePath(deviceId, workspaceId ? workspaceSessionPath(workspaceId) : '/sessions'),
    workspaceId ? { ...request, workspace_id: workspaceId } : request,
  )
  return response.data
}

export async function getSession(
  deviceId: string,
  workspaceId: string,
  sessionId: string,
): Promise<SessionSummary> {
  const response = await apiClient.get<SessionSummary>(
    devicePath(deviceId, workspaceSessionPath(workspaceId, sessionId)),
  )
  return response.data
}

export async function updateSession(
  deviceId: string,
  workspaceId: string,
  sessionId: string,
  request: UpdateSessionReq,
): Promise<SessionSummary> {
  const response = await apiClient.patch<SessionSummary>(
    devicePath(deviceId, workspaceSessionPath(workspaceId, sessionId)),
    request,
  )
  return response.data
}

export async function deleteSession(
  deviceId: string,
  workspaceId: string,
  sessionId: string,
): Promise<void> {
  await apiClient.delete(devicePath(deviceId, workspaceSessionPath(workspaceId, sessionId)))
}

export async function readHistory(
  deviceId: string,
  workspaceId: string,
  sessionId: string,
): Promise<ApiResult<string>> {
  const response = await apiClient.get<string>(
    devicePath(deviceId, `${workspaceSessionPath(workspaceId, sessionId)}/history`),
    { responseType: 'text' },
  )
  return { data: response.data, offline: offline(response) }
}

export async function closeSession(
  deviceId: string,
  workspaceId: string,
  sessionId: string,
): Promise<SessionSummary> {
  const response = await apiClient.post<SessionSummary>(
    devicePath(deviceId, `${workspaceSessionPath(workspaceId, sessionId)}/close`),
  )
  return response.data
}

export async function rerunSession(
  deviceId: string,
  workspaceId: string,
  sessionId: string,
  request: RerunSessionReq,
): Promise<CreateSessionResp> {
  const response = await apiClient.post<CreateSessionResp>(
    devicePath(deviceId, `${workspaceSessionPath(workspaceId, sessionId)}/rerun`),
    request,
  )
  return response.data
}

export async function updateSessionOrder(
  deviceId: string,
  workspaceId: string,
  sessionIds: string[],
): Promise<WorkspaceTreeSession[]> {
  const response = await apiClient.patch<WorkspaceTreeSession[] | null>(
    devicePath(deviceId, `${workspaceSessionPath(workspaceId)}/order`),
    { session_ids: sessionIds },
  )
  return response.data ?? []
}

export function terminalWsUrl(
  deviceId: string,
  workspaceId: string,
  sessionId: string,
  token?: string,
  size?: { cols: number; rows: number } | null,
): string {
  const path = devicePath(deviceId, `${workspaceSessionPath(workspaceId, sessionId)}/ws`)
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
  return query ? `${path}?${query}` : path
}
