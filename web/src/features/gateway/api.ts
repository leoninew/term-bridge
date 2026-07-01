import type { ListResp } from '../../protocol/terminal'
import { apiClient } from '../api/client'

export type DeviceSummary = {
  id: string
  name: string
  online: boolean
  status: 'online' | 'offline'
  connected_at: string
  last_seen: string
}

export type UserInfo = {
  id: string
  email: string
  display_name: string
  provider: string
  email_verified: boolean
}

export type AuthCapabilities = {
  mode: 'local' | 'cloud'
  providers: string[]
  password_reset_enabled: boolean
  email_verification_enabled: boolean
  account_auth_enabled: boolean
  cloud_oauth_enabled: boolean
}

export type CloudSessionSummary = {
  gate_url: string
  device_id: string
  device_name: string
  connected_at: string
}

export type AuthMeResp = {
  authenticated: boolean
  user?: UserInfo
  capabilities?: AuthCapabilities
  cloud_session?: CloudSessionSummary | null
}

export type TokenResp = {
  access_token: string
  token_type: string
}

export type CloudOAuthCallbackResp = {
  cloud_session: CloudSessionSummary
  redirect: string
}

export type GoogleAuthURLResp = {
  auth_url: string
}

export type CloudOAuthStartResp = {
  authorize_url: string
}

export type CloudOAuthAuthorizeResp = {
  redirect_url: string
}

export type ListDevicesResp = ListResp<DeviceSummary>

export async function authMe(): Promise<AuthMeResp> {
  try {
    const response = await apiClient.get<AuthMeResp>('/api/auth/me')
    return response.data
  } catch (err) {
    if (isUnauthorizedApiError(err)) {
      return { authenticated: false }
    }
    throw err
  }
}

export async function authLogin(email: string, password: string): Promise<TokenResp> {
  const response = await apiClient.post<TokenResp>('/api/auth/login', { email, password })
  return response.data
}

export async function authLogout(): Promise<void> {
  await apiClient.post('/api/auth/logout')
}

export async function authRegister(email: string, password: string): Promise<void> {
  await apiClient.post('/api/auth/register', { email, password })
}

export async function authVerifyEmail(email: string, code: string): Promise<void> {
  await apiClient.post('/api/auth/email/verify', { email, code })
}

export async function authResendVerification(email: string): Promise<void> {
  await apiClient.post('/api/auth/email/verification/resend', { email })
}

export async function authPasswordResetRequest(email: string): Promise<void> {
  await apiClient.post('/api/auth/password-reset/request', { email })
}

export async function authPasswordResetConfirm(
  email: string,
  code: string,
  newPassword: string,
): Promise<void> {
  await apiClient.post('/api/auth/password-reset/confirm', {
    email,
    code,
    new_password: newPassword,
  })
}

export async function authChangePassword(
  currentPassword: string,
  newPassword: string,
): Promise<void> {
  await apiClient.post('/api/auth/password/change', {
    current_password: currentPassword,
    new_password: newPassword,
  })
}

export async function authGoogleURL(): Promise<string> {
  const response = await apiClient.get<GoogleAuthURLResp>('/api/auth/google')
  return response.data.auth_url
}

export async function authGoogleCallback(code: string, state: string): Promise<TokenResp> {
  const response = await apiClient.post<TokenResp>('/api/auth/google/callback', { code, state })
  return response.data
}

export function cloudOAuthStartURL(): string {
  return '/cloud/oauth/start'
}

export async function cloudOAuthStart(redirect?: string): Promise<string> {
  const response = await apiClient.get<CloudOAuthStartResp>('/api/cloud-oauth/start', {
    params: redirect ? { redirect } : undefined,
  })
  return response.data.authorize_url
}

export async function cloudOAuthCallback(
  code: string,
  state: string,
): Promise<CloudOAuthCallbackResp> {
  const response = await apiClient.post<CloudOAuthCallbackResp>('/api/cloud-oauth/callback', {
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
  const response = await apiClient.get<CloudOAuthAuthorizeResp>('/api/cloud-oauth/authorize', {
    params: { client_id: clientId, redirect_uri: redirectUri, state },
  })
  return response.data.redirect_url
}

export async function deleteDevice(deviceId: string): Promise<void> {
  await apiClient.delete(`/api/devices/${encodeURIComponent(deviceId)}`)
}

export async function listDevices(): Promise<DeviceSummary[]> {
  const response = await apiClient.get<ListDevicesResp>('/api/devices')
  return response.data.items
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
