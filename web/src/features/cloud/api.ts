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
import { cloudApiClient } from '../api/client'
import { workspaceSessionPath, type ApiResult } from '../sessions/runtime'
import type { RuntimeTarget } from '../runtimeTarget'
import type {
  AuthMeResp,
  CloudOAuthAuthorizeResp,
  DeviceSummary,
  GoogleAuthUrlResp,
  ListDevicesResp,
  TokenResp,
} from '../types'

export type ListWorkspacesResp = ListResp<WorkspaceSummary>
export type WorkspaceTreeResp = ListResp<WorkspaceTreeSummary>
export type UpdateWorkspaceOrderResp = ListResp<WorkspaceSummary>
export type UpdateSessionOrderResp = ListResp<WorkspaceTreeSession>

export async function authMeViaCloud(): Promise<AuthMeResp> {
  try {
    const response = await cloudApiClient.get<AuthMeResp>('/auth/me')
    return response.data
  } catch (err) {
    if (isUnauthorizedApiError(err)) {
      return { authenticated: false, username: '' }
    }
    throw err
  }
}

export async function authLogin(email: string, password: string): Promise<TokenResp> {
  const response = await cloudApiClient.post<TokenResp>('/auth/login', { email, password })
  return response.data
}

export async function authLogout(): Promise<void> {
  await cloudApiClient.post('/auth/logout')
}

export async function authRegister(email: string, password: string): Promise<void> {
  await cloudApiClient.post('/auth/register', { email, password })
}

export async function authVerifyEmail(email: string, code: string): Promise<void> {
  await cloudApiClient.post('/auth/email/verify', { email, code })
}

export async function authResendVerification(email: string): Promise<void> {
  await cloudApiClient.post('/auth/email/verification/resend', { email })
}

export async function authPasswordResetRequest(email: string): Promise<void> {
  await cloudApiClient.post('/auth/password-reset/request', { email })
}

export async function authPasswordResetConfirm(
  email: string,
  code: string,
  newPassword: string,
): Promise<void> {
  await cloudApiClient.post('/auth/password-reset/confirm', {
    email,
    code,
    new_password: newPassword,
  })
}

export async function authChangePassword(
  currentPassword: string,
  newPassword: string,
): Promise<void> {
  await cloudApiClient.post('/auth/password/change', {
    current_password: currentPassword,
    new_password: newPassword,
  })
}

export async function authGoogleUrl(): Promise<string> {
  const response = await cloudApiClient.get<GoogleAuthUrlResp>('/auth/google')
  return response.data.auth_url
}

export async function authGoogleCallback(code: string, state: string): Promise<TokenResp> {
  const response = await cloudApiClient.post<TokenResp>('/auth/google/callback', {
    code,
    state,
  })
  return response.data
}

export async function cloudOAuthAuthorize(
  clientId: string,
  redirectUri: string,
  state: string,
): Promise<string> {
  const response = await cloudApiClient.get<CloudOAuthAuthorizeResp>('/cloud-oauth/authorize', {
    params: { client_id: clientId, redirect_uri: redirectUri, state },
  })
  return response.data.redirect_url
}

export async function deleteDevice(deviceId: string): Promise<void> {
  await cloudApiClient.delete(`/devices/${encodeURIComponent(deviceId)}`)
}

export async function listDevices(): Promise<DeviceSummary[]> {
  const response = await cloudApiClient.get<ListDevicesResp>('/devices')
  return response.data.items
}

export async function createSession(
  target: RuntimeTarget,
  workspaceId: string | null,
  request: CreateSessionReq,
): Promise<CreateSessionResp> {
  const response = await cloudApiClient.post<CreateSessionResp>(
    cloudRuntimePath(target, workspaceId ? workspaceSessionPath(workspaceId) : '/sessions'),
    workspaceId ? { ...request, workspace_id: workspaceId } : request,
  )
  return response.data
}

export async function getSession(
  target: RuntimeTarget,
  workspaceId: string,
  sessionId: string,
): Promise<SessionSummary> {
  const response = await cloudApiClient.get<SessionSummary>(
    cloudRuntimePath(target, workspaceSessionPath(workspaceId, sessionId)),
  )
  return response.data
}

export async function updateSession(
  target: RuntimeTarget,
  workspaceId: string,
  sessionId: string,
  request: UpdateSessionReq,
): Promise<SessionSummary> {
  const response = await cloudApiClient.patch<SessionSummary>(
    cloudRuntimePath(target, workspaceSessionPath(workspaceId, sessionId)),
    request,
  )
  return response.data
}

export async function deleteSession(
  target: RuntimeTarget,
  workspaceId: string,
  sessionId: string,
): Promise<void> {
  await cloudApiClient.delete(
    cloudRuntimePath(target, workspaceSessionPath(workspaceId, sessionId)),
  )
}

export async function readHistory(
  target: RuntimeTarget,
  workspaceId: string,
  sessionId: string,
): Promise<ApiResult<string>> {
  const response = await cloudApiClient.get<string>(
    cloudRuntimePath(target, `${workspaceSessionPath(workspaceId, sessionId)}/history`),
    { responseType: 'text' },
  )
  return { data: response.data, offline: offline(response) }
}

export async function closeSession(
  target: RuntimeTarget,
  workspaceId: string,
  sessionId: string,
): Promise<SessionSummary> {
  const response = await cloudApiClient.post<SessionSummary>(
    cloudRuntimePath(target, `${workspaceSessionPath(workspaceId, sessionId)}/close`),
  )
  return response.data
}

export async function rerunSession(
  target: RuntimeTarget,
  workspaceId: string,
  sessionId: string,
  request: RerunSessionReq,
): Promise<CreateSessionResp> {
  const response = await cloudApiClient.post<CreateSessionResp>(
    cloudRuntimePath(target, `${workspaceSessionPath(workspaceId, sessionId)}/rerun`),
    request,
  )
  return response.data
}

export async function updateSessionOrder(
  target: RuntimeTarget,
  workspaceId: string,
  sessionIds: string[],
): Promise<WorkspaceTreeSession[]> {
  const response = await cloudApiClient.patch<UpdateSessionOrderResp>(
    cloudRuntimePath(target, `${workspaceSessionPath(workspaceId)}/order`),
    { session_ids: sessionIds },
  )
  return response.data.items
}

export async function listWorkspaces(
  target: RuntimeTarget,
): Promise<ApiResult<WorkspaceSummary[]>> {
  const response = await cloudApiClient.get<ListWorkspacesResp>(
    cloudRuntimePath(target, '/workspaces'),
  )
  return {
    data: response.data.items,
    offline: offline(response),
  }
}

export async function listWorkspaceTree(
  target: RuntimeTarget,
): Promise<ApiResult<WorkspaceTreeSummary[]>> {
  const response = await cloudApiClient.get<WorkspaceTreeResp>(
    cloudRuntimePath(target, '/workspaces/tree'),
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
  const response = await cloudApiClient.patch<UpdateWorkspaceOrderResp>(
    cloudRuntimePath(target, '/workspaces/order'),
    request,
  )
  return response.data.items
}

export async function deleteWorkspace(target: RuntimeTarget, workspaceId: string): Promise<void> {
  await cloudApiClient.delete(
    cloudRuntimePath(target, `/workspaces/${encodeURIComponent(workspaceId)}`),
  )
}

function cloudRuntimePath(target: RuntimeTarget, path: string): string {
  if (target.mode !== 'cloud') {
    throw new Error('cloud runtime API requires a cloud target')
  }
  return `/devices/${encodeURIComponent(target.deviceId)}${path}`
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
