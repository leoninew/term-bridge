import { describe, expect, it, vi } from 'vitest'
import { useAsyncAction } from './useAsyncAction'

describe('useAsyncAction', () => {
  it('returns the task value and clears running after success', async () => {
    const action = useAsyncAction()
    const result = await action.run(async () => 'ok')

    expect(result).toEqual({ ok: true, value: 'ok' })
    expect(action.running).toBe(false)
    expect(action.error).toBeNull()
  })

  it('captures failures, notifies once, and stays idle afterwards', async () => {
    const onError = vi.fn()
    const action = useAsyncAction({ onError })
    const cause = new Error('boom')

    const result = await action.run(async () => {
      throw cause
    })

    expect(result).toEqual({ ok: false, error: 'boom', cause })
    expect(action.running).toBe(false)
    expect(action.error).toBe('boom')
    expect(onError).toHaveBeenCalledWith(cause, 'boom')
  })

  it('rejects concurrent runs while an action is in flight', async () => {
    const action = useAsyncAction()
    let release!: () => void
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })

    const pending = action.run(async () => {
      await gate
      return 'first'
    })

    expect(action.running).toBe(true)
    const blocked = await action.run(async () => 'second')
    expect(blocked.ok).toBe(false)
    if (!blocked.ok) {
      expect(blocked.error).toBe('busy')
    }

    release()
    await expect(pending).resolves.toEqual({ ok: true, value: 'first' })
    expect(action.running).toBe(false)
  })

  it('allows a per-run onError override', async () => {
    const defaultOnError = vi.fn()
    const runOnError = vi.fn()
    const action = useAsyncAction({ onError: defaultOnError })
    const cause = new Error('override')

    await action.run(
      async () => {
        throw cause
      },
      { onError: runOnError },
    )

    expect(runOnError).toHaveBeenCalledWith(cause, 'override')
    expect(defaultOnError).not.toHaveBeenCalled()
  })
})
