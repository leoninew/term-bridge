import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'

type ResizeCallback = (cols: number, rows: number) => void

type XtermController = {
  terminal: Terminal
  open: (element: HTMLElement) => void
  write: (data: Uint8Array) => void
  dispose: () => void
}

export function createXterm(
  onData: (data: string) => void,
  onBinary: (data: string) => void,
  onResize: ResizeCallback,
): XtermController {
  const terminal = new Terminal({
    convertEol: true,
    cursorBlink: true,
    fontFamily: 'Cascadia Mono, Consolas, monospace',
    fontSize: 14,
    scrollback: 5000,
    theme: {
      background: '#0b1020',
      foreground: '#d7deea',
      cursor: '#f8fafc',
    },
  })
  const fitAddon = new FitAddon()
  const webLinksAddon = new WebLinksAddon()
  const disposables = [terminal.onData(onData), terminal.onBinary(onBinary)]
  let observer: ResizeObserver | undefined
  let lastCols = 0
  let lastRows = 0
  let resizeTimer: number | undefined

  terminal.loadAddon(fitAddon)
  terminal.loadAddon(webLinksAddon)

  function emitResize() {
    fitAddon.fit()
    if (terminal.cols !== lastCols || terminal.rows !== lastRows) {
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

  return {
    terminal,
    open(element: HTMLElement) {
      terminal.open(element)
      emitResize()
      observer = new ResizeObserver(scheduleResize)
      observer.observe(element)
      terminal.focus()
    },
    write(data: Uint8Array) {
      terminal.write(data)
    },
    dispose() {
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
