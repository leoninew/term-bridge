import type { AxiosResponse } from 'axios'
import type {
  ListWorkspacesResp,
  SessionSummary,
  UpdateWorkspaceOrderReq,
  UpdateWorkspaceOrderResp,
  Workspace,
  WorkspaceTreeNode,
  WorkspaceTreeResp,
} from '../../gen/proto/termbridge/agent/v1/workspace'
import type {
  CreateSessionReq,
  CreateSessionResp,
  RerunSessionReq,
  UpdateSessionReq,
  UpdateSessionOrderResp,
} from '../../gen/proto/termbridge/agent/v1/session'
import type { AuthMeResp, TokenResp } from '../../gen/proto/termbridge/cloud/v1/auth'
import type { CloudConnectResp } from '../../gen/proto/termbridge/cloud/v1/session'
import { agentApiClient } from '../api/client'
import { workspaceSessionPath, type ApiResult } from '../sessions/runtime'

export async function authMe(): Promise<AuthMeResp> {
  try {
    const response = await agentApiClient.get<AuthMeResp>('/auth/me')
    return response.data
  } catch (err) {
    if (isUnauthorizedApiError(err)) {
      return {
        authenticated: false,
        username: '',
        user: undefined,
        cloud_session: undefined,
        device: undefined,
      }
    }
    throw err
  }
}

export async function authLoginViaAgent(): Promise<TokenResp> {
  const response = await agentApiClient.post<TokenResp>('/auth/login')
  return response.data
}

export async function authLogout(): Promise<void> {
  await agentApiClient.post('/auth/logout')
}

export async function connectCloudWithCurrentAccount(code: string): Promise<CloudConnectResp> {
  const response = await agentApiClient.post<CloudConnectResp>('/cloud/connect', { code })
  return response.data
}

export async function disconnectCloud(): Promise<void> {
  await agentApiClient.post('/cloud/disconnect')
}

export async function createSession(
  workspaceId: string | null,
  request: CreateSessionReq,
): Promise<CreateSessionResp> {
  const response = await agentApiClient.post<CreateSessionResp>(
    agentRuntimePath(workspaceId ? workspaceSessionPath(workspaceId) : '/sessions'),
    workspaceId ? { ...request, workspace_id: workspaceId } : request,
  )
  return response.data
}

export async function getSession(workspaceId: string, sessionId: string): Promise<SessionSummary> {
  const response = await agentApiClient.get<SessionSummary>(
    agentRuntimePath(workspaceSessionPath(workspaceId, sessionId)),
  )
  return response.data
}

export async function updateSession(
  workspaceId: string,
  sessionId: string,
  request: UpdateSessionReq,
): Promise<SessionSummary> {
  const response = await agentApiClient.patch<SessionSummary>(
    agentRuntimePath(workspaceSessionPath(workspaceId, sessionId)),
    request,
  )
  return response.data
}

export async function deleteSession(workspaceId: string, sessionId: string): Promise<void> {
  await agentApiClient.delete(agentRuntimePath(workspaceSessionPath(workspaceId, sessionId)))
}

export async function readHistory(
  workspaceId: string,
  sessionId: string,
): Promise<ApiResult<string>> {
  const response = await agentApiClient.get<string>(
    agentRuntimePath(`${workspaceSessionPath(workspaceId, sessionId)}/history`),
    { responseType: 'text' },
  )
  return { data: response.data, offline: offline(response) }
}

export async function closeSession(
  workspaceId: string,
  sessionId: string,
): Promise<SessionSummary> {
  const response = await agentApiClient.post<SessionSummary>(
    agentRuntimePath(`${workspaceSessionPath(workspaceId, sessionId)}/close`),
  )
  return response.data
}

export async function rerunSession(
  workspaceId: string,
  sessionId: string,
  request: RerunSessionReq,
): Promise<CreateSessionResp> {
  const response = await agentApiClient.post<CreateSessionResp>(
    agentRuntimePath(`${workspaceSessionPath(workspaceId, sessionId)}/rerun`),
    request,
  )
  return response.data
}

export async function updateSessionOrder(
  workspaceId: string,
  sessionIds: string[],
): Promise<SessionSummary[]> {
  const response = await agentApiClient.patch<UpdateSessionOrderResp>(
    agentRuntimePath(`${workspaceSessionPath(workspaceId)}/order`),
    { session_ids: sessionIds },
  )
  return response.data.items
}

export async function listWorkspaces(): Promise<ApiResult<Workspace[]>> {
  const response = await agentApiClient.get<ListWorkspacesResp>(agentRuntimePath('/workspaces'))
  return {
    data: response.data.items,
    offline: offline(response),
  }
}

export async function listWorkspaceTree(): Promise<ApiResult<WorkspaceTreeNode[]>> {
  const response = await agentApiClient.get<WorkspaceTreeResp>(agentRuntimePath('/workspaces/tree'))
  return {
    data: response.data.items,
    offline: offline(response),
  }
}

export async function updateWorkspaceOrder(workspaceIds: string[]): Promise<Workspace[]> {
  const request: UpdateWorkspaceOrderReq = { workspace_ids: workspaceIds }
  const response = await agentApiClient.patch<UpdateWorkspaceOrderResp>(
    agentRuntimePath('/workspaces/order'),
    request,
  )
  return response.data.items
}

export async function deleteWorkspace(workspaceId: string): Promise<void> {
  await agentApiClient.delete(agentRuntimePath(`/workspaces/${encodeURIComponent(workspaceId)}`))
}

function agentRuntimePath(path: string): string {
  return path
}

function offline(response: AxiosResponse): boolean {
  return String(response.headers['x-termbridge-offline'] ?? '').toLowerCase() === 'true'
}

function isUnauthorizedApiError(err: unknown): boolean {
  return (
    err instanceof Error &&
    'status' in err &&
    'code' in err &&
    err.status === 401 &&
    err.code === 'unauthorized'
  )
}
