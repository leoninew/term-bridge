import type {
  ClientControlMessage,
  ServerControlMessage,
} from '../gen/proto/termbridge/agent/v1/terminal'

export const terminalSubprotocol = 'termbridge.terminal'
export const minTerminalCols = 1
export const maxTerminalCols = 1000
export const minTerminalRows = 1
export const maxTerminalRows = 1000

export type TerminalSize = {
  cols: number
  rows: number
}

export function clampTerminalSize(size: TerminalSize): TerminalSize {
  return {
    cols: Math.max(minTerminalCols, Math.min(maxTerminalCols, size.cols)),
    rows: Math.max(minTerminalRows, Math.min(maxTerminalRows, size.rows)),
  }
}

export function encodeControl(message: ClientControlMessage): string {
  return JSON.stringify(message)
}

export function decodeControl(data: string): ServerControlMessage {
  const parsed = JSON.parse(data) as ServerControlMessage
  if (!parsed || typeof parsed.type !== 'string') {
    throw new Error('invalid server control message')
  }
  switch (parsed.type) {
    case 'started':
      if (
        !parsed.session_id ||
        !parsed.workspace_id ||
        (!parsed.state && !parsed.lifecycle_state)
      ) {
        throw new Error('invalid started message')
      }
      break
    case 'state':
      if (!parsed.state && !parsed.lifecycle_state) {
        throw new Error('invalid state message')
      }
      break
    case 'exited':
      if (typeof parsed.exit_code !== 'number' || (!parsed.state && !parsed.lifecycle_state)) {
        throw new Error('invalid exited message')
      }
      break
    case 'error':
      if (!parsed.code || !parsed.message) {
        throw new Error('invalid error message')
      }
      break
    case 'pong':
      if (typeof parsed.nonce !== 'string') {
        throw new Error('invalid pong message')
      }
      break
    case 'replay_started':
    case 'replay_finished':
      break
    default:
      throw new Error(`unknown server control message ${parsed.type}`)
  }
  return parsed
}
