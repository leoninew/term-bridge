type DiagnosticDetails = Record<string, unknown>

const prefix = '[termbridge:terminal]'

function diagnosticsEnabled(): boolean {
  return window.localStorage.getItem('termbridge.terminalDebug') === '1'
}

export function logTerminalDiagnostic(event: string, details: DiagnosticDetails = {}) {
  if (!diagnosticsEnabled()) {
    return
  }
  console.info(prefix, event, details)
}

export function logTerminalDiagnosticSample(
  event: string,
  details: DiagnosticDetails,
  count: number,
) {
  if (count <= 5 || count % 50 === 0) {
    logTerminalDiagnostic(event, details)
  }
}

export function diagnosticWebSocketPath(url: string): string {
  try {
    const parsed = new URL(url, window.location.href)
    return parsed.pathname
  } catch {
    return url
  }
}
