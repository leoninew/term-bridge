import type { AxiosInstance, AxiosResponse } from 'axios'
import type { ListResp, WorkspaceSummary, WorkspaceTreeSummary } from '../../protocol/terminal'
import { agentApiClient, cloudApiClient } from '../api/client'
import type { ApiResult } from '../sessions/api'
import { runtimeApiTarget, runtimePath, type RuntimeTarget } from '../runtimeTarget'

type ListWorkspacesResp = ListResp<WorkspaceSummary>
type WorkspaceTreeResp = ListResp<WorkspaceTreeSummary>
type UpdateWorkspaceOrderResp = ListResp<WorkspaceSummary>

function apiClientForTarget(target: RuntimeTarget): AxiosInstance {
  return runtimeApiTarget(target) === 'cloud' ? cloudApiClient : agentApiClient
}

function offline(response: AxiosResponse): boolean {
  return String(response.headers['x-termbridge-offline'] ?? '').toLowerCase() === 'true'
}

export async function listWorkspaces(
  target: RuntimeTarget,
): Promise<ApiResult<WorkspaceSummary[]>> {
  const response = await apiClientForTarget(target).get<ListWorkspacesResp>(
    runtimePath(target, '/workspaces'),
  )
  return {
    data: response.data.items,
    offline: offline(response),
  }
}

export async function listWorkspaceTree(
  target: RuntimeTarget,
): Promise<ApiResult<WorkspaceTreeSummary[]>> {
  const response = await apiClientForTarget(target).get<WorkspaceTreeResp>(
    runtimePath(target, '/workspaces/tree'),
  )
  return {
    data: response.data.items,
    offline: offline(response),
  }
}

export type UpdateWorkspaceOrderReq = {
  workspace_ids: string[]
}

export async function updateWorkspaceOrder(
  target: RuntimeTarget,
  workspaceIds: string[],
): Promise<WorkspaceSummary[]> {
  const request: UpdateWorkspaceOrderReq = { workspace_ids: workspaceIds }
  const response = await apiClientForTarget(target).patch<UpdateWorkspaceOrderResp>(
    runtimePath(target, '/workspaces/order'),
    request,
  )
  return response.data.items
}

export async function deleteWorkspace(target: RuntimeTarget, workspaceId: string): Promise<void> {
  await apiClientForTarget(target).delete(
    runtimePath(target, `/workspaces/${encodeURIComponent(workspaceId)}`),
  )
}
