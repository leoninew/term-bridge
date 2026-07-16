import type { AxiosInstance } from 'axios'
import { FileEntryKind } from '../../gen/proto/termbridge/agent/v1/file'
import type {
  CreateDirectoryReq,
  CreateDirectoryResp,
  CreateFileReq,
  CreateFileResp,
  DeleteEntryReq,
  DeleteEntryResp,
  FileConflict,
  FileEntry,
  FileMutationResult,
  ListFilesResp,
  MoveEntryReq,
  MoveEntryResp,
  ReadFileResp,
  RenameEntryReq,
  RenameEntryResp,
  WriteFileReq,
  WriteFileResp,
} from '../../gen/proto/termbridge/agent/v1/file'
import {
  GitLayer,
  GitState,
  type GitChange,
  type GitDiffResp,
  type GitStatusResp,
} from '../../gen/proto/termbridge/agent/v1/git'
import { ApiClientError, cloudApiClient, localApiClient } from '../api/client'
import { runtimePath, type RuntimeTarget } from '../runtimeTarget'

export type RuntimeRequestOptions = {
  signal?: AbortSignal
}

export type CreateFileRequest = Omit<CreateFileReq, 'workspace_id'>
export type CreateDirectoryRequest = Omit<CreateDirectoryReq, 'workspace_id'>
export type WriteFileRequest = Omit<WriteFileReq, 'workspace_id'>
export type RenameEntryRequest = Omit<RenameEntryReq, 'workspace_id'>
export type MoveEntryRequest = Omit<MoveEntryReq, 'workspace_id'>
export type DeleteEntryRequest = Omit<DeleteEntryReq, 'workspace_id'>

export type FileGitApiErrorDetails = FileConflict | undefined

export class FileGitApiError extends Error {
  readonly status: number
  readonly code: string
  readonly requestId: string
  readonly details: FileGitApiErrorDetails

  constructor(error: ApiClientError) {
    super(error.message)
    this.name = 'FileGitApiError'
    this.status = error.status
    this.code = error.code
    this.requestId = error.requestId
    this.details = fileConflictDetails(error.details)
  }
}

export type FileRuntimeApi = {
  listFiles(
    workspaceId: string,
    path: string,
    options?: RuntimeRequestOptions,
  ): Promise<ListFilesResp>
  readFile(
    workspaceId: string,
    path: string,
    options?: RuntimeRequestOptions,
  ): Promise<ReadFileResp>
  createFile(
    workspaceId: string,
    request: CreateFileRequest,
    options?: RuntimeRequestOptions,
  ): Promise<CreateFileResp>
  createDirectory(
    workspaceId: string,
    request: CreateDirectoryRequest,
    options?: RuntimeRequestOptions,
  ): Promise<CreateDirectoryResp>
  writeFile(
    workspaceId: string,
    request: WriteFileRequest,
    options?: RuntimeRequestOptions,
  ): Promise<WriteFileResp>
  renameEntry(
    workspaceId: string,
    request: RenameEntryRequest,
    options?: RuntimeRequestOptions,
  ): Promise<RenameEntryResp>
  moveEntry(
    workspaceId: string,
    request: MoveEntryRequest,
    options?: RuntimeRequestOptions,
  ): Promise<MoveEntryResp>
  deleteEntry(
    workspaceId: string,
    request: DeleteEntryRequest,
    options?: RuntimeRequestOptions,
  ): Promise<DeleteEntryResp>
}

export type GitRuntimeApi = {
  gitStatus(workspaceId: string, options?: RuntimeRequestOptions): Promise<GitStatusResp>
  gitDiff(
    workspaceId: string,
    path: string,
    layer: GitLayer,
    options?: RuntimeRequestOptions,
  ): Promise<GitDiffResp>
}

export type FileGitRuntimeApi = FileRuntimeApi & GitRuntimeApi

type RuntimeHttpClients = {
  local: Pick<AxiosInstance, 'get' | 'post' | 'put' | 'delete'>
  cloud: Pick<AxiosInstance, 'get' | 'post' | 'put' | 'delete'>
}

const defaultRuntimeHttpClients: RuntimeHttpClients = {
  local: localApiClient,
  cloud: cloudApiClient,
}

const gitLayerQueryValues = {
  [GitLayer.GIT_LAYER_STAGED]: 'staged',
  [GitLayer.GIT_LAYER_UNSTAGED]: 'unstaged',
  [GitLayer.GIT_LAYER_UNTRACKED]: 'untracked',
} as const

export function createFileGitRuntimeApi(
  target: RuntimeTarget,
  clients: RuntimeHttpClients = defaultRuntimeHttpClients,
): FileGitRuntimeApi {
  const client = target.mode === 'cloud' ? clients.cloud : clients.local

  return {
    listFiles: (workspaceId, path, options) =>
      request(() =>
        client
          .get<ListFilesResp>(filePath(target, workspaceId, '/files/tree'), {
            params: { path },
            signal: options?.signal,
          })
          .then((response) => decodeListFilesResponse(response.data)),
      ),
    readFile: (workspaceId, path, options) =>
      request(() =>
        client
          .get<ReadFileResp>(filePath(target, workspaceId, '/files/content'), {
            params: { path },
            signal: options?.signal,
          })
          .then((response) => decodeReadFileResponse(response.data)),
      ),
    createFile: (workspaceId, file, options) =>
      request(() =>
        client
          .post<CreateFileResp>(
            filePath(target, workspaceId, '/files'),
            withWorkspaceId(workspaceId, file),
            { signal: options?.signal },
          )
          .then((response) => decodeCreateFileResponse(response.data)),
      ),
    createDirectory: (workspaceId, directory, options) =>
      request(() =>
        client
          .post<CreateDirectoryResp>(
            filePath(target, workspaceId, '/directories'),
            withWorkspaceId(workspaceId, directory),
            { signal: options?.signal },
          )
          .then((response) => decodeCreateDirectoryResponse(response.data)),
      ),
    writeFile: (workspaceId, file, options) =>
      request(() =>
        client
          .put<WriteFileResp>(
            filePath(target, workspaceId, '/files/content'),
            withWorkspaceId(workspaceId, file),
            { signal: options?.signal },
          )
          .then((response) => decodeWriteFileResponse(response.data)),
      ),
    renameEntry: (workspaceId, entry, options) =>
      request(() =>
        client
          .post<RenameEntryResp>(
            filePath(target, workspaceId, '/entries:rename'),
            withWorkspaceId(workspaceId, entry),
            { signal: options?.signal },
          )
          .then((response) => decodeRenameEntryResponse(response.data)),
      ),
    moveEntry: (workspaceId, entry, options) =>
      request(() =>
        client
          .post<MoveEntryResp>(
            filePath(target, workspaceId, '/entries:move'),
            withWorkspaceId(workspaceId, entry),
            { signal: options?.signal },
          )
          .then((response) => decodeMoveEntryResponse(response.data)),
      ),
    deleteEntry: (workspaceId, entry, options) =>
      request(() =>
        client
          .delete<DeleteEntryResp>(filePath(target, workspaceId, '/entries'), {
            params: entry,
            signal: options?.signal,
          })
          .then((response) => decodeDeleteEntryResponse(response.data)),
      ),
    gitStatus: (workspaceId, options) =>
      request(() =>
        client
          .get<GitStatusResp>(filePath(target, workspaceId, '/git/status'), {
            signal: options?.signal,
          })
          .then((response) => decodeGitStatusResponse(response.data)),
      ),
    gitDiff: (workspaceId, path, layer, options) =>
      request(() =>
        client
          .get<GitDiffResp>(filePath(target, workspaceId, '/git/diff'), {
            params: { path, layer: gitLayerQueryValue(layer) },
            signal: options?.signal,
          })
          .then((response) => decodeGitDiffResponse(response.data)),
      ),
  }
}

function filePath(target: RuntimeTarget, workspaceId: string, suffix: string): string {
  return runtimePath(target, `/workspaces/${encodeURIComponent(workspaceId)}${suffix}`)
}

function withWorkspaceId<T extends object>(
  workspaceId: string,
  request: T,
): T & { workspace_id: string } {
  return { ...request, workspace_id: workspaceId }
}

function gitLayerQueryValue(layer: GitLayer): string {
  const queryValue = gitLayerQueryValues[layer as keyof typeof gitLayerQueryValues]
  if (!queryValue) {
    throw new Error('Git diff requires a staged, unstaged, or untracked layer')
  }
  return queryValue
}

async function request<T>(operation: () => Promise<T>): Promise<T> {
  try {
    return await operation()
  } catch (error) {
    if (error instanceof ApiClientError) {
      throw new FileGitApiError(error)
    }
    throw error
  }
}

function decodeListFilesResponse(value: unknown): ListFilesResp {
  const response = record(value, 'List files response')
  return {
    directory: decodeFileEntry(response.directory),
    items: array(response.items, 'List files response.items').map(decodeRequiredFileEntry),
    truncated: boolean(response.truncated, 'List files response.truncated'),
  }
}

function decodeReadFileResponse(value: unknown): ReadFileResp {
  const response = record(value, 'Read file response')
  return {
    entry: decodeFileEntry(response.entry),
    text: string(response.text, 'Read file response.text'),
  }
}

function decodeCreateFileResponse(value: unknown): CreateFileResp {
  return { result: decodeMutationResponse(value, 'Create file response') }
}

function decodeCreateDirectoryResponse(value: unknown): CreateDirectoryResp {
  return { result: decodeMutationResponse(value, 'Create directory response') }
}

function decodeWriteFileResponse(value: unknown): WriteFileResp {
  return { result: decodeMutationResponse(value, 'Write file response') }
}

function decodeRenameEntryResponse(value: unknown): RenameEntryResp {
  return { result: decodeMutationResponse(value, 'Rename entry response') }
}

function decodeMoveEntryResponse(value: unknown): MoveEntryResp {
  return { result: decodeMutationResponse(value, 'Move entry response') }
}

function decodeDeleteEntryResponse(value: unknown): DeleteEntryResp {
  return { result: decodeMutationResponse(value, 'Delete entry response') }
}

function decodeMutationResponse(value: unknown, label: string): FileMutationResult | undefined {
  const response = record(value, label)
  return response.result === undefined ? undefined : decodeMutationResult(response.result)
}

function decodeGitStatusResponse(value: unknown): GitStatusResp {
  const response = record(value, 'Git status response')
  return {
    state: decodeGitState(response.state),
    changes: array(response.changes, 'Git status response.changes').map(decodeGitChange),
    message: string(response.message, 'Git status response.message'),
  }
}

function decodeGitDiffResponse(value: unknown): GitDiffResp {
  const response = record(value, 'Git diff response')
  return {
    state: decodeGitState(response.state),
    original_path: string(response.original_path, 'Git diff response.original_path'),
    modified_path: string(response.modified_path, 'Git diff response.modified_path'),
    original_text: string(response.original_text, 'Git diff response.original_text'),
    modified_text: string(response.modified_text, 'Git diff response.modified_text'),
    message: string(response.message, 'Git diff response.message'),
  }
}

function decodeGitChange(value: unknown): GitChange {
  const change = record(value, 'Git status change')
  return {
    path: string(change.path, 'Git status change.path'),
    original_path: string(change.original_path, 'Git status change.original_path'),
    index_status: string(change.index_status, 'Git status change.index_status'),
    worktree_status: string(change.worktree_status, 'Git status change.worktree_status'),
    untracked: boolean(change.untracked, 'Git status change.untracked'),
    unmerged: boolean(change.unmerged, 'Git status change.unmerged'),
    available_layers: array(change.available_layers, 'Git status change.available_layers').map(
      decodeGitLayer,
    ),
  }
}

function decodeGitState(value: unknown): GitState {
  return enumValue(value, GitState, 'Git state') as GitState
}

function decodeGitLayer(value: unknown): GitLayer {
  return enumValue(value, GitLayer, 'Git layer') as GitLayer
}

function enumValue(value: unknown, values: object, label: string): number {
  if (typeof value === 'number' && Number.isInteger(value) && value in values) {
    return value
  }
  if (typeof value === 'string') {
    const enumValue = (values as Record<string, unknown>)[value]
    if (typeof enumValue === 'number') {
      return enumValue
    }
  }
  throw new Error(`${label} does not match the API contract`)
}

function decodeMutationResult(value: unknown): FileMutationResult {
  const result = record(value, 'File mutation result')
  return {
    source_entry: decodeFileEntry(result.source_entry),
    destination_entry: decodeFileEntry(result.destination_entry),
    affected_count: number(result.affected_count, 'File mutation result.affected_count'),
    affected_path_prefixes: array(
      result.affected_path_prefixes,
      'File mutation result.affected_path_prefixes',
    ).map((prefix) => string(prefix, 'File mutation result.affected_path_prefixes[]')),
  }
}

function fileConflictDetails(details: unknown): FileGitApiErrorDetails {
  const candidate = recordOrUndefined(details)
  if (candidate?.type !== 'revision_conflict') {
    return undefined
  }
  const currentEntry = decodeFileEntryOrUndefined(candidate.current_entry, true)
  return currentEntry ? { type: 'revision_conflict', current_entry: currentEntry } : undefined
}

function decodeFileEntry(value: unknown): FileEntry | undefined {
  return decodeFileEntryOrUndefined(value, false)
}

function decodeRequiredFileEntry(value: unknown): FileEntry {
  const entry = decodeFileEntry(value)
  if (!entry) {
    throw new Error('File entry does not match the API contract')
  }
  return entry
}

function decodeFileEntryOrUndefined(
  value: unknown,
  conflictDetails = false,
): FileEntry | undefined {
  if (value === undefined || value === null) {
    return undefined
  }
  const entry = record(value, 'File entry')
  return {
    path: string(entry.path, 'File entry.path'),
    name: string(entry.name, 'File entry.name'),
    kind: decodeFileEntryKind(entry.kind, conflictDetails),
    size: number(entry.size, 'File entry.size'),
    modified_at: optionalString(entry.modified_at, 'File entry.modified_at'),
    revision: string(entry.revision, 'File entry.revision'),
    has_children: optionalBoolean(entry.has_children, 'File entry.has_children'),
  }
}

function decodeFileEntryKind(value: unknown, conflictDetails: boolean): FileEntryKind {
  if (value === FileEntryKind.FILE_ENTRY_KIND_FILE || value === 'FILE_ENTRY_KIND_FILE') {
    return FileEntryKind.FILE_ENTRY_KIND_FILE
  }
  if (value === FileEntryKind.FILE_ENTRY_KIND_DIRECTORY || value === 'FILE_ENTRY_KIND_DIRECTORY') {
    return FileEntryKind.FILE_ENTRY_KIND_DIRECTORY
  }
  if (conflictDetails && value === 'file') {
    return FileEntryKind.FILE_ENTRY_KIND_FILE
  }
  if (conflictDetails && value === 'directory') {
    return FileEntryKind.FILE_ENTRY_KIND_DIRECTORY
  }
  throw new Error('File entry.kind is invalid')
}

function record(value: unknown, label: string): Record<string, unknown> {
  const result = recordOrUndefined(value)
  if (!result) {
    throw new Error(`${label} does not match the API contract`)
  }
  return result
}

function recordOrUndefined(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : undefined
}

function array(value: unknown, label: string): unknown[] {
  if (!Array.isArray(value)) {
    throw new Error(`${label} does not match the API contract`)
  }
  return value
}

function string(value: unknown, label: string): string {
  if (typeof value !== 'string') {
    throw new Error(`${label} does not match the API contract`)
  }
  return value
}

function optionalString(value: unknown, label: string): string | undefined {
  return value === undefined ? undefined : string(value, label)
}

function boolean(value: unknown, label: string): boolean {
  if (typeof value !== 'boolean') {
    throw new Error(`${label} does not match the API contract`)
  }
  return value
}

function optionalBoolean(value: unknown, label: string): boolean | undefined {
  return value === undefined ? undefined : boolean(value, label)
}

function number(value: unknown, label: string): number {
  if (typeof value === 'number' && Number.isSafeInteger(value)) {
    return value
  }
  if (typeof value === 'string' && /^-?\d+$/.test(value)) {
    const parsed = Number(value)
    if (Number.isSafeInteger(parsed)) {
      return parsed
    }
  }
  throw new Error(`${label} does not match the API contract`)
}
