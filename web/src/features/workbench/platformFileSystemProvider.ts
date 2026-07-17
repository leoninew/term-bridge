import {
  Emitter,
  type Event,
} from '@codingame/monaco-vscode-api/vscode/vs/base/common/event'
import type { URI } from '@codingame/monaco-vscode-api/vscode/vs/base/common/uri'
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
  type IWatchOptions,
} from '@codingame/monaco-vscode-files-service-override'
import { FileType as ProtoFileType } from '../../gen/proto/termbridge/agent/v1/file'
import type { WorkbenchApiError, WorkbenchFsApi } from './api'
import { uriToWorkspacePath } from './uri'

/**
 * Main-thread FileService provider (registered via registerCustomProvider).
 * Must be registered BEFORE workbench initialize so Explorer uses remote FS.
 *
 * Product model (demo-aligned):
 * - readFile is byte-transparent (images / media preview / VS Code FS contract)
 * - writeFile remains a text-editor filesystem (UTF-8, no NUL, size limits)
 * - backend write rejects non-text with file_not_text / file_too_large
 */
export class TermBridgePlatformFileSystemProvider
  implements IFileSystemProviderWithFileReadWriteCapability
{
  private api: WorkbenchFsApi | null = null
  private onMutated: (() => void) | null = null
  private readonly changeEmitter = new Emitter<readonly IFileChange[]>()
  private readonly capabilitiesEmitter = new Emitter<void>()
  /** path -> etag from last successful stat/read/write */
  private readonly etags = new Map<string, string>()

  readonly capabilities =
    FileSystemProviderCapabilities.FileReadWrite |
    FileSystemProviderCapabilities.PathCaseSensitive

  readonly onDidChangeCapabilities: Event<void> = this.capabilitiesEmitter.event
  readonly onDidChangeFile: Event<readonly IFileChange[]> = this.changeEmitter.event

  bind(api: WorkbenchFsApi, onMutated?: () => void): void {
    this.api = api
    this.onMutated = onMutated ?? null
    this.etags.clear()
  }

  private notifyMutated(): void {
    try {
      this.onMutated?.()
    } catch {
      // ignore refresh errors
    }
  }

  private rememberEtag(path: string, etag: string | undefined): void {
    if (etag) {
      this.etags.set(path, etag)
    }
  }

  private forgetEtag(path: string): void {
    this.etags.delete(path)
  }

  watch(_resource: URI, _opts: IWatchOptions): IDisposable {
    return { dispose() {} }
  }

  async stat(resource: URI): Promise<IStat> {
    const api = this.requireApi()
    const path = uriToWorkspacePath(resource)
    try {
      const stat = await api.stat(path)
      this.rememberEtag(path, stat.etag)
      return {
        type: toPlatformFileType(stat.type),
        ctime: Number(stat.ctime) || 0,
        mtime: Number(stat.mtime) || 0,
        size: Number(stat.size) || 0,
        permissions:
          Number(stat.permissions) === 1 ? FilePermission.Readonly : undefined,
      }
    } catch (error) {
      throw mapPlatformError(error)
    }
  }

  async readdir(resource: URI): Promise<[string, FileType][]> {
    const api = this.requireApi()
    try {
      const result = await api.readDirectory(uriToWorkspacePath(resource))
      return (result.entries ?? []).map((entry) => [
        entry.name,
        toPlatformFileType(entry.type),
      ])
    } catch (error) {
      throw mapPlatformError(error)
    }
  }

  async readFile(resource: URI): Promise<Uint8Array> {
    const api = this.requireApi()
    const path = uriToWorkspacePath(resource)
    try {
      const result = await api.readFile(path)
      this.rememberEtag(path, result.stat?.etag)
      return result.content
    } catch (error) {
      throw mapPlatformError(error)
    }
  }

  async writeFile(resource: URI, content: Uint8Array, opts: IFileWriteOptions): Promise<void> {
    const api = this.requireApi()
    const path = uriToWorkspacePath(resource)
    try {
      const stat = await api.writeFile(path, content, {
        create: opts.create,
        overwrite: opts.overwrite,
        // Prefer last known revision so multi-client saves surface revision_conflict.
        etag: this.etags.get(path),
      })
      if (stat?.etag) {
        this.rememberEtag(path, stat.etag)
      } else {
        // Force next write to re-stat if backend omitted etag.
        this.forgetEtag(path)
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
      const stat = await api.createDirectory(path)
      this.rememberEtag(path, stat?.etag)
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
      this.forgetEtag(path)
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
      this.forgetEtag(fromPath)
      this.rememberEtag(toPath, stat?.etag)
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
  const message =
    error instanceof Error ? error.message : 'TermBridge filesystem error'
  switch (code) {
    case 'not_found':
    case 'file_not_found':
      return FileSystemProviderError.create(message, FileSystemProviderErrorCode.FileNotFound)
    case 'already_exists':
    case 'conflict':
    case 'revision_conflict':
      // revision_conflict: concurrent edit; FileExists is the closest provider code
      // (VS Code FileService maps provider FileExists to dirty-write UX).
      return FileSystemProviderError.create(message, FileSystemProviderErrorCode.FileExists)
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
