export const terminalSubprotocol = 'termbridge.terminal.v1'

export type LifecycleState = 'starting' | 'running' | 'stopping' | 'stopped' | 'failed'
export type AttachmentState = 'unattached' | 'attached' | 'detached' | 'reattaching'

export type ClientControlMessage =
  | { type: 'hello' }
  | { type: 'resize'; cols: number; rows: number }
  | { type: 'detach' }
  | { type: 'ping'; nonce: string }

export type ServerControlMessage =
  | {
      type: 'started'
      session_id: string
      workspace_id: string
      state: LifecycleState
      lifecycle_state?: LifecycleState
      attachment_state?: AttachmentState
    }
  | { type: 'replay_started' }
  | { type: 'replay_finished'; truncated?: boolean }
  | {
      type: 'state'
      state?: LifecycleState
      lifecycle_state?: LifecycleState
      attachment_state?: AttachmentState
      reason?: string
    }
  | { type: 'exited'; exit_code: number; state: 'stopped' | 'failed' }
  | { type: 'error'; code: string; message: string; error?: string }
  | { type: 'pong'; nonce: string }

export type ApiErrorResponse = {
  code: string
  message: string
  error: string
}

export type WorkspaceSummary = {
  id: string
  name: string
  path: string
  sort_order: number
  updated_at: string
}

export type SessionSummary = {
  id: string
  name: string
  workspace_id: string
  command: string
  cwd: string
  lifecycle_state: LifecycleState
  attachment_state?: AttachmentState
  exit_code?: number
  updated_at: string
}

export type WorkspaceTreeSession = Omit<SessionSummary, 'workspace_id'> & {
  workspace_id?: string
}

export type WorkspaceTreeSummary = WorkspaceSummary & {
  children: WorkspaceTreeSession[]
}

export type CreateSessionRequest = {
  workspace_id?: string
  name: string
  cwd: string
  command: string[]
  cols: number
  rows: number
}

export type UpdateSessionRequest = {
  name: string
}

export type CreateSessionResponse = {
  session_id: string
  workspace_id: string
  state: LifecycleState
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
      if (!parsed.session_id || !parsed.workspace_id || !parsed.state) {
        throw new Error('invalid started message')
      }
      break
    case 'state':
      if (!parsed.state && !parsed.lifecycle_state) {
        throw new Error('invalid state message')
      }
      break
    case 'exited':
      if (typeof parsed.exit_code !== 'number' || !parsed.state) {
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
      throw new Error(`unknown server control message ${(parsed as { type: string }).type}`)
  }
  return parsed
}

export function commandFromText(value: string): string[] {
  return value
    .trim()
    .split(/\s+/)
    .filter((part) => part.length > 0)
}
