export const terminalSubprotocol = 'termbridge.terminal.v1'

export type LifecycleState = 'starting' | 'running' | 'stopping' | 'stopped' | 'failed'
export type AttachmentState = 'unattached' | 'attached' | 'detached' | 'reattaching'

export type ClientControlMessage =
  | { type: 'hello'; last_seq?: number }
  | { type: 'resize'; cols: number; rows: number }
  | { type: 'detach' }
  | { type: 'close' }
  | { type: 'ping'; nonce: string }

export type ServerControlMessage =
  | {
      type: 'started'
      session_id: string
      workspace_id: string
      workspace_key?: string
      state: LifecycleState
      lifecycle_state?: LifecycleState
      attachment_state?: AttachmentState
    }
  | {
      type: 'state'
      state?: LifecycleState
      lifecycle_state?: LifecycleState
      attachment_state?: AttachmentState
      reason?: string
    }
  | { type: 'exited'; exit_code: number; state: 'stopped' | 'failed' }
  | { type: 'error'; code: string; message: string }
  | { type: 'pong'; nonce: string }

export type WorkspaceSummary = {
  id: string
  key: string
  name: string
  path: string
  updated_at: string
}

export type SessionSummary = {
  id: string
  workspace_id: string
  workspace_key: string
  command: string
  cwd: string
  lifecycle_state: LifecycleState
  attachment_state?: AttachmentState
  exit_code?: number
  updated_at: string
  log_path: string
}

export type CreateSessionRequest = {
  cwd: string
  command: string[]
  cols: number
  rows: number
}

export type CreateSessionResponse = {
  session_id: string
  workspace_id: string
  workspace_key: string
  state: LifecycleState
  ws_url: string
}

export function encodeControl(message: ClientControlMessage): string {
  return JSON.stringify(message)
}

export function decodeControl(data: string): ServerControlMessage {
  const parsed = JSON.parse(data) as ServerControlMessage
  if (!parsed || typeof parsed.type !== 'string') {
    throw new Error('invalid server control message')
  }
  return parsed
}

export function commandFromText(value: string): string[] {
  return value
    .trim()
    .split(/\s+/)
    .filter((part) => part.length > 0)
}
