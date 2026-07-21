import {
  readLocalStorageValue,
  writeLocalStorageValue,
  removeLocalStorageValue,
} from '../../store/storage'

export const TERMINAL_SCROLL_FAB_POSITION_KEY = 'termbridge.terminal.scrollFab.position'

export type TerminalScrollFabPosition = {
  left: number
  top: number
}

const dragThresholdPx = 5

export function isDragPastThreshold(dx: number, dy: number, threshold = dragThresholdPx): boolean {
  return Math.hypot(dx, dy) >= threshold
}

export function clampFabPosition(
  position: TerminalScrollFabPosition,
  bounds: { width: number; height: number },
  size: { width: number; height: number },
): TerminalScrollFabPosition {
  const maxLeft = Math.max(0, bounds.width - size.width)
  const maxTop = Math.max(0, bounds.height - size.height)
  return {
    left: clampNumber(position.left, 0, maxLeft),
    top: clampNumber(position.top, 0, maxTop),
  }
}

export function parseFabPosition(raw: string | null): TerminalScrollFabPosition | null {
  if (!raw) {
    return null
  }
  try {
    const parsed = JSON.parse(raw) as Partial<TerminalScrollFabPosition>
    if (
      typeof parsed.left !== 'number' ||
      typeof parsed.top !== 'number' ||
      !Number.isFinite(parsed.left) ||
      !Number.isFinite(parsed.top)
    ) {
      return null
    }
    return { left: parsed.left, top: parsed.top }
  } catch {
    return null
  }
}

export function loadFabPosition(): TerminalScrollFabPosition | null {
  return parseFabPosition(readLocalStorageValue(TERMINAL_SCROLL_FAB_POSITION_KEY))
}

export function saveFabPosition(position: TerminalScrollFabPosition): void {
  writeLocalStorageValue(TERMINAL_SCROLL_FAB_POSITION_KEY, JSON.stringify(position))
}

export function clearFabPosition(): void {
  removeLocalStorageValue(TERMINAL_SCROLL_FAB_POSITION_KEY)
}

function clampNumber(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max)
}
