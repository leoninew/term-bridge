// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { i18n } from '../../i18n'
import type { FileDocumentState } from '../../store/fileWorkbench'
import WorkspaceEditorTabs from './WorkspaceEditorTabs.vue'

function document(path: string, overrides: Partial<FileDocumentState> = {}): FileDocumentState {
  return {
    key: `local:workspace:${path}`,
    path,
    entry: null,
    baseRevision: '',
    originalText: '',
    draftText: '',
    dirty: false,
    loading: false,
    saving: false,
    conflict: null,
    error: null,
    ...overrides,
  }
}

describe('WorkspaceEditorTabs', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders document state and keeps close activation separate from tab activation', async () => {
    const active = document('notes.txt', { dirty: true })
    const pending = document('pending.txt', { loading: true })
    const wrapper = mount(WorkspaceEditorTabs, {
      props: { documents: [active, pending], activeDocument: active },
      global: { plugins: [i18n] },
    })

    const tabs = wrapper.findAll('[role="tab"]')
    expect(wrapper.find('[role="tablist"]').exists()).toBe(true)
    expect(tabs[0].attributes('aria-selected')).toBe('true')
    expect(tabs[1].attributes('aria-selected')).toBe('false')
    expect(tabs[0].text()).toContain('●')
    expect(tabs[1].text()).toContain('…')

    await tabs[1].trigger('keydown', { key: 'Enter' })
    await wrapper.findAll('button.file-workbench-tab-close')[0].trigger('click')

    expect(wrapper.emitted('activate')).toEqual([['pending.txt']])
    expect(wrapper.emitted('close')).toEqual([['notes.txt']])
  })
})
