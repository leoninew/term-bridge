import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { watch } from 'vue'

vi.mock('../router', () => ({
  router: {
    currentRoute: { value: { name: 'local-sessions', fullPath: '/sessions' } },
    push: vi.fn(),
  },
}))

import { FileEntryKind, type FileEntry } from '../gen/proto/termbridge/agent/v1/file'
import { GitLayer, GitState } from '../gen/proto/termbridge/agent/v1/git'
import type { FileGitRuntimeApi } from '../features/files/runtime'
import { useFileWorkbenchStore } from './fileWorkbench'

function entry(path: string, revision = 'revision-1'): FileEntry {
  const name = path.split('/').at(-1) ?? ''
  return {
    path,
    name,
    kind: path.endsWith('/')
      ? FileEntryKind.FILE_ENTRY_KIND_DIRECTORY
      : FileEntryKind.FILE_ENTRY_KIND_FILE,
    size: 12,
    modified_at: undefined,
    revision,
  }
}

function api(): FileGitRuntimeApi {
  return {
    listFiles: vi.fn(),
    readFile: vi.fn(),
    createFile: vi.fn(),
    createDirectory: vi.fn(),
    writeFile: vi.fn(),
    renameEntry: vi.fn(),
    moveEntry: vi.fn(),
    deleteEntry: vi.fn(),
    gitStatus: vi.fn(),
    gitDiff: vi.fn(),
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((nextResolve, nextReject) => {
    resolve = nextResolve
    reject = nextReject
  })
  return { promise, resolve, reject }
}

describe('useFileWorkbenchStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('isolates cached state when the runtime source changes', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    vi.mocked(runtimeApi.listFiles).mockResolvedValueOnce({
      directory: entry('', 'root-local'),
      items: [entry('notes.txt')],
      truncated: false,
    })

    store.openWorkspace({ mode: 'local' }, 'workspace-1')
    await store.refreshDirectory(runtimeApi, '')
    store.openWorkspace({ mode: 'cloud', deviceId: 'device-1' }, 'workspace-1')

    expect(store.directories).toEqual([])
    expect(store.documents).toEqual([])
    expect(store.target).toEqual({ mode: 'cloud', deviceId: 'device-1' })
    expect(store.workspaceId).toBe('workspace-1')
  })

  it('loads direct children once and refreshes only on explicit request', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    vi.mocked(runtimeApi.listFiles).mockResolvedValueOnce({
      directory: entry('', 'root-1'),
      items: [entry('notes.txt')],
      truncated: true,
    })
    vi.mocked(runtimeApi.listFiles).mockResolvedValueOnce({
      directory: entry('', 'root-2'),
      items: [entry('new.txt')],
      truncated: false,
    })
    store.openWorkspace({ mode: 'local' }, 'workspace-1')

    await store.ensureDirectoryLoaded(runtimeApi, '')
    await store.ensureDirectoryLoaded(runtimeApi, '')

    expect(runtimeApi.listFiles).toHaveBeenCalledTimes(1)
    expect(store.directoryFor('')).toMatchObject({ loaded: true, truncated: true })

    await store.refreshDirectory(runtimeApi, '')

    expect(runtimeApi.listFiles).toHaveBeenCalledTimes(2)
    expect(store.directoryFor('')?.items).toEqual([entry('new.txt')])
  })

  it('does not apply an obsolete directory result after a source reset', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    const pending = deferred<Awaited<ReturnType<FileGitRuntimeApi['listFiles']>>>()
    vi.mocked(runtimeApi.listFiles).mockReturnValueOnce(pending.promise)
    store.openWorkspace({ mode: 'local' }, 'workspace-1')

    const loading = store.refreshDirectory(runtimeApi, '')
    store.resetForSourceChange()
    pending.resolve({ directory: entry('', 'root-1'), items: [entry('old.txt')], truncated: false })

    await expect(loading).resolves.toBeNull()
    expect(store.directories).toEqual([])
  })

  it('cancels readonly requests without discarding cached workspace state', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    const pending = deferred<Awaited<ReturnType<FileGitRuntimeApi['listFiles']>>>()
    vi.mocked(runtimeApi.listFiles).mockReturnValueOnce(pending.promise)
    store.openWorkspace({ mode: 'local' }, 'workspace-1')

    const loading = store.refreshDirectory(runtimeApi, '')
    store.cancelReadonlyRequests()
    pending.resolve({
      directory: entry('', 'root-1'),
      items: [entry('late.txt')],
      truncated: false,
    })

    await expect(loading).resolves.toBeNull()
    expect(store.directoryFor('')).toMatchObject({ loading: false, loaded: false })
    expect(store.workspaceId).toBe('workspace-1')
  })


  it('clears document loading reactively after content loads', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    const pending = deferred<Awaited<ReturnType<FileGitRuntimeApi['readFile']>>>()
    vi.mocked(runtimeApi.readFile).mockReturnValueOnce(pending.promise)
    store.openWorkspace({ mode: 'local' }, 'workspace-1')

    const loadingStates: boolean[] = []
    const stop = watch(
      () => store.documentFor('notes.txt')?.loading ?? false,
      (value) => {
        loadingStates.push(value)
      },
      { flush: 'sync' },
    )

    const opening = store.openDocument(runtimeApi, 'notes.txt')
    expect(store.documentFor('notes.txt')).toMatchObject({ loading: true })
    expect(loadingStates).toContain(true)

    pending.resolve({ entry: entry('notes.txt'), text: 'base' })
    await expect(opening).resolves.toBeNull()

    expect(store.documentFor('notes.txt')).toMatchObject({
      loading: false,
      draftText: 'base',
      entry: entry('notes.txt'),
    })
    expect(loadingStates.at(-1)).toBe(false)
    stop()
  })

  it('clears directory loading reactively after tree loads', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    const pending = deferred<Awaited<ReturnType<FileGitRuntimeApi['listFiles']>>>()
    vi.mocked(runtimeApi.listFiles).mockReturnValueOnce(pending.promise)
    store.openWorkspace({ mode: 'local' }, 'workspace-1')

    const loadingStates: boolean[] = []
    const stop = watch(
      () => store.directoryFor('')?.loading ?? false,
      (value) => {
        loadingStates.push(value)
      },
      { flush: 'sync' },
    )

    const loading = store.refreshDirectory(runtimeApi, '')
    expect(store.directoryFor('')).toMatchObject({ loading: true })
    expect(loadingStates).toContain(true)

    pending.resolve({
      directory: entry('', 'root-1'),
      items: [entry('notes.txt')],
      truncated: false,
    })
    await expect(loading).resolves.toBeNull()

    expect(store.directoryFor('')).toMatchObject({ loading: false, loaded: true })
    expect(loadingStates.at(-1)).toBe(false)
    stop()
  })
  it('does not replace an in-flight document read with a second request', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    const pending = deferred<Awaited<ReturnType<FileGitRuntimeApi['readFile']>>>()
    vi.mocked(runtimeApi.readFile).mockReturnValueOnce(pending.promise)
    store.openWorkspace({ mode: 'local' }, 'workspace-1')

    const opening = store.openDocument(runtimeApi, 'notes.txt')
    const reloading = store.reloadDocumentDiscardingDraft(runtimeApi, 'notes.txt')

    expect(runtimeApi.readFile).toHaveBeenCalledTimes(1)
    pending.resolve({ entry: entry('notes.txt'), text: 'base' })

    await expect(Promise.all([opening, reloading])).resolves.toEqual([null, null])
    expect(store.documentFor('notes.txt')).toMatchObject({ loading: false, draftText: 'base' })
  })

  it('opens one document, preserves its draft, and prevents refresh from discarding it', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    vi.mocked(runtimeApi.readFile).mockResolvedValueOnce({
      entry: entry('notes.txt'),
      text: 'base',
    })
    store.openWorkspace({ mode: 'local' }, 'workspace-1')

    await store.openDocument(runtimeApi, 'notes.txt')
    await store.openDocument(runtimeApi, 'notes.txt')
    store.setDocumentDraft('notes.txt', 'draft')

    await expect(store.refreshDocument(runtimeApi, 'notes.txt')).resolves.toBe('discard_required')
    expect(store.documents).toHaveLength(1)
    expect(store.documentFor('notes.txt')).toMatchObject({ draftText: 'draft', dirty: true })
    expect(runtimeApi.readFile).toHaveBeenCalledTimes(1)
  })

  it('updates the save baseline but keeps edits made while saving dirty', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    vi.mocked(runtimeApi.readFile).mockResolvedValueOnce({
      entry: entry('notes.txt', 'revision-1'),
      text: 'base',
    })
    const pending = deferred<Awaited<ReturnType<FileGitRuntimeApi['writeFile']>>>()
    vi.mocked(runtimeApi.writeFile).mockReturnValueOnce(pending.promise)
    store.openWorkspace({ mode: 'local' }, 'workspace-1')
    await store.openDocument(runtimeApi, 'notes.txt')
    store.setDocumentDraft('notes.txt', 'saved')

    const saving = store.saveDocument(runtimeApi, 'notes.txt')
    store.setDocumentDraft('notes.txt', 'newer draft')
    pending.resolve({
      result: {
        source_entry: entry('notes.txt', 'revision-2'),
        destination_entry: undefined,
        affected_count: 1,
        affected_path_prefixes: [''],
      },
    })

    await expect(saving).resolves.toBeNull()
    expect(runtimeApi.writeFile).toHaveBeenCalledWith('workspace-1', {
      path: 'notes.txt',
      text: 'saved',
      expected_revision: 'revision-1',
      force: false,
    })
    expect(store.documentFor('notes.txt')).toMatchObject({
      baseRevision: 'revision-2',
      originalText: 'saved',
      draftText: 'newer draft',
      dirty: true,
    })
  })

  it('retains a draft and structured conflict after a revision conflict', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    vi.mocked(runtimeApi.readFile).mockResolvedValueOnce({
      entry: entry('notes.txt'),
      text: 'base',
    })
    vi.mocked(runtimeApi.writeFile).mockRejectedValueOnce({
      name: 'FileGitApiError',
      code: 'revision_conflict',
      message: 'The workspace entry changed.',
      details: { type: 'revision_conflict', current_entry: entry('notes.txt', 'revision-2') },
    })
    store.openWorkspace({ mode: 'local' }, 'workspace-1')
    await store.openDocument(runtimeApi, 'notes.txt')
    store.setDocumentDraft('notes.txt', 'draft')

    await expect(store.saveDocument(runtimeApi, 'notes.txt')).resolves.toBe(
      'The workspace entry changed.',
    )

    expect(store.documentFor('notes.txt')).toMatchObject({
      draftText: 'draft',
      dirty: true,
      conflict: { current_entry: { revision: 'revision-2' } },
    })
  })

  it('keeps the renamed clean document active with its canonical path', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    vi.mocked(runtimeApi.readFile).mockResolvedValueOnce({
      entry: entry('old.txt', 'revision-1'),
      text: 'base',
    })
    vi.mocked(runtimeApi.renameEntry).mockResolvedValueOnce({
      result: {
        source_entry: entry('old.txt', 'revision-1'),
        destination_entry: entry('new.txt', 'revision-2'),
        affected_count: 1,
        affected_path_prefixes: ['old.txt', 'new.txt'],
      },
    })
    store.openWorkspace({ mode: 'local' }, 'workspace-1')
    await store.openDocument(runtimeApi, 'old.txt')

    await store.renameEntry(runtimeApi, {
      path: 'old.txt',
      new_name: 'new.txt',
      expected_source_revision: 'revision-1',
      expected_parent_revision: 'parent-1',
      expected_destination_revision: '',
    })

    expect(store.activeDocument?.path).toBe('new.txt')
    expect(store.documentFor('old.txt')).toBeNull()
  })

  it('does not apply an old workspace mutation to a new workspace', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    const pending = deferred<Awaited<ReturnType<FileGitRuntimeApi['deleteEntry']>>>()
    vi.mocked(runtimeApi.deleteEntry).mockReturnValueOnce(pending.promise)
    store.openWorkspace({ mode: 'local' }, 'workspace-1')

    const deleting = store.deleteEntry(runtimeApi, {
      path: 'notes.txt',
      expected_revision: 'revision-1',
      expected_parent_revision: 'parent-1',
      recursive: false,
    })
    await Promise.resolve()
    store.openWorkspace({ mode: 'cloud', deviceId: 'device-1' }, 'workspace-2')
    store.setDirectoryExpanded('', true)
    pending.resolve({
      result: {
        source_entry: entry('notes.txt'),
        destination_entry: undefined,
        affected_count: 1,
        affected_path_prefixes: ['notes.txt'],
      },
    })

    await deleting
    expect(store.directoryFor('')).toMatchObject({ stale: false, expanded: true })
    expect(store.gitStatusStale).toBe(false)
  })

  it('keeps Git requests explicit and rejects unavailable layer selections', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    vi.mocked(runtimeApi.gitStatus).mockResolvedValueOnce({
      state: GitState.GIT_STATE_AVAILABLE,
      changes: [
        {
          path: 'notes.txt',
          original_path: '',
          index_status: 'M',
          worktree_status: 'M',
          untracked: false,
          unmerged: false,
          available_layers: [GitLayer.GIT_LAYER_STAGED, GitLayer.GIT_LAYER_UNSTAGED],
        },
      ],
      message: '',
    })
    store.openWorkspace({ mode: 'local' }, 'workspace-1')

    expect(runtimeApi.gitStatus).not.toHaveBeenCalled()
    await store.refreshGitStatus(runtimeApi)

    expect(store.selectGitChange('notes.txt', GitLayer.GIT_LAYER_UNSTAGED)).toBe(true)
    expect(store.selectGitChange('notes.txt', GitLayer.GIT_LAYER_UNTRACKED)).toBe(false)
    expect(runtimeApi.gitDiff).not.toHaveBeenCalled()
  })

  it('ignores an old Git diff after a new layer selection', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    const pending = deferred<Awaited<ReturnType<FileGitRuntimeApi['gitDiff']>>>()
    vi.mocked(runtimeApi.gitStatus).mockResolvedValueOnce({
      state: GitState.GIT_STATE_AVAILABLE,
      changes: [
        {
          path: 'notes.txt',
          original_path: '',
          index_status: 'M',
          worktree_status: 'M',
          untracked: false,
          unmerged: false,
          available_layers: [GitLayer.GIT_LAYER_STAGED, GitLayer.GIT_LAYER_UNSTAGED],
        },
      ],
      message: '',
    })
    vi.mocked(runtimeApi.gitDiff).mockReturnValueOnce(pending.promise)
    store.openWorkspace({ mode: 'local' }, 'workspace-1')
    await store.refreshGitStatus(runtimeApi)
    store.selectGitChange('notes.txt', GitLayer.GIT_LAYER_STAGED)
    const loading = store.refreshGitDiff(runtimeApi)
    store.selectGitChange('notes.txt', GitLayer.GIT_LAYER_UNSTAGED)
    pending.resolve({
      state: GitState.GIT_STATE_AVAILABLE,
      original_path: 'notes.txt',
      modified_path: 'notes.txt',
      original_text: 'old',
      modified_text: 'new',
      message: '',
    })

    await expect(loading).resolves.toBeNull()
    expect(store.gitSelection).toEqual({ path: 'notes.txt', layer: GitLayer.GIT_LAYER_UNSTAGED })
    expect(store.gitDiff).toBeNull()
  })
})
