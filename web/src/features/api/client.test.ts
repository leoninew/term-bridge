import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  AxiosError,
  AxiosHeaders,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from 'axios'
import { createPinia, setActivePinia } from 'pinia'
import type { BrowserRuntimeConfig } from '../../config'
import { router } from '../../router'
import { useCloudAuthStore } from '../../store/cloudAuth'
import {
  ApiClientError,
  ApiContractMismatchError,
  localApiClient,
  localCloudApiClient,
  apiClient,
  cloudApiClient,
  ensureRequestId,
  errorFromResponse,
  requestIdHeader,
} from './client'

vi.mock('../../router', () => ({
  router: {
    currentRoute: { value: { name: 'local-sessions', fullPath: '/sessions' } },
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
      apiBasePath: '/local-api',
      cloudOAuth: {
        clientId: 'termbridge-agent',
        redirectUrl: 'http://localhost:9030/oauth/callback',
        scopes: ['openid', 'email', 'profile'],
      },
      ...overrides.local,
    },
    cloud: {
      publicUrl: 'http://localhost:9030',
      apiBaseUrl: 'https://cloud.example.test/api',
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
  vi.clearAllMocks()
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

function rejectedResponse(status: number, data: unknown) {
  return async (config: InternalAxiosRequestConfig) => {
    throw new AxiosError('Request failed', undefined, config, undefined, response(status, data))
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
    expect(err.message).toBe('Device is offline.')
    expect(err).toMatchObject({ status: 503, code: 'device_offline', requestId: 'req_123' })
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

  it('uses configured Local API base path for local requests', async () => {
    stubBrowser(runtimeConfig({ local: { apiBasePath: '/configured-api/' } }))
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

    expect(result.data).toBe('/configured-api')
  })

  it('uses configured Cloud API base URL for cloud requests', async () => {
    stubBrowser(runtimeConfig({ cloud: { apiBaseUrl: 'https://cloud.example.test/api' } }))
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

    expect(result.data).toBe('https://cloud.example.test/api')
  })

  it('sends a Cloud bearer only to Cloud requests', async () => {
    const localStorage = storageMock()
    localStorage.setItem('termbridge_cloud_token', 'cloud-token')
    stubBrowser(runtimeConfig(), localStorage)
    setActivePinia(createPinia())

    const localResult = await localApiClient.get<string>('/agent/status', {
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

    expect(localResult.data).toBe('undefined')
    expect(cloudResult.data).toBe('Bearer cloud-token')
  })

  it('clears Cloud credentials after a local Cloud relay 401', async () => {
    const cloudAuth = useCloudAuthStore()
    cloudAuth.setToken('cloud-token')

    await expect(
      localCloudApiClient.get('/cloud/devices', {
        adapter: rejectedResponse(401, {
          code: 'unauthorized',
          error: 'Authentication is required.',
          request_id: 'req_local_cloud_401',
        }),
      }),
    ).rejects.toMatchObject({ status: 401, code: 'unauthorized', requestId: 'req_local_cloud_401' })

    expect(cloudAuth.cloudToken).toBeNull()
    expect(router.push).toHaveBeenCalledWith({
      name: 'cloud-login',
      query: { redirect: '/sessions' },
    })
  })

  it('does not clear Cloud credentials after an OAuth exchange 401 on the ordinary local client', async () => {
    const cloudAuth = useCloudAuthStore()
    cloudAuth.setToken('cloud-token')

    await expect(
      localApiClient.post(
        '/cloud/oauth/exchange',
        {},
        {
          adapter: rejectedResponse(401, {
            code: 'unauthorized',
            error: 'Authentication is required.',
            request_id: 'req_oauth_401',
          }),
        },
      ),
    ).rejects.toMatchObject({ status: 401, code: 'unauthorized', requestId: 'req_oauth_401' })

    expect(cloudAuth.cloudToken).toBe('cloud-token')
    expect(router.push).not.toHaveBeenCalled()
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
