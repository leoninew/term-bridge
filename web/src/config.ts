export type ApiTarget = 'agent' | 'cloud'
export type FrontendMode = 'agent' | 'cloud' | 'hybrid'

export interface RuntimeConfig {
  agentApiBaseUrl?: string
  cloudApiBaseUrl?: string
  cloudOAuth?: CloudOAuthRuntimeConfig
  frontendMode?: FrontendMode
}

export interface CloudOAuthRuntimeConfig {
  clientId?: string
  redirectUrl?: string
  scopes?: string[]
}

const defaultApiBaseUrls: Record<ApiTarget, string> = {
  agent: '/agent-api',
  cloud: '/cloud-api',
}

declare global {
  interface Window {
    __CONFIG__?: RuntimeConfig
  }
}

export function getFrontendMode(): FrontendMode {
  const runtimeConfig = typeof window !== 'undefined' ? window.__CONFIG__ : undefined
  const mode = runtimeConfig?.frontendMode || import.meta.env.TERMBRIDGE_FRONTEND_MODE || 'agent'
  return isFrontendMode(mode) ? mode : 'agent'
}

export function getApiBaseUrl(target: ApiTarget = 'agent'): string {
  const runtimeConfig = typeof window !== 'undefined' ? window.__CONFIG__ : undefined
  const value =
    target === 'cloud'
      ? runtimeConfig?.cloudApiBaseUrl || import.meta.env.TERMBRIDGE_CLOUD_API_BASE_URL
      : runtimeConfig?.agentApiBaseUrl || import.meta.env.TERMBRIDGE_AGENT_API_BASE_URL
  return normalizeBaseUrl(value || defaultApiBaseUrls[target])
}

export function buildApiUrl(path: string, target: ApiTarget = 'agent'): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  return `${getApiBaseUrl(target)}${normalizedPath}`
}

export function buildCloudPageUrl(path: string): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const baseUrl = getApiBaseUrl('cloud')
  if (baseUrl.startsWith('/')) {
    return normalizedPath
  }
  const url = new URL(baseUrl)
  const apiPath = url.pathname.replace(/\/+$/, '')
  if (apiPath.endsWith('/cloud-api')) {
    url.pathname = `${apiPath.slice(0, -'/cloud-api'.length)}${normalizedPath}`
  } else {
    url.pathname = normalizedPath
  }
  url.search = ''
  url.hash = ''
  return url.toString()
}

export function buildApiWebSocketUrl(path: string, target: ApiTarget = 'agent'): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const targetBaseUrl = getApiBaseUrl(target)
  if (targetBaseUrl.startsWith('/')) {
    return `${targetBaseUrl}${normalizedPath}`
  }
  const url = new URL(`${targetBaseUrl}${normalizedPath}`)
  if (url.protocol === 'https:') {
    url.protocol = 'wss:'
  } else if (url.protocol === 'http:') {
    url.protocol = 'ws:'
  }
  return url.toString()
}

function isFrontendMode(value: string): value is FrontendMode {
  return value === 'agent' || value === 'cloud' || value === 'hybrid'
}

function normalizeBaseUrl(value: string): string {
  return value.trim().replace(/\/+$/, '')
}

function splitScopes(value: string | undefined): string[] {
  return value
    ? value
        .split(',')
        .map((scope) => scope.trim())
        .filter(Boolean)
    : []
}

export const runtimeConfig = {
  get frontendMode() {
    return getFrontendMode()
  },
  get agentApiBaseUrl() {
    return getApiBaseUrl('agent')
  },
  get cloudApiBaseUrl() {
    return getApiBaseUrl('cloud')
  },
  get cloudOAuth(): CloudOAuthRuntimeConfig | undefined {
    const runtimeCloudOAuth = typeof window !== 'undefined' ? window.__CONFIG__?.cloudOAuth : undefined
    if (runtimeCloudOAuth) {
      return runtimeCloudOAuth
    }
    const clientId = import.meta.env.TERMBRIDGE_CLOUD_OAUTH_CLIENT_ID
    const redirectUrl = import.meta.env.TERMBRIDGE_CLOUD_OAUTH_REDIRECT_URL
    if (!clientId && !redirectUrl) {
      return undefined
    }
    return {
      clientId,
      redirectUrl,
      scopes: splitScopes(import.meta.env.TERMBRIDGE_CLOUD_OAUTH_SCOPES),
    }
  },
}
