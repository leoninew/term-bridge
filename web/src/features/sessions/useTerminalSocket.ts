import { ref, shallowRef } from 'vue'
import {
  decodeControl,
  encodeControl,
  terminalSubprotocol,
  type ClientControlMessage,
  type ServerControlMessage,
} from '../../protocol/terminal'
export function useTerminalSocket(
  onOutput: (data: Uint8Array) => void,
  onControl: (message: ServerControlMessage) => void,
  onError: (message: string) => void,
) {
  const socket = shallowRef<WebSocket | null>(null)
  const status = ref<'idle' | 'connecting' | 'connected' | 'closed' | 'error'>('idle')
  const error = ref<string | null>(null)

  function connect(url: string) {
    close()
    status.value = 'connecting'
    error.value = null
    const next = new WebSocket(new URL(url, window.location.href).toString(), terminalSubprotocol)
    next.binaryType = 'arraybuffer'
    next.onopen = () => {
      status.value = 'connected'
      sendControl({ type: 'hello' })
    }
    next.onmessage = (event: MessageEvent<string | ArrayBuffer | Blob>) => {
      if (typeof event.data === 'string') {
        try {
          onControl(decodeControl(event.data))
        } catch (err) {
          error.value = err instanceof Error ? err.message : String(err)
          onError(error.value)
        }
        return
      }
      if (event.data instanceof ArrayBuffer) {
        onOutput(new Uint8Array(event.data))
      }
    }
    next.onerror = () => {
      status.value = 'error'
      error.value = 'websocket error'
      onError(error.value)
    }
    next.onclose = () => {
      status.value = 'closed'
    }
    socket.value = next
  }

  function sendInput(data: string) {
    if (socket.value?.readyState === WebSocket.OPEN) {
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
    socket.value.send(bytes)
  }

  function sendControl(message: ClientControlMessage) {
    if (socket.value?.readyState === WebSocket.OPEN) {
      socket.value.send(encodeControl(message))
    }
  }

  function close() {
    if (socket.value) {
      socket.value.close()
      socket.value = null
    }
  }

  return { status, error, connect, close, sendInput, sendBinary, sendControl }
}
