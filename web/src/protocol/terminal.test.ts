import { describe, expect, it } from 'vitest'
import { fitSafeTerminalSize } from './terminal'

describe('fitSafeTerminalSize', () => {
  it('keeps xterm and PTY away from exact fit boundaries by one cell', () => {
    expect(fitSafeTerminalSize({ cols: 120, rows: 32 })).toEqual({ cols: 119, rows: 31 })
  })

  it('does not shrink below protocol minimums', () => {
    expect(fitSafeTerminalSize({ cols: 1, rows: 1 })).toEqual({ cols: 1, rows: 1 })
  })
})
