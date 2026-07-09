import { reactive } from 'vue'
import { defineStore } from 'pinia'
import type { AgentMode, BrowserRuntimeConfig, CloudOAuthConfig, RuntimeConfig } from '../config'
import { readSessionStorageValue, writeSessionStorageValue } from './storage'

declare global {
  interface Window {
    __CONFIG__?: BrowserRuntimeConfig
  }
}

export type RuntimeMode = 'agent' | 'cloud'

const ACTIVE_MODE_KEY = 'termbridge.active_mode'

export const useRuntimeConfigStore = defineStore('runtimeConfig', () => {
  const config = parseRuntimeConfig(runtimeConfigSource())
  const view = reactive({ mode: initialViewMode(config.agent.mode) })

  function switchMode(mode: RuntimeMode) {
    if (config.agent.mode !== 'hybrid' && config.agent.mode !== mode) {
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
      agent: {
        mode: import.meta.env.TERMBRIDGE_AGENT__MODE as AgentMode | undefined,
        publicUrl: import.meta.env.TERMBRIDGE_AGENT__PUBLIC_URL,
        apiBaseUrl: import.meta.env.TERMBRIDGE_AGENT__API_BASE_URL,
        cloudOAuth: {
          clientId: import.meta.env.TERMBRIDGE_AGENT__OAUTH__CLIENT_ID,
          redirectUrl: import.meta.env.TERMBRIDGE_AGENT__OAUTH__REDIRECT_URL,
          scopes: splitScopes(import.meta.env.TERMBRIDGE_AGENT__OAUTH__SCOPES),
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
  return !!config?.agent || !!config?.cloud
}

function parseRuntimeConfig(source: RuntimeConfigSource): RuntimeConfig {
  const errors: string[] = []
  const agent = source.value.agent
  const cloud = source.value.cloud

  if (!agent) {
    errors.push('agent is required')
  }
  if (!cloud) {
    errors.push('cloud is required')
  }

  const config = {
    agent: {
      mode: parseAgentMode(agent?.mode, errors),
      publicUrl: requiredHTTPURL('agent.publicUrl', agent?.publicUrl, errors),
      apiBaseUrl: requiredApiBaseUrl('agent.apiBaseUrl', agent?.apiBaseUrl, errors),
      cloudOAuth: parseCloudOAuth(agent?.cloudOAuth, errors),
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

function parseAgentMode(value: string | undefined, errors: string[]): AgentMode {
  const mode = requiredString('agent.mode', value, errors)
  if (mode === 'agent' || mode === 'cloud' || mode === 'hybrid') {
    return mode
  }
  errors.push('agent.mode must be agent, cloud, or hybrid')
  return 'agent'
}

function parseCloudOAuth(
  value: Partial<CloudOAuthConfig> | undefined,
  errors: string[],
): CloudOAuthConfig {
  if (!value) {
    errors.push('agent.cloudOAuth is required')
    return { clientId: '', redirectUrl: '', scopes: [] }
  }
  const clientId = requiredString('agent.cloudOAuth.clientId', value.clientId, errors)
  const redirectUrl = requiredHTTPURL('agent.cloudOAuth.redirectUrl', value.redirectUrl, errors)
  const scopes = value.scopes ?? []
  if (scopes.length === 0) {
    errors.push('agent.cloudOAuth.scopes is required')
  }
  return { clientId, redirectUrl, scopes }
}

function requiredString(key: string, value: string | undefined, errors: string[]): string {
  const trimmed = value?.trim() ?? ''
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

function initialViewMode(configuredMode: AgentMode): RuntimeMode {
  if (configuredMode === 'agent' || configuredMode === 'cloud') {
    return configuredMode
  }
  const saved = readSessionStorageValue(ACTIVE_MODE_KEY)
  return saved === 'cloud' ? 'cloud' : 'agent'
}
