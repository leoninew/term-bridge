import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AxiosHeaders, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'
import { createPinia, setActivePinia } from 'pinia'
import type { BrowserRuntimeConfig } from '../../config'
import {
  ApiClientError,
  ApiContractMismatchError,
  localApiClient,
  apiClient,
  cloudApiClient,
  ensureRequestId,
  errorFromResponse,
  requestIdHeader,
} from './client'

vi.mock('../../router', () => ({
  router: {
    currentRoute: { value: { name: 'local-sessions' } },
    push: vi.fn(),
  },
}))

function storageMock(): Storage {
  const values = new Map<string, string>()
  return {
    get length() {
      return values.size
    },
    clear: vi.fn(() => values.clear()),
    getItem: vi.fn((key: string) => values.get(key) ?? null),
    key: vi.fn((index: number) => Array.from(values.keys())[index] ?? null),
    removeItem: vi.fn((key: string) => values.delete(key)),
    setItem: vi.fn((key: string, value: string) => values.set(key, value)),
  }
}

function runtimeConfig(overrides: BrowserRuntimeConfig = {}): BrowserRuntimeConfig {
  return {
    local: {
      mode: 'hybrid',
      publicUrl: 'http://localhost:9030',
      apiBaseUrl: '/local-api',
      cloudOAuth: {
        clientId: 'termbridge-agent',
        redirectUrl: 'http://localhost:9030/oauth/callback',
        scopes: ['openid', 'email', 'profile'],
      },
      ...overrides.local,
    },
    cloud: {
      publicUrl: 'http://localhost:9030',
      apiBaseUrl: '/cloud-api',
      ...overrides.cloud,
    },
  }
}

function stubBrowser(config: BrowserRuntimeConfig = runtimeConfig(), localStorage = storageMock()) {
  vi.stubGlobal('window', {
    __CONFIG__: config,
    localStorage,
    sessionStorage: storageMock(),
  })
}

beforeEach(() => {
  setActivePinia(createPinia())
  stubBrowser()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

function requestConfig(headers?: Record<string, string>): InternalAxiosRequestConfig {
  return {
    headers: AxiosHeaders.from(headers ?? {}),
    method: 'get',
    url: '/test',
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

  it('uses local API base URL for local requests', async () => {
    stubBrowser(runtimeConfig({ local: { apiBaseUrl: 'https://local.example.com/' } }))
    setActivePinia(createPinia())

    const result = await apiClient.get<string>('/history', {
      adapter: async (config) => ({
        data: config.baseURL,
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }),
    })

    expect(result.data).toBe('https://local.example.com')
  })

  it('uses cloud API base URL for cloud requests', async () => {
    stubBrowser(runtimeConfig({ cloud: { apiBaseUrl: '/cloud-api' } }))
    setActivePinia(createPinia())

    const result = await cloudApiClient.get<string>('/devices', {
      adapter: async (config) => ({
        data: config.baseURL,
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }),
    })

    expect(result.data).toBe('/cloud-api')
  })

  it('sends only the token for the requested API target', async () => {
    const localStorage = storageMock()
    localStorage.setItem('termbridge_local_token', 'local-token')
    localStorage.setItem('termbridge_cloud_token', 'cloud-token')
    stubBrowser(runtimeConfig(), localStorage)
    setActivePinia(createPinia())

    const localResult = await localApiClient.get<string>('/auth/me', {
      adapter: async (config) => ({
        data: String(AxiosHeaders.from(config.headers).get('Authorization')),
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }),
    })
    const cloudResult = await cloudApiClient.get<string>('/auth/me', {
      adapter: async (config) => ({
        data: String(AxiosHeaders.from(config.headers).get('Authorization')),
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }),
    })

    expect(localResult.data).toBe('Bearer local-token')
    expect(cloudResult.data).toBe('Bearer cloud-token')
  })

  it('does not parse successful text responses as JSON', async () => {
    const result = await apiClient.get<string>('/history', {
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
