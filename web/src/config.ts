export type ApiTarget = 'agent' | 'cloud'

export interface RuntimeConfig {
  apiBaseUrl?: string
  agentApiBaseUrl?: string
  cloudApiBaseUrl?: string
}

declare global {
  interface Window {
    __CONFIG__?: RuntimeConfig
  }
}

export function getApiBaseUrl(target: ApiTarget = 'agent'): string {
  const runtimeConfig = typeof window !== 'undefined' ? window.__CONFIG__ : undefined
  if (target === 'cloud') {
    return normalizeBaseUrl(
      runtimeConfig?.cloudApiBaseUrl ||
        import.meta.env.VITE_CLOUD_API_BASE_URL ||
        runtimeConfig?.apiBaseUrl ||
        import.meta.env.VITE_API_BASE_URL ||
        '',
    )
  }
  return normalizeBaseUrl(
    runtimeConfig?.agentApiBaseUrl ||
      runtimeConfig?.apiBaseUrl ||
      import.meta.env.VITE_AGENT_API_BASE_URL ||
      import.meta.env.VITE_API_BASE_URL ||
      '',
  )
}

export function buildApiUrl(path: string, target: ApiTarget = 'agent'): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const apiBaseUrl = getApiBaseUrl(target)
  return apiBaseUrl ? `${apiBaseUrl}${normalizedPath}` : normalizedPath
}

export function buildApiWebSocketUrl(path: string, target: ApiTarget = 'agent'): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const apiBaseUrl = getApiBaseUrl(target)
  if (!apiBaseUrl) {
    return normalizedPath
  }
  if (apiBaseUrl.startsWith('/')) {
    return `${apiBaseUrl}${normalizedPath}`
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
  get agentApiBaseUrl() {
    return getApiBaseUrl('agent')
  },
  get cloudApiBaseUrl() {
    return getApiBaseUrl('cloud')
  },
}
