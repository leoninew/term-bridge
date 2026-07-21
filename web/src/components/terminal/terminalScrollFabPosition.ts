import {
  readLocalStorageValue,
  writeLocalStorageValue,
  removeLocalStorageValue,
} from '../../store/storage'

export const TERMINAL_SCROLL_FAB_POSITION_KEY = 'termbridge.terminal.scrollFab.position'

export type HorizontalEdge = 'left' | 'right'
export type VerticalEdge = 'top' | 'bottom'

/**
 * Edge-anchored position. Pin to the nearer edges at drag end; store px distance
 * from those edges so shell resize keeps the rail by that corner.
 */
export type TerminalScrollFabAnchor = {
  xEdge: HorizontalEdge
  yEdge: VerticalEdge
  /** Distance in CSS px from the chosen horizontal edge to the rail box. */
  x: number
  /** Distance in CSS px from the chosen vertical edge to the rail box. */
  y: number
}

/** Transient absolute coords used while dragging / measuring. */
export type TerminalScrollFabAbsolute = {
  left: number
  top: number
}

const dragThresholdPx = 5

export function isDragPastThreshold(dx: number, dy: number, threshold = dragThresholdPx): boolean {
  return Math.hypot(dx, dy) >= threshold
}

function clampNumber(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max)
}

export function clampAbsolutePosition(
  position: TerminalScrollFabAbsolute,
  bounds: { width: number; height: number },
  size: { width: number; height: number },
): TerminalScrollFabAbsolute {
  const maxLeft = Math.max(0, bounds.width - size.width)
  const maxTop = Math.max(0, bounds.height - size.height)
  return {
    left: clampNumber(position.left, 0, maxLeft),
    top: clampNumber(position.top, 0, maxTop),
  }
}

/** Convert absolute left/top into nearest-edge anchors (px offsets, not ratios). */
export function absoluteToAnchor(
  position: TerminalScrollFabAbsolute,
  bounds: { width: number; height: number },
  size: { width: number; height: number },
): TerminalScrollFabAnchor {
  const clamped = clampAbsolutePosition(position, bounds, size)
  const maxLeft = Math.max(0, bounds.width - size.width)
  const maxTop = Math.max(0, bounds.height - size.height)
  const right = maxLeft - clamped.left
  const bottom = maxTop - clamped.top
  const xEdge: HorizontalEdge = clamped.left <= right ? 'left' : 'right'
  const yEdge: VerticalEdge = clamped.top <= bottom ? 'top' : 'bottom'
  return {
    xEdge,
    yEdge,
    x: xEdge === 'left' ? clamped.left : right,
    y: yEdge === 'top' ? clamped.top : bottom,
  }
}

export function anchorToAbsolute(
  anchor: TerminalScrollFabAnchor,
  bounds: { width: number; height: number },
  size: { width: number; height: number },
): TerminalScrollFabAbsolute {
  const maxLeft = Math.max(0, bounds.width - size.width)
  const maxTop = Math.max(0, bounds.height - size.height)
  const x = clampNumber(anchor.x, 0, maxLeft)
  const y = clampNumber(anchor.y, 0, maxTop)
  return {
    left: anchor.xEdge === 'left' ? x : maxLeft - x,
    top: anchor.yEdge === 'top' ? y : maxTop - y,
  }
}

/**
 * Inline style for a custom-positioned rail. Sets only the two anchored sides so
 * shell resize keeps px edge distance.
 */
export function anchorToStyle(
  anchor: TerminalScrollFabAnchor,
  bounds: { width: number; height: number },
  size: { width: number; height: number },
): Record<'left' | 'right' | 'top' | 'bottom', string> {
  const maxX = Math.max(0, bounds.width - size.width)
  const maxY = Math.max(0, bounds.height - size.height)
  const x = clampNumber(anchor.x, 0, maxX)
  const y = clampNumber(anchor.y, 0, maxY)
  return {
    left: anchor.xEdge === 'left' ? `${x}px` : 'auto',
    right: anchor.xEdge === 'right' ? `${x}px` : 'auto',
    top: anchor.yEdge === 'top' ? `${y}px` : 'auto',
    bottom: anchor.yEdge === 'bottom' ? `${y}px` : 'auto',
  }
}

export function absoluteToStyle(
  position: TerminalScrollFabAbsolute,
): Record<'left' | 'right' | 'top' | 'bottom', string> {
  return {
    left: `${position.left}px`,
    right: 'auto',
    top: `${position.top}px`,
    bottom: 'auto',
  }
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

function isHorizontalEdge(value: unknown): value is HorizontalEdge {
  return value === 'left' || value === 'right'
}

function isVerticalEdge(value: unknown): value is VerticalEdge {
  return value === 'top' || value === 'bottom'
}

/** Parse storage: only `{ xEdge, yEdge, x, y }`. Anything else is ignored. */
export function parseFabAnchor(raw: string | null): TerminalScrollFabAnchor | null {
  if (!raw) {
    return null
  }
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    if (
      !isHorizontalEdge(parsed.xEdge) ||
      !isVerticalEdge(parsed.yEdge) ||
      !isFiniteNumber(parsed.x) ||
      !isFiniteNumber(parsed.y) ||
      parsed.x < 0 ||
      parsed.y < 0
    ) {
      return null
    }
    return {
      xEdge: parsed.xEdge,
      yEdge: parsed.yEdge,
      x: parsed.x,
      y: parsed.y,
    }
  } catch {
    return null
  }
}

export function loadFabAnchor(): TerminalScrollFabAnchor | null {
  return parseFabAnchor(readLocalStorageValue(TERMINAL_SCROLL_FAB_POSITION_KEY))
}

export function saveFabAnchor(anchor: TerminalScrollFabAnchor): void {
  writeLocalStorageValue(
    TERMINAL_SCROLL_FAB_POSITION_KEY,
    JSON.stringify({
      xEdge: anchor.xEdge,
      yEdge: anchor.yEdge,
      x: anchor.x,
      y: anchor.y,
    }),
  )
}

export function clearFabPosition(): void {
  removeLocalStorageValue(TERMINAL_SCROLL_FAB_POSITION_KEY)
}
