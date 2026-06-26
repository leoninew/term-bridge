import { ref, shallowRef } from 'vue'
import {
  decodeControl,
  encodeControl,
  terminalSubprotocol,
  type ClientControlMessage,
  type ServerControlMessage,
} from '../../protocol/terminal'
import {
  diagnosticWebSocketPath,
  logTerminalDiagnostic,
  logTerminalDiagnosticError,
  logTerminalDiagnosticSample,
} from '../../components/terminal/diagnostics'

export function useTerminalSocket(
  onOutput: (data: Uint8Array) => void,
  onControl: (message: ServerControlMessage) => void,
  onError: (message: string) => void,
) {
  const socket = shallowRef<WebSocket | null>(null)
  const status = ref<'idle' | 'connecting' | 'connected' | 'closed' | 'error'>('idle')
  const error = ref<string | null>(null)
  let binaryMessageCount = 0
  let controlMessageCount = 0
  let pendingResize: Extract<ClientControlMessage, { type: 'resize' }> | null = null

  function connect(url: string) {
    close()
    status.value = 'connecting'
    error.value = null
    binaryMessageCount = 0
    controlMessageCount = 0
    const wsUrl = new URL(url, window.location.href).toString()
    logTerminalDiagnostic('socket.connect', { path: diagnosticWebSocketPath(wsUrl) })
    const next = new WebSocket(wsUrl, terminalSubprotocol)
    next.binaryType = 'arraybuffer'
    next.onopen = () => {
      status.value = 'connected'
      logTerminalDiagnostic('socket.open', { path: diagnosticWebSocketPath(wsUrl) })
      sendControl({ type: 'hello' })
      if (pendingResize) {
        sendControl(pendingResize)
      }
    }
    next.onmessage = (event: MessageEvent<string | ArrayBuffer | Blob>) => {
      if (typeof event.data === 'string') {
        controlMessageCount += 1
        try {
          const message = decodeControl(event.data)
          logTerminalDiagnostic('socket.control.received', {
            path: diagnosticWebSocketPath(wsUrl),
            type: message.type,
            count: controlMessageCount,
          })
          onControl(message)
        } catch (err) {
          error.value = err instanceof Error ? err.message : String(err)
          logTerminalDiagnosticError('socket.control.decode-error', {
            path: diagnosticWebSocketPath(wsUrl),
            message: error.value,
          })
          onError(error.value)
        }
        return
      }
      if (event.data instanceof ArrayBuffer) {
        binaryMessageCount += 1
        logTerminalDiagnosticSample(
          'socket.binary.received',
          {
            path: diagnosticWebSocketPath(wsUrl),
            bytes: event.data.byteLength,
            count: binaryMessageCount,
          },
          binaryMessageCount,
        )
        onOutput(new Uint8Array(event.data))
      }
    }
    next.onerror = () => {
      status.value = 'error'
      error.value = 'websocket error'
      logTerminalDiagnosticError('socket.error', { path: diagnosticWebSocketPath(wsUrl) })
      onError(error.value)
    }
    next.onclose = (event) => {
      status.value = 'closed'
      logTerminalDiagnostic('socket.close', {
        path: diagnosticWebSocketPath(wsUrl),
        code: event.code,
        reason: event.reason,
        wasClean: event.wasClean,
        binaryMessages: binaryMessageCount,
        controlMessages: controlMessageCount,
      })
    }
    socket.value = next
  }

  function sendInput(data: string) {
    if (socket.value?.readyState === WebSocket.OPEN) {
      logTerminalDiagnostic('socket.input.send', {
        bytes: new TextEncoder().encode(data).byteLength,
      })
      socket.value.send(new TextEncoder().encode(data))
    }
  }

  function sendBinary(data: string) {
    if (socket.value?.readyState !== WebSocket.OPEN) {
      return
    }
    const bytes = new Uint8Array(data.length)
    for (let index = 0; index < data.length; index += 1) {
      bytes[index] = data.charCodeAt(index) & 0xff
    }
    logTerminalDiagnostic('socket.binary.send', { bytes: bytes.byteLength })
    socket.value.send(bytes)
  }

  function sendControl(message: ClientControlMessage) {
    if (message.type === 'resize') {
      pendingResize = message
    }
    if (socket.value?.readyState !== WebSocket.OPEN) {
      logTerminalDiagnostic('socket.control.deferred', {
        type: message.type,
        cols: 'cols' in message ? message.cols : undefined,
        rows: 'rows' in message ? message.rows : undefined,
        readyState: socket.value?.readyState ?? null,
      })
      return
    }
    logTerminalDiagnostic('socket.control.send', {
      type: message.type,
      cols: 'cols' in message ? message.cols : undefined,
      rows: 'rows' in message ? message.rows : undefined,
    })
    socket.value.send(encodeControl(message))
    if (message.type === 'resize' && pendingResize === message) {
      pendingResize = null
    }
  }

  function close() {
    if (socket.value) {
      logTerminalDiagnostic('socket.close.requested')
      socket.value.close()
      socket.value = null
    }
  }

  return { status, error, connect, close, sendInput, sendBinary, sendControl }
}
