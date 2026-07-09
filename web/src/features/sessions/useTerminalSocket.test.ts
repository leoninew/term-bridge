import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { ClientControlMessage } from '../../gen/proto/termbridge/agent/v1/terminal'
import { useTerminalSocket } from './useTerminalSocket'

class FakeWebSocket {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSING = 2
  static readonly CLOSED = 3

  static instances: FakeWebSocket[] = []

  readonly url: string
  readonly protocol?: string
  binaryType: BinaryType = 'blob'
  readyState = FakeWebSocket.CONNECTING
  sent: Array<string | ArrayBufferLike | Blob | ArrayBufferView> = []
  onopen: (() => void) | null = null
  onmessage: ((event: MessageEvent<string | ArrayBuffer | Blob>) => void) | null = null
  onerror: (() => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null

  constructor(url: string, protocol?: string) {
    this.url = url
    this.protocol = protocol
    FakeWebSocket.instances.push(this)
  }

  send(data: string | ArrayBufferLike | Blob | ArrayBufferView) {
    this.sent.push(data)
  }

  close() {
    this.readyState = FakeWebSocket.CLOSED
    this.onclose?.({ code: 1000, reason: '', wasClean: true } as CloseEvent)
  }

  open() {
    this.readyState = FakeWebSocket.OPEN
    this.onopen?.()
  }
}

function sentControls(socket: FakeWebSocket): ClientControlMessage[] {
  return socket.sent
    .filter((item): item is string => typeof item === 'string')
    .map((item) => JSON.parse(item) as ClientControlMessage)
}

describe('useTerminalSocket', () => {
  beforeEach(() => {
    FakeWebSocket.instances = []
    vi.stubGlobal('WebSocket', FakeWebSocket)
    vi.stubGlobal('window', { location: { href: 'http://127.0.0.1:9031/' } })
    vi.spyOn(console, 'info').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('sends a deferred resize after hello when the socket opens', () => {
    const terminal = useTerminalSocket(vi.fn(), vi.fn(), vi.fn())

    terminal.sendControl({ type: 'resize', cols: 120, rows: 32, nonce: '' })
    terminal.connect('/local-api/sessions/session-1/ws')

    const socket = FakeWebSocket.instances[0]
    expect(socket).toBeDefined()

    socket.open()

    expect(sentControls(socket)).toEqual([
      { type: 'hello', cols: 0, rows: 0, nonce: '' },
      { type: 'resize', cols: 120, rows: 32, nonce: '' },
    ])
  })

  it('keeps only the latest deferred resize before connection opens', () => {
    const terminal = useTerminalSocket(vi.fn(), vi.fn(), vi.fn())

    terminal.sendControl({ type: 'resize', cols: 100, rows: 24, nonce: '' })
    terminal.sendControl({ type: 'resize', cols: 140, rows: 40, nonce: '' })
    terminal.connect('/local-api/sessions/session-1/ws')

    const socket = FakeWebSocket.instances[0]
    socket.open()

    expect(sentControls(socket)).toEqual([
      { type: 'hello', cols: 0, rows: 0, nonce: '' },
      { type: 'resize', cols: 140, rows: 40, nonce: '' },
    ])
  })

  it('defers resize messages sent while the socket is connecting', () => {
    const terminal = useTerminalSocket(vi.fn(), vi.fn(), vi.fn())

    terminal.connect('/local-api/sessions/session-1/ws')
    const socket = FakeWebSocket.instances[0]

    terminal.sendControl({ type: 'resize', cols: 132, rows: 35, nonce: '' })
    expect(socket.sent).toEqual([])

    socket.open()

    expect(sentControls(socket)).toEqual([
      { type: 'hello', cols: 0, rows: 0, nonce: '' },
      { type: 'resize', cols: 132, rows: 35, nonce: '' },
    ])
  })

  it('does not replay a resize that was already sent on a later reconnect', () => {
    const terminal = useTerminalSocket(vi.fn(), vi.fn(), vi.fn())

    terminal.connect('/local-api/sessions/session-1/ws')
    const firstSocket = FakeWebSocket.instances[0]
    firstSocket.open()

    terminal.sendControl({ type: 'resize', cols: 150, rows: 45, nonce: '' })
    expect(sentControls(firstSocket)).toEqual([
      { type: 'hello', cols: 0, rows: 0, nonce: '' },
      { type: 'resize', cols: 150, rows: 45, nonce: '' },
    ])

    terminal.connect('/local-api/sessions/session-1/ws')
    const secondSocket = FakeWebSocket.instances[1]
    secondSocket.open()

    expect(sentControls(secondSocket)).toEqual([{ type: 'hello', cols: 0, rows: 0, nonce: '' }])
  })
})
