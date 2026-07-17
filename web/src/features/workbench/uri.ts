import type { Uri } from 'vscode'

/** Custom scheme for TermBridge workspace files (FileSystemProvider). */
export const WORKBENCH_SCHEME = 'tb'

/** Virtual scheme for SCM original (left-side) content in vscode.diff. */
export const SCM_ORIGINAL_SCHEME = 'tb-scm'

export function workspaceRootUri(vscode: typeof import('vscode')): Uri {
  return vscode.Uri.from({ scheme: WORKBENCH_SCHEME, path: '/' })
}

export function uriToWorkspacePath(uri: Uri): string {
  const path = uri.path.replace(/^\/+/, '')
  return path
}

export function workspacePathToUri(vscode: typeof import('vscode'), path: string): Uri {
  const normalized = path.replace(/^\/+/, '')
  return vscode.Uri.from({
    scheme: WORKBENCH_SCHEME,
    path: normalized ? `/${normalized}` : '/',
  })
}

/**
 * Left-side URI for vscode.diff.
 * Shape: tb-scm://{groupId}/{workspace-relative-path}
 * Content is served by TextDocumentContentProvider via ScmOriginalContent.
 */
export function scmOriginalUri(
  vscode: typeof import('vscode'),
  path: string,
  groupId: string,
): Uri {
  const normalized = path.replace(/^\/+/, '')
  const authority = groupId || 'changes'
  return vscode.Uri.from({
    scheme: SCM_ORIGINAL_SCHEME,
    authority,
    path: normalized ? `/${normalized}` : '/',
  })
}

export function parseScmOriginalUri(uri: Uri): { path: string; groupId: string } {
  const path = uri.path.replace(/^\/+/, '')
  const groupId = uri.authority || 'changes'
  return { path, groupId }
}
