import { describe, expect, it, vi } from 'vitest'
import {
  applyTouchScrollDelta,
  estimateLineHeight,
  shouldEnableTouchScroll,
  TOUCH_SCROLL_LOCK_PX,
} from './touchScroll'

describe('shouldEnableTouchScroll', () => {
  it('enables for coarse pointer', () => {
    expect(
      shouldEnableTouchScroll({
        matchMedia: (query) => ({ matches: query === '(pointer: coarse)' }),
        maxTouchPoints: 0,
      }),
    ).toBe(true)
  })

  it('enables for hover none', () => {
    expect(
      shouldEnableTouchScroll({
        matchMedia: (query) => ({ matches: query === '(hover: none)' }),
        maxTouchPoints: 0,
      }),
    ).toBe(true)
  })

  it('enables when maxTouchPoints > 0', () => {
    expect(
      shouldEnableTouchScroll({
        matchMedia: () => ({ matches: false }),
        maxTouchPoints: 5,
      }),
    ).toBe(true)
  })

  it('disables for fine desktop-only pointer without touch', () => {
    expect(
      shouldEnableTouchScroll({
        matchMedia: () => ({ matches: false }),
        maxTouchPoints: 0,
      }),
    ).toBe(false)
  })
})

describe('estimateLineHeight', () => {
  it('divides host height by rows', () => {
    expect(estimateLineHeight(320, 20)).toBe(16)
  })

  it('falls back when host height is zero', () => {
    expect(estimateLineHeight(0, 20)).toBe(16)
  })
})

describe('applyTouchScrollDelta', () => {
  it('maps finger-up to positive scrollLines (later content)', () => {
    const scrollLines = vi.fn()
    const result = applyTouchScrollDelta({ rows: 20, scrollLines }, 0, -32, 16)
    expect(scrollLines).toHaveBeenCalledWith(2)
    expect(result.scrolledLines).toBe(2)
    expect(result.residualPx).toBe(0)
  })

  it('maps finger-down to negative scrollLines (earlier history)', () => {
    const scrollLines = vi.fn()
    const result = applyTouchScrollDelta({ rows: 20, scrollLines }, 0, 20, 16)
    expect(scrollLines).toHaveBeenCalledWith(-1)
    expect(result.scrolledLines).toBe(-1)
    expect(result.residualPx).toBe(-4)
  })

  it('accumulates residual below one line without scrolling', () => {
    const scrollLines = vi.fn()
    const first = applyTouchScrollDelta({ rows: 20, scrollLines }, 0, -8, 16)
    expect(scrollLines).not.toHaveBeenCalled()
    expect(first.residualPx).toBe(8)

    const second = applyTouchScrollDelta({ rows: 20, scrollLines }, first.residualPx, -10, 16)
    expect(scrollLines).toHaveBeenCalledWith(1)
    expect(second.scrolledLines).toBe(1)
    expect(second.residualPx).toBe(2)
  })
})

describe('TOUCH_SCROLL_LOCK_PX', () => {
  it('uses a small lock threshold for vertical capture', () => {
    expect(TOUCH_SCROLL_LOCK_PX).toBeGreaterThanOrEqual(8)
    expect(TOUCH_SCROLL_LOCK_PX).toBeLessThanOrEqual(16)
  })
})
