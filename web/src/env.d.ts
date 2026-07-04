/// <reference types="vite/client" />

declare module '*.css'

interface ImportMetaEnv {
  readonly VITE_AGENT_API_BASE_URL?: string
  readonly VITE_CLOUD_API_BASE_URL?: string
  readonly VITE_FRONTEND_MODE?: string
}
