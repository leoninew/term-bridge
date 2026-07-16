// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { i18n } from '../../i18n'
import type { FileDocumentState } from '../../store/fileWorkbench'

const monacoMocks = vi.hoisted(() => ({
  createModel: vi.fn(),
  createEditor: vi.fn(),
  applyMonacoTheme: vi.fn(),
  fileUri: vi.fn(() => ({ path: '/content/notes.txt' })),
  languageForPath: vi.fn(() => 'typescript'),
}))

vi.mock('../../features/files/monaco', () => ({
  applyMonacoTheme: monacoMocks.applyMonacoTheme,
  editorOptions: {},
  fileUri: monacoMocks.fileUri,
  languageForPath: monacoMocks.languageForPath,
  monaco: {
    KeyCode: { KeyS: 1 },
    KeyMod: { CtrlCmd: 2 },
    editor: { create: monacoMocks.createEditor, createModel: monacoMocks.createModel },
  },
}))

import WorkspaceTextEditor from './WorkspaceTextEditor.vue'

function document(overrides: Partial<FileDocumentState> = {}): FileDocumentState {
  return {
    key: 'document:notes.txt',
    path: 'notes.txt',
    entry: {
      path: 'notes.txt',
      name: 'notes.txt',
      kind: 1,
      size: 1,
      modified_at: undefined,
      revision: 'r1',
    },
    baseRevision: 'r1',
    originalText: 'base',
    draftText: 'draft',
    dirty: true,
    loading: false,
    saving: false,
    conflict: null,
    error: null,
    ...overrides,
  }
}

class ResizeObserverMock {
  observe = vi.fn()
  disconnect = vi.fn()
}

describe('WorkspaceTextEditor', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.stubGlobal('ResizeObserver', ResizeObserverMock)
    const listener = { dispose: vi.fn() }
    const model = {
      getValue: vi.fn(() => 'draft'),
      setValue: vi.fn(),
      onDidChangeContent: vi.fn(() => listener),
      dispose: vi.fn(),
    }
    monacoMocks.createModel.mockReturnValue(model)
    monacoMocks.createEditor.mockReturnValue({
      addAction: vi.fn(),
      layout: vi.fn(),
      dispose: vi.fn(),
    })
  })

  it('creates a scoped model, synchronizes drafts, and disposes resources', async () => {
    const wrapper = mount(WorkspaceTextEditor, {
      props: {
        target: { mode: 'cloud', deviceId: 'device-1' },
        workspaceId: 'workspace-1',
        document: document(),
      },
      global: { plugins: [i18n] },
    })
    await vi.waitFor(() => expect(monacoMocks.createModel).toHaveBeenCalled())

    expect(monacoMocks.languageForPath).toHaveBeenCalledWith('notes.txt')
    expect(monacoMocks.fileUri).toHaveBeenCalledWith(
      { mode: 'cloud', deviceId: 'device-1' },
      'workspace-1',
      'notes.txt',
    )
    expect(monacoMocks.createModel).toHaveBeenCalledWith('draft', 'typescript', {
      path: '/content/notes.txt',
    })
    const saveAction = monacoMocks.createEditor.mock.results[0].value.addAction.mock
      .calls[0][0] as {
      run: () => void
    }
    saveAction.run()
    expect(wrapper.emitted('save')).toEqual([['notes.txt', false]])

    const model = monacoMocks.createModel.mock.results[0].value
    await wrapper.setProps({ document: document({ draftText: 'programmatic' }) })
    expect(model.setValue).toHaveBeenCalledWith('programmatic')
    expect(wrapper.emitted('draft')).toBeUndefined()

    await wrapper.unmount()
    expect(monacoMocks.createEditor.mock.results[0].value.dispose).toHaveBeenCalled()
    expect(model.dispose).toHaveBeenCalled()
  })

  it('does not construct Monaco for loading documents or render file-path chrome', () => {
    const wrapper = mount(WorkspaceTextEditor, {
      props: {
        target: { mode: 'local' },
        workspaceId: 'workspace-1',
        document: document({ loading: true, dirty: false }),
      },
      global: { plugins: [i18n] },
    })

    expect(monacoMocks.createModel).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('正在加载文件')
    expect(wrapper.text()).not.toContain('notes.txt')
  })
})
