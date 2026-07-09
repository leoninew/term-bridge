import type { CloudSessionSummary } from '../../gen/proto/termbridge/cloud/v1/session'
import { useRuntimeConfigStore } from '../../store/runtimeConfig'
import {
  readLocalStorageValue,
  readSessionStorageValue,
  removeLocalStorageValue,
  removeSessionStorageValue,
  writeLocalStorageValue,
  writeSessionStorageValue,
} from '../../store/storage'

const cloudOAuthStateKey = 'termbridge.cloud.oauth2.state'
const cloudOAuthRedirectKey = 'termbridge.cloud.oauth2.redirect'
const cloudConnectionConnectedAtKey = 'termbridge.cloud.connection.connected_at'

export function cloudOAuthConfigured(): boolean {
  const config = useRuntimeConfigStore().config.local.cloudOAuth
  return !!config
}

export function startCloudOAuth(postAuthRedirect: string) {
  const config = useRuntimeConfigStore().config.local.cloudOAuth
  const state = randomState()
  writeLocalStorageValue(cloudOAuthStateKey, state)
  writeLocalStorageValue(cloudOAuthRedirectKey, safeLocalRedirect(postAuthRedirect) || '/')
  window.location.href = cloudAuthorizeUrl(config, state)
}

export function assertCloudOAuthState(state: string) {
  const storedState = readLocalStorageValue(cloudOAuthStateKey)
  removeLocalStorageValue(cloudOAuthStateKey)
  if (!storedState || state !== storedState) {
    throw new Error('Cloud OAuth state mismatch')
  }
}

export function consumeCloudOAuthRedirect(): string {
  const redirect = safeLocalRedirect(readLocalStorageValue(cloudOAuthRedirectKey))
  removeLocalStorageValue(cloudOAuthRedirectKey)
  return redirect || '/'
}

export function applyCloudConnectionTime(
  summary: CloudSessionSummary | null,
): CloudSessionSummary | null {
  if (!summary) {
    return null
  }
  return {
    ...summary,
    connected_at: readSessionStorageValue(cloudConnectionConnectedAtKey) ?? summary.connected_at,
  }
}

export function markCloudConnected(
  summary: CloudSessionSummary | undefined,
): CloudSessionSummary | null {
  if (!summary) {
    return null
  }
  const connectedAt = new Date().toISOString()
  writeSessionStorageValue(cloudConnectionConnectedAtKey, connectedAt)
  return { ...summary, connected_at: connectedAt }
}

export function clearCloudConnectionTime() {
  removeSessionStorageValue(cloudConnectionConnectedAtKey)
}

export function hasCloudConnectionTime(): boolean {
  return readSessionStorageValue(cloudConnectionConnectedAtKey) !== null
}

function cloudAuthorizeUrl(
  config: { clientId: string; redirectUrl: string; scopes: string[] },
  state: string,
): string {
  const runtimeConfig = useRuntimeConfigStore().config
  const url = new URL('/oauth2/authorize', runtimeConfig.cloud.publicUrl)
  url.searchParams.set('response_type', 'code')
  url.searchParams.set('client_id', config.clientId)
  url.searchParams.set('redirect_uri', config.redirectUrl)
  url.searchParams.set('scope', config.scopes.join(' '))
  url.searchParams.set('state', state)
  return url.toString()
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
