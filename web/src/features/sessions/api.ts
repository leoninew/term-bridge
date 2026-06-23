import type {
  ApiErrorResponse,
  CreateSessionRequest,
  CreateSessionResponse,
  SessionSummary,
  UpdateSessionRequest,
} from '../../protocol/terminal'

export type ApiResult<T> = {
  data: T
  offline: boolean
}

function devicePath(deviceId: string, path: string): string {
  return `/api/devices/${encodeURIComponent(deviceId)}${path}`
}

function offline(response: Response): boolean {
  return response.headers.get('x-termbridge-offline') === 'true'
}

export async function listSessions(deviceId: string): Promise<ApiResult<SessionSummary[]>> {
  const response = await fetch(devicePath(deviceId, '/sessions'))
  if (!response.ok) {
    throw new Error(await responseError('List sessions failed', response))
  }
  return { data: ((await response.json()) as SessionSummary[] | null) ?? [], offline: offline(response) }
}

export async function createSession(
  deviceId: string,
  request: CreateSessionRequest,
): Promise<CreateSessionResponse> {
  const response = await fetch(devicePath(deviceId, '/sessions'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  })
  if (!response.ok) {
    throw new Error(await responseError('Create session failed', response))
  }
  return (await response.json()) as CreateSessionResponse
}

export async function getSession(deviceId: string, sessionId: string): Promise<SessionSummary> {
  const response = await fetch(devicePath(deviceId, `/sessions/${encodeURIComponent(sessionId)}`))
  if (!response.ok) {
    throw new Error(await responseError('Get session failed', response))
  }
  return (await response.json()) as SessionSummary
}

export async function updateSession(
  deviceId: string,
  sessionId: string,
  request: UpdateSessionRequest,
): Promise<SessionSummary> {
  const response = await fetch(devicePath(deviceId, `/sessions/${encodeURIComponent(sessionId)}`), {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  })
  if (!response.ok) {
    throw new Error(await responseError('Rename session failed', response))
  }
  return (await response.json()) as SessionSummary
}

export async function deleteSession(deviceId: string, sessionId: string): Promise<void> {
  const response = await fetch(devicePath(deviceId, `/sessions/${encodeURIComponent(sessionId)}`), {
    method: 'DELETE',
  })
  if (!response.ok) {
    throw new Error(await responseError('Delete session failed', response))
  }
}

export async function readHistory(deviceId: string, sessionId: string): Promise<ApiResult<string>> {
  const response = await fetch(
    devicePath(deviceId, `/sessions/${encodeURIComponent(sessionId)}/history`),
  )
  if (!response.ok) {
    throw new Error(await responseError('Read history failed', response))
  }
  return { data: await response.text(), offline: offline(response) }
}

export async function closeSession(deviceId: string, sessionId: string): Promise<SessionSummary> {
  const response = await fetch(devicePath(deviceId, `/sessions/${encodeURIComponent(sessionId)}/close`), {
    method: 'POST',
  })
  if (!response.ok) {
    throw new Error(await responseError('Close session failed', response))
  }
  return (await response.json()) as SessionSummary
}

export function terminalWsUrl(deviceId: string, sessionId: string): string {
  return devicePath(deviceId, `/sessions/${encodeURIComponent(sessionId)}/ws`)
}

export async function responseError(prefix: string, response: Response): Promise<string> {
  const body = (await response.text()).trim()
  if (!body) {
    return `${prefix} (${response.status})`
  }
  try {
    const parsed = JSON.parse(body) as Partial<ApiErrorResponse>
    if (typeof parsed.code === 'string' && typeof parsed.message === 'string') {
      return `${prefix} (${response.status} ${parsed.code}): ${parsed.message}`
    }
  } catch {
    return `${prefix} (${response.status}): ${body}`
  }
  return `${prefix} (${response.status}): ${body}`
}
