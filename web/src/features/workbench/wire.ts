/** Normalize protojson wire values (enum names, int64 strings) for TS clients. */

import { FilePermission, FileType } from '../../gen/proto/termbridge/agent/v1/file'
import {
  ScmOperationState,
  ScmResourceState,
  ScmState,
} from '../../gen/proto/termbridge/agent/v1/git'

export function asNumber(value: unknown, fallback = 0): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) {
      return parsed
    }
  }
  return fallback
}

function enumFromNameOrNumber(
  value: unknown,
  byName: Record<string, number>,
  fallback: number,
): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value
  }
  if (typeof value === 'string') {
    if (Object.prototype.hasOwnProperty.call(byName, value)) {
      return byName[value]!
    }
    const asNum = Number(value)
    if (Number.isFinite(asNum)) {
      return asNum
    }
  }
  return fallback
}

const fileTypeByName: Record<string, number> = {
  FILE_TYPE_UNKNOWN: FileType.FILE_TYPE_UNKNOWN,
  FILE_TYPE_FILE: FileType.FILE_TYPE_FILE,
  FILE_TYPE_DIRECTORY: FileType.FILE_TYPE_DIRECTORY,
  FILE_TYPE_SYMBOLIC_LINK: FileType.FILE_TYPE_SYMBOLIC_LINK,
}

const filePermissionByName: Record<string, number> = {
  FILE_PERMISSION_UNSPECIFIED: FilePermission.FILE_PERMISSION_UNSPECIFIED,
  FILE_PERMISSION_READONLY: FilePermission.FILE_PERMISSION_READONLY,
}

const scmStateByName: Record<string, number> = {
  SCM_STATE_UNSPECIFIED: ScmState.SCM_STATE_UNSPECIFIED,
  SCM_STATE_AVAILABLE: ScmState.SCM_STATE_AVAILABLE,
  SCM_STATE_NOT_REPOSITORY: ScmState.SCM_STATE_NOT_REPOSITORY,
  SCM_STATE_EXECUTABLE_UNAVAILABLE: ScmState.SCM_STATE_EXECUTABLE_UNAVAILABLE,
  SCM_STATE_UNAVAILABLE: ScmState.SCM_STATE_UNAVAILABLE,
}

const scmResourceStateByName: Record<string, number> = {
  SCM_RESOURCE_STATE_UNSPECIFIED: ScmResourceState.SCM_RESOURCE_STATE_UNSPECIFIED,
  SCM_RESOURCE_STATE_AVAILABLE: ScmResourceState.SCM_RESOURCE_STATE_AVAILABLE,
  SCM_RESOURCE_STATE_BINARY: ScmResourceState.SCM_RESOURCE_STATE_BINARY,
  SCM_RESOURCE_STATE_TOO_LARGE: ScmResourceState.SCM_RESOURCE_STATE_TOO_LARGE,
  SCM_RESOURCE_STATE_UNMERGED: ScmResourceState.SCM_RESOURCE_STATE_UNMERGED,
  SCM_RESOURCE_STATE_SUBMODULE: ScmResourceState.SCM_RESOURCE_STATE_SUBMODULE,
  SCM_RESOURCE_STATE_UNAVAILABLE: ScmResourceState.SCM_RESOURCE_STATE_UNAVAILABLE,
}

const scmOperationStateByName: Record<string, number> = {
  SCM_OPERATION_STATE_UNSPECIFIED: ScmOperationState.SCM_OPERATION_STATE_UNSPECIFIED,
  SCM_OPERATION_STATE_OK: ScmOperationState.SCM_OPERATION_STATE_OK,
  SCM_OPERATION_STATE_CLEAN_WORKTREE_REQUIRED:
    ScmOperationState.SCM_OPERATION_STATE_CLEAN_WORKTREE_REQUIRED,
  SCM_OPERATION_STATE_NO_STAGED_CHANGES: ScmOperationState.SCM_OPERATION_STATE_NO_STAGED_CHANGES,
  SCM_OPERATION_STATE_BRANCH_EXISTS: ScmOperationState.SCM_OPERATION_STATE_BRANCH_EXISTS,
  SCM_OPERATION_STATE_BRANCH_NOT_FOUND: ScmOperationState.SCM_OPERATION_STATE_BRANCH_NOT_FOUND,
  SCM_OPERATION_STATE_INVALID_BRANCH_NAME:
    ScmOperationState.SCM_OPERATION_STATE_INVALID_BRANCH_NAME,
  SCM_OPERATION_STATE_INVALID_COMMIT_MESSAGE:
    ScmOperationState.SCM_OPERATION_STATE_INVALID_COMMIT_MESSAGE,
  SCM_OPERATION_STATE_CONFLICT: ScmOperationState.SCM_OPERATION_STATE_CONFLICT,
  SCM_OPERATION_STATE_FAILED: ScmOperationState.SCM_OPERATION_STATE_FAILED,
  SCM_OPERATION_STATE_UNAVAILABLE: ScmOperationState.SCM_OPERATION_STATE_UNAVAILABLE,
}

export function decodeFileType(value: unknown): FileType {
  return enumFromNameOrNumber(value, fileTypeByName, FileType.FILE_TYPE_UNKNOWN) as FileType
}

export function decodeFilePermission(value: unknown): FilePermission {
  return enumFromNameOrNumber(
    value,
    filePermissionByName,
    FilePermission.FILE_PERMISSION_UNSPECIFIED,
  ) as FilePermission
}

export function decodeScmState(value: unknown): ScmState {
  return enumFromNameOrNumber(value, scmStateByName, ScmState.SCM_STATE_UNSPECIFIED) as ScmState
}

export function decodeScmResourceState(value: unknown): ScmResourceState {
  return enumFromNameOrNumber(
    value,
    scmResourceStateByName,
    ScmResourceState.SCM_RESOURCE_STATE_UNSPECIFIED,
  ) as ScmResourceState
}

export function decodeScmOperationState(value: unknown): ScmOperationState {
  return enumFromNameOrNumber(
    value,
    scmOperationStateByName,
    ScmOperationState.SCM_OPERATION_STATE_UNSPECIFIED,
  ) as ScmOperationState
}

export function decodeFileStat(raw: unknown):
  | {
      type: FileType
      ctime: number
      mtime: number
      size: number
      permissions: FilePermission
      etag: string
    }
  | undefined {
  if (!raw || typeof raw !== 'object') {
    return undefined
  }
  const value = raw as Record<string, unknown>
  return {
    type: decodeFileType(value.type),
    ctime: asNumber(value.ctime),
    mtime: asNumber(value.mtime),
    size: asNumber(value.size),
    permissions: decodeFilePermission(value.permissions),
    etag: typeof value.etag === 'string' ? value.etag : '',
  }
}
