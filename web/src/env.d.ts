/// <reference types="vite/client" />

declare module '*.css'

interface ImportMetaEnv {
  readonly TERMBRIDGE_AGENT_API_BASE_URL?: string
  readonly TERMBRIDGE_CLOUD_API_BASE_URL?: string
  readonly TERMBRIDGE_FRONTEND_MODE?: string
  readonly TERMBRIDGE_CLOUD_OAUTH_CLIENT_ID?: string
  readonly TERMBRIDGE_CLOUD_OAUTH_REDIRECT_URL?: string
  readonly TERMBRIDGE_CLOUD_OAUTH_SCOPES?: string
}
