/// <reference types="vite/client" />

declare module '*.css'

interface ImportMetaEnv {
  readonly TERMBRIDGE_AGENT__API_BASE_URL?: string
  readonly TERMBRIDGE_AGENT__PUBLIC_URL?: string
  readonly TERMBRIDGE_AGENT__MODE?: string
  readonly TERMBRIDGE_AGENT__OAUTH__CLIENT_ID?: string
  readonly TERMBRIDGE_AGENT__OAUTH__REDIRECT_URL?: string
  readonly TERMBRIDGE_AGENT__OAUTH__SCOPES?: string
  readonly TERMBRIDGE_CLOUD__PUBLIC_URL?: string
  readonly TERMBRIDGE_CLOUD__API_BASE_URL?: string
}
