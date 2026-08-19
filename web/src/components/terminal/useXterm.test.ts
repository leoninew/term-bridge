import { describe, expect, it, vi } from 'vitest'

const harness = vi.hoisted(() => {
  class FakeTerminal {
    static instances: FakeTerminal[] = []

    cols = 80
    rows = 24
    options: Record<string, unknown> = {}
    buffer = { active: { viewportY: 0, baseY: 0 } }
    writes: Uint8Array[] = []
    callbacks: Array<(() => void) | undefined> = []
    resetCalls = 0

    constructor() {
      FakeTerminal.instances.push(this)
    }

    loadAddon() {}
    onData() {
      return { dispose() {} }
    }
    onBinary() {
      return { dispose() {} }
    }
    onScroll() {
      return { dispose() {} }
    }
    onWriteParsed() {
      return { dispose() {} }
    }
    write(data: Uint8Array, callback?: () => void) {
      this.writes.push(new Uint8Array(data))
      this.callbacks.push(callback)
    }
    completeNextWrite() {
      this.callbacks.shift()?.()
    }
    reset() {
      this.resetCalls += 1
    }
    resize(cols: number, rows: number) {
      this.cols = cols
      this.rows = rows
    }
    focus() {}
    refresh() {}
    scrollLines() {}
    open() {}
    dispose() {}
  }

  return { FakeTerminal }
})

vi.mock('@xterm/xterm', () => ({ Terminal: harness.FakeTerminal }))
vi.mock('@xterm/addon-fit', () => ({
  FitAddon: class {
    proposeDimensions() {
      return { cols: 80, rows: 24 }
    }
  },
}))
vi.mock('@xterm/addon-web-links', () => ({ WebLinksAddon: class {} }))
vi.mock('./touchScroll', () => ({ attachTouchScroll: () => () => undefined }))

import { createXterm } from './useXterm'

const encoder = new TextEncoder()
const decoder = new TextDecoder()

function writeTexts(terminal: InstanceType<typeof harness.FakeTerminal>) {
  return terminal.writes.map((data) => decoder.decode(data))
}

describe('createXterm replay restore', () => {
  it('waits for the in-flight write before resetting and drops queued stale output', () => {
    const controller = createXterm(
      () => undefined,
      () => undefined,
      () => undefined,
      { source: 'live' },
    )
    const terminal = harness.FakeTerminal.instances.at(-1)
    expect(terminal).toBeDefined()
    if (!terminal) {
      return
    }

    controller.write(encoder.encode('old-in-flight'))
    controller.write(encoder.encode('old-queued'))
    controller.beginReplayRestore()
    controller.write(encoder.encode('replay'))

    expect(terminal.resetCalls).toBe(0)
    expect(writeTexts(terminal)).toEqual(['old-in-flight'])

    terminal.completeNextWrite()

    expect(terminal.resetCalls).toBe(1)
    expect(writeTexts(terminal)).toEqual(['old-in-flight', 'replay'])

    terminal.completeNextWrite()
    expect(controller.pendingBytes()).toBe(0)
  })
})
