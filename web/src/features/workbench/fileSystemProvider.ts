import type {
  Disposable,
  FileChangeEvent,
  FileStat,
  FileSystemProvider,
  FileType as VscodeFileType,
  Uri,
} from 'vscode'
import { FileType as ProtoFileType } from '../../gen/proto/termbridge/agent/v1/file'
import type { WorkbenchFsApi, WorkbenchApiError } from './api'
import { uriToWorkspacePath } from './uri'

type VscodeApi = typeof import('vscode')

export function createTermBridgeFileSystemProvider(
  vscode: VscodeApi,
  api: WorkbenchFsApi,
): FileSystemProvider & { dispose: () => void } {
  const emitter = new vscode.EventEmitter<FileChangeEvent[]>()
  let disposed = false

  const provider: FileSystemProvider & { dispose: () => void } = {
    onDidChangeFile: emitter.event,

    watch(): Disposable {
      return new vscode.Disposable(() => undefined)
    },

    async stat(uri: Uri): Promise<FileStat> {
      try {
        const stat = await api.stat(uriToWorkspacePath(uri))
        return toVscodeStat(vscode, stat)
      } catch (error) {
        throw mapFsError(vscode, error)
      }
    },

    async readDirectory(uri: Uri): Promise<[string, VscodeFileType][]> {
      try {
        const result = await api.readDirectory(uriToWorkspacePath(uri))
        return (result.entries ?? []).map((entry: { name: string; type: ProtoFileType }) => [
          entry.name,
          toVscodeFileType(vscode, entry.type),
        ])
      } catch (error) {
        throw mapFsError(vscode, error)
      }
    },

    async readFile(uri: Uri): Promise<Uint8Array> {
      try {
        const result = await api.readFile(uriToWorkspacePath(uri))
        return result.content
      } catch (error) {
        throw mapFsError(vscode, error)
      }
    },

    async writeFile(
      uri: Uri,
      content: Uint8Array,
      options: { create: boolean; overwrite: boolean },
    ): Promise<void> {
      try {
        await api.writeFile(uriToWorkspacePath(uri), content, {
          create: options.create,
          overwrite: options.overwrite,
        })
        if (!disposed) {
          emitter.fire([{ type: vscode.FileChangeType.Changed, uri }])
        }
      } catch (error) {
        throw mapFsError(vscode, error)
      }
    },

    async createDirectory(uri: Uri): Promise<void> {
      try {
        await api.createDirectory(uriToWorkspacePath(uri))
        if (!disposed) {
          emitter.fire([{ type: vscode.FileChangeType.Created, uri }])
        }
      } catch (error) {
        throw mapFsError(vscode, error)
      }
    },

    async delete(uri: Uri, options: { recursive: boolean }): Promise<void> {
      try {
        await api.delete(uriToWorkspacePath(uri), { recursive: options.recursive })
        if (!disposed) {
          emitter.fire([{ type: vscode.FileChangeType.Deleted, uri }])
        }
      } catch (error) {
        throw mapFsError(vscode, error)
      }
    },

    async rename(oldUri: Uri, newUri: Uri, options: { overwrite: boolean }): Promise<void> {
      try {
        await api.rename(uriToWorkspacePath(oldUri), uriToWorkspacePath(newUri), {
          overwrite: options.overwrite,
        })
        if (!disposed) {
          emitter.fire([
            { type: vscode.FileChangeType.Deleted, uri: oldUri },
            { type: vscode.FileChangeType.Created, uri: newUri },
          ])
        }
      } catch (error) {
        throw mapFsError(vscode, error)
      }
    },

    dispose() {
      disposed = true
      emitter.dispose()
    },
  }

  return provider
}

function toVscodeFileType(vscode: VscodeApi, type: ProtoFileType): VscodeFileType {
  switch (type) {
    case ProtoFileType.FILE_TYPE_DIRECTORY:
      return vscode.FileType.Directory
    case ProtoFileType.FILE_TYPE_SYMBOLIC_LINK:
      return vscode.FileType.SymbolicLink
    case ProtoFileType.FILE_TYPE_FILE:
      return vscode.FileType.File
    default:
      return vscode.FileType.Unknown
  }
}

function toVscodeStat(
  vscode: VscodeApi,
  stat: import('../../gen/proto/termbridge/agent/v1/file').FileStat,
): FileStat {
  return {
    type: toVscodeFileType(vscode, stat.type),
    ctime: Number(stat.ctime) || 0,
    mtime: Number(stat.mtime) || 0,
    size: Number(stat.size) || 0,
    permissions: Number(stat.permissions) === 1 ? vscode.FilePermission.Readonly : undefined,
  }
}

function mapFsError(vscode: VscodeApi, error: unknown): Error {
  if (!(error instanceof Error)) {
    return vscode.FileSystemError.Unavailable(String(error))
  }
  const apiError = error as WorkbenchApiError
  const code = typeof apiError.code === 'string' ? apiError.code : ''
  const message = apiError.message || 'File system error'
  switch (code) {
    case 'not_found':
      return vscode.FileSystemError.FileNotFound(message)
    case 'conflict':
      return vscode.FileSystemError.FileExists(message)
    case 'unauthorized':
    case 'forbidden':
      return vscode.FileSystemError.NoPermissions(message)
    case 'device_offline':
    case 'service_unavailable':
      return vscode.FileSystemError.Unavailable(message)
    default:
      if (apiError.status === 404) {
        return vscode.FileSystemError.FileNotFound(message)
      }
      if (apiError.status === 409) {
        return vscode.FileSystemError.FileExists(message)
      }
      return vscode.FileSystemError.Unavailable(message)
  }
}
