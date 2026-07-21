/**
 * Touch-driven xterm buffer scroll for mobile / coarse-pointer devices.
 * Maps vertical finger movement to Terminal.scrollLines instead of relying
 * on native DOM scroll of the canvas-backed viewport.
 */

export const TOUCH_SCROLL_LOCK_PX = 10

export type TouchScrollTerminal = {
  rows: number
  scrollLines: (amount: number) => void
}

export type TouchScrollSession = {
  activeId: number | null
  startX: number
  startY: number
  lastY: number
  residual: number
  mode: 'pending' | 'scroll' | 'ignore'
}

export function createTouchScrollSession(): TouchScrollSession {
  return {
    activeId: null,
    startX: 0,
    startY: 0,
    lastY: 0,
    residual: 0,
    mode: 'pending',
  }
}

/** Enable custom touch scroll when the device is coarse/touch primary (not fine desktop-only). */
export function shouldEnableTouchScroll(
  env: {
    matchMedia?: (query: string) => { matches: boolean }
    maxTouchPoints?: number
  } = typeof window !== 'undefined'
    ? { matchMedia: window.matchMedia.bind(window), maxTouchPoints: navigator.maxTouchPoints }
    : {},
): boolean {
  try {
    if (env.matchMedia?.('(pointer: coarse)').matches) {
      return true
    }
    if (env.matchMedia?.('(hover: none)').matches) {
      return true
    }
  } catch {
    // matchMedia can throw in restricted environments
  }
  return (env.maxTouchPoints ?? 0) > 0
}

export function estimateLineHeight(hostHeight: number, rows: number): number {
  const safeRows = Math.max(rows, 1)
  if (hostHeight > 0) {
    return hostHeight / safeRows
  }
  return 16
}

/**
 * Accumulate vertical finger delta into whole buffer line scrolls.
 * Finger down (positive dy) → earlier history (negative scrollLines).
 * Returns remaining sub-line residual in pixels.
 */
export function applyTouchScrollDelta(
  terminal: TouchScrollTerminal,
  residualPx: number,
  deltaY: number,
  lineHeight: number,
): { residualPx: number; scrolledLines: number } {
  const lh = lineHeight > 0 ? lineHeight : 16
  let residual = residualPx - deltaY
  const lines = Math.trunc(residual / lh)
  if (lines !== 0) {
    terminal.scrollLines(lines)
    residual -= lines * lh
  }
  return { residualPx: residual, scrolledLines: lines }
}

/**
 * Bind non-passive touch handlers on the xterm host so preventDefault works.
 * Returns a disposer; no-op when touch scroll should not enable.
 */
export function attachTouchScroll(
  host: HTMLElement,
  terminal: TouchScrollTerminal,
  onScrolled: () => void,
): () => void {
  if (!shouldEnableTouchScroll()) {
    return () => undefined
  }

  const session = createTouchScrollSession()
  host.classList.add('xterm-touch-scroll')

  function resetSession() {
    session.activeId = null
    session.residual = 0
    session.mode = 'pending'
  }

  function onTouchStart(event: TouchEvent) {
    if (event.touches.length !== 1) {
      session.mode = 'ignore'
      return
    }
    const touch = event.touches[0]
    session.activeId = touch.identifier
    session.startX = touch.clientX
    session.startY = touch.clientY
    session.lastY = touch.clientY
    session.residual = 0
    session.mode = 'pending'
  }

  function onTouchMove(event: TouchEvent) {
    if (session.activeId === null || session.mode === 'ignore') {
      return
    }
    const touch = Array.from(event.touches).find((item) => item.identifier === session.activeId)
    if (!touch) {
      return
    }

    if (session.mode === 'pending') {
      const totalDx = touch.clientX - session.startX
      const totalDy = touch.clientY - session.startY
      if (Math.abs(totalDx) < TOUCH_SCROLL_LOCK_PX && Math.abs(totalDy) < TOUCH_SCROLL_LOCK_PX) {
        return
      }
      if (Math.abs(totalDy) >= Math.abs(totalDx)) {
        session.mode = 'scroll'
        session.lastY = touch.clientY
        session.residual = 0
      } else {
        session.mode = 'ignore'
      }
      return
    }

    if (session.mode !== 'scroll') {
      return
    }

    if (event.cancelable) {
      event.preventDefault()
    }

    const deltaY = touch.clientY - session.lastY
    session.lastY = touch.clientY
    const lineHeight = estimateLineHeight(host.clientHeight, terminal.rows)
    const result = applyTouchScrollDelta(terminal, session.residual, deltaY, lineHeight)
    session.residual = result.residualPx
    if (result.scrolledLines !== 0) {
      onScrolled()
    }
  }

  function onTouchEnd(event: TouchEvent) {
    if (session.activeId === null) {
      return
    }
    const stillActive = Array.from(event.touches).some(
      (item) => item.identifier === session.activeId,
    )
    if (!stillActive) {
      resetSession()
    }
  }

  const passive = { passive: true } as const
  const active = { passive: false } as const
  host.addEventListener('touchstart', onTouchStart, passive)
  host.addEventListener('touchmove', onTouchMove, active)
  host.addEventListener('touchend', onTouchEnd, passive)
  host.addEventListener('touchcancel', onTouchEnd, passive)

  return () => {
    host.removeEventListener('touchstart', onTouchStart)
    host.removeEventListener('touchmove', onTouchMove)
    host.removeEventListener('touchend', onTouchEnd)
    host.removeEventListener('touchcancel', onTouchEnd)
    host.classList.remove('xterm-touch-scroll')
    resetSession()
  }
}
