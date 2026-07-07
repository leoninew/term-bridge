import { proxyRefs, ref } from 'vue'
import { commandFromText } from '../protocol/terminal'
import type { Workspace as WorkspaceSummary } from '../gen/proto/termbridge/agent/v1/workspace'

export type ValidCreateSessionDraft = {
  workspaceId: string | null
  name: string
  cwd: string
  command: string[]
}

export type CreateSessionDraftError = 'name-required' | 'cwd-required' | 'command-required'

export function useCreateSessionDraft() {
  const workspace = ref<WorkspaceSummary | null>(null)
  const sessionName = ref('')
  const cwd = ref('')
  const commandText = ref(defaultCommand())

  function reset(nextWorkspace: WorkspaceSummary | undefined, defaultName: string) {
    workspace.value = nextWorkspace ?? null
    sessionName.value = defaultName
    cwd.value = nextWorkspace?.path ?? '~'
    commandText.value = defaultCommand()
  }

  function validate():
    | { value: ValidCreateSessionDraft; error: null }
    | { value: null; error: CreateSessionDraftError } {
    const name = sessionName.value.trim()
    const trimmedCwd = cwd.value.trim()
    const command = commandFromText(commandText.value)
    if (!name) {
      return { value: null, error: 'name-required' }
    }
    if (!trimmedCwd) {
      return { value: null, error: 'cwd-required' }
    }
    if (command.length === 0) {
      return { value: null, error: 'command-required' }
    }
    return {
      value: {
        workspaceId: workspace.value?.id ?? null,
        name,
        cwd: trimmedCwd,
        command,
      },
      error: null,
    }
  }

  return proxyRefs({
    workspace,
    sessionName,
    cwd,
    commandText,
    reset,
    validate,
  })
}

export function defaultCommand() {
  const userAgent = window.navigator.userAgent.toLowerCase()
  if (userAgent.includes('windows')) {
    return 'cmd'
  }
  if (userAgent.includes('mac os') || userAgent.includes('macintosh')) {
    return 'zsh'
  }
  if (userAgent.includes('linux')) {
    return 'bash'
  }
  return 'bash'
}
