import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import { terminalDebug } from './diagnostics'
import { attachTouchScroll } from './touchScroll'
import { clampTerminalSize } from '../../protocol/terminal'
import type { AppTheme } from '../../store/theme'

type ResizeCallback = (cols: number, rows: number) => void

type FitReason =
  | 'open'
  | 'open-raf'
  | 'open-settle'
  | 'invalid-retry'
  | 'fonts-ready'
  | 'resize-observer'
  | 'explicit'
  | 'started'
  | string

export type TerminalScrollEdges = {
  atTop: boolean
  atBottom: boolean
}

type XtermController = {
  terminal: Terminal
  open: (element: HTMLElement) => void
  fit: (reason?: FitReason) => void
  focus: () => void
  scrollToTop: () => void
  scrollToBottom: () => void
  getScrollEdges: () => TerminalScrollEdges
  setScrollEdgesListener: (listener: ((edges: TerminalScrollEdges) => void) | null) => void
  write: (data: Uint8Array) => void
  pendingBytes: () => number
  setTheme: (theme: AppTheme) => void
  dispose: () => void
}

type XtermDiagnostics = {
  source: 'live' | 'history' | 'measure'
  sessionId?: string | null
}

const maxPendingBytes = 4 * 1024 * 1024
/** Extra fits after open so Splitter/flex layout can settle without waiting for F12. */
const openSettleDelaysMs = [50, 100, 250, 500, 1000]
const maxInvalidSizeRetries = 12
const invalidSizeRetryBaseMs = 50
/** Reject fits while Splitter/flex still has a collapsed workbench (log showed ~73px → 6 cols). */
const minTrustedClientWidth = 160
const minTrustedClientHeight = 80
const minTrustedCols = 20
const minTrustedRows = 8

export function xtermThemeFor(theme: AppTheme) {
  if (theme === 'light') {
    return {
      background: '#ffffff',
      foreground: '#0f172a',
      cursor: '#0f172a',
      selectionBackground: '#bfdbfe',
    }
  }

  return {
    background: '#020617',
    foreground: '#d7deea',
    cursor: '#f8fafc',
    selectionBackground: '#1e3a5f',
  }
}

/** Hide terminal cursor (DECTCEM). Used for read-only history replay. */
const hideCursorSequence = new TextEncoder().encode('\u001b[?25l')

function terminalOptions(theme: AppTheme = 'dark', mode: XtermDiagnostics['source'] = 'live') {
  const readOnly = mode === 'history' || mode === 'measure'
  return {
    cursorBlink: !readOnly,
    cursorInactiveStyle: readOnly ? ('none' as const) : undefined,
    disableStdin: readOnly,
    fontFamily: 'Cascadia Mono, Consolas, monospace',
    fontSize: 13,
    scrollback: 5000,
    theme: xtermThemeFor(theme),
  }
}

function captureElementGeometry(element: HTMLElement | null | undefined) {
  if (!element) {
    return { missing: true as const }
  }

  const rect = element.getBoundingClientRect()
  const style = window.getComputedStyle(element)
  const parent = element.parentElement
  const parentRect = parent?.getBoundingClientRect()
  const shell = element.closest('.terminal-shell') as HTMLElement | null
  const shellRect = shell?.getBoundingClientRect()
  const screen = element.querySelector('.xterm-screen') as HTMLElement | null
  const viewport = element.querySelector('.xterm-viewport') as HTMLElement | null
  const canvas = element.querySelector('canvas') as HTMLCanvasElement | null

  return {
    missing: false as const,
    tag: element.tagName,
    className: element.className,
    clientWidth: element.clientWidth,
    clientHeight: element.clientHeight,
    offsetWidth: element.offsetWidth,
    offsetHeight: element.offsetHeight,
    scrollWidth: element.scrollWidth,
    scrollHeight: element.scrollHeight,
    rect: {
      width: Number(rect.width.toFixed(2)),
      height: Number(rect.height.toFixed(2)),
      top: Number(rect.top.toFixed(2)),
      left: Number(rect.left.toFixed(2)),
    },
    style: {
      display: style.display,
      visibility: style.visibility,
      overflow: style.overflow,
      padding: style.padding,
      boxSizing: style.boxSizing,
    },
    parent: parent
      ? {
          className: parent.className,
          clientWidth: parent.clientWidth,
          clientHeight: parent.clientHeight,
          rect: parentRect
            ? {
                width: Number(parentRect.width.toFixed(2)),
                height: Number(parentRect.height.toFixed(2)),
              }
            : null,
        }
      : null,
    shell: shell
      ? {
          clientWidth: shell.clientWidth,
          clientHeight: shell.clientHeight,
          rect: shellRect
            ? {
                width: Number(shellRect.width.toFixed(2)),
                height: Number(shellRect.height.toFixed(2)),
              }
            : null,
        }
      : null,
    xtermScreen: screen
      ? { clientWidth: screen.clientWidth, clientHeight: screen.clientHeight }
      : null,
    xtermViewport: viewport
      ? { clientWidth: viewport.clientWidth, clientHeight: viewport.clientHeight }
      : null,
    canvas: canvas
      ? {
          width: canvas.width,
          height: canvas.height,
          styleWidth: canvas.style.width,
          styleHeight: canvas.style.height,
          clientWidth: canvas.clientWidth,
          clientHeight: canvas.clientHeight,
        }
      : null,
    window: {
      innerWidth: window.innerWidth,
      innerHeight: window.innerHeight,
      devicePixelRatio: window.devicePixelRatio,
    },
  }
}

function describeUnsettledLayout(
  element: HTMLElement | null | undefined,
  cols: number,
  rows: number,
): string | null {
  if (!element) {
    return 'missing-host'
  }
  if (element.clientWidth < minTrustedClientWidth) {
    return `narrow-width:${element.clientWidth}`
  }
  if (element.clientHeight < minTrustedClientHeight) {
    return `short-height:${element.clientHeight}`
  }
  if (cols < minTrustedCols) {
    return `few-cols:${cols}`
  }
  if (rows < minTrustedRows) {
    return `few-rows:${rows}`
  }
  return null
}

export function measureXtermSize(element: HTMLElement): { cols: number; rows: number } | null {
  const terminal = new Terminal(terminalOptions('dark', 'measure'))
  const fitAddon = new FitAddon()
  terminal.loadAddon(fitAddon)
  try {
    terminal.open(element)
    const dimensions = fitAddon.proposeDimensions()
    if (!dimensions || dimensions.cols < 1 || dimensions.rows < 1) {
      terminalDebug('xterm.measure.invalid-size', {
        source: 'measure',
        geometry: captureElementGeometry(element),
      })
      return null
    }
    const measuredCols = dimensions.cols
    const measuredRows = dimensions.rows
    const size = clampTerminalSize({ cols: measuredCols, rows: measuredRows })
    terminal.resize(size.cols, size.rows)
    terminalDebug('xterm.measure', {
      source: 'measure',
      measuredCols,
      measuredRows,
      cols: size.cols,
      rows: size.rows,
      geometry: captureElementGeometry(element),
    })
    return size
  } finally {
    terminal.dispose()
  }
}

export function createXterm(
  onData: (data: string) => void,
  onBinary: (data: string) => void,
  onResize: ResizeCallback,
  diagnostics: XtermDiagnostics,
  theme: AppTheme = 'dark',
): XtermController {
  const readOnly = diagnostics.source === 'history'
  const terminal = new Terminal(terminalOptions(theme, diagnostics.source))
  const fitAddon = new FitAddon()
  const webLinksAddon = new WebLinksAddon()
  const disposables = readOnly ? [] : [terminal.onData(onData), terminal.onBinary(onBinary)]
  let hostElement: HTMLElement | undefined
  let observer: ResizeObserver | undefined
  let lastCols = 0
  let lastRows = 0
  let fitSeq = 0
  let resizeTimer: number | undefined
  let invalidRetryTimer: number | undefined
  let invalidRetryCount = 0
  let disposed = false
  let scrollEdgesListener: ((edges: TerminalScrollEdges) => void) | null = null
  const scrollDisposables: Array<{ dispose: () => void }> = []
  let detachTouchScroll: (() => void) | undefined

  function getScrollEdges(): TerminalScrollEdges {
    const buffer = terminal.buffer.active
    return {
      atTop: buffer.viewportY <= 0,
      atBottom: buffer.viewportY >= buffer.baseY,
    }
  }

  function emitScrollEdges() {
    if (disposed || !scrollEdgesListener) {
      return
    }
    scrollEdgesListener(getScrollEdges())
  }

  const settleTimers: number[] = []
  const writeQueue: Uint8Array[] = []
  let writing = false
  let pending = 0
  let writeCount = 0
  let drainCount = 0

  terminal.loadAddon(fitAddon)
  terminal.loadAddon(webLinksAddon)

  function diagnosticDetails(extra: Record<string, unknown> = {}) {
    return {
      source: diagnostics.source,
      sessionId: diagnostics.sessionId ?? undefined,
      ...extra,
    }
  }

  function clearSettleTimers() {
    for (const timer of settleTimers) {
      window.clearTimeout(timer)
    }
    settleTimers.length = 0
  }

  function clearInvalidRetryTimer() {
    if (invalidRetryTimer !== undefined) {
      window.clearTimeout(invalidRetryTimer)
      invalidRetryTimer = undefined
    }
  }

  function scheduleInvalidSizeRetry(reason: FitReason) {
    if (disposed || invalidRetryCount >= maxInvalidSizeRetries || invalidRetryTimer !== undefined) {
      if (invalidRetryCount >= maxInvalidSizeRetries) {
        terminalDebug(
          'xterm.fit.invalid-size.give-up',
          diagnosticDetails({
            reason,
            attempt: invalidRetryCount,
            geometry: captureElementGeometry(hostElement),
            terminalCols: terminal.cols,
            terminalRows: terminal.rows,
          }),
        )
      }
      return
    }
    const delay = invalidSizeRetryBaseMs * 2 ** Math.min(invalidRetryCount, 4)
    invalidRetryCount += 1
    terminalDebug(
      'xterm.fit.invalid-size.retry',
      diagnosticDetails({
        reason,
        attempt: invalidRetryCount,
        delayMs: delay,
        geometry: captureElementGeometry(hostElement),
      }),
    )
    invalidRetryTimer = window.setTimeout(() => {
      invalidRetryTimer = undefined
      if (!disposed) {
        emitResize(`invalid-retry#${invalidRetryCount}`)
      }
    }, delay)
  }

  function emitResize(reason: FitReason = 'explicit') {
    if (disposed) {
      return
    }
    fitSeq += 1
    const seq = fitSeq
    const beforeCols = terminal.cols
    const beforeRows = terminal.rows
    const geometryBefore = captureElementGeometry(hostElement)
    const dimensions = fitAddon.proposeDimensions()

    terminalDebug(
      'xterm.fit.attempt',
      diagnosticDetails({
        reason,
        seq,
        beforeCols,
        beforeRows,
        lastCols,
        lastRows,
        proposed: dimensions ? { cols: dimensions.cols, rows: dimensions.rows } : null,
        geometry: geometryBefore,
      }),
    )

    if (!dimensions || dimensions.cols < 1 || dimensions.rows < 1) {
      terminalDebug(
        'xterm.fit.invalid-size',
        diagnosticDetails({
          reason,
          seq,
          attempt: invalidRetryCount,
          proposed: dimensions ?? null,
          geometry: geometryBefore,
          terminalCols: beforeCols,
          terminalRows: beforeRows,
        }),
      )
      scheduleInvalidSizeRetry(reason)
      return
    }
    const measuredCols = dimensions.cols
    const measuredRows = dimensions.rows
    const unsettled = describeUnsettledLayout(hostElement, measuredCols, measuredRows)
    if (unsettled) {
      terminalDebug(
        'xterm.fit.unsettled',
        diagnosticDetails({
          reason,
          seq,
          unsettled,
          proposed: { cols: measuredCols, rows: measuredRows },
          geometry: geometryBefore,
          terminalCols: beforeCols,
          terminalRows: beforeRows,
          lastCols,
          lastRows,
        }),
      )
      scheduleInvalidSizeRetry(reason)
      return
    }

    invalidRetryCount = 0
    clearInvalidRetryTimer()

    const size = clampTerminalSize({ cols: measuredCols, rows: measuredRows })
    const terminalResized = terminal.cols !== size.cols || terminal.rows !== size.rows
    if (terminalResized) {
      terminal.resize(size.cols, size.rows)
    }

    const geometryAfter = captureElementGeometry(hostElement)
    const notified = size.cols !== lastCols || size.rows !== lastRows

    if (notified) {
      terminalDebug(
        'xterm.resize',
        diagnosticDetails({
          reason,
          seq,
          previousCols: lastCols,
          previousRows: lastRows,
          measuredCols,
          measuredRows,
          cols: size.cols,
          rows: size.rows,
          terminalResized,
          geometryAfter,
        }),
      )
      lastCols = size.cols
      lastRows = size.rows
      onResize(size.cols, size.rows)
      emitScrollEdges()
      return
    }

    terminalDebug(
      'xterm.fit.unchanged',
      diagnosticDetails({
        reason,
        seq,
        measuredCols,
        measuredRows,
        cols: size.cols,
        rows: size.rows,
        terminalResized,
        terminalCols: terminal.cols,
        terminalRows: terminal.rows,
        geometryAfter,
      }),
    )
    emitScrollEdges()
  }

  function scheduleResize(reason: FitReason = 'resize-observer') {
    if (disposed) {
      return
    }
    if (resizeTimer !== undefined) {
      window.clearTimeout(resizeTimer)
    }
    resizeTimer = window.setTimeout(() => {
      resizeTimer = undefined
      emitResize(reason)
    }, 100)
  }

  function scheduleOpenSettleFits() {
    // Double rAF waits for the next paint after Splitter/flex layout.
    window.requestAnimationFrame(() => {
      if (disposed) {
        return
      }
      window.requestAnimationFrame(() => {
        if (disposed) {
          return
        }
        emitResize('open-raf')
        for (const delayMs of openSettleDelaysMs) {
          const timer = window.setTimeout(() => {
            if (!disposed) {
              emitResize(`open-settle:${delayMs}ms`)
            }
          }, delayMs)
          settleTimers.push(timer)
        }
      })
    })
  }

  function scheduleFontsReadyFit() {
    const fonts = document.fonts
    if (!fonts?.ready) {
      terminalDebug('xterm.fit.fonts-ready.unavailable', diagnosticDetails())
      return
    }
    terminalDebug(
      'xterm.fit.fonts-ready.wait',
      diagnosticDetails({
        status: fonts.status,
      }),
    )
    void fonts.ready
      .then(() => {
        if (!disposed) {
          terminalDebug(
            'xterm.fit.fonts-ready',
            diagnosticDetails({
              status: fonts.status,
              geometry: captureElementGeometry(hostElement),
            }),
          )
          emitResize('fonts-ready')
        }
      })
      .catch((error: unknown) => {
        terminalDebug(
          'xterm.fit.fonts-ready.error',
          diagnosticDetails({
            error: error instanceof Error ? error.message : String(error),
          }),
        )
      })
  }

  function drainWrites() {
    if (writing) {
      return
    }
    const next = writeQueue.shift()
    if (!next) {
      return
    }
    writing = true
    terminal.write(next, () => {
      pending -= next.byteLength
      writing = false
      drainCount += 1
      terminalDebug(
        'xterm.write.drained',
        {
          ...diagnosticDetails(),
          chunkBytes: next.byteLength,
          pendingBytes: pending,
          queuedChunks: writeQueue.length,
        },
        { sample: drainCount },
      )
      if (writeQueue.length === 0 && readOnly) {
        terminal.write(hideCursorSequence)
      }
      drainWrites()
    })
  }

  return {
    terminal,
    open(element: HTMLElement) {
      hostElement = element
      terminalDebug(
        'xterm.open',
        diagnosticDetails({
          geometry: captureElementGeometry(element),
          terminalCols: terminal.cols,
          terminalRows: terminal.rows,
        }),
      )
      terminal.open(element)
      terminalDebug(
        'xterm.open.after-dom',
        diagnosticDetails({
          geometry: captureElementGeometry(element),
          terminalCols: terminal.cols,
          terminalRows: terminal.rows,
        }),
      )
      emitResize('open')
      scheduleOpenSettleFits()
      scheduleFontsReadyFit()
      observer = new ResizeObserver((entries) => {
        const entry = entries[0]
        const contentRect = entry?.contentRect
        terminalDebug(
          'xterm.resize-observer',
          diagnosticDetails({
            contentRect: contentRect
              ? {
                  width: Number(contentRect.width.toFixed(2)),
                  height: Number(contentRect.height.toFixed(2)),
                  top: Number(contentRect.top.toFixed(2)),
                  left: Number(contentRect.left.toFixed(2)),
                }
              : null,
            geometry: captureElementGeometry(hostElement),
            lastCols,
            lastRows,
            terminalCols: terminal.cols,
            terminalRows: terminal.rows,
          }),
        )
        scheduleResize('resize-observer')
      })
      observer.observe(element)
      scrollDisposables.push(
        terminal.onScroll(() => {
          emitScrollEdges()
        }),
      )
      // line feed / buffer growth can change baseY without a user scroll
      scrollDisposables.push(
        terminal.onWriteParsed(() => {
          emitScrollEdges()
        }),
      )
      // Custom touch → scrollLines path for mobile/coarse devices (CSS pan-y alone is unreliable).
      detachTouchScroll?.()
      detachTouchScroll = attachTouchScroll(element, terminal, () => {
        if (!disposed) {
          emitScrollEdges()
        }
      })
      emitScrollEdges()
      if (readOnly) {
        terminal.write(hideCursorSequence)
      } else {
        terminal.focus()
      }
    },
    fit(reason: FitReason = 'explicit') {
      emitResize(reason)
    },
    focus() {
      if (disposed) {
        return
      }
      terminal.focus()
    },
    scrollToTop() {
      if (disposed) {
        return
      }
      terminal.scrollToTop()
      emitScrollEdges()
    },
    scrollToBottom() {
      if (disposed) {
        return
      }
      terminal.scrollToBottom()
      emitScrollEdges()
    },
    getScrollEdges() {
      return getScrollEdges()
    },
    setScrollEdgesListener(listener: ((edges: TerminalScrollEdges) => void) | null) {
      scrollEdgesListener = listener
      if (!disposed && listener) {
        listener(getScrollEdges())
      }
    },
    write(data: Uint8Array) {
      writeCount += 1
      if (pending + data.byteLength > maxPendingBytes) {
        terminalDebug('xterm.write.dropped', {
          ...diagnosticDetails(),
          chunkBytes: data.byteLength,
          pendingBytes: pending,
          queuedChunks: writeQueue.length,
        })
        terminal.writeln('\r\n[termbridge] output backlog exceeded; dropping browser-side chunk')
        return
      }
      const copy = new Uint8Array(data)
      writeQueue.push(copy)
      pending += copy.byteLength
      terminalDebug(
        'xterm.write.queued',
        {
          ...diagnosticDetails(),
          chunkBytes: copy.byteLength,
          pendingBytes: pending,
          queuedChunks: writeQueue.length,
        },
        { sample: writeCount },
      )
      drainWrites()
    },
    pendingBytes() {
      return pending
    },
    setTheme(theme: AppTheme) {
      terminal.options.theme = xtermThemeFor(theme)
    },
    dispose() {
      disposed = true
      terminalDebug(
        'xterm.dispose',
        diagnosticDetails({
          pendingBytes: pending,
          queuedChunks: writeQueue.length,
          lastCols,
          lastRows,
          terminalCols: terminal.cols,
          terminalRows: terminal.rows,
          geometry: captureElementGeometry(hostElement),
        }),
      )
      if (resizeTimer !== undefined) {
        window.clearTimeout(resizeTimer)
        resizeTimer = undefined
      }
      clearInvalidRetryTimer()
      clearSettleTimers()
      observer?.disconnect()
      detachTouchScroll?.()
      detachTouchScroll = undefined
      for (const disposable of scrollDisposables) {
        disposable.dispose()
      }
      scrollDisposables.length = 0
      scrollEdgesListener = null
      for (const disposable of disposables) {
        disposable.dispose()
      }
      terminal.dispose()
      hostElement = undefined
    },
  }
}
