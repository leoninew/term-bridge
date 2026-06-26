import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import { logTerminalDiagnostic, logTerminalDiagnosticSample } from './diagnostics'
import { fitSafeTerminalSize } from '../../protocol/terminal'
import type { AppTheme } from '../../store/theme'

type ResizeCallback = (cols: number, rows: number) => void

type XtermController = {
  terminal: Terminal
  open: (element: HTMLElement) => void
  fit: () => void
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

function terminalOptions(theme: AppTheme = 'dark') {
  return {
    cursorBlink: true,
    fontFamily: 'Cascadia Mono, Consolas, monospace',
    fontSize: 12,
    scrollback: 5000,
    theme: xtermThemeFor(theme),
  }
}

export function measureXtermSize(element: HTMLElement): { cols: number; rows: number } | null {
  const terminal = new Terminal(terminalOptions())
  const fitAddon = new FitAddon()
  terminal.loadAddon(fitAddon)
  try {
    terminal.open(element)
    const dimensions = fitAddon.proposeDimensions()
    if (!dimensions || dimensions.cols < 1 || dimensions.rows < 1) {
      logTerminalDiagnostic('xterm.measure.invalid-size', {
        source: 'measure',
        clientWidth: element.clientWidth,
        clientHeight: element.clientHeight,
      })
      return null
    }
    const measuredCols = dimensions.cols
    const measuredRows = dimensions.rows
    const size = fitSafeTerminalSize({ cols: measuredCols, rows: measuredRows })
    terminal.resize(size.cols, size.rows)
    logTerminalDiagnostic('xterm.measure', {
      source: 'measure',
      clientWidth: element.clientWidth,
      clientHeight: element.clientHeight,
      measuredCols,
      measuredRows,
      cols: size.cols,
      rows: size.rows,
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
  const terminal = new Terminal(terminalOptions(theme))
  const fitAddon = new FitAddon()
  const webLinksAddon = new WebLinksAddon()
  const disposables = [terminal.onData(onData), terminal.onBinary(onBinary)]
  let observer: ResizeObserver | undefined
  let lastCols = 0
  let lastRows = 0
  let resizeTimer: number | undefined
  const writeQueue: Uint8Array[] = []
  let writing = false
  let pending = 0
  let writeCount = 0
  let drainCount = 0

  terminal.loadAddon(fitAddon)
  terminal.loadAddon(webLinksAddon)

  function diagnosticDetails() {
    return {
      source: diagnostics.source,
      sessionId: diagnostics.sessionId ?? undefined,
    }
  }

  function emitResize() {
    const dimensions = fitAddon.proposeDimensions()
    if (!dimensions || dimensions.cols < 1 || dimensions.rows < 1) {
      logTerminalDiagnostic('xterm.fit.invalid-size', diagnosticDetails())
      return
    }
    const measuredCols = dimensions.cols
    const measuredRows = dimensions.rows
    const size = fitSafeTerminalSize({ cols: measuredCols, rows: measuredRows })
    if (terminal.cols !== size.cols || terminal.rows !== size.rows) {
      terminal.resize(size.cols, size.rows)
    }
    if (size.cols !== lastCols || size.rows !== lastRows) {
      logTerminalDiagnostic('xterm.resize', {
        ...diagnosticDetails(),
        previousCols: lastCols,
        previousRows: lastRows,
        measuredCols,
        measuredRows,
        cols: size.cols,
        rows: size.rows,
      })
      lastCols = size.cols
      lastRows = size.rows
      onResize(size.cols, size.rows)
    }
  }

  function scheduleResize() {
    if (resizeTimer !== undefined) {
      window.clearTimeout(resizeTimer)
    }
    resizeTimer = window.setTimeout(emitResize, 100)
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
      logTerminalDiagnosticSample(
        'xterm.write.drained',
        {
          ...diagnosticDetails(),
          chunkBytes: next.byteLength,
          pendingBytes: pending,
          queuedChunks: writeQueue.length,
        },
        drainCount,
      )
      drainWrites()
    })
  }

  return {
    terminal,
    open(element: HTMLElement) {
      logTerminalDiagnostic('xterm.open', {
        ...diagnosticDetails(),
        clientWidth: element.clientWidth,
        clientHeight: element.clientHeight,
      })
      terminal.open(element)
      emitResize()
      observer = new ResizeObserver(scheduleResize)
      observer.observe(element)
      terminal.focus()
    },
    fit() {
      emitResize()
    },
    write(data: Uint8Array) {
      writeCount += 1
      if (pending + data.byteLength > maxPendingBytes) {
        logTerminalDiagnostic('xterm.write.dropped', {
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
      logTerminalDiagnosticSample(
        'xterm.write.queued',
        {
          ...diagnosticDetails(),
          chunkBytes: copy.byteLength,
          pendingBytes: pending,
          queuedChunks: writeQueue.length,
        },
        writeCount,
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
      logTerminalDiagnostic('xterm.dispose', {
        ...diagnosticDetails(),
        pendingBytes: pending,
        queuedChunks: writeQueue.length,
      })
      if (resizeTimer !== undefined) {
        window.clearTimeout(resizeTimer)
      }
      observer?.disconnect()
      for (const disposable of disposables) {
        disposable.dispose()
      }
      terminal.dispose()
    },
  }
}
