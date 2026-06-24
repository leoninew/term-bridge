import type { AxiosResponse } from 'axios'
import type { WorkspaceSummary, WorkspaceTreeSummary } from '../../protocol/terminal'
import { apiClient } from '../api/client'
import type { ApiResult } from '../sessions/api'

function devicePath(deviceId: string, path: string): string {
  return `/api/devices/${encodeURIComponent(deviceId)}${path}`
}

function offline(response: AxiosResponse): boolean {
  return String(response.headers['x-termbridge-offline'] ?? '').toLowerCase() === 'true'
}

export async function listWorkspaces(deviceId: string): Promise<ApiResult<WorkspaceSummary[]>> {
  const response = await apiClient.get<WorkspaceSummary[] | null>(
    devicePath(deviceId, '/workspaces'),
  )
  return {
    data: response.data ?? [],
    offline: offline(response),
  }
}

export async function listWorkspaceTree(
  deviceId: string,
): Promise<ApiResult<WorkspaceTreeSummary[]>> {
  const response = await apiClient.get<WorkspaceTreeSummary[] | null>(
    devicePath(deviceId, '/workspaces/tree'),
  )
  return {
    data: response.data ?? [],
    offline: offline(response),
  }
}

export type UpdateWorkspaceOrderReq = {
  workspace_ids: string[]
}

export async function updateWorkspaceOrder(
  deviceId: string,
  workspaceIds: string[],
): Promise<WorkspaceSummary[]> {
  const request: UpdateWorkspaceOrderReq = { workspace_ids: workspaceIds }
  const response = await apiClient.patch<WorkspaceSummary[] | null>(
    devicePath(deviceId, '/workspaces/order'),
    request,
  )
  return response.data ?? []
}

export async function deleteWorkspace(deviceId: string, workspaceId: string): Promise<void> {
  await apiClient.delete(devicePath(deviceId, `/workspaces/${encodeURIComponent(workspaceId)}`))
}
