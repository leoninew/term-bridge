import { describe, expect, it } from 'vitest'
import type { Shortcut } from '../gen/proto/termbridge/agent/v1/shortcut'
import type {
  SessionSummary,
  Workspace as WorkspaceSummary,
} from '../gen/proto/termbridge/agent/v1/workspace'
import { useCreateSessionDraft } from './useCreateSessionDraft'

function shortcut(id: string, name: string, command: string): Shortcut {
  return {
    id,
    name,
    command,
    description: undefined,
    icon: undefined,
    enabled: undefined,
    tags: [],
    last_used_at: undefined,
    created_at: undefined,
    updated_at: undefined,
  }
}

const workspace: WorkspaceSummary = {
  id: 'workspace-1',
  name: 'Workspace One',
  path: '/work/one',
  updated_at: undefined,
}

function session(overrides: Partial<SessionSummary> = {}): SessionSummary {
  return {
    id: 'session-1',
    workspace_id: workspace.id,
    name: 'Build',
    command: 'npm run build',
    cwd: workspace.path,
    lifecycle_state: 'running',
    attachment_state: '',
    updated_at: undefined,
    command_source: 'command',
    shortcut_id_snapshot: '',
    shortcut_name_snapshot: '',
    ...overrides,
  }
}

describe('useCreateSessionDraft', () => {
  it('resets from an optional workspace and selects the first shortcut command', () => {
    const draft = useCreateSessionDraft()
    const firstShortcut = shortcut('shortcut-1', 'Review', 'codex review')

    draft.reset(
      {
        id: 'workspace-1',
        name: 'Workspace One',
        path: '/work/one',
        updated_at: '2026-06-24T00:00:00Z',
      },
      'New Session',
      [firstShortcut],
    )

    expect(draft.workspace?.id).toBe('workspace-1')
    expect(draft.sessionName).toBe('New Session')
    expect(draft.cwd).toBe('/work/one')
    expect(draft.commandText).toBe('codex review')
    expect(draft.commandSource).toBe('shortcut')
    expect(draft.selectedShortcutId).toBe('shortcut-1')
    expect(draft.validate()).toEqual({
      value: {
        workspaceId: 'workspace-1',
        name: 'New Session',
        cwd: '/work/one',
        commandText: 'codex review',
        commandSource: 'shortcut',
        shortcutIdSnapshot: 'shortcut-1',
        shortcutNameSnapshot: 'Review',
      },
      error: null,
    })
  })

  it('preserves raw command text while validating create-session input', () => {
    const draft = useCreateSessionDraft()

    draft.reset(undefined, 'New Session', [])
    draft.commandSource = 'command'
    draft.cwd = '  /tmp  '
    draft.commandText = 'ccs list --filter "my project"'

    expect(draft.validate()).toEqual({
      value: {
        workspaceId: null,
        name: 'New Session',
        cwd: '/tmp',
        commandText: 'ccs list --filter "my project"',
        commandSource: 'command',
        shortcutIdSnapshot: '',
        shortcutNameSnapshot: '',
      },
      error: null,
    })
  })

  it('restores the previously selected shortcut after a direct command round trip', () => {
    const draft = useCreateSessionDraft()
    const shortcuts = [
      shortcut('shortcut-1', 'Review', 'codex review'),
      shortcut('shortcut-2', 'Run', 'ccs run c1'),
    ]

    draft.reset(undefined, 'New Session', shortcuts)
    draft.selectShortcut(shortcuts[1])
    draft.selectCommandSource('command', shortcuts)
    draft.commandText = 'manual command'
    draft.selectCommandSource('shortcut', shortcuts)

    expect(draft.commandSource).toBe('shortcut')
    expect(draft.selectedShortcutId).toBe('shortcut-2')
    expect(draft.commandText).toBe('ccs run c1')
  })

  it('falls back to the first available shortcut when the remembered shortcut is missing', () => {
    const draft = useCreateSessionDraft()
    const shortcuts = [
      shortcut('shortcut-1', 'Review', 'codex review'),
      shortcut('shortcut-2', 'Run', 'ccs run c1'),
    ]

    draft.reset(undefined, 'New Session', [])
    draft.selectedShortcutId = 'removed-shortcut'
    draft.commandText = 'manual command'
    draft.selectCommandSource('shortcut', shortcuts)

    expect(draft.selectedShortcutId).toBe('shortcut-1')
    expect(draft.commandText).toBe('codex review')
  })

  it('keeps a direct command when no shortcuts are available', () => {
    const draft = useCreateSessionDraft()

    draft.reset(undefined, 'New Session', [])
    draft.commandSource = 'command'
    draft.commandText = 'ccs run c1'
    draft.selectedShortcutId = 'removed-shortcut'
    draft.selectCommandSource('shortcut', [])

    expect(draft.commandSource).toBe('shortcut')
    expect(draft.selectedShortcutId).toBeNull()
    expect(draft.commandText).toBe('ccs run c1')
  })

  it('prefills a copied direct-command session with the first available name suffix', () => {
    const draft = useCreateSessionDraft()

    draft.populateFromSession(
      session(),
      workspace,
      ['Build', 'Build_2', 'Build_3'],
      [shortcut('shortcut-1', 'Review', 'codex review')],
    )

    expect(draft.workspace?.id).toBe(workspace.id)
    expect(draft.sessionName).toBe('Build_1')
    expect(draft.cwd).toBe(workspace.path)
    expect(draft.commandSource).toBe('command')
    expect(draft.commandText).toBe('npm run build')
    expect(draft.selectedShortcutId).toBeNull()
  })

  it('increments a trailing numeric suffix when copying a numbered session name', () => {
    const draft = useCreateSessionDraft()

    draft.populateFromSession(session({ name: 'Build_1' }), workspace, ['Build_1'], [])

    expect(draft.sessionName).toBe('Build_2')
  })

  it('fills the first free suffix after stripping a trailing numeric suffix', () => {
    const draft = useCreateSessionDraft()

    draft.populateFromSession(session({ name: 'Build_3' }), workspace, ['Build_1', 'Build_3'], [])

    expect(draft.sessionName).toBe('Build_2')
  })

  it('starts at suffix 1 when only the source name exists in the provided name set', () => {
    const draft = useCreateSessionDraft()

    // Callers should pass same-workspace names only; cross-workspace names must not consume suffixes.
    draft.populateFromSession(session({ name: '开发' }), workspace, ['开发'], [])

    expect(draft.sessionName).toBe('开发_1')
  })

  it('prefills a copied shortcut session from its current shortcut', () => {
    const draft = useCreateSessionDraft()
    const copiedShortcut = shortcut('shortcut-1', 'Review', 'codex review')

    draft.populateFromSession(
      session({
        name: 'Review',
        command: 'codex review --saved',
        command_source: 'shortcut',
        shortcut_id_snapshot: copiedShortcut.id,
        shortcut_name_snapshot: copiedShortcut.name,
      }),
      workspace,
      ['Review', 'Review_1'],
      [copiedShortcut],
    )

    expect(draft.sessionName).toBe('Review_2')
    expect(draft.commandSource).toBe('shortcut')
    expect(draft.commandText).toBe('codex review --saved')
    expect(draft.selectedShortcutId).toBe(copiedShortcut.id)
    expect(draft.selectedShortcutName).toBe(copiedShortcut.name)
    expect(draft.validate()).toMatchObject({
      value: {
        commandSource: 'shortcut',
        shortcutIdSnapshot: copiedShortcut.id,
        shortcutNameSnapshot: copiedShortcut.name,
      },
      error: null,
    })
  })

  it('falls back to a direct command when the copied shortcut is unavailable', () => {
    const draft = useCreateSessionDraft()

    draft.populateFromSession(
      session({
        command_source: 'shortcut',
        shortcut_id_snapshot: 'removed-shortcut',
        shortcut_name_snapshot: 'Removed',
      }),
      workspace,
      [],
      [],
    )

    expect(draft.commandSource).toBe('command')
    expect(draft.commandText).toBe('npm run build')
    expect(draft.selectedShortcutId).toBeNull()
  })

  it('returns specific validation errors', () => {
    const draft = useCreateSessionDraft()

    draft.sessionName = ''
    expect(draft.validate()).toEqual({ value: null, error: 'name-required' })

    draft.sessionName = 'Named Session'
    draft.cwd = '   '
    expect(draft.validate()).toEqual({ value: null, error: 'cwd-required' })

    draft.cwd = '/tmp'
    draft.commandText = '   '
    expect(draft.validate()).toEqual({ value: null, error: 'command-required' })
  })
})
