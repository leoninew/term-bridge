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
import type {
  CreateShortcutReq,
  ListShortcutsResp,
  Shortcut,
  UpdateShortcutOrderReq,
  UpdateShortcutOrderResp,
  UpdateShortcutReq,
} from '../../gen/proto/termbridge/agent/v1/shortcut'
import type {
  AuthMeResp,
  CloudOAuthExchangeReq,
  CloudOAuthExchangeResp,
} from '../../gen/proto/termbridge/cloud/v1/auth'
import type { CloudConnectResp } from '../../gen/proto/termbridge/cloud/v1/session'
import { localApiClient } from '../api/client'
import { workspaceSessionPath, type ApiResult } from '../sessions/runtime'

function cloudAuthorizationHeaders(cloudToken: string) {
  return { Authorization: 'Bearer ' + cloudToken }
}

export async function agentStatus(): Promise<AuthMeResp> {
  const response = await localApiClient.get<AuthMeResp>('/agent/status')
  return response.data
}

export async function fetchCloudIdentityViaLocalApi(cloudToken: string): Promise<AuthMeResp> {
  const response = await localApiClient.get<AuthMeResp>('/cloud/auth/me', {
    headers: cloudAuthorizationHeaders(cloudToken),
  })
  return response.data
}

export async function connectCloudWithToken(cloudToken: string): Promise<CloudConnectResp> {
  const response = await localApiClient.post<CloudConnectResp>(
    '/cloud/connect',
    {},
    { headers: cloudAuthorizationHeaders(cloudToken) },
  )
  return response.data
}

export async function disconnectCloud(): Promise<void> {
  await localApiClient.post('/cloud/disconnect')
}

export async function exchangeOAuthCode(code: string): Promise<string> {
  const request: CloudOAuthExchangeReq = { code }
  const response = await localApiClient.post<CloudOAuthExchangeResp>(
    '/cloud/oauth/exchange',
    request,
  )
  return response.data.access_token
}

export async function createSession(
  workspaceId: string | null,
  request: CreateSessionReq,
): Promise<CreateSessionResp> {
  const response = await localApiClient.post<CreateSessionResp>(
    localRuntimePath(workspaceId ? workspaceSessionPath(workspaceId) : '/sessions'),
    workspaceId ? { ...request, workspace_id: workspaceId } : request,
  )
  return response.data
}

export async function getSession(workspaceId: string, sessionId: string): Promise<SessionSummary> {
  const response = await localApiClient.get<SessionSummary>(
    localRuntimePath(workspaceSessionPath(workspaceId, sessionId)),
  )
  return response.data
}

export async function updateSession(
  workspaceId: string,
  sessionId: string,
  request: UpdateSessionReq,
): Promise<SessionSummary> {
  const response = await localApiClient.patch<SessionSummary>(
    localRuntimePath(workspaceSessionPath(workspaceId, sessionId)),
    request,
  )
  return response.data
}

export async function listShortcuts(): Promise<Shortcut[]> {
  const response = await localApiClient.get<ListShortcutsResp>(localRuntimePath('/shortcuts'))
  return response.data.items
}

export async function createShortcut(request: CreateShortcutReq): Promise<Shortcut> {
  const response = await localApiClient.post<Shortcut>(localRuntimePath('/shortcuts'), request)
  return response.data
}

export async function updateShortcut(
  shortcutId: string,
  request: UpdateShortcutReq,
): Promise<Shortcut> {
  const response = await localApiClient.patch<Shortcut>(
    localRuntimePath(`/shortcuts/${encodeURIComponent(shortcutId)}`),
    request,
  )
  return response.data
}

export async function updateShortcutOrder(request: UpdateShortcutOrderReq): Promise<Shortcut[]> {
  const response = await localApiClient.patch<UpdateShortcutOrderResp>(
    localRuntimePath('/shortcuts/order'),
    request,
  )
  return response.data.items
}

export async function deleteShortcut(shortcutId: string): Promise<void> {
  await localApiClient.delete(localRuntimePath(`/shortcuts/${encodeURIComponent(shortcutId)}`))
}

export async function deleteSession(workspaceId: string, sessionId: string): Promise<void> {
  await localApiClient.delete(localRuntimePath(workspaceSessionPath(workspaceId, sessionId)))
}

export async function readHistory(
  workspaceId: string,
  sessionId: string,
): Promise<ApiResult<string>> {
  const response = await localApiClient.get<string>(
    localRuntimePath(`${workspaceSessionPath(workspaceId, sessionId)}/history`),
    { responseType: 'text' },
  )
  return { data: response.data, offline: offline(response) }
}

export async function closeSession(
  workspaceId: string,
  sessionId: string,
): Promise<SessionSummary> {
  const response = await localApiClient.post<SessionSummary>(
    localRuntimePath(`${workspaceSessionPath(workspaceId, sessionId)}/close`),
  )
  return response.data
}

export async function rerunSession(
  workspaceId: string,
  sessionId: string,
  request: RerunSessionReq,
): Promise<CreateSessionResp> {
  const response = await localApiClient.post<CreateSessionResp>(
    localRuntimePath(`${workspaceSessionPath(workspaceId, sessionId)}/rerun`),
    request,
  )
  return response.data
}

export async function updateSessionOrder(
  workspaceId: string,
  sessionIds: string[],
): Promise<SessionSummary[]> {
  const response = await localApiClient.patch<UpdateSessionOrderResp>(
    localRuntimePath(`${workspaceSessionPath(workspaceId)}/order`),
    { session_ids: sessionIds },
  )
  return response.data.items
}

export async function listWorkspaces(): Promise<ApiResult<Workspace[]>> {
  const response = await localApiClient.get<ListWorkspacesResp>(localRuntimePath('/workspaces'))
  return {
    data: response.data.items,
    offline: offline(response),
  }
}

export async function listWorkspaceTree(): Promise<ApiResult<WorkspaceTreeNode[]>> {
  const response = await localApiClient.get<WorkspaceTreeResp>(localRuntimePath('/workspaces/tree'))
  return {
    data: response.data.items,
    offline: offline(response),
  }
}

export async function updateWorkspaceOrder(workspaceIds: string[]): Promise<Workspace[]> {
  const request: UpdateWorkspaceOrderReq = { workspace_ids: workspaceIds }
  const response = await localApiClient.patch<UpdateWorkspaceOrderResp>(
    localRuntimePath('/workspaces/order'),
    request,
  )
  return response.data.items
}

export async function deleteWorkspace(workspaceId: string): Promise<void> {
  await localApiClient.delete(localRuntimePath(`/workspaces/${encodeURIComponent(workspaceId)}`))
}

function localRuntimePath(path: string): string {
  return path
}

function offline(response: AxiosResponse): boolean {
  return String(response.headers['x-termbridge-offline'] ?? '').toLowerCase() === 'true'
}
