// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { GitState, type GitDiffResp } from '../../gen/proto/termbridge/agent/v1/git'
import { i18n } from '../../i18n'

const monacoMocks = vi.hoisted(() => ({
  createModel: vi.fn(),
  createDiffEditor: vi.fn(),
  applyMonacoTheme: vi.fn(),
  fileUri: vi.fn((_, __, path, side) => ({ path: `/${side}/${path}` })),
  languageForPath: vi.fn(() => 'go'),
}))

vi.mock('../../features/files/monaco', () => ({
  applyMonacoTheme: monacoMocks.applyMonacoTheme,
  diffEditorOptions: { readOnly: true, originalEditable: false },
  fileUri: monacoMocks.fileUri,
  languageForPath: monacoMocks.languageForPath,
  monaco: {
    editor: {
      createDiffEditor: monacoMocks.createDiffEditor,
      createModel: monacoMocks.createModel,
    },
  },
}))

import GitDiffEditor from './GitDiffEditor.vue'

class ResizeObserverMock {
  observe = vi.fn()
  disconnect = vi.fn()
}

function diff(overrides: Partial<GitDiffResp> = {}): GitDiffResp {
  return {
    state: GitState.GIT_STATE_AVAILABLE,
    original_path: 'old.go',
    modified_path: 'new.go',
    original_text: 'old',
    modified_text: 'new',
    message: '',
    ...overrides,
  }
}

describe('GitDiffEditor', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.stubGlobal('ResizeObserver', ResizeObserverMock)
    monacoMocks.createModel.mockImplementation((text) => ({ text, dispose: vi.fn() }))
    monacoMocks.createDiffEditor.mockReturnValue({
      setModel: vi.fn(),
      layout: vi.fn(),
      dispose: vi.fn(),
    })
  })

  it('builds read-only scoped original and modified models then disposes them', async () => {
    const wrapper = mount(GitDiffEditor, {
      props: {
        target: { mode: 'local' },
        workspaceId: 'workspace-1',
        diff: diff(),
        loading: false,
        error: null,
      },
      global: { plugins: [i18n] },
    })
    await vi.waitFor(() => expect(monacoMocks.createDiffEditor).toHaveBeenCalled())

    expect(monacoMocks.languageForPath).toHaveBeenCalledWith('new.go')
    expect(monacoMocks.createModel).toHaveBeenNthCalledWith(1, 'old', 'go', {
      path: '/original/old.go',
    })
    expect(monacoMocks.createModel).toHaveBeenNthCalledWith(2, 'new', 'go', {
      path: '/modified/new.go',
    })
    expect(monacoMocks.createDiffEditor.mock.calls[0][1]).toMatchObject({
      readOnly: true,
      originalEditable: false,
    })

    const original = monacoMocks.createModel.mock.results[0].value
    const modified = monacoMocks.createModel.mock.results[1].value
    await wrapper.unmount()
    expect(monacoMocks.createDiffEditor.mock.results[0].value.dispose).toHaveBeenCalled()
    expect(original.dispose).toHaveBeenCalled()
    expect(modified.dispose).toHaveBeenCalled()
  })

  it('does not create Monaco resources for unavailable Git content', () => {
    const wrapper = mount(GitDiffEditor, {
      props: {
        target: { mode: 'local' },
        workspaceId: 'workspace-1',
        diff: diff({ state: GitState.GIT_STATE_BINARY, message: 'Binary file' }),
        loading: false,
        error: null,
      },
      global: { plugins: [i18n] },
    })

    expect(monacoMocks.createModel).not.toHaveBeenCalled()
    expect(monacoMocks.createDiffEditor).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Binary file')
  })
})
