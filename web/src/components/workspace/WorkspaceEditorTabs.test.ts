// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { i18n } from '../../i18n'
import type { FileDocumentState } from '../../store/fileWorkbench'
import WorkspaceEditorTabs from './WorkspaceEditorTabs.vue'

function fileDocument(path: string, overrides: Partial<FileDocumentState> = {}): FileDocumentState {
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
    const active = fileDocument('notes.txt', { dirty: true })
    const pending = fileDocument('pending.txt', { loading: true })
    const wrapper = mount(WorkspaceEditorTabs, {
      props: { documents: [active, pending], activeDocument: active },
      global: { plugins: [i18n] },
    })

    const tabs = wrapper.findAll('[role="tab"]')
    expect(wrapper.find('[role="tablist"]').exists()).toBe(true)
    expect(tabs[0].attributes('aria-selected')).toBe('true')
    expect(tabs[1].attributes('aria-selected')).toBe('false')
    expect(tabs[0].text()).toContain('●')

    await tabs[1].trigger('keydown', { key: 'Enter' })
    await wrapper.findAll('button.file-workbench-tab-close')[0].trigger('click')

    expect(wrapper.emitted('activate')).toEqual([['pending.txt']])
    expect(wrapper.emitted('close')).toEqual([['notes.txt']])
  })

  it('keeps tab actions visible and emits Close All and Close Others independently', async () => {
    const active = fileDocument('notes.txt')
    const other = fileDocument('other.txt')
    const wrapper = mount(WorkspaceEditorTabs, {
      props: { documents: [active, other], activeDocument: active },
      global: { plugins: [i18n] },
    })

    const menuTrigger = wrapper.find('button.file-workbench-tab-menu-trigger')
    expect(menuTrigger.exists()).toBe(true)
    expect(menuTrigger.attributes('title')).toBe('文件标签操作')

    await menuTrigger.trigger('click')
    const menuItems = document.body.querySelectorAll('[role="menuitem"]')
    expect(menuItems).toHaveLength(2)
    expect(menuItems[0]?.textContent).toContain('关闭全部')
    expect(menuItems[1]?.textContent).toContain('关闭其他')

    await menuItems[0]?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await menuItems[1]?.dispatchEvent(new MouseEvent('click', { bubbles: true }))

    expect(wrapper.emitted('closeAll')).toEqual([[]])
    expect(wrapper.emitted('closeOthers')).toEqual([[]])
  })

  it('disables unsafe tab actions while a target document is saving', async () => {
    const active = fileDocument('notes.txt')
    const saving = fileDocument('saving.txt', { saving: true })
    const wrapper = mount(WorkspaceEditorTabs, {
      props: { documents: [active, saving], activeDocument: active },
      global: { plugins: [i18n] },
    })

    expect(
      wrapper.findAll('button.file-workbench-tab-close')[1].attributes('disabled'),
    ).toBeDefined()
    await wrapper.find('button.file-workbench-tab-menu-trigger').trigger('click')

    const menuItems = document.body.querySelectorAll('[role="menuitem"]')
    expect(menuItems[0]?.getAttribute('aria-disabled')).toBe('true')
    expect(menuItems[1]?.getAttribute('aria-disabled')).toBe('true')
  })

  it('adds a title only when a file name is truncated', async () => {
    const observer = vi.fn()
    vi.stubGlobal(
      'ResizeObserver',
      class {
        observe = observer
        unobserve = vi.fn()
        disconnect = vi.fn()
      },
    )
    const active = fileDocument('very-long-document-name.txt')
    const wrapper = mount(WorkspaceEditorTabs, {
      props: { documents: [active], activeDocument: active },
      global: { plugins: [i18n] },
    })
    const title = wrapper.find('.file-workbench-tab-title').element
    Object.defineProperty(title, 'clientWidth', { configurable: true, value: 20 })
    Object.defineProperty(title, 'scrollWidth', { configurable: true, value: 100 })

    await wrapper.vm.$nextTick()
    await wrapper.setProps({ documents: [active] })
    expect(wrapper.find('.file-workbench-tab-title').attributes('title')).toBe(
      'very-long-document-name.txt',
    )

    Object.defineProperty(title, 'scrollWidth', { configurable: true, value: 20 })
    await wrapper.setProps({ documents: [] })
    await wrapper.setProps({ documents: [active] })
    expect(wrapper.find('.file-workbench-tab-title').attributes('title')).toBeUndefined()
    vi.unstubAllGlobals()
  })
})
