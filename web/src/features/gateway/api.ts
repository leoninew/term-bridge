import type { SessionSummary, WorkspaceTreeSummary } from '../../protocol/terminal'
import { responseError } from '../sessions/api'

export type GatewayDeviceSummary = {
  id: string
  name: string
  online: boolean
  connected_at: string
  last_seen: string
}

export async function gatewayMe(): Promise<{ authenticated: boolean; username: string }> {
  const response = await fetch('/api/gateway/me')
  if (response.status === 401) {
    return { authenticated: false, username: '' }
  }
  if (!response.ok) {
    throw new Error(await responseError('Gateway auth check failed', response))
  }
  return (await response.json()) as { authenticated: boolean; username: string }
}

export async function gatewayLogin(username: string, password: string): Promise<void> {
  const response = await fetch('/api/gateway/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  if (!response.ok) {
    throw new Error(await responseError('Gateway login failed', response))
  }
}

export async function gatewayLogout(): Promise<void> {
  const response = await fetch('/api/gateway/logout', { method: 'POST' })
  if (!response.ok) {
    throw new Error(await responseError('Gateway logout failed', response))
  }
}

export async function listGatewayDevices(): Promise<GatewayDeviceSummary[]> {
  const response = await fetch('/api/gateway/devices')
  if (!response.ok) {
    throw new Error(await responseError('List gateway devices failed', response))
  }
  return ((await response.json()) as GatewayDeviceSummary[] | null) ?? []
}

export async function listGatewayWorkspaceTree(deviceId: string): Promise<WorkspaceTreeSummary[]> {
  const response = await fetch(`/api/gateway/devices/${encodeURIComponent(deviceId)}/workspaces/tree`)
  if (!response.ok) {
    throw new Error(await responseError('List gateway workspace tree failed', response))
  }
  return ((await response.json()) as WorkspaceTreeSummary[] | null) ?? []
}

export async function listGatewaySessions(deviceId: string): Promise<SessionSummary[]> {
  const response = await fetch(`/api/gateway/devices/${encodeURIComponent(deviceId)}/sessions`)
  if (!response.ok) {
    throw new Error(await responseError('List gateway sessions failed', response))
  }
  return ((await response.json()) as SessionSummary[] | null) ?? []
}

export async function readGatewayHistory(deviceId: string, sessionId: string): Promise<string> {
  const response = await fetch(`/api/gateway/devices/${encodeURIComponent(deviceId)}/sessions/${encodeURIComponent(sessionId)}/history`)
  if (!response.ok) {
    throw new Error(await responseError('Read gateway history failed', response))
  }
  return response.text()
}
