import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import { logTerminalDiagnostic, logTerminalDiagnosticSample } from './diagnostics'

type ResizeCallback = (cols: number, rows: number) => void

type XtermController = {
  terminal: Terminal
  open: (element: HTMLElement) => void
  write: (data: Uint8Array) => void
  pendingBytes: () => number
  dispose: () => void
}

type XtermDiagnostics = {
  source: 'live' | 'history'
  sessionId?: string | null
}

const maxPendingBytes = 4 * 1024 * 1024

export function createXterm(
  onData: (data: string) => void,
  onBinary: (data: string) => void,
  onResize: ResizeCallback,
  diagnostics: XtermDiagnostics,
): XtermController {
  const terminal = new Terminal({
    convertEol: true,
    cursorBlink: true,
    fontFamily: 'Cascadia Mono, Consolas, monospace',
    fontSize: 12,
    scrollback: 5000,
    theme: {
      background: '#020617',
      foreground: '#d7deea',
      cursor: '#f8fafc',
      selectionBackground: '#1e3a5f',
    },
  })
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
    fitAddon.fit()
    if (terminal.cols < 1 || terminal.rows < 1) {
      logTerminalDiagnostic('xterm.fit.invalid-size', diagnosticDetails())
      return
    }
    if (terminal.cols !== lastCols || terminal.rows !== lastRows) {
      logTerminalDiagnostic('xterm.resize', {
        ...diagnosticDetails(),
        previousCols: lastCols,
        previousRows: lastRows,
        cols: terminal.cols,
        rows: terminal.rows,
      })
      lastCols = terminal.cols
      lastRows = terminal.rows
      onResize(terminal.cols, terminal.rows)
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
