import {
  getService,
  initialize as initializeMonacoService,
  IViewsService,
  type IWorkbenchConstructionOptions,
} from '@codingame/monaco-vscode-api'
import { ExtensionHostKind, registerExtension } from '@codingame/monaco-vscode-api/extensions'
import getConfigurationServiceOverride, {
  initUserConfiguration,
} from '@codingame/monaco-vscode-configuration-service-override'
import getDialogsServiceOverride from '@codingame/monaco-vscode-dialogs-service-override'
import getEnvironmentServiceOverride from '@codingame/monaco-vscode-environment-service-override'
import getExplorerServiceOverride from '@codingame/monaco-vscode-explorer-service-override'
import getExtensionServiceOverride from '@codingame/monaco-vscode-extensions-service-override'
import {
  createIndexedDBProviders,
  registerCustomProvider,
} from '@codingame/monaco-vscode-files-service-override'
import getKeybindingsServiceOverride, {
  initUserKeybindings,
} from '@codingame/monaco-vscode-keybindings-service-override'
import getLanguagesServiceOverride from '@codingame/monaco-vscode-languages-service-override'
import getLifecycleServiceOverride from '@codingame/monaco-vscode-lifecycle-service-override'
import getLogServiceOverride from '@codingame/monaco-vscode-log-service-override'
import getModelServiceOverride from '@codingame/monaco-vscode-model-service-override'
import getNotificationServiceOverride from '@codingame/monaco-vscode-notifications-service-override'
import getQuickAccessServiceOverride from '@codingame/monaco-vscode-quickaccess-service-override'
import getScmServiceOverride from '@codingame/monaco-vscode-scm-service-override'
import getStorageServiceOverride from '@codingame/monaco-vscode-storage-service-override'
import getTextmateServiceOverride from '@codingame/monaco-vscode-textmate-service-override'
import getThemeServiceOverride from '@codingame/monaco-vscode-theme-service-override'
import getBannerServiceOverride from '@codingame/monaco-vscode-view-banner-service-override'
import getStatusBarServiceOverride from '@codingame/monaco-vscode-view-status-bar-service-override'
import getTitleBarServiceOverride from '@codingame/monaco-vscode-view-title-bar-service-override'
import getWorkbenchServiceOverride from '@codingame/monaco-vscode-workbench-service-override'
import getWorkingCopyServiceOverride from '@codingame/monaco-vscode-working-copy-service-override'
import getImageResizeServiceOverride from '@codingame/monaco-vscode-image-resize-service-override'
import getWorkspaceTrustOverride from '@codingame/monaco-vscode-workspace-trust-service-override'
import * as monaco from 'monaco-editor'
import type { RuntimeTarget } from '../runtimeTarget'
import { createWorkbenchRuntimeApi } from './api'
import { TermBridgePlatformFileSystemProvider } from './platformFileSystemProvider'
import { registerTermBridgeScm, type ScmController } from './scmProvider'
import { WORKBENCH_SCHEME, workspaceRootUri } from './uri'

// Themes + file icons (demo: theme-defaults + theme-seti)
import '@codingame/monaco-vscode-theme-defaults-default-extension'
import '@codingame/monaco-vscode-theme-seti-default-extension'
import '@codingame/monaco-vscode-media-preview-default-extension'
// Grammar basics for common workspace languages (subset of demo language extensions)
import '@codingame/monaco-vscode-javascript-default-extension'
import '@codingame/monaco-vscode-typescript-basics-default-extension'
import '@codingame/monaco-vscode-json-default-extension'
import '@codingame/monaco-vscode-markdown-basics-default-extension'
import '@codingame/monaco-vscode-python-default-extension'
import '@codingame/monaco-vscode-go-default-extension'
import '@codingame/monaco-vscode-yaml-default-extension'
import '@codingame/monaco-vscode-shellscript-default-extension'
import '@codingame/monaco-vscode-html-default-extension'
import '@codingame/monaco-vscode-css-default-extension'
import 'vscode/localExtensionHost'

export type CodeWorkbenchContext = {
  runtimeTarget: RuntimeTarget
  workspaceId: string
}

type MountedWorkbench = {
  workspaceKey: string
  disposables: { dispose: () => void }[]
  scm?: ScmController
}

type VscodeApi = typeof import('vscode')

/** Demo-aligned user settings (subset; no debug/typescript.tsserver noise). */
const USER_CONFIGURATION = {
  'workbench.colorTheme': 'Default Dark+',
  'workbench.iconTheme': 'vs-seti',
  'workbench.sideBar.location': 'left',
  'workbench.startupEditor': 'none',
  'editor.fontSize': 12,
  'editor.autoClosingBrackets': 'languageDefined',
  'editor.autoClosingQuotes': 'languageDefined',
  'editor.scrollBeyondLastLine': true,
  'editor.mouseWheelZoom': true,
  'editor.wordBasedSuggestions': 'off',
  'editor.acceptSuggestionOnEnter': 'on',
  'editor.foldingHighlight': false,
  'editor.semanticHighlighting.enabled': true,
  'editor.bracketPairColorization.enabled': false,
  'files.autoSave': 'off',
  'explorer.autoReveal': true,
  'files.exclude': {
    '**/.git': true,
  },
  'window.title': 'TermBridge${separator}${dirty}${activeEditorShort}',
}

const platformFsProvider = new TermBridgePlatformFileSystemProvider()
let customProviderRegistered = false
let initPromise: Promise<void> | null = null
let vscodeApiPromise: Promise<VscodeApi> | null = null
let mounted: MountedWorkbench | null = null

function workspaceKey(ctx: CodeWorkbenchContext): string {
  if (ctx.runtimeTarget.mode === 'cloud') {
    return `cloud:${ctx.runtimeTarget.deviceId}:${ctx.workspaceId}`
  }
  return `local:${ctx.workspaceId}`
}

/**
 * Workers — same labels as demo setup.common.ts (editor / extensionHost / TextMate).
 * Demo stores url+options descriptors; runtime creates workers via getWorkerUrl/getWorkerOptions.
 */
function configureWorkers(): void {
  type WorkerDescriptor = { url: string | URL; options?: WorkerOptions }
  const moduleWorker = (url: string | URL): WorkerDescriptor => ({
    url,
    options: { type: 'module' },
  })
  const editorWorker = moduleWorker(
    new URL('monaco-editor/esm/vs/editor/editor.worker.js', import.meta.url),
  )
  const workers: Partial<Record<string, WorkerDescriptor>> = {
    editorWorkerService: editorWorker,
    TextEditorWorker: editorWorker,
    extensionHostWorkerMain: moduleWorker(
      new URL('@codingame/monaco-vscode-api/workers/extensionHost.worker', import.meta.url),
    ),
    TextMateWorker: moduleWorker(
      new URL('@codingame/monaco-vscode-textmate-service-override/worker', import.meta.url),
    ),
  }
  window.MonacoEnvironment = {
    getWorkerUrl(_moduleId: string, label: string) {
      return workers[label]?.url.toString()
    },
    getWorkerOptions(_moduleId: string, label: string) {
      return workers[label]?.options
    },
  }
}

/**
 * Create the workbench root like demo: Shadow DOM so app Tailwind/preflight
 * cannot reshape tree twisties, icons, or list row metrics.
 */
export function createWorkbenchMountRoot(container: HTMLElement): HTMLElement {
  container.replaceChildren()
  container.style.position = container.style.position || 'relative'
  container.style.width = '100%'
  container.style.height = '100%'
  container.style.minHeight = '0'
  container.style.overflow = 'hidden'

  const outer = document.createElement('div')
  outer.className = 'termbridge-code-workbench-host'
  outer.style.width = '100%'
  outer.style.height = '100%'
  outer.style.minHeight = '0'
  container.appendChild(outer)

  const shadowRoot = outer.attachShadow({ mode: 'open' })
  const workbenchElement = document.createElement('div')
  workbenchElement.className = 'termbridge-code-workbench-root'
  workbenchElement.style.width = '100%'
  workbenchElement.style.height = '100%'
  workbenchElement.style.minHeight = '0'
  workbenchElement.style.position = 'relative'
  workbenchElement.style.overflow = 'hidden'
  shadowRoot.appendChild(workbenchElement)
  return workbenchElement
}

async function waitForWorkspaceFolderUri(vscode: VscodeApi, timeoutMs = 5000): Promise<import('vscode').Uri> {
  const existing = vscode.workspace.workspaceFolders?.[0]?.uri
  if (existing) {
    return existing
  }
  return await new Promise((resolve) => {
    const fallback = workspaceRootUri(vscode)
    const timer = setTimeout(() => {
      disposable.dispose()
      resolve(fallback)
    }, timeoutMs)
    const disposable = vscode.workspace.onDidChangeWorkspaceFolders(() => {
      const folder = vscode.workspace.workspaceFolders?.[0]?.uri
      if (folder) {
        clearTimeout(timer)
        disposable.dispose()
        resolve(folder)
      }
    })
  })
}

async function initializeWorkbench(container: HTMLElement): Promise<void> {
  configureWorkers()
  await createIndexedDBProviders()

  if (!customProviderRegistered) {
    registerCustomProvider(WORKBENCH_SCHEME, platformFsProvider)
    customProviderRegistered = true
  }

  await Promise.all([
    initUserConfiguration(JSON.stringify(USER_CONFIGURATION, null, 2)),
    initUserKeybindings('[]'),
  ])

  const folderUri = monaco.Uri.from({ scheme: WORKBENCH_SCHEME, path: '/' })

  const constructOptions: IWorkbenchConstructionOptions = {
    enableWorkspaceTrust: true,
    windowIndicator: {
      label: 'TermBridge',
      tooltip: 'TermBridge Code Workbench',
      command: '',
    },
    workspaceProvider: {
      trusted: true,
      async open() {
        return false
      },
      workspace: {
        folderUri,
      },
    },
    configurationDefaults: {
      ...USER_CONFIGURATION,
    },
    defaultLayout: {
      views: [
        {
          id: 'workbench.view.explorer',
        },
      ],
      force: true,
    },
    productConfiguration: {
      nameShort: 'TermBridge',
      nameLong: 'TermBridge Code',
      applicationName: 'termbridge',
      dataFolderName: '.termbridge-code',
      version: '0.1.0',
    },
  }

  await initializeMonacoService(
    {
      ...getLogServiceOverride(),
      ...getExtensionServiceOverride({
        enableWorkerExtensionHost: true,
      }),
      ...getModelServiceOverride(),
      ...getNotificationServiceOverride(),
      ...getDialogsServiceOverride(),
      ...getConfigurationServiceOverride(),
      ...getKeybindingsServiceOverride(),
      ...getTextmateServiceOverride(),
      ...getLanguagesServiceOverride(),
      ...getThemeServiceOverride(),
      ...getBannerServiceOverride(),
      ...getStatusBarServiceOverride(),
      ...getTitleBarServiceOverride(),
      ...getStorageServiceOverride({
        fallbackOverride: {
          'workbench.activity.showAccounts': false,
        },
      }),
      ...getLifecycleServiceOverride(),
      ...getEnvironmentServiceOverride(),
      ...getWorkspaceTrustOverride(),
      ...getWorkingCopyServiceOverride(),
      ...getScmServiceOverride(),
      ...getExplorerServiceOverride(),
      ...getImageResizeServiceOverride(),
      ...getWorkbenchServiceOverride(),
      ...getQuickAccessServiceOverride({
        isKeybindingConfigurationVisible: () => false,
        shouldUseGlobalPicker: () => true,
      }),
    },
    container,
    constructOptions,
    {
      userHome: monaco.Uri.file('/'),
    },
  )

  // Single system extension (demo pattern for scmActionButton + createSourceControl).
  // Avoid a second LocalProcess extension that can leave SCM unregistered.
  const { getApi, setAsDefaultApi } = registerExtension(
    termBridgeWorkbenchExtensionManifest,
    ExtensionHostKind.LocalProcess,
    { system: true },
  )
  vscodeApiPromise = getApi()
  await setAsDefaultApi()
  await vscodeApiPromise
}

/**
 * Mount the VS Code workbench into `container` for one workspace.
 * initialize() runs once per page lifetime; workspace switch forces reload.
 */
export async function mountCodeWorkbench(
  container: HTMLElement,
  ctx: CodeWorkbenchContext,
): Promise<void> {
  const key = workspaceKey(ctx)

  if (mounted && mounted.workspaceKey !== key) {
    window.location.reload()
    return
  }

  // Bind remote API BEFORE initialize so startup readdir/stat hit TermBridge FS.
  const api = createWorkbenchRuntimeApi(ctx.runtimeTarget, ctx.workspaceId)
  platformFsProvider.bind(api)

  if (!initPromise) {
    const root = createWorkbenchMountRoot(container)
    initPromise = initializeWorkbench(root)
  } else if (!container.querySelector('.termbridge-code-workbench-host')) {
    window.location.reload()
    return
  }

  await initPromise

  if (mounted && mounted.workspaceKey === key) {
    // Re-bind API + refresh SCM when remounting same workspace key.
    if (mounted.scm) {
      platformFsProvider.bind(api, () => {
        void mounted?.scm?.refresh()
      })
      void mounted.scm.refresh()
    }
    return
  }

  if (!vscodeApiPromise) {
    throw new Error('Workbench VS Code API is not initialized.')
  }
  const vscode = await vscodeApiPromise
  const rootUri = await waitForWorkspaceFolderUri(vscode)

  const disposables: { dispose: () => void }[] = []
  const scm = await registerTermBridgeScm(vscode, api, rootUri)
  disposables.push(scm)

  platformFsProvider.bind(api, () => {
    void scm.refresh()
  })

  // Refresh when user opens Source Control (activity bar / view switch).
  // Without this, status is only fetched once at register time.
  try {
    const viewsService = await getService(IViewsService)
    disposables.push(
      viewsService.onDidChangeViewContainerVisibility((e) => {
        if (e.visible && e.id === 'workbench.view.scm') {
          void scm.refresh()
        }
      }),
      viewsService.onDidChangeViewVisibility((e) => {
        if (e.visible && (e.id === 'workbench.scm' || e.id === 'workbench.view.scm')) {
          void scm.refresh()
        }
      }),
    )
  } catch (error) {
    console.warn('[termbridge-workbench] SCM view visibility hook unavailable', error)
  }

  // Window focus can mean returning after external git changes.
  disposables.push(
    vscode.window.onDidChangeWindowState((state) => {
      if (state.focused) {
        void scm.refresh()
      }
    }),
  )

  try {
    await vscode.commands.executeCommand('workbench.view.explorer')
  } catch {
    // optional
  }

  window.dispatchEvent(new Event('resize'))

  mounted = {
    workspaceKey: key,
    disposables,
    scm,
  }
}

export function disposeMountedWorkbench(): void {
  if (!mounted) {
    return
  }
  for (const disposable of mounted.disposables.splice(0)) {
    disposable.dispose()
  }
  mounted = null
}
import { termBridgeWorkbenchExtensionManifest } from './scmExtensionManifest'
