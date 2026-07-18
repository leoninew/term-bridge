/**
 * Split workbench VS Code default extensions into:
 * - core: required before monaco initialize (themes, icons, high-frequency languages)
 * - deferred: load after first paint so the main bootstrap chunk stays smaller
 */
let coreLoaded = false
let deferredPromise: Promise<void> | null = null

export async function loadCoreWorkbenchExtensions(): Promise<void> {
  if (coreLoaded) {
    return
  }
  await Promise.all([
    import('@codingame/monaco-vscode-theme-defaults-default-extension'),
    import('@codingame/monaco-vscode-theme-seti-default-extension'),
    import('@codingame/monaco-vscode-json-default-extension'),
    import('@codingame/monaco-vscode-javascript-default-extension'),
    import('@codingame/monaco-vscode-typescript-basics-default-extension'),
    import('@codingame/monaco-vscode-markdown-basics-default-extension'),
    import('vscode/localExtensionHost'),
  ])
  coreLoaded = true
}

/** Language / media extensions that are useful but not required for first paint. */
export function scheduleDeferredWorkbenchExtensions(): Promise<void> {
  if (deferredPromise) {
    return deferredPromise
  }
  deferredPromise = Promise.all([
    import('@codingame/monaco-vscode-python-default-extension'),
    import('@codingame/monaco-vscode-go-default-extension'),
    import('@codingame/monaco-vscode-yaml-default-extension'),
    import('@codingame/monaco-vscode-shellscript-default-extension'),
    import('@codingame/monaco-vscode-html-default-extension'),
    import('@codingame/monaco-vscode-css-default-extension'),
    import('@codingame/monaco-vscode-media-preview-default-extension'),
  ]).then(() => undefined)
  void deferredPromise.catch((error: unknown) => {
    console.warn('[termbridge-workbench] deferred language extensions failed', error)
  })
  return deferredPromise
}
