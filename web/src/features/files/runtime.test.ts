import { describe, expect, it, vi } from 'vitest'
import type { AxiosInstance } from 'axios'

vi.mock('../../router', () => ({
  router: {
    currentRoute: { value: { name: 'local-sessions', fullPath: '/sessions' } },
    push: vi.fn(),
  },
}))
import { FileEntryKind } from '../../gen/proto/termbridge/agent/v1/file'
import { GitLayer } from '../../gen/proto/termbridge/agent/v1/git'
import { ApiClientError } from '../api/client'
import { createFileGitRuntimeApi, FileGitApiError } from './runtime'

type HttpMethod = (url: string, ...args: unknown[]) => Promise<{ data: unknown }>

function clients() {
  return {
    local: {
      get: vi.fn<HttpMethod>(),
      post: vi.fn<HttpMethod>(),
      put: vi.fn<HttpMethod>(),
      delete: vi.fn<HttpMethod>(),
    } as unknown as Pick<AxiosInstance, 'get' | 'post' | 'put' | 'delete'>,
    cloud: {
      get: vi.fn<HttpMethod>(),
      post: vi.fn<HttpMethod>(),
      put: vi.fn<HttpMethod>(),
      delete: vi.fn<HttpMethod>(),
    } as unknown as Pick<AxiosInstance, 'get' | 'post' | 'put' | 'delete'>,
  }
}

describe('File/Git runtime API', () => {
  it('uses the local workspace route with typed query data', async () => {
    const httpClients = clients()
    vi.mocked(httpClients.local.get).mockResolvedValueOnce({
      data: {
        directory: {
          path: '',
          name: '',
          kind: 'FILE_ENTRY_KIND_DIRECTORY',
          size: '0',
          revision: 'root-1',
        },
        items: [
          {
            path: 'nested/notes.txt',
            name: 'notes.txt',
            kind: 'FILE_ENTRY_KIND_FILE',
            size: '12',
            revision: 'entry-1',
          },
        ],
        truncated: false,
      },
    })
    const api = createFileGitRuntimeApi({ mode: 'local' }, httpClients)

    await expect(api.listFiles('workspace 1', 'nested/notes.txt')).resolves.toMatchObject({
      directory: { kind: FileEntryKind.FILE_ENTRY_KIND_DIRECTORY, size: 0 },
      items: [{ kind: FileEntryKind.FILE_ENTRY_KIND_FILE, size: 12 }],
      truncated: false,
    })

    expect(httpClients.local.get).toHaveBeenCalledWith('/workspaces/workspace%201/files/tree', {
      params: { path: 'nested/notes.txt' },
      signal: undefined,
    })
    expect(httpClients.cloud.get).not.toHaveBeenCalled()
  })

  it('uses the Cloud device route and forwards cancellation', async () => {
    const httpClients = clients()
    const controller = new AbortController()
    vi.mocked(httpClients.cloud.get).mockResolvedValueOnce({
      data: {
        state: 'GIT_STATE_AVAILABLE',
        changes: [
          {
            path: 'notes.txt',
            original_path: '',
            index_status: 'M',
            worktree_status: '',
            untracked: false,
            unmerged: false,
            available_layers: ['GIT_LAYER_STAGED'],
          },
        ],
        message: '',
      },
    })
    const api = createFileGitRuntimeApi({ mode: 'cloud', deviceId: 'device-1' }, httpClients)

    await expect(
      api.gitStatus('workspace-1', { signal: controller.signal }),
    ).resolves.toMatchObject({
      state: 1,
      changes: [{ available_layers: [GitLayer.GIT_LAYER_STAGED] }],
    })

    expect(httpClients.cloud.get).toHaveBeenCalledWith(
      '/devices/device-1/workspaces/workspace-1/git/status',
      { signal: controller.signal },
    )
    expect(httpClients.local.get).not.toHaveBeenCalled()
  })

  it('adds the workspace ID to typed mutation payloads without mutating callers', async () => {
    const httpClients = clients()
    const request = {
      path: 'new file.txt',
      text: 'hello',
      overwrite: false,
      expected_parent_revision: 'parent-1',
      expected_destination_revision: '',
    }
    vi.mocked(httpClients.local.post).mockResolvedValueOnce({
      data: { result: undefined },
    })
    const api = createFileGitRuntimeApi({ mode: 'local' }, httpClients)

    await api.createFile('workspace-1', request)

    expect(httpClients.local.post).toHaveBeenCalledWith(
      '/workspaces/workspace-1/files',
      { ...request, workspace_id: 'workspace-1' },
      { signal: undefined },
    )
    expect(request).not.toHaveProperty('workspace_id')
  })

  it('serializes delete and Git diff query parameters according to route contracts', async () => {
    const httpClients = clients()
    vi.mocked(httpClients.local.delete).mockResolvedValueOnce({ data: { result: undefined } })
    vi.mocked(httpClients.local.get).mockResolvedValueOnce({
      data: {
        state: 1,
        original_path: 'old.txt',
        modified_path: 'new.txt',
        original_text: 'before',
        modified_text: 'after',
        message: '',
      },
    })
    const api = createFileGitRuntimeApi({ mode: 'local' }, httpClients)

    await api.deleteEntry('workspace-1', {
      path: 'nested/notes.txt',
      expected_revision: 'entry-1',
      expected_parent_revision: 'parent-1',
      recursive: true,
    })
    await api.gitDiff('workspace-1', 'nested/notes.txt', GitLayer.GIT_LAYER_UNSTAGED)

    expect(httpClients.local.delete).toHaveBeenCalledWith('/workspaces/workspace-1/entries', {
      params: {
        path: 'nested/notes.txt',
        expected_revision: 'entry-1',
        expected_parent_revision: 'parent-1',
        recursive: true,
      },
      signal: undefined,
    })
    expect(httpClients.local.get).toHaveBeenCalledWith('/workspaces/workspace-1/git/diff', {
      params: { path: 'nested/notes.txt', layer: 'unstaged' },
      signal: undefined,
    })
  })

  it('rejects unspecified Git diff layers before making a request', async () => {
    const httpClients = clients()
    const api = createFileGitRuntimeApi({ mode: 'local' }, httpClients)

    await expect(
      api.gitDiff('workspace-1', 'notes.txt', GitLayer.GIT_LAYER_UNSPECIFIED),
    ).rejects.toThrow('Git diff requires a staged, unstaged, or untracked layer')

    expect(httpClients.local.get).not.toHaveBeenCalled()
  })

  it('preserves revision conflict details in a typed File/Git API error', async () => {
    const httpClients = clients()
    vi.mocked(httpClients.local.put).mockRejectedValueOnce(
      new ApiClientError(409, {
        code: 'revision_conflict',
        error: 'The workspace entry changed.',
        request_id: 'req_123',
        details: {
          type: 'revision_conflict',
          current_entry: {
            path: 'notes.txt',
            name: 'notes.txt',
            kind: 'file',
            size: 12,
            revision: 'current-revision',
          },
        },
      }),
    )
    const api = createFileGitRuntimeApi({ mode: 'local' }, httpClients)

    await expect(
      api.writeFile('workspace-1', {
        path: 'notes.txt',
        text: 'draft',
        expected_revision: 'stale-revision',
        force: false,
      }),
    ).rejects.toMatchObject({
      name: 'FileGitApiError',
      status: 409,
      code: 'revision_conflict',
      requestId: 'req_123',
      details: {
        type: 'revision_conflict',
        current_entry: {
          path: 'notes.txt',
          name: 'notes.txt',
          kind: FileEntryKind.FILE_ENTRY_KIND_FILE,
          size: 12,
          modified_at: undefined,
          revision: 'current-revision',
        },
      },
    } satisfies Partial<FileGitApiError>)
  })
})
