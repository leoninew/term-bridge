import { describe, expect, it } from 'vitest'
import { useCreateSessionDraft } from './useCreateSessionDraft'

describe('useCreateSessionDraft', () => {
  it('resets from an optional workspace and default session name', () => {
    const draft = useCreateSessionDraft()

    draft.reset(
      {
        id: 'workspace-1',
        name: 'Workspace One',
        path: '/work/one',
        updated_at: '2026-06-24T00:00:00Z',
      },
      'New Session',
    )

    expect(draft.workspace?.id).toBe('workspace-1')
    expect(draft.sessionName).toBe('New Session')
    expect(draft.cwd).toBe('/work/one')
    expect(draft.commandText).toBe('')
    expect(draft.commandSource).toBe('shortcut')
    expect(draft.selectedShortcutId).toBeNull()
  })

  it('preserves raw command text while validating create-session input', () => {
    const draft = useCreateSessionDraft()

    draft.reset(undefined, 'New Session')
    draft.cwd = '  /tmp  '
    draft.commandText = 'ccs list --filter "my project"'

    expect(draft.validate()).toEqual({
      value: {
        workspaceId: null,
        name: 'New Session',
        cwd: '/tmp',
        commandText: 'ccs list --filter "my project"',
      },
      error: null,
    })
  })

  it('copies the selected shortcut command without changing its raw text', () => {
    const draft = useCreateSessionDraft()
    const command = `codex --dangerously-bypass-approvals-and-sandbox -c "review changes"`

    draft.reset(undefined, 'New Session')
    draft.selectShortcut({
      id: 'shortcut-1',
      name: 'Review',
      command,
      description: undefined,
      created_at: undefined,
      updated_at: undefined,
    })
    draft.commandSource = 'command'

    expect(draft.selectedShortcutId).toBe('shortcut-1')
    expect(draft.commandText).toBe(command)
    expect(draft.commandSource).toBe('command')
    draft.selectedShortcutId = null
    expect(draft.commandText).toBe(command)
    expect(draft.validate()).toEqual({
      value: {
        workspaceId: null,
        name: 'New Session',
        cwd: '~',
        commandText: command,
      },
      error: null,
    })
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
