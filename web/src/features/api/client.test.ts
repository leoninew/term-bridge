import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AxiosHeaders, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'
import { createPinia, setActivePinia } from 'pinia'
import {
  ApiClientError,
  ApiContractMismatchError,
  apiClient,
  cloudApiClient,
  ensureRequestId,
  errorFromResponse,
  requestIdHeader,
} from './client'

vi.mock('../../router', () => ({
  router: {
    currentRoute: { value: { name: 'sessions' } },
    push: vi.fn(),
  },
}))

beforeEach(() => {
  setActivePinia(createPinia())
  globalThis.localStorage = {
    getItem: vi.fn(),
    setItem: vi.fn(),
    removeItem: vi.fn(),
    clear: vi.fn(),
    length: 0,
    key: vi.fn(),
  }
})

function requestConfig(headers?: Record<string, string>): InternalAxiosRequestConfig {
  return {
    headers: AxiosHeaders.from(headers ?? {}),
    method: 'get',
    url: '/api/test',
  } as InternalAxiosRequestConfig
}

function response(status: number, data: unknown): AxiosResponse {
  return {
    data,
    status,
    statusText: String(status),
    headers: {},
    config: requestConfig(),
  }
}

describe('api client', () => {
  it('adds a request id header when missing', () => {
    const config = requestConfig()

    ensureRequestId(config)

    expect(config.headers.get(requestIdHeader)).toMatch(/^req_/)
  })

  it('keeps an existing request id header', () => {
    const config = requestConfig({ [requestIdHeader]: 'req_existing' })

    ensureRequestId(config)

    expect(config.headers.get(requestIdHeader)).toBe('req_existing')
  })

  it('turns structured error responses into ApiClientError', () => {
    const err = errorFromResponse(
      response(503, { code: 'device_offline', error: 'Device is offline.', request_id: 'req_123' }),
    )

    expect(err).toBeInstanceOf(ApiClientError)
    expect(err.message).toContain('503 device_offline')
    expect(err.message).toContain('req_123')
  })

  it('rejects legacy error responses as contract mismatch', () => {
    const err = errorFromResponse(
      response(400, { code: 'bad_request', message: 'Bad request.', error: 'bad request' }),
    )

    expect(err).toBeInstanceOf(ApiContractMismatchError)
  })

  it('rejects text error responses as contract mismatch', () => {
    const err = errorFromResponse(response(500, 'internal server error'))

    expect(err).toBeInstanceOf(ApiContractMismatchError)
  })

  it('uses runtime API base URL for requests', async () => {
    vi.stubGlobal('window', { __CONFIG__: { apiBaseUrl: 'https://api.example.com/' } })

    const result = await apiClient.get<string>('/api/history', {
      adapter: async (config) => ({
        data: config.baseURL,
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }),
    })

    expect(result.data).toBe('https://api.example.com')
    vi.unstubAllGlobals()
  })

  it('uses cloud API base URL for cloud requests', async () => {
    vi.stubGlobal('window', {
      __CONFIG__: { agentApiBaseUrl: '/agent-api', cloudApiBaseUrl: '/cloud-api' },
    })

    const result = await cloudApiClient.get<string>('/api/devices', {
      adapter: async (config) => ({
        data: config.baseURL,
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }),
    })

    expect(result.data).toBe('/cloud-api')
    vi.unstubAllGlobals()
  })

  it('does not parse successful text responses as JSON', async () => {
    const result = await apiClient.get<string>('/api/history', {
      responseType: 'text',
      adapter: async (config) => ({
        data: 'terminal history',
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }),
    })

    expect(result.data).toBe('terminal history')
  })
})
