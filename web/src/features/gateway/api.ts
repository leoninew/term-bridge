import { apiClient } from '../api/client'

export type DeviceSummary = {
  id: string
  name: string
  online: boolean
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
  mode: 'local' | 'remote'
  providers: string[]
  password_reset_enabled: boolean
  email_verification_enabled: boolean
}

export type AuthMeResp = {
  authenticated: boolean
  user?: UserInfo
  capabilities?: AuthCapabilities
}

export type TokenResp = {
  access_token: string
  token_type: string
}

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
  const response = await apiClient.get<{ auth_url: string }>('/api/auth/google')
  return response.data.auth_url
}

export async function authGoogleCallback(code: string, state: string): Promise<TokenResp> {
  const response = await apiClient.post<TokenResp>('/api/auth/google/callback', { code, state })
  return response.data
}

export async function listDevices(): Promise<DeviceSummary[]> {
  const response = await apiClient.get<DeviceSummary[] | null>('/api/devices')
  return response.data ?? []
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
