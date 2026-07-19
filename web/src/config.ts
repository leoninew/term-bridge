export type LocalMode = 'local' | 'cloud' | 'hybrid'

export interface CloudOAuthConfig {
  clientId: string
  redirectUrl: string
  scopes: string[]
}

export interface RuntimeLocalConfig {
  mode: LocalMode
  publicUrl: string
  apiBasePath: string
  cloudOAuth: CloudOAuthConfig
}

export interface RuntimeCloudConfig {
  publicUrl: string
  apiBaseUrl: string
  externalAuthProviderIds: string[]
}

export interface RuntimeTerminalKeepAliveConfig {
  maxHotTerminals: number
  disposeDelayMs: number
}

export interface RuntimeTerminalConfig {
  keepAlive: RuntimeTerminalKeepAliveConfig
}

export interface RuntimeConfig {
  version: string
  local: RuntimeLocalConfig
  cloud: RuntimeCloudConfig
  terminal: RuntimeTerminalConfig
}

export type BrowserRuntimeLocalConfig = Partial<{
  mode: LocalMode
  publicUrl: string
  apiBasePath: string
  cloudOAuth: Partial<CloudOAuthConfig>
}>

export type BrowserRuntimeCloudConfig = Partial<RuntimeCloudConfig>

export type BrowserRuntimeTerminalConfig = Partial<{
  keepAlive: Partial<RuntimeTerminalKeepAliveConfig>
}>

export type BrowserRuntimeConfig = Partial<{
  version: string
  local: BrowserRuntimeLocalConfig
  cloud: BrowserRuntimeCloudConfig
  terminal: BrowserRuntimeTerminalConfig
}>