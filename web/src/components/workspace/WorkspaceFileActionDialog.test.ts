// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { i18n } from '../../i18n'
import WorkspaceFileActionDialog from './WorkspaceFileActionDialog.vue'

const baseProps = {
  open: true,
  title: 'Rename',
  description: 'notes.txt',
  label: 'Name',
  placeholder: 'new name',
  requiresValue: true,
  initialValue: 'notes.txt',
}

const dialogStubs = {
  DialogRoot: { template: '<div><slot /></div>' },
  DialogPortal: { template: '<div><slot /></div>' },
  DialogOverlay: { template: '<div />' },
  DialogContent: { template: '<div><slot /></div>' },
  DialogTitle: { template: '<h2><slot /></h2>' },
  DialogClose: { template: '<span><slot /></span>' },
}

describe('WorkspaceFileActionDialog', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('resets the controlled value on open and rejects whitespace-only confirmation', async () => {
    const wrapper = mount(WorkspaceFileActionDialog, {
      attachTo: document.body,
      props: baseProps,
      global: { plugins: [i18n], stubs: dialogStubs },
    })
    const input = wrapper.find('input')
    expect((input.element as HTMLInputElement).value).toBe('notes.txt')

    await input.setValue('   ')
    expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeDefined()

    await wrapper.setProps({ open: false })
    await wrapper.setProps({ initialValue: 'renamed.txt', open: true })
    expect((input.element as HTMLInputElement).value).toBe('renamed.txt')

    await input.setValue('final.txt')
    await wrapper.find('form').trigger('submit')
    expect(wrapper.emitted('confirm')).toEqual([['final.txt']])
  })

  it('uses the destructive confirmation style for confirm-only actions', async () => {
    const wrapper = mount(WorkspaceFileActionDialog, {
      attachTo: document.body,
      props: { ...baseProps, requiresValue: false, danger: true },
      global: { plugins: [i18n], stubs: dialogStubs },
    })

    const confirm = wrapper.findAll('button').find((button) => button.text() === '确定')
    expect(confirm?.classes()).toContain('button-danger')
    await confirm?.trigger('click')
    expect(wrapper.emitted('confirm')).toEqual([['']])
  })
})
