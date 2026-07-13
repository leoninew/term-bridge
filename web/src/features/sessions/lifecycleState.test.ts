import { describe, expect, it } from 'vitest'
import { lifecycleStateClassName } from './lifecycleState'

describe('lifecycleStateClassName', () => {
  it.each([
    ['running', 'text-[var(--color-state-running-text)]'],
    ['stopped', 'text-[var(--color-state-stopped-text)]'],
    ['failed', 'text-[var(--color-state-stopped-text)]'],
  ])('maps %s to its semantic theme token', (state, expected) => {
    expect(lifecycleStateClassName(state)).toBe(expected)
  })

  it('uses the neutral text token for an unknown lifecycle state', () => {
    expect(lifecycleStateClassName('starting')).toBe('text-[var(--color-text-muted)]')
  })
})
