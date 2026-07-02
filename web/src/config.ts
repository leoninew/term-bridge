export interface RuntimeConfig {
  apiBaseUrl?: string
}

declare global {
  interface Window {
    __CONFIG__?: RuntimeConfig
  }
}

export function getApiBaseUrl(): string {
  const runtimeBaseUrl = typeof window !== 'undefined' ? window.__CONFIG__?.apiBaseUrl : undefined
  return normalizeBaseUrl(runtimeBaseUrl || import.meta.env.VITE_API_BASE_URL || '')
}

export function buildApiUrl(path: string): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const apiBaseUrl = getApiBaseUrl()
  return apiBaseUrl ? `${apiBaseUrl}${normalizedPath}` : normalizedPath
}

export function buildApiWebSocketUrl(path: string): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const apiBaseUrl = getApiBaseUrl()
  if (!apiBaseUrl) {
    return normalizedPath
  }
  const url = new URL(`${apiBaseUrl}${normalizedPath}`)
  if (url.protocol === 'https:') {
    url.protocol = 'wss:'
  } else if (url.protocol === 'http:') {
    url.protocol = 'ws:'
  }
  return url.toString()
}

function normalizeBaseUrl(value: string): string {
  return value.trim().replace(/\/+$/, '')
}

export const runtimeConfig = {
  get apiBaseUrl() {
    return getApiBaseUrl()
  },
}
