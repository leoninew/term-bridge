import type { AxiosResponse } from 'axios'
import type { ListResp, WorkspaceSummary, WorkspaceTreeSummary } from '../../protocol/terminal'
import { apiClient } from '../api/client'
import type { ApiResult } from '../sessions/api'

type ListWorkspacesResp = ListResp<WorkspaceSummary>
type WorkspaceTreeResp = ListResp<WorkspaceTreeSummary>
type UpdateWorkspaceOrderResp = ListResp<WorkspaceSummary>

function devicePath(deviceId: string, path: string): string {
  return `/api/devices/${encodeURIComponent(deviceId)}${path}`
}

function offline(response: AxiosResponse): boolean {
  return String(response.headers['x-termbridge-offline'] ?? '').toLowerCase() === 'true'
}

export async function listWorkspaces(deviceId: string): Promise<ApiResult<WorkspaceSummary[]>> {
  const response = await apiClient.get<ListWorkspacesResp>(devicePath(deviceId, '/workspaces'))
  return {
    data: response.data.items,
    offline: offline(response),
  }
}

export async function listWorkspaceTree(
  deviceId: string,
): Promise<ApiResult<WorkspaceTreeSummary[]>> {
  const response = await apiClient.get<WorkspaceTreeResp>(devicePath(deviceId, '/workspaces/tree'))
  return {
    data: response.data.items,
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
  const response = await apiClient.patch<UpdateWorkspaceOrderResp>(
    devicePath(deviceId, '/workspaces/order'),
    request,
  )
  return response.data.items
}

export async function deleteWorkspace(deviceId: string, workspaceId: string): Promise<void> {
  await apiClient.delete(devicePath(deviceId, `/workspaces/${encodeURIComponent(workspaceId)}`))
}
