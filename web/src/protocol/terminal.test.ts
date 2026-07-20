import { describe, expect, it } from 'vitest'
import { clampTerminalSize } from './terminal'

describe('clampTerminalSize', () => {
  it('keeps measured terminal size without shrinking by a safety margin', () => {
    expect(clampTerminalSize({ cols: 120, rows: 32 })).toEqual({ cols: 120, rows: 32 })
  })

  it('does not shrink below protocol minimums', () => {
    expect(clampTerminalSize({ cols: 1, rows: 1 })).toEqual({ cols: 1, rows: 1 })
  })
})
