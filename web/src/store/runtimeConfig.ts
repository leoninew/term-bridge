import { reactive } from 'vue'
import { defineStore } from 'pinia'
import type { LocalMode, BrowserRuntimeConfig, CloudOAuthConfig, RuntimeConfig } from '../config'
import { readSessionStorageValue, writeSessionStorageValue } from './storage'

declare global {
  interface Window {
    __CONFIG__?: BrowserRuntimeConfig
  }
}

export type RuntimeMode = 'local' | 'cloud'

const ACTIVE_MODE_KEY = 'termbridge.active_mode'

export const useRuntimeConfigStore = defineStore('runtimeConfig', () => {
  const config = parseRuntimeConfig(runtimeConfigSource())
  const view = reactive({ mode: initialViewMode(config.local.mode) })

  function switchMode(mode: RuntimeMode) {
    if (config.local.mode !== 'hybrid' && config.local.mode !== mode) {
      return false
    }
    view.mode = mode
    writeSessionStorageValue(ACTIVE_MODE_KEY, mode)
    return true
  }

  return {
    config,
    view,
    switchMode,
  }
})

type RuntimeConfigSource = {
  name: string
  value: BrowserRuntimeConfig
}

function runtimeConfigSource(): RuntimeConfigSource {
  if (typeof window !== 'undefined' && injectedRuntimeConfig(window.__CONFIG__)) {
    return { name: 'window.__CONFIG__', value: window.__CONFIG__ }
  }
  return {
    name: 'import.meta.env',
    value: {
      version: import.meta.env.DEV ? import.meta.env.TERMBRIDGE_LOCAL__VERSION : undefined,
      local: {
        mode: import.meta.env.TERMBRIDGE_LOCAL__MODE as LocalMode | undefined,
        publicUrl: import.meta.env.TERMBRIDGE_LOCAL__PUBLIC_URL,
        apiBaseUrl: import.meta.env.TERMBRIDGE_LOCAL__API_BASE_URL,
        cloudOAuth: {
          clientId: import.meta.env.TERMBRIDGE_LOCAL__OAUTH__CLIENT_ID,
          redirectUrl: import.meta.env.TERMBRIDGE_LOCAL__OAUTH__REDIRECT_URL,
          scopes: splitScopes(import.meta.env.TERMBRIDGE_LOCAL__OAUTH__SCOPES),
        },
      },
      cloud: {
        publicUrl: import.meta.env.TERMBRIDGE_CLOUD__PUBLIC_URL,
        apiBaseUrl: import.meta.env.TERMBRIDGE_CLOUD__API_BASE_URL,
      },
    },
  }
}

function injectedRuntimeConfig(
  config: BrowserRuntimeConfig | undefined,
): config is BrowserRuntimeConfig {
  return !!config?.local || !!config?.cloud
}

function parseRuntimeConfig(source: RuntimeConfigSource): RuntimeConfig {
  const errors: string[] = []
  const local = source.value.local
  const cloud = source.value.cloud

  if (!local) {
    errors.push('local is required')
  }
  if (!cloud) {
    errors.push('cloud is required')
  }

  const config = {
    version: optionalString(source.value.version),
    local: {
      mode: parseLocalMode(local?.mode, errors),
      publicUrl: requiredHTTPURL('local.publicUrl', local?.publicUrl, errors),
      apiBaseUrl: requiredApiBaseUrl('local.apiBaseUrl', local?.apiBaseUrl, errors),
      cloudOAuth: parseCloudOAuth(local?.cloudOAuth, errors),
    },
    cloud: {
      publicUrl: requiredHTTPURL('cloud.publicUrl', cloud?.publicUrl, errors),
      apiBaseUrl: requiredApiBaseUrl('cloud.apiBaseUrl', cloud?.apiBaseUrl, errors),
    },
  }

  if (errors.length > 0) {
    throw new Error(`Invalid runtime config from ${source.name}: ${errors.join('; ')}`)
  }

  return config
}

function parseLocalMode(value: string | undefined, errors: string[]): LocalMode {
  const mode = requiredString('local.mode', value, errors)
  if (mode === 'local' || mode === 'cloud' || mode === 'hybrid') {
    return mode
  }
  errors.push('local.mode must be local, cloud, or hybrid')
  return 'local'
}

function parseCloudOAuth(
  value: Partial<CloudOAuthConfig> | undefined,
  errors: string[],
): CloudOAuthConfig {
  if (!value) {
    errors.push('local.cloudOAuth is required')
    return { clientId: '', redirectUrl: '', scopes: [] }
  }
  const clientId = requiredString('local.cloudOAuth.clientId', value.clientId, errors)
  const redirectUrl = requiredHTTPURL('local.cloudOAuth.redirectUrl', value.redirectUrl, errors)
  const scopes = value.scopes ?? []
  if (scopes.length === 0) {
    errors.push('local.cloudOAuth.scopes is required')
  }
  return { clientId, redirectUrl, scopes }
}

function optionalString(value: string | undefined): string {
  return value?.trim() ?? ''
}

function requiredString(key: string, value: string | undefined, errors: string[]): string {
  const trimmed = optionalString(value)
  if (!trimmed) {
    errors.push(`${key} is required`)
  }
  return trimmed
}

function requiredHTTPURL(key: string, value: string | undefined, errors: string[]): string {
  const trimmed = requiredString(key, value, errors).replace(/\/+$/, '')
  if (!trimmed) {
    return ''
  }
  try {
    const url = new URL(trimmed)
    if (url.protocol !== 'http:' && url.protocol !== 'https:') {
      errors.push(`${key} must use http or https`)
    }
  } catch {
    errors.push(`${key} must be an absolute URL`)
  }
  return trimmed
}

function requiredApiBaseUrl(key: string, value: string | undefined, errors: string[]): string {
  const trimmed = requiredString(key, value, errors).replace(/\/+$/, '')
  if (!trimmed) {
    return ''
  }
  if (trimmed.startsWith('//')) {
    errors.push(`${key} must not be a protocol-relative URL`)
    return trimmed
  }
  if (trimmed.startsWith('/')) {
    return trimmed
  }
  try {
    const url = new URL(trimmed)
    if (url.protocol !== 'http:' && url.protocol !== 'https:') {
      errors.push(`${key} must use http or https`)
    }
  } catch {
    errors.push(`${key} must be an absolute URL or same-origin path`)
  }
  return trimmed
}

function splitScopes(value: string | undefined): string[] {
  return value
    ? value
        .split(',')
        .map((scope) => scope.trim())
        .filter(Boolean)
    : []
}

function initialViewMode(configuredMode: LocalMode): RuntimeMode {
  if (configuredMode === 'local' || configuredMode === 'cloud') {
    return configuredMode
  }
  const saved = readSessionStorageValue(ACTIVE_MODE_KEY)
  return saved === 'cloud' ? 'cloud' : 'local'
}
