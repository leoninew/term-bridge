import { describe, expect, it, vi } from 'vitest'
import { createWorkbenchRuntimeApi } from './api'
import { bytesToWire } from './bytes'

type HttpClients = {
  get: ReturnType<typeof vi.fn>
  post: ReturnType<typeof vi.fn>
  put: ReturnType<typeof vi.fn>
}

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

    const api = createWorkbenchRuntimeApi(
      { mode: 'local' },
      'workspace-1',
      {
        local: clients as never,
        cloud: clients as never,
      },
    )

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
    const api = createWorkbenchRuntimeApi(
      { mode: 'cloud', deviceId: 'device-9' },
      'ws-1',
      {
        local: { get: vi.fn(), post: vi.fn(), put: vi.fn() } as never,
        cloud,
      },
    )
    await api.status()
    expect(get).toHaveBeenCalledWith(
      '/devices/device-9/workspaces/ws-1/scm/status',
      expect.any(Object),
    )
  })
})
