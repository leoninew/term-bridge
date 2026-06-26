import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defaultCommand, useCreateSessionDraft } from './useCreateSessionDraft'

function stubUserAgent(userAgent: string) {
  vi.stubGlobal('window', { navigator: { userAgent } })
}

describe('useCreateSessionDraft', () => {
  beforeEach(() => {
    stubUserAgent('Mozilla/5.0 (Windows NT 10.0; Win64; x64)')
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('chooses a default command from the browser platform', () => {
    expect(defaultCommand()).toBe('cmd')

    stubUserAgent('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)')
    expect(defaultCommand()).toBe('zsh')

    stubUserAgent('Mozilla/5.0 (X11; Linux x86_64)')
    expect(defaultCommand()).toBe('bash')
  })

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
    expect(draft.commandText).toBe('cmd')
  })

  it('validates and normalizes create-session input', () => {
    const draft = useCreateSessionDraft()

    draft.reset(undefined, 'New Session')
    draft.cwd = '  /tmp  '
    draft.commandText = 'bash -lc echo'

    expect(draft.validate()).toEqual({
      value: {
        workspaceId: null,
        name: 'New Session',
        cwd: '/tmp',
        command: ['bash', '-lc', 'echo'],
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
