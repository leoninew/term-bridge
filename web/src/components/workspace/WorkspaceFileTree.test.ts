// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { FileEntryKind, type FileEntry } from '../../gen/proto/termbridge/agent/v1/file'
import { i18n } from '../../i18n'
import type { FileGitRuntimeApi } from '../../features/files/runtime'
import { useFileWorkbenchStore } from '../../store/fileWorkbench'
import WorkspaceFileTree from './WorkspaceFileTree.vue'

function entry(path: string, kind: FileEntryKind, revision = 'revision-1'): FileEntry {
  return {
    path,
    name: path.split('/').at(-1) ?? '',
    kind,
    size: 1,
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

describe('WorkspaceFileTree', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('loads a directory only after expansion and opens files without reading directory content', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    const folder = entry('docs', FileEntryKind.FILE_ENTRY_KIND_DIRECTORY)
    const file = entry('notes.txt', FileEntryKind.FILE_ENTRY_KIND_FILE)
    vi.mocked(runtimeApi.listFiles).mockResolvedValueOnce({
      directory: entry('', FileEntryKind.FILE_ENTRY_KIND_DIRECTORY),
      items: [folder, file],
      truncated: true,
    })
    store.openWorkspace({ mode: 'local' }, 'workspace-1')
    await store.refreshDirectory(runtimeApi, '')

    const openDocument = vi.spyOn(store, 'openDocument').mockResolvedValue(null)
    const ensureDirectoryLoaded = vi.spyOn(store, 'ensureDirectoryLoaded').mockResolvedValue(null)
    const wrapper = mount(WorkspaceFileTree, {
      props: { api: runtimeApi, workspaceName: 'Workspace' },
      global: { plugins: [i18n] },
    })

    expect(wrapper.text()).toContain('目录结果已截断')
    await wrapper.findAll('[role="treeitem"]')[0].find('.file-workbench-tree-row').trigger('click')
    await wrapper
      .findAll('[role="treeitem"]')[1]
      .find('.file-workbench-tree-row')
      .trigger('keydown.enter')

    expect(store.directoryFor('docs')?.expanded).toBe(true)
    expect(ensureDirectoryLoaded).toHaveBeenCalledWith(runtimeApi, 'docs')
    expect(openDocument).toHaveBeenCalledWith(runtimeApi, 'notes.txt')
  })

  it('emits back when the return control is activated', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    store.openWorkspace({ mode: 'local' }, 'workspace-1')
    const wrapper = mount(WorkspaceFileTree, {
      props: { api: runtimeApi, workspaceName: 'Workspace' },
      global: { plugins: [i18n] },
    })

    await wrapper.findAll('header button')[0].trigger('click')
    expect(wrapper.emitted('back')).toEqual([[]])
  })

  it('emits back from the sessions switch in the left panel footer', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    store.openWorkspace({ mode: 'local' }, 'workspace-1')
    const wrapper = mount(WorkspaceFileTree, {
      props: { api: runtimeApi, workspaceName: 'Workspace' },
      global: { plugins: [i18n] },
    })

    const switchButton = wrapper.find('footer button')
    expect(switchButton.attributes('disabled')).toBeUndefined()
    await switchButton.trigger('click')
    expect(wrapper.emitted('back')).toEqual([[]])
  })

  it('emits root and entry mutations and refreshes only when the user asks', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    const file = entry('notes.txt', FileEntryKind.FILE_ENTRY_KIND_FILE)
    vi.mocked(runtimeApi.listFiles).mockResolvedValueOnce({
      directory: entry('', FileEntryKind.FILE_ENTRY_KIND_DIRECTORY),
      items: [file],
      truncated: false,
    })
    store.openWorkspace({ mode: 'local' }, 'workspace-1')
    await store.refreshDirectory(runtimeApi, '')
    const refresh = vi.spyOn(store, 'refreshDirectory').mockResolvedValue(null)
    const wrapper = mount(WorkspaceFileTree, {
      props: { api: runtimeApi, workspaceName: 'Workspace' },
      global: { plugins: [i18n] },
    })

    expect(refresh).not.toHaveBeenCalled()
    await wrapper.findAll('header button')[3].trigger('click')
    await wrapper.findAll('.file-workbench-tree-action')[0].trigger('click')

    expect(refresh).toHaveBeenCalledWith(runtimeApi, '')
    expect(wrapper.emitted('action')).toEqual([[{ type: 'rename', entry: file }]])
  })
})
