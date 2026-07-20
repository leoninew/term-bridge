import { proxyRefs, ref } from 'vue'
import type { Shortcut } from '../gen/proto/termbridge/agent/v1/shortcut'
import type {
  SessionSummary,
  Workspace as WorkspaceSummary,
} from '../gen/proto/termbridge/agent/v1/workspace'

export type CommandSource = 'shortcut' | 'command'

export type ValidCreateSessionDraft = {
  workspaceId: string | null
  name: string
  cwd: string
  commandText: string
  commandSource: CommandSource
  shortcutIdSnapshot: string
  shortcutNameSnapshot: string
}

export type CreateSessionDraftError = 'name-required' | 'cwd-required' | 'command-required'

export function useCreateSessionDraft() {
  const workspace = ref<WorkspaceSummary | null>(null)
  const sessionName = ref('')
  const cwd = ref('')
  const commandText = ref('')
  const commandSource = ref<CommandSource>('shortcut')
  const selectedShortcutId = ref<string | null>(null)
  const selectedShortcutName = ref<string | null>(null)

  function reset(
    nextWorkspace: WorkspaceSummary | undefined,
    defaultName: string,
    shortcuts: Shortcut[],
  ) {
    workspace.value = nextWorkspace ?? null
    sessionName.value = defaultName
    cwd.value = nextWorkspace?.path ?? '~'
    commandText.value = ''
    commandSource.value = 'shortcut'
    selectedShortcutId.value = null
    selectedShortcutName.value = null
    selectShortcut(shortcuts[0])
  }

  function populateFromSession(
    session: SessionSummary,
    nextWorkspace: WorkspaceSummary,
    existingSessionNames: Iterable<string>,
    shortcuts: Shortcut[],
  ) {
    workspace.value = nextWorkspace
    sessionName.value = nextCopiedSessionName(session.name, existingSessionNames)
    cwd.value = session.cwd
    commandText.value = session.command

    const matchingShortcut = shortcuts.find(
      (shortcut) => shortcut.id === session.shortcut_id_snapshot,
    )
    if (
      session.command_source === 'shortcut' &&
      matchingShortcut &&
      session.shortcut_name_snapshot
    ) {
      commandSource.value = 'shortcut'
      selectedShortcutId.value = matchingShortcut.id
      selectedShortcutName.value = session.shortcut_name_snapshot
      return
    }

    commandSource.value = 'command'
    selectedShortcutId.value = null
    selectedShortcutName.value = null
  }

  function selectCommandSource(source: CommandSource, shortcuts: Shortcut[]) {
    commandSource.value = source
    if (source === 'command') return

    selectShortcut(
      shortcuts.find((shortcut) => shortcut.id === selectedShortcutId.value) ?? shortcuts[0],
    )
    if (shortcuts.length === 0) selectedShortcutId.value = null
  }

  function selectShortcut(value: Shortcut | undefined) {
    if (!value) return
    selectedShortcutId.value = value.id
    selectedShortcutName.value = value.name
    commandText.value = value.command
  }

  function validate():
    | { value: ValidCreateSessionDraft; error: null }
    | { value: null; error: CreateSessionDraftError } {
    const name = sessionName.value.trim()
    const trimmedCwd = cwd.value.trim()
    if (!name) {
      return { value: null, error: 'name-required' }
    }
    if (!trimmedCwd) {
      return { value: null, error: 'cwd-required' }
    }
    if (!commandText.value.trim()) {
      return { value: null, error: 'command-required' }
    }
    if (
      commandSource.value === 'shortcut' &&
      (!selectedShortcutId.value || !selectedShortcutName.value)
    ) {
      return { value: null, error: 'command-required' }
    }
    return {
      value: {
        workspaceId: workspace.value?.id ?? null,
        name,
        cwd: trimmedCwd,
        commandText: commandText.value,
        commandSource: commandSource.value,
        shortcutIdSnapshot:
          commandSource.value === 'shortcut' ? (selectedShortcutId.value ?? '') : '',
        shortcutNameSnapshot:
          commandSource.value === 'shortcut' ? (selectedShortcutName.value ?? '') : '',
      },
      error: null,
    }
  }

  return proxyRefs({
    workspace,
    sessionName,
    cwd,
    commandText,
    commandSource,
    selectedShortcutId,
    selectedShortcutName,
    reset,
    populateFromSession,
    selectCommandSource,
    selectShortcut,
    validate,
  })
}

function nextCopiedSessionName(name: string, existingSessionNames: Iterable<string>) {
  const existingNames = new Set(existingSessionNames)
  const baseName = baseCopiedSessionName(name)
  let sequence = 1
  while (existingNames.has(`${baseName}_${sequence}`)) {
    sequence += 1
  }
  return `${baseName}_${sequence}`
}

function baseCopiedSessionName(name: string) {
  const match = /^(.*)_(\d+)$/.exec(name)
  return match?.[1] ?? name
}
