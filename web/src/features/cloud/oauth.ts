import { buildCloudPageUrl, runtimeConfig } from '../../config'
import { readStorageValue, removeStorageValue, writeStorageValue } from '../../store/storage'

const cloudOAuthStateKey = 'termbridge.cloud.oauth2.state'
const cloudOAuthRedirectKey = 'termbridge.cloud.oauth2.redirect'

export type CloudOAuthConfig = {
  clientId: string
  redirectUrl: string
  scopes: string[]
}

export function cloudOAuthConfigured(): boolean {
  const config = runtimeConfig.cloudOAuth
  return Boolean(config?.clientId && config.redirectUrl && isAbsoluteCloudApiBaseUrl())
}

export function startCloudOAuth(postAuthRedirect: string) {
  const config = requireCloudOAuthConfig()
  const state = randomState()
  writeStorageValue(cloudOAuthStateKey, state)
  writeStorageValue(cloudOAuthRedirectKey, safeLocalRedirect(postAuthRedirect) || '/agent/dashboard')
  window.location.href = cloudAuthorizeUrl(config, state)
}

export function assertCloudOAuthState(state: string) {
  const storedState = readStorageValue(cloudOAuthStateKey)
  removeStorageValue(cloudOAuthStateKey)
  if (!storedState || state !== storedState) {
    throw new Error('Cloud OAuth state mismatch')
  }
}

export function consumeCloudOAuthRedirect(): string {
  const redirect = safeLocalRedirect(readStorageValue(cloudOAuthRedirectKey))
  removeStorageValue(cloudOAuthRedirectKey)
  return redirect || '/agent/dashboard'
}

function cloudAuthorizeUrl(config: CloudOAuthConfig, state: string): string {
  const url = new URL(buildCloudPageUrl('/oauth2/authorize'), window.location.origin)
  url.searchParams.set('response_type', 'code')
  url.searchParams.set('client_id', config.clientId)
  url.searchParams.set('redirect_uri', config.redirectUrl)
  if (config.scopes.length > 0) {
    url.searchParams.set('scope', config.scopes.join(' '))
  }
  url.searchParams.set('state', state)
  return url.toString()
}

function requireCloudOAuthConfig(): CloudOAuthConfig {
  const config = runtimeConfig.cloudOAuth
  const clientId = config?.clientId
  const redirectUrl = config?.redirectUrl
  if (!clientId || !redirectUrl || !isAbsoluteCloudApiBaseUrl()) {
    throw new Error('Cloud OAuth is not configured')
  }
  return { clientId, redirectUrl, scopes: config.scopes ?? [] }
}

function isAbsoluteCloudApiBaseUrl(): boolean {
  return /^https?:\/\//.test(runtimeConfig.cloudApiBaseUrl)
}

function randomState(): string {
  const bytes = new Uint8Array(32)
  if (typeof crypto === 'undefined' || typeof crypto.getRandomValues !== 'function') {
    throw new Error('Browser crypto is unavailable')
  }
  crypto.getRandomValues(bytes)
  return base64UrlEncode(bytes)
}

function base64UrlEncode(bytes: Uint8Array): string {
  let value = ''
  for (const byte of bytes) {
    value += String.fromCharCode(byte)
  }
  return btoa(value).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}

function safeLocalRedirect(value: string | null): string {
  return value && value.startsWith('/') && !value.startsWith('//') ? value : ''
}
