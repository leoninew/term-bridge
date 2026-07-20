import { describe, expect, it, vi } from 'vitest'
import { clampTerminalKeepAliveConfig } from './terminalKeepAliveConfig'
import { computeHotSessionIds, createTerminalKeepAliveController } from './terminalKeepAlive'

describe('clampTerminalKeepAliveConfig', () => {
  it('uses defaults for missing input', () => {
    expect(clampTerminalKeepAliveConfig(undefined)).toEqual({
      maxHotTerminals: 4,
      disposeDelayMs: 30_000,
    })
  })

  it('clamps out-of-range numbers', () => {
    expect(clampTerminalKeepAliveConfig({ maxHotTerminals: 99, disposeDelayMs: -5 })).toEqual({
      maxHotTerminals: 16,
      disposeDelayMs: 0,
    })
  })

  it('falls back for non-finite values', () => {
    expect(
      clampTerminalKeepAliveConfig({
        maxHotTerminals: Number.NaN,
        disposeDelayMs: Number.POSITIVE_INFINITY,
      }),
    ).toEqual({ maxHotTerminals: 4, disposeDelayMs: 30_000 })
  })
})

describe('computeHotSessionIds', () => {
  it('keeps active session and most recently activated peers', () => {
    const hot = computeHotSessionIds(
      [
        { sessionId: 'a', lastActivatedAt: 1 },
        { sessionId: 'b', lastActivatedAt: 3 },
        { sessionId: 'c', lastActivatedAt: 2 },
        { sessionId: 'd', lastActivatedAt: 4 },
        { sessionId: 'e', lastActivatedAt: 5 },
      ],
      4,
      'a',
    )
    expect(hot[0]).toBe('a')
    expect(hot).toHaveLength(4)
    expect(hot).toEqual(expect.arrayContaining(['a', 'e', 'd', 'b']))
    expect(hot).not.toContain('c')
  })
})

describe('createTerminalKeepAliveController', () => {
  it('evicts oldest hot session and disposes after delay', () => {
    vi.useFakeTimers()
    const controller = createTerminalKeepAliveController({
      config: { maxHotTerminals: 2, disposeDelayMs: 1000 },
      now: () => Date.now(),
    })

    controller.reconcile(['a', 'b'], 'a')
    expect(controller.hotSessionIds.value).toEqual(['a', 'b'])
    expect(controller.mountedSessionIds.value).toEqual(['a', 'b'])

    controller.reconcile(['a', 'b', 'c'], 'c')
    expect(controller.isHot('c')).toBe(true)
    expect(controller.isHot('a') || controller.isHot('b')).toBe(true)
    const cold = ['a', 'b'].find((id) => !controller.isHot(id))
    expect(cold).toBeTruthy()
    expect(controller.isMounted(cold!)).toBe(true)
    expect(controller.isHot(cold!)).toBe(false)

    vi.advanceTimersByTime(999)
    expect(controller.isMounted(cold!)).toBe(true)
    vi.advanceTimersByTime(1)
    expect(controller.isMounted(cold!)).toBe(false)

    controller.disposeAll()
    vi.useRealTimers()
  })

  it('cancels dispose when session returns to hot set', () => {
    vi.useFakeTimers()
    const controller = createTerminalKeepAliveController({
      config: { maxHotTerminals: 1, disposeDelayMs: 1000 },
    })

    controller.reconcile(['a'], 'a')
    controller.reconcile(['a', 'b'], 'b')
    expect(controller.isHot('a')).toBe(false)
    expect(controller.isMounted('a')).toBe(true)

    controller.reconcile(['a', 'b'], 'a')
    expect(controller.isHot('a')).toBe(true)
    vi.advanceTimersByTime(2000)
    expect(controller.isMounted('a')).toBe(true)

    controller.disposeAll()
    vi.useRealTimers()
  })
})
