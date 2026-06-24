import type { Ref } from 'vue'
import { logTerminalDiagnostic } from '../components/terminal/diagnostics'
import { measureXtermSize } from '../components/terminal/useXterm'

export function useTerminalSize(workbench: Ref<HTMLElement | null>) {
  function measureInitialTerminalSize(): { cols: number; rows: number } {
    const measured = measureCreateSessionWorkbench()
    if (measured) {
      return measured
    }
    const cols = Math.max(80, Math.min(10000, Math.floor((window.innerWidth - 360) / 9)))
    const rows = Math.max(24, Math.min(10000, Math.floor((window.innerHeight - 180) / 18)))
    logTerminalDiagnostic('xterm.measure.fallback', { cols, rows })
    return { cols, rows }
  }

  function measureCreateSessionWorkbench(): { cols: number; rows: number } | null {
    const element = workbench.value
    if (!element) {
      logTerminalDiagnostic('xterm.measure.missing-workbench')
      return null
    }
    const wrapper = document.createElement('section')
    wrapper.style.position = 'fixed'
    wrapper.style.left = '-10000px'
    wrapper.style.top = '0'
    wrapper.style.width = `${element.clientWidth}px`
    wrapper.style.height = `${element.clientHeight}px`
    wrapper.style.display = 'flex'
    wrapper.style.flexDirection = 'column'
    wrapper.style.padding = '8px'
    wrapper.style.boxSizing = 'border-box'
    wrapper.style.visibility = 'hidden'
    wrapper.style.pointerEvents = 'none'

    const shell = document.createElement('section')
    shell.className = 'terminal-shell'
    shell.style.flex = '1'

    const container = document.createElement('div')
    container.className = 'terminal-container'
    shell.appendChild(container)
    wrapper.appendChild(shell)
    document.body.appendChild(wrapper)
    try {
      return measureXtermSize(container)
    } finally {
      wrapper.remove()
    }
  }

  return { measureInitialTerminalSize }
}
