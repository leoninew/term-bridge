import type { AxiosResponse } from 'axios'
import type {
  AuthChangePasswordReq,
  AuthExternalCallbackReq,
  AuthExternalCallbackResp,
  AuthLoginReq,
  AuthLoginResp,
  AuthMeResp,
  AuthCsrfTokenResp,
  AuthSecurityConfigResp,
  AuthPasswordResetConfirmReq,
  AuthPasswordResetRequestReq,
  AuthRegisterReq,
  AuthResendVerificationReq,
  AuthVerifyEmailReq,
  ExternalAuthUrlResp,
} from '../../gen/proto/termbridge/cloud/v1/auth'
import type { DeviceSummary, ListDevicesResp } from '../../gen/proto/termbridge/cloud/v1/device'
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
import { cloudApiClient } from '../api/client'
import { workspaceSessionPath, type ApiResult } from '../sessions/runtime'
import type { RuntimeTarget } from '../runtimeTarget'

export async function authMeViaCloud(): Promise<AuthMeResp> {
  try {
    const response = await cloudApiClient.get<AuthMeResp>('/auth/me')
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

export async function authTurnstileSiteKey(): Promise<string> {
  const response = await cloudApiClient.get<AuthSecurityConfigResp>('/auth/turnstile/config')
  return response.data.turnstile_site_key
}

export async function authLoginCSRFToken(): Promise<string> {
  const response = await cloudApiClient.get<AuthCsrfTokenResp>('/auth/login/csrf')
  return response.data.token
}

export async function authLogin(
  email: string,
  password: string,
  turnstileToken: string,
  csrfToken: string,
): Promise<AuthLoginResp> {
  const request: AuthLoginReq = {
    email,
    username: '',
    password,
    turnstile_token: turnstileToken,
    csrf_token: csrfToken,
  }
  const response = await cloudApiClient.post<AuthLoginResp>('/auth/login', request)
  return response.data
}

export async function authLogout(): Promise<void> {
  await cloudApiClient.post('/auth/logout')
}

export async function authRegister(
  email: string,
  password: string,
  turnstileToken: string,
): Promise<void> {
  const request: AuthRegisterReq = { email, password, turnstile_token: turnstileToken }
  await cloudApiClient.post('/auth/register', request)
}

export async function authVerifyEmail(email: string, code: string): Promise<void> {
  const request: AuthVerifyEmailReq = { email, code }
  await cloudApiClient.post('/auth/email/verify', request)
}

export async function authResendVerification(email: string): Promise<void> {
  const request: AuthResendVerificationReq = { email }
  await cloudApiClient.post('/auth/email/verification/resend', request)
}

export async function authPasswordResetRequest(email: string): Promise<void> {
  const request: AuthPasswordResetRequestReq = { email }
  await cloudApiClient.post('/auth/password-reset/request', request)
}

export async function authPasswordResetConfirm(
  email: string,
  code: string,
  newPassword: string,
): Promise<void> {
  const request: AuthPasswordResetConfirmReq = { email, code, new_password: newPassword }
  await cloudApiClient.post('/auth/password-reset/confirm', request)
}

export async function authChangePassword(
  currentPassword: string,
  newPassword: string,
): Promise<void> {
  const request: AuthChangePasswordReq = {
    current_password: currentPassword,
    new_password: newPassword,
  }
  await cloudApiClient.post('/auth/password/change', request)
}

export async function authExternalUrl(providerId: string): Promise<string> {
  const response = await cloudApiClient.get<ExternalAuthUrlResp>(
    `/oauth2/${encodeURIComponent(providerId)}`,
  )
  return response.data.auth_url
}

export async function authExternalCallback(
  providerId: string,
  code: string,
  state: string,
): Promise<AuthExternalCallbackResp> {
  const request: AuthExternalCallbackReq = { code, state }
  const response = await cloudApiClient.post<AuthExternalCallbackResp>(
    `/oauth2/${encodeURIComponent(providerId)}/callback`,
    request,
  )
  return response.data
}

export async function cloudOAuthAuthorize(authorizePath: string): Promise<string> {
  const response = await cloudApiClient.post<{ redirect_url: string }>(
    `/oauth2/authorize${queryFromPath(authorizePath)}`,
  )
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

export async function listShortcuts(target: RuntimeTarget): Promise<Shortcut[]> {
  const response = await cloudApiClient.get<ListShortcutsResp>(
    cloudRuntimePath(target, '/shortcuts'),
  )
  return response.data.items
}

export async function createShortcut(
  target: RuntimeTarget,
  request: CreateShortcutReq,
): Promise<Shortcut> {
  const response = await cloudApiClient.post<Shortcut>(
    cloudRuntimePath(target, '/shortcuts'),
    request,
  )
  return response.data
}

export async function updateShortcut(
  target: RuntimeTarget,
  shortcutId: string,
  request: UpdateShortcutReq,
): Promise<Shortcut> {
  const response = await cloudApiClient.patch<Shortcut>(
    cloudRuntimePath(target, `/shortcuts/${encodeURIComponent(shortcutId)}`),
    request,
  )
  return response.data
}

export async function updateShortcutOrder(
  target: RuntimeTarget,
  request: UpdateShortcutOrderReq,
): Promise<Shortcut[]> {
  const response = await cloudApiClient.patch<UpdateShortcutOrderResp>(
    cloudRuntimePath(target, '/shortcuts/order'),
    request,
  )
  return response.data.items
}

export async function deleteShortcut(target: RuntimeTarget, shortcutId: string): Promise<void> {
  await cloudApiClient.delete(
    cloudRuntimePath(target, `/shortcuts/${encodeURIComponent(shortcutId)}`),
  )
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
): Promise<SessionSummary[]> {
  const response = await cloudApiClient.patch<UpdateSessionOrderResp>(
    cloudRuntimePath(target, `${workspaceSessionPath(workspaceId)}/order`),
    { session_ids: sessionIds },
  )
  return response.data.items
}

export async function listWorkspaces(target: RuntimeTarget): Promise<ApiResult<Workspace[]>> {
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
): Promise<ApiResult<WorkspaceTreeNode[]>> {
  const response = await cloudApiClient.get<WorkspaceTreeResp>(
    cloudRuntimePath(target, '/workspaces/tree'),
  )
  return {
    data: response.data.items,
    offline: offline(response),
  }
}

export async function updateWorkspaceOrder(
  target: RuntimeTarget,
  workspaceIds: string[],
): Promise<Workspace[]> {
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

function queryFromPath(path: string): string {
  const queryStart = path.indexOf('?')
  if (queryStart < 0) {
    return ''
  }
  return path.slice(queryStart)
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
