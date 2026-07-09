export type AgentMode = 'agent' | 'cloud' | 'hybrid'

export interface CloudOAuthConfig {
  clientId: string
  redirectUrl: string
  scopes: string[]
}

export interface RuntimeAgentConfig {
  mode: AgentMode
  publicUrl: string
  apiBaseUrl: string
  cloudOAuth: CloudOAuthConfig
}

export interface RuntimeCloudConfig {
  publicUrl: string
  apiBaseUrl: string
}

export interface RuntimeConfig {
  agent: RuntimeAgentConfig
  cloud: RuntimeCloudConfig
}

export type BrowserRuntimeAgentConfig = Partial<{
  mode: AgentMode
  publicUrl: string
  apiBaseUrl: string
  cloudOAuth: Partial<CloudOAuthConfig>
}>

export type BrowserRuntimeCloudConfig = Partial<RuntimeCloudConfig>

export type BrowserRuntimeConfig = Partial<{
  agent: BrowserRuntimeAgentConfig
  cloud: BrowserRuntimeCloudConfig
}>
