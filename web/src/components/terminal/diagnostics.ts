type DiagnosticDetails = Record<string, unknown>

const prefix = '[termbridge:terminal]'

function isTerminalDebugEnabled(): boolean {
  const terminalDebugKey = 'termbridge.terminalDebug'
  try {
    return window.localStorage.getItem(terminalDebugKey) === '1'
  } catch {
    return false
  }
}

function webSocketPath(url: string): string {
  try {
    return new URL(url, window.location.href).pathname
  } catch {
    return url
  }
}

function normalizeDetails(details: DiagnosticDetails): DiagnosticDetails {
  const out: DiagnosticDetails = { ...details }
  if (typeof out.url === 'string') {
    out.path = webSocketPath(out.url)
    delete out.url
  } else if (
    typeof out.path === 'string' &&
    (/^https?:\/\//.test(out.path) || out.path.includes('://'))
  ) {
    out.path = webSocketPath(out.path)
  }
  return out
}

export type TerminalDebugOptions = {
  /** Defaults to info. */
  level?: 'info' | 'error'
  /** When set, only logs the first 5 occurrences and every 50th after that. */
  sample?: number
}

/**
 * Single terminal debug entry. Enable with:
 *   localStorage.setItem('termbridge.terminalDebug', '1')
 */
export function terminalDebug(
  event: string,
  details: DiagnosticDetails = {},
  options: TerminalDebugOptions = {},
): void {
  if (!isTerminalDebugEnabled()) {
    return
  }

  const sample = options.sample
  if (sample !== undefined && sample > 5 && sample % 50 !== 0) {
    return
  }

  const payload = normalizeDetails(details)
  if (options.level === 'error') {
    console.error(prefix, event, payload)
    return
  }
  console.info(prefix, event, payload)
}
