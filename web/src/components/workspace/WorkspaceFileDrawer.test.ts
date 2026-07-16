// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { Workspace } from '../../gen/proto/termbridge/agent/v1/workspace'
import type { FileGitRuntimeApi } from '../../features/files/runtime'
import { i18n } from '../../i18n'
import { useFileWorkbenchStore } from '../../store/fileWorkbench'
import { useNotificationsStore } from '../../store/notifications'
import WorkspaceFileDrawer from './WorkspaceFileDrawer.vue'

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

const workspace: Workspace = {
  id: 'workspace-1',
  name: 'Workspace',
  path: 'D:/workspace',
  updated_at: undefined,
}

const stubs = {
  GitChangesPanel: true,
  WorkspaceEditorTabs: { template: '<div><slot /></div>' },
  WorkspaceFileTree: true,
  WorkspaceTextEditor: true,
  WorkspaceFileActionDialog: true,
}

describe('WorkspaceFileDrawer', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('opens the scoped workspace and explicitly loads root', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    const openWorkspace = vi.spyOn(store, 'openWorkspace')
    const ensureRoot = vi.spyOn(store, 'ensureDirectoryLoaded').mockResolvedValue(null)
    mount(WorkspaceFileDrawer, {
      props: {
        open: true,
        workspace,
        target: { mode: 'cloud', deviceId: 'device-1' },
        api: runtimeApi,
      },
      global: { plugins: [i18n], stubs },
    })
    await vi.waitFor(() => expect(ensureRoot).toHaveBeenCalled())

    expect(openWorkspace).toHaveBeenCalledWith(
      { mode: 'cloud', deviceId: 'device-1' },
      'workspace-1',
    )
    expect(ensureRoot).toHaveBeenCalledWith(runtimeApi, '')
  })

  it('emits closed only after its leave transform completes', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    vi.spyOn(store, 'ensureDirectoryLoaded').mockResolvedValue(null)
    const wrapper = mount(WorkspaceFileDrawer, {
      props: { open: true, workspace, target: { mode: 'local' }, api: runtimeApi },
      global: { plugins: [i18n], stubs },
    })

    await wrapper.setProps({ open: false })
    await wrapper.trigger('transitionend', { propertyName: 'opacity' })
    expect(wrapper.emitted('closed')).toBeUndefined()

    await wrapper.trigger('transitionend', { propertyName: 'transform' })
    expect(wrapper.emitted('closed')).toEqual([[]])
  })

  it('reports a clipboard failure while preserving the active draft', async () => {
    const store = useFileWorkbenchStore()
    const notifications = useNotificationsStore()
    const runtimeApi = api()
    store.openWorkspace({ mode: 'local' }, 'workspace-1')
    store.documents.push({
      key: 'document:notes.txt',
      path: 'notes.txt',
      entry: null,
      baseRevision: 'r1',
      originalText: 'base',
      draftText: 'draft',
      dirty: true,
      loading: false,
      saving: false,
      conflict: { type: 'revision_conflict', current_entry: undefined },
      error: 'revision conflict',
    })
    store.activeDocumentKey = 'document:notes.txt'
    vi.stubGlobal('navigator', {
      clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) },
    })
    const notifyError = vi.spyOn(notifications, 'notifyError')
    const wrapper = mount(WorkspaceFileDrawer, {
      props: { open: true, workspace, target: { mode: 'local' }, api: runtimeApi },
      global: { plugins: [i18n], stubs },
    })

    await wrapper.findAll('.file-workbench-conflict-actions button')[1].trigger('click')
    expect(notifyError).toHaveBeenCalled()
    expect(store.activeDocument?.draftText).toBe('draft')
  })
})
