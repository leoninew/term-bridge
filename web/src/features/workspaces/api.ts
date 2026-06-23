import type { WorkspaceSummary, WorkspaceTreeSummary } from '../../protocol/terminal'
import { responseError, type ApiResult } from '../sessions/api'

function devicePath(deviceId: string, path: string): string {
  return `/api/devices/${encodeURIComponent(deviceId)}${path}`
}

function offline(response: Response): boolean {
  return response.headers.get('x-termbridge-offline') === 'true'
}

export async function listWorkspaces(deviceId: string): Promise<ApiResult<WorkspaceSummary[]>> {
  const response = await fetch(devicePath(deviceId, '/workspaces'))
  if (!response.ok) {
    throw new Error(await responseError('List workspaces failed', response))
  }
  return {
    data: ((await response.json()) as WorkspaceSummary[] | null) ?? [],
    offline: offline(response),
  }
}

export async function listWorkspaceTree(
  deviceId: string,
): Promise<ApiResult<WorkspaceTreeSummary[]>> {
  const response = await fetch(devicePath(deviceId, '/workspaces/tree'))
  if (!response.ok) {
    throw new Error(await responseError('List workspace tree failed', response))
  }
  return {
    data: ((await response.json()) as WorkspaceTreeSummary[] | null) ?? [],
    offline: offline(response),
  }
}

export async function updateWorkspaceOrder(
  deviceId: string,
  workspaceIds: string[],
): Promise<WorkspaceSummary[]> {
  const response = await fetch(devicePath(deviceId, '/workspaces/order'), {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ workspace_ids: workspaceIds }),
  })
  if (!response.ok) {
    throw new Error(await responseError('Update workspace order failed', response))
  }
  return ((await response.json()) as WorkspaceSummary[] | null) ?? []
}

export async function deleteWorkspace(deviceId: string, workspaceId: string): Promise<void> {
  const response = await fetch(
    devicePath(deviceId, `/workspaces/${encodeURIComponent(workspaceId)}`),
    {
      method: 'DELETE',
    },
  )
  if (!response.ok) {
    throw new Error(await responseError('Remove workspace failed', response))
  }
}
