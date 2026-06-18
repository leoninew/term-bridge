import type {
  CreateSessionRequest,
  CreateSessionResponse,
  SessionSummary,
} from '../../protocol/terminal'

export async function listSessions(): Promise<SessionSummary[]> {
  const response = await fetch('/api/sessions')
  if (!response.ok) {
    throw new Error(`list sessions failed: ${response.status}`)
  }
  const body = (await response.json()) as { sessions: SessionSummary[] | null }
  return body.sessions ?? []
}

export async function createSession(request: CreateSessionRequest): Promise<CreateSessionResponse> {
  const response = await fetch('/api/sessions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  })
  if (!response.ok) {
    const body = await response.text()
    throw new Error(`create session failed: ${response.status} ${body}`)
  }
  return (await response.json()) as CreateSessionResponse
}

export async function readHistory(sessionId: string): Promise<string> {
  const response = await fetch(`/api/sessions/${encodeURIComponent(sessionId)}/history`)
  if (!response.ok) {
    throw new Error(`read history failed: ${response.status}`)
  }
  return response.text()
}
