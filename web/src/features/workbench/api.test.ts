import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { BrowserRuntimeConfig } from '../../config'
import { createWorkbenchRuntimeApi, subscribeWorkspaceChanges, workspaceChangesWsUrl } from './api'
import { bytesToWire } from './bytes'

type HttpClients = {
  get: ReturnType<typeof vi.fn>
  post: ReturnType<typeof vi.fn>
  put: ReturnType<typeof vi.fn>
}

type WebSocketHandler = ((event: Event) => void) | null
type WebSocketMessageHandler = ((event: MessageEvent<string>) => void) | null

class FakeWebSocket {
  static readonly instances: FakeWebSocket[] = []

  onopen: WebSocketHandler = null
  onclose: WebSocketHandler = null
  onerror: WebSocketHandler = null
  onmessage: WebSocketMessageHandler = null
  closed = false

  constructor(
    readonly url: string,
    readonly protocols: string | string[],
  ) {
    FakeWebSocket.instances.push(this)
  }

  close(): void {
    this.closed = true
  }

  open(): void {
    this.onopen?.(new Event('open'))
  }

  message(data: string): void {
    this.onmessage?.(new MessageEvent<string>('message', { data }))
  }

  disconnect(): void {
    this.onclose?.(new Event('close'))
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
        scopes: ['openid'],
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

beforeEach(() => {
  vi.useFakeTimers()
  FakeWebSocket.instances.length = 0
  setActivePinia(createPinia())
  vi.stubGlobal('window', {
    __CONFIG__: runtimeConfig(),
    localStorage: storageMock(),
    sessionStorage: storageMock(),
  })
  vi.stubGlobal('WebSocket', FakeWebSocket)
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('createWorkbenchRuntimeApi', () => {
  it('calls fs/readDirectory and scm/status with provider paths', async () => {
    const get = vi.fn(async (url: string) => {
      if (url.includes('/fs/readDirectory')) {
        return {
          data: {
            entries: [{ name: 'a.txt', type: 1 }],
            truncated: false,
          },
        }
      }
      if (url.includes('/scm/status')) {
        return {
          data: {
            state: 1,
            groups: [],
            count: 0,
            message: '',
          },
        }
      }
      if (url.includes('/fs/stat')) {
        return {
          data: {
            stat: {
              type: 1,
              ctime: 1,
              mtime: 2,
              size: 3,
              permissions: 0,
              etag: 'rev-1',
            },
          },
        }
      }
      if (url.includes('/fs/readFile')) {
        return {
          data: {
            content: bytesToWire(new TextEncoder().encode('hi')),
            stat: {
              type: 1,
              ctime: 1,
              mtime: 2,
              size: 2,
              permissions: 0,
              etag: 'rev-1',
            },
          },
        }
      }
      throw new Error(`unexpected get ${url}`)
    })
    const clients = {
      get,
      post: vi.fn(),
      put: vi.fn(),
    } as unknown as HttpClients

    const api = createWorkbenchRuntimeApi({ mode: 'local' }, 'workspace-1', {
      local: clients as never,
      cloud: clients as never,
    })

    const listing = await api.readDirectory('src')
    expect(listing.entries[0]?.name).toBe('a.txt')
    expect(get).toHaveBeenCalledWith('/workspaces/workspace-1/fs/readDirectory', {
      params: { path: 'src' },
      signal: undefined,
    })

    const status = await api.status()
    expect(status.state).toBe(1)
    expect(get).toHaveBeenCalledWith('/workspaces/workspace-1/scm/status', {
      signal: undefined,
    })

    const file = await api.readFile('src/a.txt')
    expect(new TextDecoder().decode(file.content)).toBe('hi')
  })

  it('prefixes cloud device path', async () => {
    const get = vi.fn(async () => ({
      data: { state: 1, groups: [], count: 0, message: '' },
    }))
    const cloud = { get, post: vi.fn(), put: vi.fn() } as never
    const api = createWorkbenchRuntimeApi({ mode: 'cloud', deviceId: 'device-9' }, 'ws-1', {
      local: { get: vi.fn(), post: vi.fn(), put: vi.fn() } as never,
      cloud,
    })
    await api.status()
    expect(get).toHaveBeenCalledWith(
      '/devices/device-9/workspaces/ws-1/scm/status',
      expect.any(Object),
    )
  })
})

describe('workspace change subscription', () => {
  it('builds local and cloud event endpoints with required auth query token', () => {
    expect(workspaceChangesWsUrl({ mode: 'local' }, 'space id')).toBe(
      '/local-api/workspaces/space%20id/fs/events',
    )
    expect(workspaceChangesWsUrl({ mode: 'cloud', deviceId: 'device/id' }, 'ws-1', 'token 1')).toBe(
      'wss://cloud.example.test/api/devices/device%2Fid/workspaces/ws-1/fs/events?token=token+1',
    )
  })

  it('accepts protobuf enum names and ignores subscribe acknowledgements', () => {
    const onChange = vi.fn()
    const onRescanRequired = vi.fn()
    const onDisconnected = vi.fn()
    const subscription = subscribeWorkspaceChanges({ mode: 'local' }, 'ws-1', {
      onChange,
      onRescanRequired,
      onDisconnected,
    })
    const socket = FakeWebSocket.instances[0]
    expect(socket).toBeDefined()
    socket?.open()
    socket?.message(JSON.stringify({ workspace_id: 'ws-1' }))
    socket?.message(
      JSON.stringify({
        workspace_id: 'ws-1',
        sequence: '7',
        kind: 'FS_CHANGE_KIND_UPDATED',
        path: 'README.md',
        old_path: '',
      }),
    )

    expect(onRescanRequired).not.toHaveBeenCalled()
    expect(onChange).toHaveBeenCalledWith({
      sequence: 7,
      kind: 2,
      path: 'README.md',
      oldPath: '',
    })
    subscription.close()
  })

  it('invalidates and reconnects after a sequence gap or unexpected disconnect', () => {
    const onChange = vi.fn()
    const onRescanRequired = vi.fn()
    const onDisconnected = vi.fn()
    const subscription = subscribeWorkspaceChanges({ mode: 'local' }, 'ws-1', {
      onChange,
      onRescanRequired,
      onDisconnected,
    })
    const socket = FakeWebSocket.instances[0]
    socket?.open()
    socket?.message(
      JSON.stringify({
        sequence: 1,
        kind: 'FS_CHANGE_KIND_UPDATED',
        path: 'README.md',
        old_path: '',
      }),
    )
    socket?.message(
      JSON.stringify({
        sequence: 3,
        kind: 'FS_CHANGE_KIND_UPDATED',
        path: 'README.md',
        old_path: '',
      }),
    )
    expect(onRescanRequired).toHaveBeenCalledTimes(1)

    socket?.disconnect()
    expect(onRescanRequired).toHaveBeenCalledTimes(2)
    expect(onDisconnected).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(250)
    expect(FakeWebSocket.instances).toHaveLength(2)
    expect(onChange).toHaveBeenCalledTimes(2)
    subscription.close()
  })

  it('does not reconnect after the owner closes the subscription', () => {
    const subscription = subscribeWorkspaceChanges({ mode: 'local' }, 'ws-1', {
      onChange: vi.fn(),
      onRescanRequired: vi.fn(),
      onDisconnected: vi.fn(),
    })
    const socket = FakeWebSocket.instances[0]
    subscription.close()
    socket?.disconnect()
    vi.advanceTimersByTime(5_000)
    expect(FakeWebSocket.instances).toHaveLength(1)
  })
})
