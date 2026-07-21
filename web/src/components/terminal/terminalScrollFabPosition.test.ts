import { afterEach, describe, expect, it } from 'vitest'
import {
  TERMINAL_SCROLL_FAB_POSITION_KEY,
  absoluteToAnchor,
  absoluteToStyle,
  anchorToAbsolute,
  anchorToStyle,
  clearFabPosition,
  clampAbsolutePosition,
  isDragPastThreshold,
  loadFabAnchor,
  parseFabAnchor,
  saveFabAnchor,
} from './terminalScrollFabPosition'

describe('terminalScrollFabPosition', () => {
  afterEach(() => {
    clearFabPosition()
  })

  it('clamps absolute coords into shell bounds', () => {
    expect(
      clampAbsolutePosition(
        { left: -20, top: 500 },
        { width: 400, height: 300 },
        { width: 40, height: 100 },
      ),
    ).toEqual({ left: 0, top: 200 })
  })

  it('pins to nearest edges as px offsets (not width ratios)', () => {
    expect(
      absoluteToAnchor(
        { left: 340, top: 180 },
        { width: 400, height: 300 },
        { width: 40, height: 100 },
      ),
    ).toEqual({ xEdge: 'right', yEdge: 'bottom', x: 20, y: 20 })

    expect(
      absoluteToAnchor(
        { left: 12, top: 24 },
        { width: 400, height: 300 },
        { width: 40, height: 100 },
      ),
    ).toEqual({ xEdge: 'left', yEdge: 'top', x: 12, y: 24 })
  })

  it('keeps edge offset when shell width changes', () => {
    const anchor = { xEdge: 'right' as const, yEdge: 'bottom' as const, x: 20, y: 16 }
    const size = { width: 40, height: 100 }

    expect(anchorToAbsolute(anchor, { width: 800, height: 400 }, size)).toEqual({
      left: 740,
      top: 284,
    })
    expect(anchorToAbsolute(anchor, { width: 300, height: 400 }, size)).toEqual({
      left: 240,
      top: 284,
    })
  })

  it('anchor style uses right/bottom when pinned there', () => {
    expect(
      anchorToStyle(
        { xEdge: 'right', yEdge: 'bottom', x: 18, y: 12 },
        { width: 500, height: 300 },
        { width: 40, height: 100 },
      ),
    ).toEqual({
      left: 'auto',
      right: '18px',
      top: 'auto',
      bottom: '12px',
    })
  })

  it('absolute style forces left/top during drag', () => {
    expect(absoluteToStyle({ left: 10, top: 20 })).toEqual({
      left: '10px',
      right: 'auto',
      top: '20px',
      bottom: 'auto',
    })
  })

  it('detects drag past threshold', () => {
    expect(isDragPastThreshold(3, 3)).toBe(false)
    expect(isDragPastThreshold(4, 4)).toBe(true)
    expect(isDragPastThreshold(5, 0)).toBe(true)
  })

  it('parses only edge-anchor payloads; rejects unknown shapes', () => {
    expect(parseFabAnchor(null)).toBeNull()
    expect(parseFabAnchor('not-json')).toBeNull()
    expect(parseFabAnchor('{"left":1}')).toBeNull()
    expect(parseFabAnchor('{"left":1476.125,"top":180}')).toBeNull()
    expect(parseFabAnchor('{"xEdge":"right","yEdge":"bottom","x":20,"y":12}')).toEqual({
      xEdge: 'right',
      yEdge: 'bottom',
      x: 20,
      y: 12,
    })
  })

  it('round-trips edge anchor through localStorage', () => {
    saveFabAnchor({ xEdge: 'right', yEdge: 'bottom', x: 22, y: 14 })
    expect(loadFabAnchor()).toEqual({ xEdge: 'right', yEdge: 'bottom', x: 22, y: 14 })
    expect(window.localStorage.getItem(TERMINAL_SCROLL_FAB_POSITION_KEY)).toContain('right')
    clearFabPosition()
    expect(loadFabAnchor()).toBeNull()
  })
})
