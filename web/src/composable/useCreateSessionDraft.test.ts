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
