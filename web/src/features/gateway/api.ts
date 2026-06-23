import { responseError } from '../sessions/api'

export type DeviceSummary = {
  id: string
  name: string
  online: boolean
  connected_at: string
  last_seen: string
}

export async function authMe(): Promise<{ authenticated: boolean; username: string }> {
  const response = await fetch('/api/me')
  if (response.status === 401) {
    return { authenticated: false, username: '' }
  }
  if (!response.ok) {
    throw new Error(await responseError('Auth check failed', response))
  }
  return (await response.json()) as { authenticated: boolean; username: string }
}

export async function authLogin(username: string, password: string): Promise<void> {
  const response = await fetch('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  if (!response.ok) {
    throw new Error(await responseError('Login failed', response))
  }
}

export async function authLogout(): Promise<void> {
  const response = await fetch('/api/logout', { method: 'POST' })
  if (!response.ok) {
    throw new Error(await responseError('Logout failed', response))
  }
}

export async function listDevices(): Promise<DeviceSummary[]> {
  const response = await fetch('/api/devices')
  if (!response.ok) {
    throw new Error(await responseError('List devices failed', response))
  }
  return ((await response.json()) as DeviceSummary[] | null) ?? []
}
