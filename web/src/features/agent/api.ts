import type { AxiosResponse } from 'axios'
import type {
  CreateSessionReq,
  CreateSessionResp,
  ListResp,
  RerunSessionReq,
  SessionSummary,
  UpdateSessionReq,
  WorkspaceSummary,
  WorkspaceTreeSession,
  WorkspaceTreeSummary,
} from '../../protocol/terminal'
import { agentApiClient } from '../api/client'
import { workspaceSessionPath, type ApiResult } from '../sessions/runtime'
import type {
  AuthCapabilities,
  AuthMeResp,
  CloudOAuthCallbackResp,
  CloudOAuthStartResp,
  CloudSessionSummary,
  TokenResp,
  UserInfo,
} from '../types'

export type ListWorkspacesResp = ListResp<WorkspaceSummary>
export type WorkspaceTreeResp = ListResp<WorkspaceTreeSummary>
export type UpdateWorkspaceOrderResp = ListResp<WorkspaceSummary>
export type UpdateSessionOrderResp = ListResp<WorkspaceTreeSession>

export async function authMe(): Promise<AuthMeResp> {
  try {
    const response = await agentApiClient.get<AuthMeResp>('/api/auth/me')
    return response.data
  } catch (err) {
    if (isUnauthorizedApiError(err)) {
      return { authenticated: false, username: '' }
    }
    throw err
  }
}

export async function authLogout(): Promise<void> {
  await agentApiClient.post('/api/auth/logout')
}

export function cloudOAuthStartURL(): string {
  return '/cloud/oauth/start'
}

export async function cloudOAuthStart(redirect?: string): Promise<string> {
  const response = await agentApiClient.get<CloudOAuthStartResp>('/api/cloud-oauth/start', {
    params: redirect ? { redirect } : undefined,
  })
  return response.data.authorize_url
}

export async function cloudOAuthCallback(
  code: string,
  state: string,
): Promise<CloudOAuthCallbackResp> {
  const response = await agentApiClient.post<CloudOAuthCallbackResp>('/api/cloud-oauth/callback', {
    code,
    state,
  })
  return response.data
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
): Promise<WorkspaceTreeSession[]> {
  const response = await agentApiClient.patch<UpdateSessionOrderResp>(
    agentRuntimePath(`${workspaceSessionPath(workspaceId)}/order`),
    { session_ids: sessionIds },
  )
  return response.data.items
}

export async function listWorkspaces(): Promise<ApiResult<WorkspaceSummary[]>> {
  const response = await agentApiClient.get<ListWorkspacesResp>(agentRuntimePath('/workspaces'))
  return {
    data: response.data.items,
    offline: offline(response),
  }
}

export async function listWorkspaceTree(): Promise<ApiResult<WorkspaceTreeSummary[]>> {
  const response = await agentApiClient.get<WorkspaceTreeResp>(agentRuntimePath('/workspaces/tree'))
  return {
    data: response.data.items,
    offline: offline(response),
  }
}

export type UpdateWorkspaceOrderReq = {
  workspace_ids: string[]
}

export async function updateWorkspaceOrder(workspaceIds: string[]): Promise<WorkspaceSummary[]> {
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
  return `/api${path}`
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

export type { AuthCapabilities, AuthMeResp, CloudSessionSummary, TokenResp, UserInfo }
