import { Emitter, type Event } from '@codingame/monaco-vscode-api/vscode/vs/base/common/event'
import { URI } from '@codingame/monaco-vscode-api/vscode/vs/base/common/uri'
import type { IDisposable } from '@codingame/monaco-vscode-api/vscode/vs/base/common/lifecycle'
import {
  FileChangeType,
  FilePermission,
  FileSystemProviderCapabilities,
  FileSystemProviderError,
  FileSystemProviderErrorCode,
  FileType,
  type IFileChange,
  type IFileDeleteOptions,
  type IFileOverwriteOptions,
  type IFileSystemProviderWithFileReadWriteCapability,
  type IFileWriteOptions,
  type IStat,
} from '@codingame/monaco-vscode-files-service-override'
import { FileType as ProtoFileType, FsChangeKind } from '../../gen/proto/termbridge/agent/v1/file'
import type { WorkbenchApiError, WorkbenchFsApi, WorkbenchWorkspaceChange } from './api'
import { uriToWorkspacePath, WORKBENCH_SCHEME } from './uri'

type CachedContent = {
  content: Uint8Array
  mtime: number
  size: number
}

/**
 * Cache bounds live with the Provider cache policy, rather than scattered
 * across request paths. They cap retained workspace state without weakening
 * FileService's correctness guarantees.
 */
const WORKSPACE_CACHE_LIMITS = {
  contentEntries: 64,
  contentBytes: 16 * 1024 * 1024,
  statEntries: 512,
  readdirEntries: 256,
  writeBaseEntries: 512,
} as const

/**
 * Main-thread FileService provider (registered via registerCustomProvider).
 * Must be registered BEFORE workbench initialize so Explorer uses remote FS.
 *
 * Product model (demo-aligned):
 * - readFile is byte-transparent (images / media preview / VS Code FS contract)
 * - writeFile remains a text-editor filesystem (UTF-8, no NUL, size limits)
 * - backend write rejects non-text with file_not_text / file_too_large
 *
 * Remote cost note (vs demo memory FS):
 * Workbench open path legitimately calls provider.stat/readFile many times
 * (FileService validate + text model + language features + re-focus). Demo
 * pays nothing because RegisteredMemoryFile is in-process. We:
 * - coalesce in-flight identical GETs
 * - cache stat/readdir/content until local mutation (no file watcher yet)
 * so Explorer host-focus refresh + autoReveal do not re-hit the network.
 */
export class TermBridgePlatformFileSystemProvider implements IFileSystemProviderWithFileReadWriteCapability {
  private api: WorkbenchFsApi | null = null
  private onMutated: (() => void) | null = null
  private readonly changeEmitter = new Emitter<readonly IFileChange[]>()
  private readonly capabilitiesEmitter = new Emitter<void>()
  /**
   * Last revision supplied with file bytes. The provider write options do not
   * carry VS Code's model ETag, so HTTP OCC uses this conservative base.
   */
  private readonly writeBaseEtags = new Map<string, string>()
  /** path -> last known IStat (for mtime/size content hits) */
  private readonly stats = new Map<string, IStat>()
  /** path -> last read content tied to mtime+size */
  private readonly contents = new Map<string, CachedContent>()
  private readonly readdirs = new Map<string, [string, FileType][]>()
  private readonly inflightStat = new Map<string, Promise<IStat>>()
  private readonly inflightRead = new Map<string, Promise<Uint8Array>>()
  private readonly inflightReaddir = new Map<string, Promise<[string, FileType][]>>()
  private readonly pathEpochs = new Map<string, number>()
  private contentBytes = 0
  private globalEpoch = 0

  readonly capabilities =
    FileSystemProviderCapabilities.FileReadWrite | FileSystemProviderCapabilities.PathCaseSensitive

  readonly onDidChangeCapabilities: Event<void> = this.capabilitiesEmitter.event
  readonly onDidChangeFile: Event<readonly IFileChange[]> = this.changeEmitter.event

  bind(api: WorkbenchFsApi, onMutated?: () => void): void {
    this.api = api
    this.onMutated = onMutated ?? null
    this.clearCaches()
  }

  private clearCaches(): void {
    // Binding selects a different workspace API. No response started for the
    // previous binding may populate this workspace's cache or write base.
    this.globalEpoch += 1
    this.writeBaseEtags.clear()
    this.stats.clear()
    this.contents.clear()
    this.readdirs.clear()
    this.pathEpochs.clear()
    this.contentBytes = 0
    this.inflightStat.clear()
    this.inflightRead.clear()
    this.inflightReaddir.clear()
  }

  private notifyMutated(): void {
    try {
      this.onMutated?.()
    } catch {
      // ignore refresh errors
    }
  }

  private forgetCachedPath(path: string): void {
    this.stats.delete(path)
    this.deleteContent(path)
    this.readdirs.delete(path)
  }

  private rememberWriteBase(path: string, etag: string | undefined): void {
    if (!etag) {
      return
    }
    this.writeBaseEtags.delete(path)
    this.writeBaseEtags.set(path, etag)
    trimOldestEntries(this.writeBaseEtags, WORKSPACE_CACHE_LIMITS.writeBaseEntries)
  }

  private currentWriteBase(path: string): string | undefined {
    const etag = this.writeBaseEtags.get(path)
    if (etag) {
      this.writeBaseEtags.delete(path)
      this.writeBaseEtags.set(path, etag)
    }
    return etag
  }

  private deleteContent(path: string): void {
    const cached = this.contents.get(path)
    if (cached) {
      this.contentBytes -= cached.content.byteLength
      this.contents.delete(path)
    }
  }

  private rememberContentEntry(path: string, content: CachedContent): void {
    this.deleteContent(path)
    this.contents.set(path, content)
    this.contentBytes += content.content.byteLength
    while (
      this.contents.size > WORKSPACE_CACHE_LIMITS.contentEntries ||
      this.contentBytes > WORKSPACE_CACHE_LIMITS.contentBytes
    ) {
      const oldestPath = this.contents.keys().next().value
      if (oldestPath === undefined) {
        break
      }
      this.deleteContent(oldestPath)
    }
  }

  private touchContent(path: string, cached: CachedContent): void {
    this.contents.delete(path)
    this.contents.set(path, cached)
  }

  private forgetPath(path: string): void {
    this.writeBaseEtags.delete(path)
    this.forgetCachedPath(path)
  }

  private pathEpoch(path: string): number {
    return this.pathEpochs.get(path) ?? 0
  }

  private markPathStale(path: string): void {
    this.pathEpochs.set(path, this.pathEpoch(path) + 1)
  }

  private markSubtreeStale(path: string): void {
    const prefix = path ? `${path}/` : ''
    for (const knownPath of [
      ...this.writeBaseEtags.keys(),
      ...this.stats.keys(),
      ...this.contents.keys(),
      ...this.readdirs.keys(),
      ...this.pathEpochs.keys(),
    ]) {
      if (!path || knownPath === path || knownPath.startsWith(prefix)) {
        this.markPathStale(knownPath)
      }
    }
    this.markPathStale(path)
  }

  private forgetCachedSubtree(path: string): void {
    const prefix = path ? `${path}/` : ''
    for (const knownPath of new Set([
      ...this.stats.keys(),
      ...this.contents.keys(),
      ...this.readdirs.keys(),
    ])) {
      if (!path || knownPath === path || knownPath.startsWith(prefix)) {
        this.forgetCachedPath(knownPath)
      }
    }
    this.forgetCachedPath(path)
  }

  /** Drop all FS metadata/content caches when event continuity is unknown. */
  invalidateAll(): void {
    this.globalEpoch += 1
    this.stats.clear()
    this.contents.clear()
    this.readdirs.clear()
    this.contentBytes = 0
  }

  /**
   * When a watch gap, reconnect, or external resume makes cache continuity
   * unknowable, ask FileService to reload from the workspace root.
   */
  rescanRequired(): void {
    this.invalidateAll()
    this.changeEmitter.fire([{ type: FileChangeType.UPDATED, resource: workspaceRootResource() }])
  }

  private parentPath(path: string): string {
    if (!path || path === '/' || path === '.') {
      return ''
    }
    const normalized = path.replace(/\\/g, '/').replace(/\/+$/, '')
    const idx = normalized.lastIndexOf('/')
    if (idx <= 0) {
      return ''
    }
    return normalized.slice(0, idx)
  }

  private forgetPathAndParents(path: string): void {
    this.markPathStale(path)
    this.forgetPath(path)
    // Parent directory listings must refresh after child mutations.
    const parent = this.parentPath(path)
    this.markPathStale(parent)
    this.readdirs.delete(parent)
    this.stats.delete(parent)
  }

  applyWorkspaceChange(change: WorkbenchWorkspaceChange): void {
    if (change.kind === FsChangeKind.FS_CHANGE_KIND_RESCAN_REQUIRED) {
      this.rescanRequired()
      return
    }
    const path = normalizeWorkspacePath(change.path)
    const oldPath = normalizeWorkspacePath(change.oldPath)
    if (!path) {
      // Never emit a synthetic root delete: it can make FileService treat all
      // open children as removed. An ambiguous path is a conservative rescan.
      this.rescanRequired()
      return
    }
    switch (change.kind) {
      case FsChangeKind.FS_CHANGE_KIND_ADDED:
        this.invalidatePathAndParent(path)
        this.changeEmitter.fire([{ type: FileChangeType.ADDED, resource: workspaceResource(path) }])
        return
      case FsChangeKind.FS_CHANGE_KIND_UPDATED:
        this.invalidatePathAndParent(path)
        this.changeEmitter.fire([
          { type: FileChangeType.UPDATED, resource: workspaceResource(path) },
        ])
        return
      case FsChangeKind.FS_CHANGE_KIND_DELETED:
        this.invalidateSubtreeAndParent(path)
        this.changeEmitter.fire([
          { type: FileChangeType.DELETED, resource: workspaceResource(path) },
        ])
        return
      case FsChangeKind.FS_CHANGE_KIND_RENAMED:
        if (!oldPath) {
          this.rescanRequired()
          return
        }
        this.invalidateSubtreeAndParent(oldPath)
        this.invalidateSubtreeAndParent(path)
        this.changeEmitter.fire([
          { type: FileChangeType.DELETED, resource: workspaceResource(oldPath) },
          { type: FileChangeType.ADDED, resource: workspaceResource(path) },
        ])
        return
      default:
        this.rescanRequired()
    }
  }

  private invalidatePathAndParent(path: string): void {
    this.markPathStale(path)
    // Keep the write base for a possibly dirty model. The next save must
    // compare it with the remote file rather than adopt the external revision.
    this.forgetCachedPath(path)
    const parent = this.parentPath(path)
    this.markPathStale(parent)
    this.readdirs.delete(parent)
    this.stats.delete(parent)
  }

  private invalidateSubtreeAndParent(path: string): void {
    this.markSubtreeStale(path)
    // Same as single-path invalidation: retain the pre-change write bases so
    // stale dirty documents cannot be silently rebased by a watcher event.
    this.forgetCachedSubtree(path)
    const parent = this.parentPath(path)
    this.markPathStale(parent)
    this.readdirs.delete(parent)
    this.stats.delete(parent)
  }

  private rememberStat(path: string, stat: IStat): IStat {
    this.stats.delete(path)
    this.stats.set(path, stat)
    trimOldestEntries(this.stats, WORKSPACE_CACHE_LIMITS.statEntries)
    const cached = this.contents.get(path)
    if (cached && (cached.mtime !== stat.mtime || cached.size !== stat.size)) {
      this.deleteContent(path)
    }
    return stat
  }

  private rememberReaddir(path: string, entries: [string, FileType][]): void {
    this.readdirs.delete(path)
    this.readdirs.set(path, entries)
    trimOldestEntries(this.readdirs, WORKSPACE_CACHE_LIMITS.readdirEntries)
  }

  private rememberContent(
    path: string,
    content: Uint8Array,
    meta?: { mtime?: number; size?: number; etag?: string },
  ): Uint8Array {
    const prev = this.stats.get(path)
    const mtime = typeof meta?.mtime === 'number' ? meta.mtime : (prev?.mtime ?? 0)
    const resolvedSize =
      typeof meta?.size === 'number' ? meta.size : (prev?.size ?? content.byteLength)
    this.rememberContentEntry(path, { content, mtime, size: resolvedSize })
    this.rememberWriteBase(path, meta?.etag)
    if (prev) {
      this.rememberStat(path, {
        ...prev,
        mtime: typeof meta?.mtime === 'number' ? meta.mtime : prev.mtime,
        size: resolvedSize,
      })
    }
    return content
  }

  private cachedContent(path: string): Uint8Array | undefined {
    const cached = this.contents.get(path)
    if (!cached) {
      return undefined
    }
    const stat = this.stats.get(path)
    if (!stat) {
      // Content without a matching recent stat is still usable for pure
      // duplicate readFile storms right after the first read populated both.
      this.touchContent(path, cached)
      return cached.content
    }
    if (cached.mtime === stat.mtime && cached.size === stat.size) {
      this.touchContent(path, cached)
      return cached.content
    }
    return undefined
  }

  watch(): IDisposable {
    return { dispose() {} }
  }

  async stat(resource: URI): Promise<IStat> {
    const api = this.requireApi()
    const path = uriToWorkspacePath(resource)
    const cached = this.stats.get(path)
    if (cached) {
      return cached
    }
    const existing = this.inflightStat.get(path)
    if (existing) {
      return existing
    }
    const globalEpoch = this.globalEpoch
    const pathEpoch = this.pathEpoch(path)
    const pending = (async () => {
      try {
        const stat = await api.stat(path)
        const value = {
          type: toPlatformFileType(stat.type),
          ctime: Number(stat.ctime) || 0,
          mtime: Number(stat.mtime) || 0,
          size: Number(stat.size) || 0,
          permissions: Number(stat.permissions) === 1 ? FilePermission.Readonly : undefined,
        }
        if (this.globalEpoch === globalEpoch && this.pathEpoch(path) === pathEpoch) {
          return this.rememberStat(path, value)
        }
        return value
      } catch (error) {
        this.forgetPath(path)
        throw mapPlatformError(error)
      } finally {
        this.inflightStat.delete(path)
      }
    })()
    this.inflightStat.set(path, pending)
    return pending
  }

  async readdir(resource: URI): Promise<[string, FileType][]> {
    const api = this.requireApi()
    const path = uriToWorkspacePath(resource)
    const cached = this.readdirs.get(path)
    if (cached) {
      return cached
    }
    const existing = this.inflightReaddir.get(path)
    if (existing) {
      return existing
    }
    const globalEpoch = this.globalEpoch
    const pathEpoch = this.pathEpoch(path)
    const pending = (async () => {
      try {
        const result = await api.readDirectory(path)
        if (result.truncated) {
          throw FileSystemProviderError.create(
            'Directory listing exceeds the workspace entry limit.',
            FileSystemProviderErrorCode.Unknown,
          )
        }
        const entries: [string, FileType][] = (result.entries ?? []).map((entry) => [
          entry.name,
          toPlatformFileType(entry.type),
        ])
        if (this.globalEpoch === globalEpoch && this.pathEpoch(path) === pathEpoch) {
          this.rememberReaddir(path, entries)
        }
        return entries
      } catch (error) {
        throw mapPlatformError(error)
      } finally {
        this.inflightReaddir.delete(path)
      }
    })()
    this.inflightReaddir.set(path, pending)
    return pending
  }

  async readFile(resource: URI): Promise<Uint8Array> {
    const api = this.requireApi()
    const path = uriToWorkspacePath(resource)
    const hit = this.cachedContent(path)
    if (hit) {
      return hit
    }
    const existing = this.inflightRead.get(path)
    if (existing) {
      return existing
    }
    const globalEpoch = this.globalEpoch
    const pathEpoch = this.pathEpoch(path)
    const pending = (async () => {
      try {
        // Re-check after joining queue — earlier peer may have filled cache.
        const again = this.cachedContent(path)
        if (again) {
          return again
        }
        const result = await api.readFile(path)
        if (this.globalEpoch !== globalEpoch || this.pathEpoch(path) !== pathEpoch) {
          return result.content
        }
        if (result.stat) {
          this.rememberStat(path, {
            type: toPlatformFileType(result.stat.type),
            ctime: Number(result.stat.ctime) || 0,
            mtime: Number(result.stat.mtime) || 0,
            size: Number(result.stat.size) || 0,
            permissions:
              Number(result.stat.permissions) === 1 ? FilePermission.Readonly : undefined,
          })
        }
        return this.rememberContent(path, result.content, {
          mtime: result.stat ? Number(result.stat.mtime) || 0 : undefined,
          size: result.stat ? Number(result.stat.size) || 0 : result.content.byteLength,
          etag: result.stat?.etag,
        })
      } catch (error) {
        throw mapPlatformError(error)
      } finally {
        this.inflightRead.delete(path)
      }
    })()
    this.inflightRead.set(path, pending)
    return pending
  }

  async writeFile(resource: URI, content: Uint8Array, opts: IFileWriteOptions): Promise<void> {
    const api = this.requireApi()
    const path = uriToWorkspacePath(resource)
    try {
      const stat = await api.writeFile(path, content, {
        create: opts.create,
        overwrite: opts.overwrite,
        // The public provider contract omits the model's revision. Use the
        // last content revision, and let an unknown base fail safely server-side.
        etag: this.currentWriteBase(path),
      })
      this.markPathStale(path)
      this.readdirs.delete(this.parentPath(path))
      if (stat) {
        this.rememberStat(path, {
          type: toPlatformFileType(stat.type),
          ctime: Number(stat.ctime) || 0,
          mtime: Number(stat.mtime) || 0,
          size: Number(stat.size) || content.byteLength,
          permissions: Number(stat.permissions) === 1 ? FilePermission.Readonly : undefined,
        })
        this.rememberContent(path, content, {
          mtime: Number(stat.mtime) || 0,
          size: Number(stat.size) || content.byteLength,
          etag: stat.etag,
        })
      } else {
        this.forgetPathAndParents(path)
      }
      this.changeEmitter.fire([{ type: FileChangeType.UPDATED, resource }])
      this.notifyMutated()
    } catch (error) {
      throw mapPlatformError(error)
    }
  }

  async mkdir(resource: URI): Promise<void> {
    const api = this.requireApi()
    const path = uriToWorkspacePath(resource)
    try {
      this.markPathStale(path)
      this.readdirs.delete(this.parentPath(path))
      const stat = await api.createDirectory(path)
      if (stat) {
        this.rememberStat(path, {
          type: toPlatformFileType(stat.type),
          ctime: Number(stat.ctime) || 0,
          mtime: Number(stat.mtime) || 0,
          size: Number(stat.size) || 0,
          permissions: Number(stat.permissions) === 1 ? FilePermission.Readonly : undefined,
        })
      }
      this.changeEmitter.fire([{ type: FileChangeType.ADDED, resource }])
      this.notifyMutated()
    } catch (error) {
      throw mapPlatformError(error)
    }
  }

  async delete(resource: URI, opts: IFileDeleteOptions): Promise<void> {
    const api = this.requireApi()
    const path = uriToWorkspacePath(resource)
    try {
      await api.delete(path, { recursive: opts.recursive })
      this.forgetPathAndParents(path)
      this.changeEmitter.fire([{ type: FileChangeType.DELETED, resource }])
      this.notifyMutated()
    } catch (error) {
      throw mapPlatformError(error)
    }
  }

  async rename(from: URI, to: URI, opts: IFileOverwriteOptions): Promise<void> {
    const api = this.requireApi()
    const fromPath = uriToWorkspacePath(from)
    const toPath = uriToWorkspacePath(to)
    try {
      const stat = await api.rename(fromPath, toPath, {
        overwrite: opts.overwrite,
      })
      const cached = this.contents.get(fromPath)
      this.forgetPathAndParents(fromPath)
      this.readdirs.delete(this.parentPath(toPath))
      if (stat) {
        this.rememberStat(toPath, {
          type: toPlatformFileType(stat.type),
          ctime: Number(stat.ctime) || 0,
          mtime: Number(stat.mtime) || 0,
          size: Number(stat.size) || 0,
          permissions: Number(stat.permissions) === 1 ? FilePermission.Readonly : undefined,
        })
        if (cached) {
          this.rememberContent(toPath, cached.content, {
            mtime: Number(stat.mtime) || cached.mtime,
            size: Number(stat.size) || cached.size,
            etag: stat.etag,
          })
        }
      } else {
        this.forgetPath(toPath)
      }
      this.changeEmitter.fire([
        { type: FileChangeType.DELETED, resource: from },
        { type: FileChangeType.ADDED, resource: to },
      ])
      this.notifyMutated()
    } catch (error) {
      throw mapPlatformError(error)
    }
  }

  private requireApi(): WorkbenchFsApi {
    if (!this.api) {
      throw FileSystemProviderError.create(
        'TermBridge filesystem is not bound to a workspace yet.',
        FileSystemProviderErrorCode.Unavailable,
      )
    }
    return this.api
  }
}

function trimOldestEntries<T>(entries: Map<string, T>, limit: number): void {
  while (entries.size > limit) {
    const oldestPath = entries.keys().next().value
    if (oldestPath === undefined) {
      return
    }
    entries.delete(oldestPath)
  }
}

function workspaceRootResource(): URI {
  return URI.from({ scheme: WORKBENCH_SCHEME, path: '/' })
}

function workspaceResource(path: string): URI {
  // FileService change handlers call URI methods such as resource.with().
  // Plain shape objects cast as URI crash Explorer refresh with:
  // "TypeError: resource.with is not a function".
  return URI.from({
    scheme: WORKBENCH_SCHEME,
    path: path ? `/${path}` : '/',
  })
}

function normalizeWorkspacePath(path: string): string {
  if (!path || path.startsWith('/') || path.includes('\\') || path.includes('\0')) {
    return ''
  }
  const normalized = path.replace(/\/+$/, '')
  if (
    !normalized ||
    normalized.split('/').some((part) => part === '' || part === '.' || part === '..')
  ) {
    return ''
  }
  return normalized
}

function toPlatformFileType(type: ProtoFileType | number): FileType {
  switch (Number(type)) {
    case ProtoFileType.FILE_TYPE_DIRECTORY:
      return FileType.Directory
    case ProtoFileType.FILE_TYPE_SYMBOLIC_LINK:
      return FileType.SymbolicLink
    case ProtoFileType.FILE_TYPE_FILE:
      return FileType.File
    default:
      return FileType.Unknown
  }
}

function mapPlatformError(error: unknown): Error {
  if (error instanceof FileSystemProviderError) {
    return error
  }
  const apiError = error as WorkbenchApiError
  const code = typeof apiError?.code === 'string' ? apiError.code : ''
  const message = error instanceof Error ? error.message : 'TermBridge filesystem error'
  switch (code) {
    case 'not_found':
    case 'file_not_found':
      return FileSystemProviderError.create(message, FileSystemProviderErrorCode.FileNotFound)
    case 'already_exists':
    case 'conflict':
    case 'revision_conflict':
      // The public files-service override only exposes provider error codes;
      // it does not export VS Code's FILE_MODIFIED_SINCE operation result.
      // Preserve OCC by rejecting the write, but do not claim this generic
      // provider code unlocks the native dirty-write recovery flow.
      return FileSystemProviderError.create(message, FileSystemProviderErrorCode.FileExists)
    case 'precondition_required':
      return FileSystemProviderError.create(message, FileSystemProviderErrorCode.Unknown)
    case 'file_too_large':
      return FileSystemProviderError.create(message, FileSystemProviderErrorCode.FileTooLarge)
    case 'file_not_text':
    case 'type_mismatch':
    case 'symlink_unsupported':
      // Text-editor FS limits; surface as Unknown with explicit message rather than silent fail.
      return FileSystemProviderError.create(message, FileSystemProviderErrorCode.Unknown)
    case 'unauthorized':
    case 'forbidden':
      return FileSystemProviderError.create(message, FileSystemProviderErrorCode.NoPermissions)
    case 'device_offline':
    case 'service_unavailable':
      return FileSystemProviderError.create(message, FileSystemProviderErrorCode.Unavailable)
    default:
      if (apiError?.status === 404) {
        return FileSystemProviderError.create(message, FileSystemProviderErrorCode.FileNotFound)
      }
      if (apiError?.status === 409) {
        return FileSystemProviderError.create(message, FileSystemProviderErrorCode.FileExists)
      }
      if (apiError?.status === 422) {
        return FileSystemProviderError.create(message, FileSystemProviderErrorCode.Unknown)
      }
      return FileSystemProviderError.create(message, FileSystemProviderErrorCode.Unknown)
  }
}
