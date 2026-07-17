import { describe, expect, it, vi } from 'vitest'

vi.mock('@codingame/monaco-vscode-api/vscode/vs/base/common/event', () => ({
  Emitter: class<T> {
    private readonly listeners = new Set<(value: T) => void>()
    readonly event = (listener: (value: T) => void) => {
      this.listeners.add(listener)
      return { dispose: () => this.listeners.delete(listener) }
    }
    fire(value: T): void {
      for (const listener of this.listeners) {
        listener(value)
      }
    }
  },
}))

vi.mock('@codingame/monaco-vscode-files-service-override', () => ({
  FileChangeType: { UPDATED: 0, ADDED: 1, DELETED: 2 },
  FilePermission: { Readonly: 1 },
  FileSystemProviderCapabilities: { FileReadWrite: 2, PathCaseSensitive: 1024 },
  FileSystemProviderError: {
    create: (message: string) => new Error(message),
  },
  FileSystemProviderErrorCode: {
    Unknown: 0,
    FileNotFound: 1,
    FileExists: 2,
    FileTooLarge: 3,
    NoPermissions: 4,
    Unavailable: 5,
  },
  FileType: { Unknown: 0, File: 1, Directory: 2, SymbolicLink: 64 },
}))

import { FsChangeKind, type FileStat } from '../../gen/proto/termbridge/agent/v1/file'
import type { WorkbenchFsApi } from './api'
import { TermBridgePlatformFileSystemProvider } from './platformFileSystemProvider'

const documentUri = {
  scheme: 'tb',
  authority: '',
  path: '/README.md',
  query: '',
  fragment: '',
} as import('@codingame/monaco-vscode-api/vscode/vs/base/common/uri').URI

function fileStat(etag: string, size = 4): FileStat {
  return {
    type: 1,
    ctime: 1,
    mtime: 1,
    size,
    permissions: 0,
    etag,
  }
}

function createApi(overrides: Partial<WorkbenchFsApi> = {}): WorkbenchFsApi {
  return {
    stat: vi.fn(async () => fileStat('revision-b')),
    readDirectory: vi.fn(async () => ({ entries: [], truncated: false })),
    readFile: vi.fn(async () => ({
      content: new TextEncoder().encode('base'),
      stat: fileStat('revision-a'),
    })),
    writeFile: vi.fn(async () => fileStat('revision-c')),
    createDirectory: vi.fn(async () => undefined),
    delete: vi.fn(async () => undefined),
    rename: vi.fn(async () => undefined),
    ...overrides,
  }
}

describe('TermBridgePlatformFileSystemProvider', () => {
  it('keeps a dirty document write base when external metadata is refreshed', async () => {
    const api = createApi()
    const provider = new TermBridgePlatformFileSystemProvider()
    provider.bind(api)

    await provider.readFile(documentUri)
    provider.applyWorkspaceChange({
      sequence: 2,
      kind: FsChangeKind.FS_CHANGE_KIND_UPDATED,
      path: 'README.md',
      oldPath: '',
    })
    await provider.stat(documentUri)
    await provider.writeFile(documentUri, new TextEncoder().encode('draft'), {
      create: false,
      overwrite: true,
      unlock: false,
      atomic: false,
    })

    expect(api.writeFile).toHaveBeenCalledWith(
      'README.md',
      new TextEncoder().encode('draft'),
      expect.objectContaining({ etag: 'revision-a' }),
    )
  })

  it('does not allow an external event to repopulate a stale stat request', async () => {
    let resolveFirstStat: ((value: FileStat) => void) | undefined
    const stat = vi
      .fn()
      .mockImplementationOnce(
        () =>
          new Promise<FileStat>((resolve) => {
            resolveFirstStat = resolve
          }),
      )
      .mockResolvedValueOnce(fileStat('revision-b'))
    const provider = new TermBridgePlatformFileSystemProvider()
    provider.bind(createApi({ stat }))

    const staleRead = provider.stat(documentUri)
    provider.applyWorkspaceChange({
      sequence: 1,
      kind: FsChangeKind.FS_CHANGE_KIND_UPDATED,
      path: 'README.md',
      oldPath: '',
    })
    resolveFirstStat?.(fileStat('revision-a'))
    await staleRead
    await provider.stat(documentUri)

    expect(stat).toHaveBeenCalledTimes(2)
  })

  it('evicts least-recently-used content after the bounded content cache fills', async () => {
    const api = createApi({
      readFile: vi.fn(async (path: string) => ({
        content: new TextEncoder().encode(path),
        stat: fileStat(`revision-${path}`, path.length),
      })),
    })
    const provider = new TermBridgePlatformFileSystemProvider()
    provider.bind(api)

    await provider.readFile(documentUri)
    for (let index = 0; index < 64; index += 1) {
      await provider.readFile({
        ...documentUri,
        path: `/file-${index}.txt`,
      } as typeof documentUri)
    }
    await provider.readFile({ ...documentUri, path: '/overflow.txt' } as typeof documentUri)
    await provider.readFile(documentUri)

    expect(api.readFile).toHaveBeenCalledTimes(67)
  })

  it('uses root update rather than a synthetic root delete for ambiguous events', () => {
    const provider = new TermBridgePlatformFileSystemProvider()
    const changes: unknown[] = []
    provider.onDidChangeFile((events) => changes.push(...events))

    provider.applyWorkspaceChange({
      sequence: 1,
      kind: FsChangeKind.FS_CHANGE_KIND_DELETED,
      path: '',
      oldPath: '',
    })

    expect(changes).toEqual([
      expect.objectContaining({ type: 0, resource: expect.objectContaining({ path: '/' }) }),
    ])
  })

  it('emits FileChange resources as real URI instances with with()', () => {
    const provider = new TermBridgePlatformFileSystemProvider()
    const changes: Array<{
      resource: { path: string; with?: (change: object) => { path: string } }
    }> = []
    provider.onDidChangeFile((events) => {
      changes.push(...events)
    })

    provider.applyWorkspaceChange({
      sequence: 1,
      kind: FsChangeKind.FS_CHANGE_KIND_UPDATED,
      path: 'README.md',
      oldPath: '',
    })

    expect(changes).toHaveLength(1)
    expect(typeof changes[0]?.resource.with).toBe('function')
    expect(changes[0]?.resource.path).toBe('/README.md')
    expect(changes[0]?.resource.with?.({ path: '/' }).path).toBe('/')
  })
})
