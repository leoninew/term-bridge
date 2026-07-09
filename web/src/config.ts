export type LocalMode = 'local' | 'cloud' | 'hybrid'

export interface CloudOAuthConfig {
  clientId: string
  redirectUrl: string
  scopes: string[]
}

export interface RuntimeLocalConfig {
  mode: LocalMode
  publicUrl: string
  apiBaseUrl: string
  cloudOAuth: CloudOAuthConfig
}

export interface RuntimeCloudConfig {
  publicUrl: string
  apiBaseUrl: string
}

export interface RuntimeConfig {
  local: RuntimeLocalConfig
  cloud: RuntimeCloudConfig
}

export type BrowserRuntimeLocalConfig = Partial<{
  mode: LocalMode
  publicUrl: string
  apiBaseUrl: string
  cloudOAuth: Partial<CloudOAuthConfig>
}>

export type BrowserRuntimeCloudConfig = Partial<RuntimeCloudConfig>

export type BrowserRuntimeConfig = Partial<{
  local: BrowserRuntimeLocalConfig
  cloud: BrowserRuntimeCloudConfig
}>
