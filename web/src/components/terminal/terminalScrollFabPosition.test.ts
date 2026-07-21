import { afterEach, describe, expect, it } from 'vitest'
import {
  TERMINAL_SCROLL_FAB_POSITION_KEY,
  clearFabPosition,
  clampFabPosition,
  isDragPastThreshold,
  loadFabPosition,
  parseFabPosition,
  saveFabPosition,
} from './terminalScrollFabPosition'

describe('terminalScrollFabPosition', () => {
  afterEach(() => {
    clearFabPosition()
  })

  it('clamps into shell bounds', () => {
    expect(
      clampFabPosition(
        { left: -20, top: 500 },
        { width: 400, height: 300 },
        { width: 40, height: 100 },
      ),
    ).toEqual({ left: 0, top: 200 })
  })

  it('allows free movement when rail fits shell', () => {
    expect(
      clampFabPosition(
        { left: 12, top: 34 },
        { width: 400, height: 300 },
        { width: 40, height: 100 },
      ),
    ).toEqual({ left: 12, top: 34 })
  })

  it('detects drag past threshold', () => {
    expect(isDragPastThreshold(3, 3)).toBe(false)
    expect(isDragPastThreshold(4, 4)).toBe(true)
    expect(isDragPastThreshold(5, 0)).toBe(true)
  })

  it('parses and rejects invalid storage payloads', () => {
    expect(parseFabPosition(null)).toBeNull()
    expect(parseFabPosition('not-json')).toBeNull()
    expect(parseFabPosition('{"left":1}')).toBeNull()
    expect(parseFabPosition('{"left":1,"top":"2"}')).toBeNull()
    expect(parseFabPosition('{"left":10,"top":20}')).toEqual({ left: 10, top: 20 })
  })

  it('round-trips position through localStorage', () => {
    saveFabPosition({ left: 48, top: 96 })
    expect(loadFabPosition()).toEqual({ left: 48, top: 96 })
    expect(window.localStorage.getItem(TERMINAL_SCROLL_FAB_POSITION_KEY)).toContain('48')
    clearFabPosition()
    expect(loadFabPosition()).toBeNull()
  })
})
