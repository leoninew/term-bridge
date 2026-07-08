import axios, {
  AxiosError,
  AxiosHeaders,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from 'axios'
import type { ErrorResp } from '../../gen/proto/termbridge/shared/v1/common'
import { runtimeConfig, type ApiTarget } from '../../config'
import { useAppModeStore } from '../../store/appMode'
import { useGatewayStore } from '../../store/gateway'
import { router } from '../../router'

export const requestIdHeader = 'X-Request-ID'

export class ApiClientError extends Error {
  readonly status: number
  readonly code: string
  readonly requestId: string
  readonly details?: unknown

  constructor(status: number, response: ErrorResp) {
    const requestId = response.request_id || ''
    super(
      `API request failed (${status} ${response.code}, requestId: ${requestId}): ${response.error}`,
    )
    this.name = 'ApiClientError'
    this.status = status
    this.code = response.code
    this.requestId = requestId
    this.details = response.details
  }
}

export class ApiContractMismatchError extends Error {
  readonly status?: number

  constructor(status?: number) {
    super(status ? `API error contract mismatch (${status})` : 'API error contract mismatch')
    this.name = 'ApiContractMismatchError'
    this.status = status
  }
}

export const apiClient = axios.create()
export const agentApiClient = apiClient
export const cloudApiClient = axios.create()

configureApiClient(apiClient, 'agent')
configureApiClient(cloudApiClient, 'cloud')

function configureApiClient(client: typeof apiClient, target: ApiTarget): void {
  client.interceptors.request.use((cfg) => {
    cfg.baseURL = target === 'cloud' ? runtimeConfig.cloudApiBaseUrl : runtimeConfig.agentApiBaseUrl
    return cfg
  })

  client.interceptors.request.use((cfg) => {
    ensureRequestId(cfg)
    const gateway = useGatewayStore()
    const token = gateway.tokenForTarget(target)
    if (token) {
      const headers = AxiosHeaders.from(cfg.headers)
      headers.set('Authorization', `Bearer ${token}`)
      cfg.headers = headers
    }
    return cfg
  })

  client.interceptors.response.use(
    (response) => response,
    (error: AxiosError) => {
      if (!error.response) {
        return Promise.reject(error)
      }
      if (error.response.status === 401) {
        const gateway = useGatewayStore()
        const appMode = useAppModeStore()
        gateway.clearTokenForTarget(target)
        if (
          target === 'cloud' &&
          appMode.allowsMode('cloud') &&
          router.currentRoute.value.name !== 'cloud-login'
        ) {
          router.push({
            name: 'cloud-login',
            query: { redirect: router.currentRoute.value.fullPath },
          })
        }
      }
      return Promise.reject(errorFromResponse(error.response))
    },
  )
}

export function ensureRequestId(config: InternalAxiosRequestConfig): void {
  const headers = AxiosHeaders.from(config.headers)
  if (!headers.has(requestIdHeader)) {
    headers.set(requestIdHeader, newRequestId())
  }
  config.headers = headers
}

export function errorFromResponse(response: AxiosResponse): Error {
  const body = parseErrorBody(response.data)
  if (!isErrorResp(body)) {
    return new ApiContractMismatchError(response.status)
  }
  return new ApiClientError(response.status, body)
}

function parseErrorBody(data: unknown): unknown {
  if (typeof data !== 'string') {
    return data
  }
  const trimmed = data.trim()
  if (!trimmed) {
    return null
  }
  try {
    return JSON.parse(trimmed) as unknown
  } catch {
    return null
  }
}

function isErrorResp(value: unknown): value is ErrorResp {
  if (!value || typeof value !== 'object') {
    return false
  }
  const candidate = value as Partial<ErrorResp>
  return (
    typeof candidate.code === 'string' &&
    typeof candidate.error === 'string' &&
    typeof candidate.request_id === 'string'
  )
}

function newRequestId(): string {
  const randomBytes = new Uint8Array(16)
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    crypto.getRandomValues(randomBytes)
    return `req_${Array.from(randomBytes, (value) => value.toString(16).padStart(2, '0')).join('')}`
  }
  return `req_${Date.now().toString(36)}`
}
