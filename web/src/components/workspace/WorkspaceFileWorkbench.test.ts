// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { FileEntryKind } from '../../gen/proto/termbridge/agent/v1/file'
import { i18n } from '../../i18n'
import type { FileGitRuntimeApi } from '../../features/files/runtime'
import type { Workspace } from '../../gen/proto/termbridge/agent/v1/workspace'
import { useFileWorkbenchStore } from '../../store/fileWorkbench'
import WorkspaceFileWorkbench from './WorkspaceFileWorkbench.vue'

const workspace: Workspace = {
  id: 'workspace-1',
  name: 'Workspace',
  path: 'D:/workspace',
  updated_at: undefined,
}

function api(): FileGitRuntimeApi {
  return {
    listFiles: vi.fn().mockResolvedValue({
      directory: {
        path: '',
        name: '',
        kind: FileEntryKind.FILE_ENTRY_KIND_DIRECTORY,
        size: 0,
        modified_at: undefined,
        revision: 'r1',
      },
      items: [],
      truncated: false,
    }),
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

describe('WorkspaceFileWorkbench', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('opens the scoped workspace and explicitly loads root', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    const openWorkspace = vi.spyOn(store, 'openWorkspace')
    const ensureDirectoryLoaded = vi.spyOn(store, 'ensureDirectoryLoaded').mockResolvedValue(null)

    mount(WorkspaceFileWorkbench, {
      props: { workspace, target: { mode: 'local' }, api: runtimeApi },
      global: {
        plugins: [i18n, pinia],
        stubs: { WorkspaceTextEditor: true },
      },
    })
    await flushPromises()

    expect(openWorkspace).toHaveBeenCalledWith({ mode: 'local' }, 'workspace-1')
    expect(ensureDirectoryLoaded).toHaveBeenCalled()
  })

  it('emits back when the tree requests return to sessions', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const runtimeApi = api()
    const wrapper = mount(WorkspaceFileWorkbench, {
      props: { workspace, target: { mode: 'local' }, api: runtimeApi },
      global: {
        plugins: [i18n, pinia],
        stubs: { WorkspaceTextEditor: true },
      },
    })
    await flushPromises()

    await wrapper.find('button[aria-label="返回工作区"]').trigger('click')
    expect(wrapper.emitted('back')).toEqual([[]])
  })

  it('closes all clean documents from the tab action', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    vi.mocked(runtimeApi.readFile).mockImplementation(async (_workspaceId, path) => ({
      entry: {
        path,
        name: path,
        kind: FileEntryKind.FILE_ENTRY_KIND_FILE,
        size: 0,
        modified_at: undefined,
        revision: path,
      },
      text: path,
    }))
    const wrapper = mount(WorkspaceFileWorkbench, {
      props: { workspace, target: { mode: 'local' }, api: runtimeApi },
      global: {
        plugins: [i18n, pinia],
        stubs: { WorkspaceTextEditor: true },
      },
    })
    await flushPromises()
    await store.openDocument(runtimeApi, 'first.txt')
    await store.openDocument(runtimeApi, 'second.txt')

    await wrapper.findComponent({ name: 'WorkspaceEditorTabs' }).vm.$emit('closeAll')

    expect(store.documents).toEqual([])
    expect(store.activeDocument).toBeNull()
  })

  it('confirms before closing dirty documents from Close Others', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    vi.mocked(runtimeApi.readFile).mockImplementation(async (_workspaceId, path) => ({
      entry: {
        path,
        name: path,
        kind: FileEntryKind.FILE_ENTRY_KIND_FILE,
        size: 0,
        modified_at: undefined,
        revision: path,
      },
      text: path,
    }))
    const wrapper = mount(WorkspaceFileWorkbench, {
      props: { workspace, target: { mode: 'local' }, api: runtimeApi },
      global: {
        plugins: [i18n, pinia],
        stubs: { WorkspaceTextEditor: true },
      },
    })
    await flushPromises()
    await store.openDocument(runtimeApi, 'first.txt')
    await store.openDocument(runtimeApi, 'second.txt')
    store.setActiveDocument('first.txt')
    store.setDocumentDraft('second.txt', 'draft')

    await wrapper.findComponent({ name: 'WorkspaceEditorTabs' }).vm.$emit('closeOthers')

    expect(store.documents.map((document) => document.path)).toEqual(['first.txt', 'second.txt'])
    expect(document.body.textContent).toContain('关闭未保存文档')
    await document.body.querySelector('button.button-danger')?.dispatchEvent(
      new MouseEvent('click', { bubbles: true }),
    )
    await flushPromises()

    expect(store.documents.map((document) => document.path)).toEqual(['first.txt'])
  })
})
