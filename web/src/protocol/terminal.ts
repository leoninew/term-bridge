import type { ErrorResp as ProtoErrorResp } from '../gen/proto/termbridge/common/v1/common'
import type {
  CreateSessionReq as ProtoCreateSessionReq,
  CreateSessionResp as ProtoCreateSessionResp,
  RerunSessionReq as ProtoRerunSessionReq,
  SessionSummary as ProtoSessionSummary,
  UpdateSessionReq as ProtoUpdateSessionReq,
  Workspace as ProtoWorkspace,
  WorkspaceTreeNode as ProtoWorkspaceTreeNode,
} from '../gen/proto/termbridge/runtime/v1/runtime'

export const terminalSubprotocol = 'termbridge.terminal.v1'
export const minTerminalCols = 1
export const maxTerminalCols = 1000
export const minTerminalRows = 1
export const maxTerminalRows = 1000
export const terminalFitSafetyMargin = 1

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

export function fitSafeTerminalSize(size: TerminalSize): TerminalSize {
  return clampTerminalSize({
    cols: size.cols - terminalFitSafetyMargin,
    rows: size.rows - terminalFitSafetyMargin,
  })
}

export type LifecycleState = 'running' | 'stopped' | 'failed'
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

export type ApiErrorResp<TDetails = unknown> = Omit<ProtoErrorResp, 'details'> & {
  details?: TDetails
  requestId?: string
}

export type ListResp<T> = {
  items: T[]
}

export type PaginatedResp<T> = ListResp<T> & {
  total: number
  page: number
  page_size: number
  total_pages: number
}

export type WorkspaceSummary = Omit<ProtoWorkspace, 'updated_at'> & {
  updated_at: string
}

export type SessionSummary = Omit<
  ProtoSessionSummary,
  'lifecycle_state' | 'attachment_state' | 'updated_at'
> & {
  lifecycle_state: LifecycleState
  attachment_state?: AttachmentState
  updated_at: string
}

export type WorkspaceTreeSession = Omit<SessionSummary, 'workspace_id'> & {
  workspace_id?: string
}

export type WorkspaceTreeSummary = Omit<ProtoWorkspaceTreeNode, 'children' | 'updated_at'> & {
  updated_at: string
  children: WorkspaceTreeSession[]
}

export type CreateSessionReq = Omit<ProtoCreateSessionReq, 'workspace_id'> & {
  workspace_id?: string
}

export type UpdateSessionReq = Pick<ProtoUpdateSessionReq, 'name'> & {
  name: string
}

export type RerunSessionReq = Pick<ProtoRerunSessionReq, 'cols' | 'rows'>

export type CreateSessionResp = Omit<ProtoCreateSessionResp, 'state'> & {
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
