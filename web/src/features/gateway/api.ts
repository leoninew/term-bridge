import { apiClient } from '../api/client'

export type DeviceSummary = {
  id: string
  name: string
  online: boolean
  connected_at: string
  last_seen: string
}

export type AuthMeResp = {
  authenticated: boolean
  username: string
}

export type AuthLoginReq = {
  username: string
  password: string
}

export type TokenResp = {
  access_token: string
  token_type: string
}

export async function authMe(): Promise<AuthMeResp> {
  try {
    const response = await apiClient.get<AuthMeResp>('/api/me')
    return response.data
  } catch (err) {
    if (isUnauthorizedApiError(err)) {
      return { authenticated: false, username: '' }
    }
    throw err
  }
}

export async function authLogin(username: string, password: string): Promise<TokenResp> {
  const request: AuthLoginReq = { username, password }
  const response = await apiClient.post<TokenResp>('/api/login', request)
  return response.data
}

export async function authLogout(): Promise<void> {
  await apiClient.post('/api/logout')
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
