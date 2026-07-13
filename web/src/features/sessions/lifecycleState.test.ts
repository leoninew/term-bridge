import { describe, expect, it } from 'vitest'
import { lifecycleIndicatorClassName, lifecycleStateClassName } from './lifecycleState'

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

describe('lifecycleIndicatorClassName', () => {
  it('uses the running token only for running sessions', () => {
    expect(lifecycleIndicatorClassName('running')).toBe('bg-[var(--color-state-running-text)]')
  })

  it.each(['stopped', 'failed', 'starting'])(
    'uses the same inactive token for %s sessions',
    (state) => {
      expect(lifecycleIndicatorClassName(state)).toBe('bg-[var(--color-state-stopped-text)]')
    },
  )
})
