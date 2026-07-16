// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { GitLayer, GitState } from '../../gen/proto/termbridge/agent/v1/git'
import { i18n } from '../../i18n'
import type { FileGitRuntimeApi } from '../../features/files/runtime'
import { useFileWorkbenchStore } from '../../store/fileWorkbench'
import GitChangesPanel from './GitChangesPanel.vue'

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

describe('GitChangesPanel', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('keeps Git idle until refresh and then requests only a supported layer diff', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    store.openWorkspace({ mode: 'local' }, 'workspace-1')
    store.gitStatus = {
      state: GitState.GIT_STATE_AVAILABLE,
      changes: [
        {
          path: 'notes.txt',
          original_path: '',
          index_status: 'M',
          worktree_status: 'M',
          untracked: false,
          unmerged: false,
          available_layers: [GitLayer.GIT_LAYER_UNSTAGED],
        },
      ],
      message: '',
    }
    const refreshStatus = vi.spyOn(store, 'refreshGitStatus').mockResolvedValue(null)
    const refreshDiff = vi.spyOn(store, 'refreshGitDiff').mockResolvedValue(null)
    const select = vi.spyOn(store, 'selectGitChange')
    const wrapper = mount(GitChangesPanel, {
      props: { api: runtimeApi },
      global: { plugins: [i18n], stubs: { GitDiffEditor: true } },
    })

    expect(refreshStatus).not.toHaveBeenCalled()
    expect(refreshDiff).not.toHaveBeenCalled()

    await wrapper.find('header button').trigger('click')
    await wrapper.find('li button').trigger('click')

    expect(refreshStatus).toHaveBeenCalledWith(runtimeApi)
    expect(select).toHaveBeenCalledWith('notes.txt', GitLayer.GIT_LAYER_UNSTAGED)
    expect(refreshDiff).toHaveBeenCalledWith(runtimeApi)
  })

  it('shows status errors and does not request a diff for a rejected selection', async () => {
    const store = useFileWorkbenchStore()
    const runtimeApi = api()
    store.openWorkspace({ mode: 'local' }, 'workspace-1')
    store.gitStatusError = 'Git command failed'
    const refreshDiff = vi.spyOn(store, 'refreshGitDiff').mockResolvedValue(null)
    vi.spyOn(store, 'selectGitChange').mockReturnValue(false)
    const wrapper = mount(GitChangesPanel, {
      props: { api: runtimeApi },
      global: { plugins: [i18n], stubs: { GitDiffEditor: true } },
    })

    expect(wrapper.text()).toContain('Git command failed')
    expect(refreshDiff).not.toHaveBeenCalled()
  })
})
