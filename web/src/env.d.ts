/// <reference types="vite/client" />

declare module '*.css'

interface ImportMetaEnv {
  readonly TERMBRIDGE_LOCAL__VERSION?: string
  readonly TERMBRIDGE_LOCAL__API_BASE_URL?: string
  readonly TERMBRIDGE_LOCAL__PUBLIC_URL?: string
  readonly TERMBRIDGE_LOCAL__MODE?: string
  readonly TERMBRIDGE_LOCAL__OAUTH__CLIENT_ID?: string
  readonly TERMBRIDGE_LOCAL__OAUTH__REDIRECT_URL?: string
  readonly TERMBRIDGE_LOCAL__OAUTH__SCOPES?: string
  readonly TERMBRIDGE_CLOUD__PUBLIC_URL?: string
  readonly TERMBRIDGE_CLOUD__API_BASE_URL?: string
  readonly TERMBRIDGE_CLOUD__EXTERNAL_AUTH_PROVIDER_IDS?: string
}
