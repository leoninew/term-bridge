import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import type {
  FileConflict,
  FileEntry,
  FileMutationResult,
} from '../gen/proto/termbridge/agent/v1/file'
import {
  GitLayer,
  type GitDiffResp,
  type GitStatusResp,
} from '../gen/proto/termbridge/agent/v1/git'
import {
  type CreateDirectoryRequest,
  type CreateFileRequest,
  type DeleteEntryRequest,
  type FileGitRuntimeApi,
  type MoveEntryRequest,
  type RenameEntryRequest,
} from '../features/files/runtime'
import type { RuntimeTarget } from '../features/runtimeTarget'
import { errorMessage } from './notifications'

export type FileDirectoryState = {
  key: string
  path: string
  directory: FileEntry | null
  items: FileEntry[]
  expanded: boolean
  loaded: boolean
  loading: boolean
  stale: boolean
  truncated: boolean
  error: string | null
}

export type FileDocumentState = {
  key: string
  path: string
  entry: FileEntry | null
  baseRevision: string
  originalText: string
  draftText: string
  dirty: boolean
  loading: boolean
  saving: boolean
  conflict: FileConflict | null
  error: string | null
}

export type GitSelection = {
  path: string
  layer: GitLayer
}

export const useFileWorkbenchStore = defineStore('fileWorkbench', () => {
  const target = ref<RuntimeTarget | null>(null)
  const workspaceId = ref<string | null>(null)
  const directories = ref<FileDirectoryState[]>([])
  const documents = ref<FileDocumentState[]>([])
  const activeDocumentKey = ref<string | null>(null)
  const gitStatus = ref<GitStatusResp | null>(null)
  const gitStatusLoading = ref(false)
  const gitStatusStale = ref(false)
  const gitStatusError = ref<string | null>(null)
  const gitSelection = ref<GitSelection | null>(null)
  const gitDiff = ref<GitDiffResp | null>(null)
  const gitDiffLoading = ref(false)
  const gitDiffError = ref<string | null>(null)

  let epoch = 0
  let requestSequence = 0
  let mutationTail: Promise<void> = Promise.resolve()
  const controllers = new Map<string, AbortController>()
  const requestIds = new Map<string, number>()

  const activeDocument = computed(
    () => documents.value.find((document) => document.key === activeDocumentKey.value) ?? null,
  )

  function openWorkspace(nextTarget: RuntimeTarget, nextWorkspaceId: string): boolean {
    if (scopeKey(target.value, workspaceId.value) === scopeKey(nextTarget, nextWorkspaceId)) {
      return false
    }
    resetForSourceChange()
    target.value = cloneTarget(nextTarget)
    workspaceId.value = nextWorkspaceId
    return true
  }

  function resetForSourceChange() {
    epoch += 1
    abortAllRequests()
    target.value = null
    workspaceId.value = null
    directories.value = []
    documents.value = []
    activeDocumentKey.value = null
    resetGitState()
  }

  function removeWorkspace(removedWorkspaceId: string) {
    if (workspaceId.value === removedWorkspaceId) {
      resetForSourceChange()
    }
  }

  function cancelReadonlyRequests() {
    for (const directory of directories.value) {
      abortRequest(`directory:${directory.key}`)
      directory.loading = false
    }
    for (const document of documents.value) {
      abortRequest(`document:${document.key}`)
      document.loading = false
    }
    abortRequest('git-status')
    abortRequest('git-diff')
    gitStatusLoading.value = false
    gitDiffLoading.value = false
  }

  function directoryFor(path: string): FileDirectoryState | null {
    return directories.value.find((directory) => directory.key === directoryKey(path)) ?? null
  }

  function documentFor(path: string): FileDocumentState | null {
    return documents.value.find((document) => document.key === documentKey(path)) ?? null
  }

  function documentsUnderPath(path: string): FileDocumentState[] {
    return documents.value.filter((document) => pathContains(path, document.path))
  }

  function documentsForPaths(paths: string[]): FileDocumentState[] {
    const targetPaths = new Set(paths)
    return documents.value.filter((document) => targetPaths.has(document.path))
  }

  function hasDirtyDocumentUnderPath(path: string): boolean {
    return documentsUnderPath(path).some((document) => document.dirty)
  }

  function setDirectoryExpanded(path: string, expanded: boolean) {
    const directory = ensureDirectory(path)
    directory.expanded = expanded
  }

  function toggleDirectoryExpanded(path: string): boolean {
    const directory = ensureDirectory(path)
    directory.expanded = !directory.expanded
    return directory.expanded
  }

  async function ensureDirectoryLoaded(
    api: FileGitRuntimeApi,
    path: string,
  ): Promise<string | null> {
    const directory = ensureDirectory(path)
    if ((directory.loaded && !directory.stale) || directory.loading) {
      return null
    }
    return refreshDirectory(api, path)
  }

  async function refreshDirectory(api: FileGitRuntimeApi, path: string): Promise<string | null> {
    const context = currentContext()
    if (!context) {
      return null
    }
    const directory = ensureDirectory(path)
    const request = beginRequest(`directory:${directory.key}`)
    directory.loading = true
    directory.error = null
    try {
      const response = await api.listFiles(context.workspaceId, path, {
        signal: request.controller.signal,
      })
      if (!isCurrentRequest(context, request)) {
        return null
      }
      directory.directory = response.directory ?? null
      directory.items = response.items
      directory.loaded = true
      directory.stale = false
      directory.truncated = response.truncated
      return null
    } catch (error) {
      if (!isCurrentRequest(context, request) || isAbort(error)) {
        return null
      }
      const message = errorMessage(error)
      directory.error = message
      return message
    } finally {
      const current = isCurrentRequest(context, request)
      finishRequest(request)
      if (current) {
        directory.loading = false
      }
    }
  }

  async function openDocument(api: FileGitRuntimeApi, path: string): Promise<string | null> {
    const context = currentContext()
    if (!context) {
      return null
    }
    const document = ensureDocument(path)
    activeDocumentKey.value = document.key
    if (document.entry || document.loading || document.dirty) {
      return null
    }
    return readDocument(api, document, false)
  }

  function setActiveDocument(path: string): boolean {
    const document = documentFor(path)
    if (!document) {
      return false
    }
    activeDocumentKey.value = document.key
    return true
  }

  function setDocumentDraft(path: string, draftText: string): boolean {
    const document = documentFor(path)
    if (!document || document.loading) {
      return false
    }
    document.draftText = draftText
    document.dirty = document.draftText !== document.originalText
    if (!document.conflict) {
      document.error = null
    }
    return true
  }

  function discardDocumentDraft(path: string): boolean {
    return discardDocumentDrafts([path])
  }

  function discardDocumentDrafts(paths: string[]): boolean {
    const targetDocuments = documentsForPaths(paths)
    if (
      targetDocuments.length === 0 ||
      targetDocuments.some((document) => document.loading || document.saving)
    ) {
      return false
    }
    for (const document of targetDocuments) {
      document.draftText = document.originalText
      document.dirty = false
      document.conflict = null
      document.error = null
    }
    return true
  }

  async function refreshDocument(api: FileGitRuntimeApi, path: string): Promise<string | null> {
    const document = documentFor(path)
    if (!document) {
      return openDocument(api, path)
    }
    if (document.dirty || document.conflict) {
      return 'discard_required'
    }
    return readDocument(api, document, true)
  }

  async function reloadDocumentDiscardingDraft(
    api: FileGitRuntimeApi,
    path: string,
  ): Promise<string | null> {
    const document = documentFor(path)
    if (!document) {
      return openDocument(api, path)
    }
    return readDocument(api, document, true)
  }

  async function saveDocument(
    api: FileGitRuntimeApi,
    path: string,
    force = false,
  ): Promise<string | null> {
    const context = currentContext()
    const document = documentFor(path)
    if (!context || !document || document.loading || document.saving || !document.dirty) {
      return null
    }
    if (document.conflict && !force) {
      return 'revision_conflict'
    }
    const submittedText = document.draftText
    const submittedRevision = document.baseRevision
    document.saving = true
    document.error = null
    try {
      const response = await api.writeFile(context.workspaceId, {
        path,
        text: submittedText,
        expected_revision: submittedRevision,
        force,
      })
      if (!isCurrentContext(context)) {
        return null
      }
      const entry = response.result?.destination_entry ?? response.result?.source_entry
      if (!entry) {
        throw new Error('Write file response does not include the updated entry.')
      }
      document.entry = entry
      document.baseRevision = entry.revision
      document.originalText = submittedText
      if (document.draftText === submittedText) {
        document.draftText = submittedText
      }
      document.dirty = document.draftText !== document.originalText
      document.conflict = null
      invalidateMutation([parentPath(path), ...(response.result?.affected_path_prefixes ?? [])])
      return null
    } catch (error) {
      if (isRevisionConflict(error)) {
        document.conflict = error.details
        document.error = error.message
        return error.message
      }
      const message = errorMessage(error)
      document.error = message
      return message
    } finally {
      if (isCurrentContext(context)) {
        document.saving = false
      }
    }
  }

  function closeDocument(path: string): boolean {
    return closeDocuments([path])
  }

  function closeDocuments(paths: string[]): boolean {
    const targetDocuments = documentsForPaths(paths)
    if (targetDocuments.length === 0 || targetDocuments.some((document) => document.saving)) {
      return false
    }

    const closingKeys = new Set(targetDocuments.map((document) => document.key))
    const activeIndex = documents.value.findIndex(
      (document) => document.key === activeDocumentKey.value,
    )
    const activeDocumentIsClosing =
      activeIndex !== -1 && closingKeys.has(documents.value[activeIndex]!.key)
    const nextDocument = activeDocumentIsClosing
      ? (documents.value
          .slice(activeIndex + 1)
          .find((document) => !closingKeys.has(document.key)) ??
        documents.value
          .slice(0, activeIndex)
          .reverse()
          .find((document) => !closingKeys.has(document.key)) ??
        null)
      : null

    for (const document of targetDocuments) {
      abortRequest(`document:${document.key}`)
    }
    documents.value = documents.value.filter((document) => !closingKeys.has(document.key))
    if (activeDocumentIsClosing) {
      activeDocumentKey.value = nextDocument?.key ?? null
    }
    return true
  }

  async function createFile(
    api: FileGitRuntimeApi,
    request: CreateFileRequest,
  ): Promise<FileMutationResult | null> {
    if (request.overwrite && hasDirtyDocumentUnderPath(request.path)) {
      return null
    }
    return runMutation(api, async (context) => {
      if (
        !isCurrentContext(context) ||
        (request.overwrite && hasDirtyDocumentUnderPath(request.path))
      ) {
        return undefined
      }
      const response = await api.createFile(context.workspaceId, request)
      if (!isCurrentContext(context)) {
        return response.result
      }
      applyMutation(response.result, [parentPath(request.path), request.path])
      if (request.overwrite && !hasDirtyDocumentUnderPath(request.path)) {
        closeDocument(request.path)
      }
      return response.result
    })
  }

  async function createDirectory(
    api: FileGitRuntimeApi,
    request: CreateDirectoryRequest,
  ): Promise<FileMutationResult | null> {
    return runMutation(api, async (context) => {
      if (!isCurrentContext(context)) {
        return undefined
      }
      const response = await api.createDirectory(context.workspaceId, request)
      if (!isCurrentContext(context)) {
        return response.result
      }
      applyMutation(response.result, [parentPath(request.path), request.path])
      return response.result
    })
  }

  async function renameEntry(
    api: FileGitRuntimeApi,
    request: RenameEntryRequest,
  ): Promise<FileMutationResult | null> {
    if (hasDirtyDocumentUnderPath(request.path)) {
      return null
    }
    return runMutation(api, async (context) => {
      if (!isCurrentContext(context) || hasDirtyDocumentUnderPath(request.path)) {
        return undefined
      }
      const response = await api.renameEntry(context.workspaceId, request)
      if (!isCurrentContext(context)) {
        return response.result
      }
      applyMutation(response.result, [parentPath(request.path), request.path])
      relocateCleanDocuments(request.path, response.result?.destination_entry?.path ?? request.path)
      return response.result
    })
  }

  async function moveEntry(
    api: FileGitRuntimeApi,
    request: MoveEntryRequest,
  ): Promise<FileMutationResult | null> {
    if (hasDirtyDocumentUnderPath(request.source_path)) {
      return null
    }
    return runMutation(api, async (context) => {
      if (!isCurrentContext(context) || hasDirtyDocumentUnderPath(request.source_path)) {
        return undefined
      }
      const response = await api.moveEntry(context.workspaceId, request)
      if (!isCurrentContext(context)) {
        return response.result
      }
      applyMutation(response.result, [
        parentPath(request.source_path),
        parentPath(request.destination_path),
      ])
      relocateCleanDocuments(
        request.source_path,
        response.result?.destination_entry?.path ?? request.destination_path,
      )
      return response.result
    })
  }

  async function deleteEntry(
    api: FileGitRuntimeApi,
    request: DeleteEntryRequest,
  ): Promise<FileMutationResult | null> {
    if (hasDirtyDocumentUnderPath(request.path)) {
      return null
    }
    return runMutation(api, async (context) => {
      if (!isCurrentContext(context) || hasDirtyDocumentUnderPath(request.path)) {
        return undefined
      }
      const response = await api.deleteEntry(context.workspaceId, request)
      if (!isCurrentContext(context)) {
        return response.result
      }
      applyMutation(response.result, [parentPath(request.path), request.path])
      documents.value = documents.value.filter(
        (document) => !pathContains(request.path, document.path),
      )
      if (
        activeDocumentKey.value &&
        !documents.value.some((item) => item.key === activeDocumentKey.value)
      ) {
        activeDocumentKey.value = documents.value[0]?.key ?? null
      }
      return response.result
    })
  }

  async function refreshGitStatus(api: FileGitRuntimeApi): Promise<string | null> {
    const context = currentContext()
    if (!context) {
      return null
    }
    const request = beginRequest('git-status')
    gitStatusLoading.value = true
    gitStatusError.value = null
    try {
      const response = await api.gitStatus(context.workspaceId, {
        signal: request.controller.signal,
      })
      if (!isCurrentRequest(context, request)) {
        return null
      }
      gitStatus.value = response
      gitStatusStale.value = false
      if (!selectionIsAvailable(response, gitSelection.value)) {
        clearGitSelection()
      }
      return null
    } catch (error) {
      if (!isCurrentRequest(context, request) || isAbort(error)) {
        return null
      }
      const message = errorMessage(error)
      gitStatusError.value = message
      return message
    } finally {
      const current = isCurrentRequest(context, request)
      finishRequest(request)
      if (current) {
        gitStatusLoading.value = false
      }
    }
  }

  function selectGitChange(path: string, layer: GitLayer): boolean {
    if (!selectionIsAvailable(gitStatus.value, { path, layer })) {
      return false
    }
    gitSelection.value = { path, layer }
    abortRequest('git-diff')
    gitDiff.value = null
    gitDiffError.value = null
    gitDiffLoading.value = false
    return true
  }

  function clearGitSelection() {
    abortRequest('git-diff')
    gitSelection.value = null
    gitDiff.value = null
    gitDiffLoading.value = false
    gitDiffError.value = null
  }

  async function refreshGitDiff(api: FileGitRuntimeApi): Promise<string | null> {
    const context = currentContext()
    const selection = gitSelection.value
    if (!context || !selection || !selectionIsAvailable(gitStatus.value, selection)) {
      return null
    }
    const request = beginRequest('git-diff')
    gitDiffLoading.value = true
    gitDiffError.value = null
    try {
      const response = await api.gitDiff(context.workspaceId, selection.path, selection.layer, {
        signal: request.controller.signal,
      })
      if (!isCurrentRequest(context, request) || !sameSelection(selection, gitSelection.value)) {
        return null
      }
      gitDiff.value = response
      return null
    } catch (error) {
      if (
        !isCurrentRequest(context, request) ||
        !sameSelection(selection, gitSelection.value) ||
        isAbort(error)
      ) {
        return null
      }
      const message = errorMessage(error)
      gitDiffError.value = message
      return message
    } finally {
      const current = isCurrentRequest(context, request)
      finishRequest(request)
      if (current && sameSelection(selection, gitSelection.value)) {
        gitDiffLoading.value = false
      }
    }
  }

  function ensureDirectory(path: string): FileDirectoryState {
    const existing = directoryFor(path)
    if (existing) {
      return existing
    }
    const created: FileDirectoryState = {
      key: directoryKey(path),
      path,
      directory: null,
      items: [],
      expanded: false,
      loaded: false,
      loading: false,
      stale: false,
      truncated: false,
      error: null,
    }
    directories.value.push(created)
    // Return the reactive proxy from the array. Mutating the raw object
    // used for push does not trigger Vue updates.
    return directories.value[directories.value.length - 1]!
  }

  function ensureDocument(path: string): FileDocumentState {
    const existing = documentFor(path)
    if (existing) {
      return existing
    }
    const created: FileDocumentState = {
      key: documentKey(path),
      path,
      entry: null,
      baseRevision: '',
      originalText: '',
      draftText: '',
      dirty: false,
      loading: false,
      saving: false,
      conflict: null,
      error: null,
    }
    documents.value.push(created)
    // Return the reactive proxy from the array. Mutating the raw object
    // used for push does not trigger Vue updates.
    return documents.value[documents.value.length - 1]!
  }

  async function readDocument(
    api: FileGitRuntimeApi,
    document: FileDocumentState,
    discardDraft: boolean,
  ): Promise<string | null> {
    const context = currentContext()
    if (!context || document.loading || (!discardDraft && document.dirty)) {
      return discardDraft ? null : 'discard_required'
    }
    const request = beginRequest(`document:${document.key}`)
    document.loading = true
    document.error = null
    try {
      const response = await api.readFile(context.workspaceId, document.path, {
        signal: request.controller.signal,
      })
      if (!isCurrentRequest(context, request) || documentFor(document.path)?.key !== document.key) {
        return null
      }
      if (document.dirty && !discardDraft) {
        return 'discard_required'
      }
      document.entry = response.entry ?? null
      document.baseRevision = response.entry?.revision ?? ''
      document.originalText = response.text
      document.draftText = response.text
      document.dirty = false
      document.conflict = null
      return null
    } catch (error) {
      if (!isCurrentRequest(context, request) || isAbort(error)) {
        return null
      }
      const message = errorMessage(error)
      document.error = message
      return message
    } finally {
      const current = isCurrentRequest(context, request)
      finishRequest(request)
      if (current && documentFor(document.path)?.key === document.key) {
        document.loading = false
      }
    }
  }

  async function runMutation<T>(
    _api: FileGitRuntimeApi,
    operation: (context: ActiveContext) => Promise<T | undefined>,
  ): Promise<T | null> {
    const context = currentContext()
    if (!context) {
      return null
    }
    let result: T | undefined
    let failure: unknown
    const run = mutationTail.then(async () => {
      if (!isCurrentContext(context)) {
        return
      }
      try {
        result = await operation(context)
      } catch (error) {
        failure = error
      }
    })
    mutationTail = run.then(
      () => undefined,
      () => undefined,
    )
    await run
    if (failure) {
      throw failure
    }
    return result ?? null
  }

  function applyMutation(result: FileMutationResult | undefined, paths: string[]) {
    const affected = new Set(paths)
    for (const prefix of result?.affected_path_prefixes ?? []) {
      affected.add(prefix)
      affected.add(parentPath(prefix))
    }
    for (const directory of directories.value) {
      if (
        [...affected].some(
          (path) => pathContains(path, directory.path) || pathContains(directory.path, path),
        )
      ) {
        directory.stale = true
      }
    }
    gitStatusStale.value = true
    clearGitSelection()
  }

  function invalidateMutation(paths: string[]) {
    applyMutation(undefined, paths)
  }

  function relocateCleanDocuments(sourcePath: string, destinationPath: string) {
    for (const document of documentsUnderPath(sourcePath)) {
      if (document.dirty) {
        continue
      }
      const previousKey = document.key
      const suffix = document.path.slice(sourcePath.length)
      document.path = `${destinationPath}${suffix}`
      document.key = documentKey(document.path)
      if (activeDocumentKey.value === previousKey) {
        activeDocumentKey.value = document.key
      }
      if (document.entry) {
        document.entry = { ...document.entry, path: document.path }
      }
    }
  }

  function currentContext(): ActiveContext | null {
    if (!target.value || !workspaceId.value) {
      return null
    }
    return {
      targetKey: targetKey(target.value),
      workspaceId: workspaceId.value,
      epoch,
    }
  }

  function isCurrentContext(context: ActiveContext): boolean {
    return (
      context.epoch === epoch &&
      context.targetKey === targetKey(target.value) &&
      context.workspaceId === workspaceId.value
    )
  }

  function directoryKey(path: string): string {
    return scopedKey('directory', path)
  }

  function documentKey(path: string): string {
    return scopedKey('document', path)
  }

  function scopedKey(kind: string, path: string): string {
    return JSON.stringify([kind, targetKey(target.value), workspaceId.value ?? '', path])
  }

  function beginRequest(key: string): RequestToken {
    abortRequest(key)
    const controller = new AbortController()
    const id = ++requestSequence
    controllers.set(key, controller)
    requestIds.set(key, id)
    return { key, id, controller }
  }

  function finishRequest(request: RequestToken) {
    if (requestIds.get(request.key) === request.id) {
      controllers.delete(request.key)
    }
  }

  function abortRequest(key: string) {
    controllers.get(key)?.abort()
    controllers.delete(key)
    requestIds.delete(key)
  }

  function abortAllRequests() {
    for (const controller of controllers.values()) {
      controller.abort()
    }
    controllers.clear()
    requestIds.clear()
  }

  function isCurrentRequest(context: ActiveContext, request: RequestToken): boolean {
    return isCurrentContext(context) && requestIds.get(request.key) === request.id
  }

  function resetGitState() {
    gitStatus.value = null
    gitStatusLoading.value = false
    gitStatusStale.value = false
    gitStatusError.value = null
    gitSelection.value = null
    gitDiff.value = null
    gitDiffLoading.value = false
    gitDiffError.value = null
  }

  return {
    target,
    workspaceId,
    directories,
    documents,
    activeDocumentKey,
    activeDocument,
    gitStatus,
    gitStatusLoading,
    gitStatusStale,
    gitStatusError,
    gitSelection,
    gitDiff,
    gitDiffLoading,
    gitDiffError,
    openWorkspace,
    resetForSourceChange,
    removeWorkspace,
    cancelReadonlyRequests,
    directoryFor,
    documentFor,
    documentsUnderPath,
    discardDocumentDrafts,
    hasDirtyDocumentUnderPath,
    setDirectoryExpanded,
    toggleDirectoryExpanded,
    ensureDirectoryLoaded,
    refreshDirectory,
    openDocument,
    setActiveDocument,
    setDocumentDraft,
    discardDocumentDraft,
    refreshDocument,
    reloadDocumentDiscardingDraft,
    saveDocument,
    closeDocument,
    closeDocuments,
    createFile,
    createDirectory,
    renameEntry,
    moveEntry,
    deleteEntry,
    refreshGitStatus,
    selectGitChange,
    clearGitSelection,
    refreshGitDiff,
  }
})

type ActiveContext = {
  targetKey: string
  workspaceId: string
  epoch: number
}

type RequestToken = {
  key: string
  id: number
  controller: AbortController
}

function cloneTarget(value: RuntimeTarget): RuntimeTarget {
  return value.mode === 'local' ? { mode: 'local' } : { mode: 'cloud', deviceId: value.deviceId }
}

function scopeKey(target: RuntimeTarget | null, workspaceId: string | null): string | null {
  return target && workspaceId ? JSON.stringify([targetKey(target), workspaceId]) : null
}

function targetKey(value: RuntimeTarget | null): string {
  if (!value) {
    return ''
  }
  return value.mode === 'local' ? 'local' : `cloud:${value.deviceId}`
}

function parentPath(path: string): string {
  const index = path.lastIndexOf('/')
  return index === -1 ? '' : path.slice(0, index)
}

function pathContains(parent: string, path: string): boolean {
  return parent === '' || path === parent || path.startsWith(`${parent}/`)
}

function sameSelection(left: GitSelection | null, right: GitSelection | null): boolean {
  return left?.path === right?.path && left?.layer === right?.layer
}

function selectionIsAvailable(
  status: GitStatusResp | null,
  selection: GitSelection | null,
): boolean {
  if (!status || !selection) {
    return false
  }
  return status.changes.some(
    (change) => change.path === selection.path && change.available_layers.includes(selection.layer),
  )
}

function isRevisionConflict(error: unknown): error is {
  code: string
  details: FileConflict
  message: string
} {
  return (
    typeof error === 'object' &&
    error !== null &&
    'code' in error &&
    (error as { code?: unknown }).code === 'revision_conflict' &&
    'details' in error &&
    isFileConflict((error as { details?: unknown }).details) &&
    'message' in error &&
    typeof (error as { message?: unknown }).message === 'string'
  )
}

function isFileConflict(value: unknown): value is FileConflict {
  return (
    typeof value === 'object' &&
    value !== null &&
    'type' in value &&
    (value as { type?: unknown }).type === 'revision_conflict' &&
    'current_entry' in value &&
    (value as FileConflict).current_entry !== undefined
  )
}

function isAbort(error: unknown): boolean {
  return (
    (error instanceof Error && error.name === 'AbortError') ||
    (typeof error === 'object' &&
      error !== null &&
      'code' in error &&
      (error as { code?: unknown }).code === 'ERR_CANCELED')
  )
}
