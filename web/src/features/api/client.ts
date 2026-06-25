import axios, {
  AxiosError,
  AxiosHeaders,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from 'axios'
import type { ApiErrorResp } from '../../protocol/terminal'
import { useGatewayStore } from '../../store/gateway'
import { router } from '../../router'

export const requestIdHeader = 'X-Request-ID'

export class ApiClientError extends Error {
  readonly status: number
  readonly code: string
  readonly requestId: string
  readonly details?: unknown

  constructor(status: number, response: ApiErrorResp) {
    super(
      `API request failed (${status} ${response.code}, requestId: ${response.requestId}): ${response.error}`,
    )
    this.name = 'ApiClientError'
    this.status = status
    this.code = response.code
    this.requestId = response.requestId
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

export const apiClient = axios.create({
  baseURL: '',
})

apiClient.interceptors.request.use((config) => {
  ensureRequestId(config)
  const gateway = useGatewayStore()
  if (gateway.token) {
    const headers = AxiosHeaders.from(config.headers)
    headers.set('Authorization', `Bearer ${gateway.token}`)
    config.headers = headers
  }
  return config
})

apiClient.interceptors.response.use(
  (response) => response,
  (error: AxiosError) => {
    if (!error.response) {
      return Promise.reject(error)
    }
    if (error.response.status === 401) {
      const gateway = useGatewayStore()
      gateway.clearToken()
      if (router.currentRoute.value.name !== 'login') {
        router.push({ name: 'login' })
      }
    }
    return Promise.reject(errorFromResponse(error.response))
  },
)

export function ensureRequestId(config: InternalAxiosRequestConfig): void {
  const headers = AxiosHeaders.from(config.headers)
  if (!headers.has(requestIdHeader)) {
    headers.set(requestIdHeader, newRequestId())
  }
  config.headers = headers
}

export function errorFromResponse(response: AxiosResponse): Error {
  const body = parseErrorBody(response.data)
  if (!isApiErrorResp(body)) {
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

function isApiErrorResp(value: unknown): value is ApiErrorResp {
  if (!value || typeof value !== 'object') {
    return false
  }
  const candidate = value as Partial<ApiErrorResp>
  return (
    typeof candidate.code === 'string' &&
    typeof candidate.error === 'string' &&
    typeof candidate.requestId === 'string'
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
