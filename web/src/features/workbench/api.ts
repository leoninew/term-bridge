import type { AxiosInstance } from 'axios'
import {
  FsChangeKind,
  type FileStat,
  type FsReadDirectoryResp,
} from '../../gen/proto/termbridge/agent/v1/file'
import {
  ScmCommand,
  type ScmExecuteReq,
  type ScmExecuteResp,
  type ScmOriginalContentResp,
  type ScmRepositoryResp,
  type ScmStatusResp,
} from '../../gen/proto/termbridge/agent/v1/git'
import { ApiClientError, cloudApiClient, localApiClient } from '../api/client'
import { useCloudAuthStore } from '../../store/cloudAuth'
import { useRuntimeConfigStore } from '../../store/runtimeConfig'
import { runtimePath, type RuntimeTarget } from '../runtimeTarget'
import { bytesFromWire, bytesToWire } from './bytes'
import {
  asNumber,
  decodeFileStat,
  decodeFileType,
  decodeScmOperationState,
  decodeScmResourceState,
  decodeScmState,
} from './wire'

export type WorkbenchRequestOptions = {
  signal?: AbortSignal
}

export class WorkbenchApiError extends Error {
  readonly status: number
  readonly code: string
  readonly requestId: string
  readonly details?: unknown

  constructor(error: ApiClientError) {
    super(error.message)
    this.name = 'WorkbenchApiError'
    this.status = error.status
    this.code = error.code
    this.requestId = error.requestId
    this.details = error.details
  }
}

export type WorkbenchFsApi = {
  stat(path: string, options?: WorkbenchRequestOptions): Promise<FileStat>
  readDirectory(path: string, options?: WorkbenchRequestOptions): Promise<FsReadDirectoryResp>
  readFile(
    path: string,
    options?: WorkbenchRequestOptions,
  ): Promise<{ content: Uint8Array; stat?: FileStat }>
  writeFile(
    path: string,
    content: Uint8Array,
    options: {
      create: boolean
      overwrite: boolean
      etag?: string
      signal?: AbortSignal
    },
  ): Promise<FileStat | undefined>
  createDirectory(path: string, options?: WorkbenchRequestOptions): Promise<FileStat | undefined>
  delete(
    path: string,
    options: { recursive: boolean; useTrash?: boolean; signal?: AbortSignal },
  ): Promise<void>
  rename(
    oldPath: string,
    newPath: string,
    options: { overwrite: boolean; signal?: AbortSignal },
  ): Promise<FileStat | undefined>
}

export type WorkbenchWorkspaceChange = {
  sequence: number
  kind: FsChangeKind
  path: string
  oldPath: string
}

export type WorkbenchWorkspaceChangeSubscription = {
  close(): void
}

export type WorkbenchScmApi = {
  status(options?: WorkbenchRequestOptions): Promise<ScmStatusResp>
  originalContent(
    path: string,
    groupId: string,
    options?: WorkbenchRequestOptions,
  ): Promise<{
    content: Uint8Array
    resource_state: ScmOriginalContentResp['resource_state']
    message: string
  }>
  execute(
    request: Omit<ScmExecuteReq, 'workspace_id'>,
    options?: WorkbenchRequestOptions,
  ): Promise<ScmExecuteResp>
  repository(options?: WorkbenchRequestOptions): Promise<ScmRepositoryResp>
}

export type WorkbenchRuntimeApi = WorkbenchFsApi & WorkbenchScmApi

export function workspaceChangesWsUrl(
  target: RuntimeTarget,
  workspaceId: string,
  token?: string,
): string {
  const path = runtimePath(target, `/workspaces/${encodeURIComponent(workspaceId)}/fs/events`)
  const params = new URLSearchParams()
  if (token) {
    params.set('token', token)
  }
  const runtimeConfig = useRuntimeConfigStore().config
  const apiBaseUrl =
    target.mode === 'cloud' ? runtimeConfig.cloud.apiBaseUrl : runtimeConfig.local.apiBasePath
  return buildApiWebSocketUrl(apiBaseUrl, params.size > 0 ? `${path}?${params.toString()}` : path)
}

const workspaceChangesSubprotocol = 'termbridge.workspacefs.v1'
const reconnectInitialDelayMs = 250
const reconnectMaximumDelayMs = 5_000

type WorkspaceWatchMessage =
  | { type: 'change'; change: WorkbenchWorkspaceChange }
  | { type: 'subscribed' }
  | { type: 'invalid' }

export function subscribeWorkspaceChanges(
  target: RuntimeTarget,
  workspaceId: string,
  options: {
    token?: string
    onChange(change: WorkbenchWorkspaceChange): void
    onRescanRequired(): void
    onDisconnected(): void
  },
): WorkbenchWorkspaceChangeSubscription {
  const cloudToken =
    target.mode === 'cloud' ? (useCloudAuthStore().cloudToken ?? undefined) : undefined
  const url = workspaceChangesWsUrl(target, workspaceId, options.token ?? cloudToken)
  let socket: WebSocket | undefined
  let closed = false
  let nextSequence: number | undefined
  let reconnectDelayMs = reconnectInitialDelayMs
  let reconnectTimer: ReturnType<typeof setTimeout> | undefined

  const scheduleReconnect = () => {
    if (closed || reconnectTimer !== undefined) {
      return
    }
    const delayMs = reconnectDelayMs
    reconnectDelayMs = Math.min(reconnectDelayMs * 2, reconnectMaximumDelayMs)
    reconnectTimer = setTimeout(() => {
      reconnectTimer = undefined
      connect()
    }, delayMs)
  }

  const disconnect = () => {
    if (closed) {
      return
    }
    // A closed WebSocket cannot prove that all watcher events were delivered.
    // Invalidate before reconnecting rather than treating the socket as a poll loop.
    nextSequence = undefined
    options.onRescanRequired()
    options.onDisconnected()
    scheduleReconnect()
  }

  const connect = () => {
    if (closed) {
      return
    }
    let candidate: WebSocket
    try {
      candidate = new WebSocket(url, workspaceChangesSubprotocol)
    } catch {
      disconnect()
      return
    }
    socket = candidate
    candidate.onopen = () => {
      if (closed || socket !== candidate) {
        return
      }
      reconnectDelayMs = reconnectInitialDelayMs
      nextSequence = undefined
    }
    candidate.onmessage = (event) => {
      if (closed || socket !== candidate) {
        return
      }
      if (typeof event.data !== 'string') {
        options.onRescanRequired()
        return
      }
      const message = decodeWorkspaceWatchMessage(event.data)
      if (message.type === 'subscribed') {
        return
      }
      if (message.type === 'invalid') {
        options.onRescanRequired()
        return
      }
      const { change } = message
      if (
        change.kind === FsChangeKind.FS_CHANGE_KIND_RESCAN_REQUIRED ||
        (change.sequence > 0 && nextSequence !== undefined && change.sequence !== nextSequence)
      ) {
        options.onRescanRequired()
      }
      if (change.sequence > 0) {
        nextSequence = change.sequence + 1
      }
      options.onChange(change)
    }
    candidate.onclose = () => {
      if (!closed && socket === candidate) {
        socket = undefined
        disconnect()
      }
    }
    // Browsers normally follow error with close. The close handler owns the
    // conservative reset and reconnect so the same failure is not reported twice.
    candidate.onerror = () => {}
  }

  connect()
  return {
    close() {
      closed = true
      if (reconnectTimer !== undefined) {
        clearTimeout(reconnectTimer)
        reconnectTimer = undefined
      }
      socket?.close()
      socket = undefined
    },
  }
}

function decodeWorkspaceWatchMessage(raw: string): WorkspaceWatchMessage {
  let value: unknown
  try {
    value = JSON.parse(raw)
  } catch {
    return { type: 'invalid' }
  }
  if (!isRecord(value)) {
    return { type: 'invalid' }
  }
  if (!('kind' in value)) {
    return typeof value.workspace_id === 'string' ? { type: 'subscribed' } : { type: 'invalid' }
  }
  const kind = decodeWorkspaceChangeKind(value.kind)
  const sequence = decodeWorkspaceSequence(value.sequence)
  if (kind === undefined || sequence === undefined) {
    return { type: 'invalid' }
  }
  const path = value.path
  const oldPath = value.old_path
  if (
    (path !== undefined && typeof path !== 'string') ||
    (oldPath !== undefined && typeof oldPath !== 'string')
  ) {
    return { type: 'invalid' }
  }
  return {
    type: 'change',
    change: {
      sequence,
      kind,
      path: path ?? '',
      oldPath: oldPath ?? '',
    },
  }
}

function decodeWorkspaceChangeKind(value: unknown): FsChangeKind | undefined {
  const numberValue =
    typeof value === 'number'
      ? value
      : typeof value === 'string'
        ? FsChangeKind[value as keyof typeof FsChangeKind]
        : undefined
  switch (numberValue) {
    case FsChangeKind.FS_CHANGE_KIND_ADDED:
    case FsChangeKind.FS_CHANGE_KIND_UPDATED:
    case FsChangeKind.FS_CHANGE_KIND_DELETED:
    case FsChangeKind.FS_CHANGE_KIND_RENAMED:
    case FsChangeKind.FS_CHANGE_KIND_RESCAN_REQUIRED:
      return numberValue
    default:
      return undefined
  }
}

function decodeWorkspaceSequence(value: unknown): number | undefined {
  const numberValue =
    typeof value === 'number'
      ? value
      : typeof value === 'string' && /^\d+$/.test(value)
        ? Number(value)
        : value === undefined
          ? 0
          : Number.NaN
  if (!Number.isSafeInteger(numberValue) || numberValue < 0) {
    return undefined
  }
  return numberValue
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object'
}

function buildApiWebSocketUrl(apiBaseUrl: string, path: string): string {
  const normalizedBase = apiBaseUrl.replace(/\/+$/, '')
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const url = `${normalizedBase}${normalizedPath}`
  if (apiBaseUrl.startsWith('/')) {
    return url
  }
  const parsed = new URL(url)
  parsed.protocol = parsed.protocol === 'https:' ? 'wss:' : 'ws:'
  return parsed.toString()
}

type RuntimeHttpClients = {
  local: Pick<AxiosInstance, 'get' | 'post' | 'put'>
  cloud: Pick<AxiosInstance, 'get' | 'post' | 'put'>
}

const defaultClients: RuntimeHttpClients = {
  local: localApiClient,
  cloud: cloudApiClient,
}

export function createWorkbenchRuntimeApi(
  target: RuntimeTarget,
  workspaceId: string,
  clients: RuntimeHttpClients = defaultClients,
): WorkbenchRuntimeApi {
  const client = target.mode === 'cloud' ? clients.cloud : clients.local
  const base = (suffix: string) =>
    runtimePath(target, `/workspaces/${encodeURIComponent(workspaceId)}${suffix}`)

  return {
    async stat(path, options) {
      const data = await request(() =>
        client
          .get(base('/fs/stat'), { params: { path }, signal: options?.signal })
          .then((response) => response.data as Record<string, unknown>),
      )
      const stat = decodeFileStat(data.stat)
      if (!stat) {
        throw new WorkbenchApiError(
          new ApiClientError(404, {
            code: 'not_found',
            error: 'File not found.',
            request_id: '',
            details: undefined,
          }),
        )
      }
      return stat
    },

    async readDirectory(path, options) {
      const data = await request(() =>
        client
          .get(base('/fs/readDirectory'), {
            params: { path },
            signal: options?.signal,
          })
          .then((response) => response.data as Record<string, unknown>),
      )
      const entriesRaw = Array.isArray(data.entries) ? data.entries : []
      return {
        entries: entriesRaw.map((entry) => {
          const item = (entry ?? {}) as Record<string, unknown>
          return {
            name: typeof item.name === 'string' ? item.name : '',
            type: decodeFileType(item.type),
          }
        }),
        truncated: Boolean(data.truncated),
      }
    },

    async readFile(path, options) {
      const data = await request(() =>
        client
          .get(base('/fs/readFile'), {
            params: { path },
            signal: options?.signal,
          })
          .then((response) => response.data as Record<string, unknown>),
      )
      return {
        content: bytesFromWire(data.content),
        stat: decodeFileStat(data.stat),
      }
    },

    async writeFile(path, content, options) {
      const data = await request(() =>
        client
          .put(
            base('/fs/writeFile'),
            {
              workspace_id: workspaceId,
              path,
              content: bytesToWire(content),
              create: options.create,
              overwrite: options.overwrite,
              etag: options.etag ?? '',
            },
            { signal: options.signal },
          )
          .then((response) => response.data as Record<string, unknown>),
      )
      return decodeFileStat(data.stat)
    },

    async createDirectory(path, options) {
      const data = await request(() =>
        client
          .post(
            base('/fs/createDirectory'),
            { workspace_id: workspaceId, path },
            { signal: options?.signal },
          )
          .then((response) => response.data as Record<string, unknown>),
      )
      return decodeFileStat(data.stat)
    },

    async delete(path, options) {
      await request(() =>
        client
          .post(
            base('/fs/delete'),
            {
              workspace_id: workspaceId,
              path,
              recursive: options.recursive,
              use_trash: options.useTrash ?? false,
            },
            { signal: options.signal },
          )
          .then((response) => response.data),
      )
    },

    async rename(oldPath, newPath, options) {
      const data = await request(() =>
        client
          .post(
            base('/fs/rename'),
            {
              workspace_id: workspaceId,
              old_path: oldPath,
              new_path: newPath,
              overwrite: options.overwrite,
            },
            { signal: options.signal },
          )
          .then((response) => response.data as Record<string, unknown>),
      )
      return decodeFileStat(data.stat)
    },

    async status(options) {
      const data = await request(() =>
        client
          .get(base('/scm/status'), { signal: options?.signal })
          .then((response) => response.data as Record<string, unknown>),
      )
      return decodeScmStatus(data)
    },

    async originalContent(path, groupId, options) {
      const data = await request(() =>
        client
          .get(base('/scm/originalContent'), {
            params: { path, groupId },
            signal: options?.signal,
          })
          .then((response) => response.data as Record<string, unknown>),
      )
      return {
        content: bytesFromWire(data.content),
        resource_state: decodeScmResourceState(data.resource_state),
        message: typeof data.message === 'string' ? data.message : '',
      }
    },

    async execute(body, options) {
      const data = await request(() =>
        client
          .post(
            base('/scm/execute'),
            {
              workspace_id: workspaceId,
              command: body.command ?? ScmCommand.SCM_COMMAND_UNSPECIFIED,
              path: body.path ?? '',
              group_id: body.group_id ?? '',
              message: body.message ?? '',
              branch_name: body.branch_name ?? '',
            },
            { signal: options?.signal },
          )
          .then((response) => response.data as Record<string, unknown>),
      )
      return {
        operation_state: decodeScmOperationState(data.operation_state),
        message: typeof data.message === 'string' ? data.message : '',
        status: data.status ? decodeScmStatus(data.status as Record<string, unknown>) : undefined,
        repository: data.repository
          ? decodeScmRepository(data.repository as Record<string, unknown>)
          : undefined,
      }
    },

    async repository(options) {
      const data = await request(() =>
        client
          .get(base('/scm/repository'), { signal: options?.signal })
          .then((response) => response.data as Record<string, unknown>),
      )
      return decodeScmRepository(data)
    },
  }
}

function decodeScmStatus(data: Record<string, unknown>): ScmStatusResp {
  const groupsRaw = Array.isArray(data.groups) ? data.groups : []
  return {
    state: decodeScmState(data.state),
    count: asNumber(data.count),
    message: typeof data.message === 'string' ? data.message : '',
    groups: groupsRaw.map((group) => {
      const item = (group ?? {}) as Record<string, unknown>
      const resourcesRaw = Array.isArray(item.resources) ? item.resources : []
      return {
        id: typeof item.id === 'string' ? item.id : '',
        label: typeof item.label === 'string' ? item.label : '',
        hide_when_empty: Boolean(item.hide_when_empty),
        resources: resourcesRaw.map((resource) => {
          const res = (resource ?? {}) as Record<string, unknown>
          const decorations =
            res.decorations && typeof res.decorations === 'object'
              ? (res.decorations as Record<string, unknown>)
              : undefined
          return {
            path: typeof res.path === 'string' ? res.path : '',
            original_path: typeof res.original_path === 'string' ? res.original_path : '',
            group_id: typeof res.group_id === 'string' ? res.group_id : '',
            resource_state: decodeScmResourceState(res.resource_state),
            decorations: decorations
              ? {
                  strike_through: Boolean(decorations.strike_through),
                  faded: Boolean(decorations.faded),
                  tooltip: typeof decorations.tooltip === 'string' ? decorations.tooltip : '',
                  letter: typeof decorations.letter === 'string' ? decorations.letter : '',
                  color_id: typeof decorations.color_id === 'string' ? decorations.color_id : '',
                }
              : undefined,
          }
        }),
      }
    }),
  }
}

function decodeScmRepository(data: Record<string, unknown>): ScmRepositoryResp {
  const historyRaw = Array.isArray(data.history) ? data.history : []
  const branchesRaw = Array.isArray(data.local_branches) ? data.local_branches : []
  return {
    state: decodeScmState(data.state),
    current_branch: typeof data.current_branch === 'string' ? data.current_branch : '',
    local_branches: branchesRaw.filter((item): item is string => typeof item === 'string'),
    message: typeof data.message === 'string' ? data.message : '',
    history: historyRaw.map((entry) => {
      const item = (entry ?? {}) as Record<string, unknown>
      return {
        id: typeof item.id === 'string' ? item.id : '',
        short_id: typeof item.short_id === 'string' ? item.short_id : '',
        subject: typeof item.subject === 'string' ? item.subject : '',
        author_name: typeof item.author_name === 'string' ? item.author_name : '',
        authored_at: typeof item.authored_at === 'string' ? item.authored_at : undefined,
      }
    }),
  }
}

async function request<T>(run: () => Promise<T>): Promise<T> {
  try {
    return await run()
  } catch (error) {
    if (error instanceof ApiClientError) {
      throw new WorkbenchApiError(error)
    }
    throw error
  }
}
