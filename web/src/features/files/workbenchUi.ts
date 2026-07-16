import type { FileEntry } from '../../gen/proto/termbridge/agent/v1/file'
import {
  FileEntryKind,
  type FileEntryKind as FileEntryKindType,
} from '../../gen/proto/termbridge/agent/v1/file'
import { GitLayer, GitState } from '../../gen/proto/termbridge/agent/v1/git'

export function parentPath(path: string): string {
  const index = path.lastIndexOf('/')
  return index === -1 ? '' : path.slice(0, index)
}

export function entryPath(parent: string, name: string): string {
  return parent ? `${parent}/${name}` : name
}

export function entryName(path: string): string {
  return path.split('/').at(-1) ?? ''
}

export function isDirectory(entry: FileEntry | null | undefined): boolean {
  return entry?.kind === FileEntryKind.FILE_ENTRY_KIND_DIRECTORY
}

export function isFile(entry: FileEntry | null | undefined): boolean {
  return entry?.kind === FileEntryKind.FILE_ENTRY_KIND_FILE
}

export function directoryEntry(
  path: string,
  items: FileEntry[],
  directory: FileEntry | null,
): FileEntry | null {
  if (path === '') {
    return directory
  }
  return items.find((item) => item.path === path && isDirectory(item)) ?? null
}

export function kindLabel(kind: FileEntryKindType): 'file' | 'directory' | 'unknown' {
  if (kind === FileEntryKind.FILE_ENTRY_KIND_FILE) {
    return 'file'
  }
  if (kind === FileEntryKind.FILE_ENTRY_KIND_DIRECTORY) {
    return 'directory'
  }
  return 'unknown'
}

export function gitLayerLabel(layer: GitLayer): 'staged' | 'unstaged' | 'untracked' {
  if (layer === GitLayer.GIT_LAYER_STAGED) {
    return 'staged'
  }
  if (layer === GitLayer.GIT_LAYER_UNTRACKED) {
    return 'untracked'
  }
  return 'unstaged'
}

export function gitStateLabel(state: GitState): string {
  const labels: Partial<Record<GitState, string>> = {
    [GitState.GIT_STATE_NOT_REPOSITORY]: 'notRepository',
    [GitState.GIT_STATE_EXECUTABLE_UNAVAILABLE]: 'executableUnavailable',
    [GitState.GIT_STATE_BINARY]: 'binary',
    [GitState.GIT_STATE_TOO_LARGE]: 'tooLarge',
    [GitState.GIT_STATE_UNMERGED]: 'unmerged',
    [GitState.GIT_STATE_SUBMODULE]: 'submodule',
    [GitState.GIT_STATE_LAYER_UNAVAILABLE]: 'layerUnavailable',
    [GitState.GIT_STATE_UNAVAILABLE]: 'unavailable',
  }
  return labels[state] ?? 'unavailable'
}
