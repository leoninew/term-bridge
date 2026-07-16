import { describe, expect, it } from 'vitest'
import { FileEntryKind, type FileEntry } from '../../gen/proto/termbridge/agent/v1/file'
import { GitLayer, GitState } from '../../gen/proto/termbridge/agent/v1/git'
import {
  entryName,
  entryPath,
  gitLayerLabel,
  gitStateLabel,
  isDirectory,
  isFile,
  parentPath,
} from './workbenchUi'

function entry(kind: FileEntryKind): FileEntry {
  return {
    path: 'nested/notes.txt',
    name: 'notes.txt',
    kind,
    size: 0,
    modified_at: undefined,
    revision: 'revision-1',
  }
}

describe('file workbench UI helpers', () => {
  it('keeps workspace-relative path segments intact', () => {
    expect(parentPath('nested/notes.txt')).toBe('nested')
    expect(parentPath('notes.txt')).toBe('')
    expect(entryPath('', 'notes.txt')).toBe('notes.txt')
    expect(entryPath('nested', 'notes.txt')).toBe('nested/notes.txt')
    expect(entryName('nested/notes.txt')).toBe('notes.txt')
  })

  it('distinguishes server-provided file entry kinds', () => {
    expect(isFile(entry(FileEntryKind.FILE_ENTRY_KIND_FILE))).toBe(true)
    expect(isDirectory(entry(FileEntryKind.FILE_ENTRY_KIND_FILE))).toBe(false)
    expect(isDirectory(entry(FileEntryKind.FILE_ENTRY_KIND_DIRECTORY))).toBe(true)
  })

  it('maps only explicit Git layers and unavailable states to UI labels', () => {
    expect(gitLayerLabel(GitLayer.GIT_LAYER_STAGED)).toBe('staged')
    expect(gitLayerLabel(GitLayer.GIT_LAYER_UNSTAGED)).toBe('unstaged')
    expect(gitLayerLabel(GitLayer.GIT_LAYER_UNTRACKED)).toBe('untracked')
    expect(gitStateLabel(GitState.GIT_STATE_NOT_REPOSITORY)).toBe('notRepository')
    expect(gitStateLabel(GitState.GIT_STATE_BINARY)).toBe('binary')
  })
})
