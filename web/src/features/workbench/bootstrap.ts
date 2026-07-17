import {
  getService,
  initialize as initializeMonacoService,
  IViewsService,
  type IWorkbenchConstructionOptions,
} from '@codingame/monaco-vscode-api'
import { servicesInitialized } from '@codingame/monaco-vscode-api/lifecycle'
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
import {
  createWorkbenchRuntimeApi,
  subscribeWorkspaceChanges,
  type WorkbenchWorkspaceChangeSubscription,
} from './api'
import {
  isServicesAlreadyInitializedError,
  retainHmrState,
  shouldReloadWorkbenchPage,
  startWorkbenchInitialization,
  type HmrContext,
  type WorkbenchInitializationState,
} from './bootstrapHmrState'
import { TermBridgePlatformFileSystemProvider } from './platformFileSystemProvider'
import { termBridgeWorkbenchExtensionManifest } from './scmExtensionManifest'
import { registerTermBridgeScm, type ScmController } from './scmProvider'
import { WORKBENCH_SCHEME, workspaceRootUri } from './uri'
import { ensureWorkbenchAppIconStyles } from './titlebarAppIcon'

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

import EditorWorker from '@codingame/monaco-vscode-api/workers/editor.worker?worker'
import ExtensionHostWorker from '@codingame/monaco-vscode-api/workers/extensionHost.worker?worker'
import TextMateWorker from '@codingame/monaco-vscode-textmate-service-override/worker?worker'
import editorWorkerUrl from '@codingame/monaco-vscode-api/workers/editor.worker?worker&url'
import extensionHostWorkerUrl from '@codingame/monaco-vscode-api/workers/extensionHost.worker?worker&url'
import textMateWorkerUrl from '@codingame/monaco-vscode-textmate-service-override/worker?worker&url'

export type CodeWorkbenchContext = {
  runtimeTarget: RuntimeTarget
  workspaceId: string
}

type Disposable = { dispose: () => void }

type MountedWorkbench = {
  workspaceKey: string
  disposables: Disposable[]
  scm?: ScmController
  workspaceChanges?: WorkbenchWorkspaceChangeSubscription
}

type WorkbenchMountAttempt = {
  key: string
  generation: number
  promise: Promise<void> | null
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
  // 'window.title': 'TermBridge${separator}${dirty}${activeEditorShort}',
  'window.title': '${activeEditorShort}',
}

type WorkbenchBootstrapState = WorkbenchInitializationState & {
  stateVersion: number
  platformFsProvider: TermBridgePlatformFileSystemProvider
  customProviderRegistered: boolean
  vscodeApiPromise: Promise<VscodeApi> | null
  mounted: MountedWorkbench | null
  mountAttempt: WorkbenchMountAttempt | null
  mountGeneration: number
}

const HMR_STATE_KEY = 'termbridgeWorkbenchBootstrapState'
const WORKBENCH_BOOTSTRAP_STATE_VERSION = 1
const hot = import.meta.hot as HmrContext | undefined
const state = retainHmrState(
  hot,
  HMR_STATE_KEY,
  isWorkbenchBootstrapState,
  (): WorkbenchBootstrapState => ({
    stateVersion: WORKBENCH_BOOTSTRAP_STATE_VERSION,
    platformFsProvider: new TermBridgePlatformFileSystemProvider(),
    customProviderRegistered: false,
    initPromise: null,
    monacoInitializationStarted: false,
    terminalInitializationError: null,
    vscodeApiPromise: null,
    mounted: null,
    mountAttempt: null,
    mountGeneration: 0,
  }),
)

// Workbench Monaco services are page-global. Prefer a full reload over partial HMR
// re-entry that can throw "Services are already initialized".
hot?.accept?.(() => {
  window.location.reload()
})

function isWorkbenchBootstrapState(value: unknown): value is WorkbenchBootstrapState {
  if (value === null || typeof value !== 'object') {
    return false
  }
  const candidate = value as Partial<WorkbenchBootstrapState>
  return (
    candidate.stateVersion === WORKBENCH_BOOTSTRAP_STATE_VERSION &&
    candidate.platformFsProvider instanceof TermBridgePlatformFileSystemProvider &&
    'initPromise' in candidate &&
    typeof candidate.monacoInitializationStarted === 'boolean'
  )
}

function disposeAll(disposables: Disposable[]): void {
  for (const disposable of disposables.splice(0)) {
    disposable.dispose()
  }
}

function isCurrentMountAttempt(attempt: WorkbenchMountAttempt): boolean {
  return state.mountAttempt === attempt && state.mountGeneration === attempt.generation
}

function disposeMountResources(
  disposables: Disposable[],
  workspaceChanges?: WorkbenchWorkspaceChangeSubscription,
): void {
  workspaceChanges?.close()
  disposeAll(disposables)
}

function workspaceKey(ctx: CodeWorkbenchContext): string {
  if (ctx.runtimeTarget.mode === 'cloud') {
    return `cloud:${ctx.runtimeTarget.deviceId}:${ctx.workspaceId}`
  }
  return `local:${ctx.workspaceId}`
}

/**
 * Workers must be Vite `?worker` entries so production emits real bundled chunks.
 * `new URL(pkg, import.meta.url)` collapses to data: URLs of bare source (breaks blob import).
 * Prefer getWorker (Worker ctor). getWorkerUrl is still required by extension-host iframe bootstrap.
 */
function configureWorkers(): void {
  const workerUrls: Partial<Record<string, string>> = {
    editorWorkerService: editorWorkerUrl,
    TextEditorWorker: editorWorkerUrl,
    extensionHostWorkerMain: extensionHostWorkerUrl,
    TextMateWorker: textMateWorkerUrl,
  }

  window.MonacoEnvironment = {
    getWorker(_moduleId: string, label: string): Worker | undefined {
      switch (label) {
        case 'TextMateWorker':
          return new TextMateWorker()
        case 'extensionHostWorkerMain':
          return new ExtensionHostWorker()
        case 'editorWorkerService':
        case 'TextEditorWorker':
          return new EditorWorker()
        default:
          return undefined
      }
    },
    getWorkerUrl(_moduleId: string, label: string): string | undefined {
      return workerUrls[label]
    },
    getWorkerOptions(_moduleId: string, label: string): WorkerOptions | undefined {
      if (workerUrls[label]) {
        return { type: 'module' }
      }
      return undefined
    },
  }
}

/**
 * Create the workbench root like demo: Shadow DOM so app Tailwind/preflight
 * cannot reshape tree twisties, icons, or list row metrics.
 */

/**
 * True browser external resume: document became visible, or window regained OS focus.
 * Internal Workbench focus moves do not fire window focus/blur.
 */
function bindExternalResume(onResume: () => void): { dispose: () => void } {
  let visible = typeof document !== 'undefined' ? document.visibilityState === 'visible' : true
  let windowFocused = typeof document !== 'undefined' ? document.hasFocus() : true

  const maybeResume = () => {
    const nextVisible = document.visibilityState === 'visible'
    const nextFocused = document.hasFocus()
    const becameVisible = !visible && nextVisible
    const regainedWindowFocus = !windowFocused && nextFocused
    visible = nextVisible
    windowFocused = nextFocused
    if (becameVisible || regainedWindowFocus) {
      onResume()
    }
  }

  const onVisibility = () => maybeResume()
  const onFocus = () => maybeResume()
  const onBlur = () => {
    windowFocused = document.hasFocus()
  }

  document.addEventListener('visibilitychange', onVisibility)
  window.addEventListener('focus', onFocus)
  window.addEventListener('blur', onBlur)

  return {
    dispose() {
      document.removeEventListener('visibilitychange', onVisibility)
      window.removeEventListener('focus', onFocus)
      window.removeEventListener('blur', onBlur)
    },
  }
}

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
  ensureWorkbenchAppIconStyles(shadowRoot)
  return workbenchElement
}

async function waitForWorkspaceFolderUri(
  vscode: VscodeApi,
  timeoutMs = 5000,
): Promise<import('vscode').Uri> {
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

  if (!state.customProviderRegistered) {
    registerCustomProvider(WORKBENCH_SCHEME, state.platformFsProvider)
    state.customProviderRegistered = true
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

  // Crossing initialize() is irreversible for this page lifetime.
  state.monacoInitializationStarted = true
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

  // Single system extension registers the API surface and native SCM menus.
  // Avoid a second LocalProcess extension that can leave SCM unregistered.
  const { getApi, setAsDefaultApi } = registerExtension(
    termBridgeWorkbenchExtensionManifest,
    ExtensionHostKind.LocalProcess,
    { system: true },
  )
  state.vscodeApiPromise = getApi()
  await setAsDefaultApi()
  await state.vscodeApiPromise
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
  if (state.mounted && state.mounted.workspaceKey !== key) {
    window.location.reload()
    return
  }
  if (state.mountAttempt) {
    if (state.mountAttempt.key !== key) {
      window.location.reload()
      return
    }
    return state.mountAttempt.promise ?? Promise.resolve()
  }

  const attempt: WorkbenchMountAttempt = {
    key,
    generation: state.mountGeneration,
    promise: null,
  }
  state.mountAttempt = attempt
  attempt.promise = mountWorkbenchAttempt(container, ctx, attempt).finally(() => {
    if (state.mountAttempt === attempt) {
      state.mountAttempt = null
    }
  })
  return attempt.promise
}

async function mountWorkbenchAttempt(
  container: HTMLElement,
  ctx: CodeWorkbenchContext,
  attempt: WorkbenchMountAttempt,
): Promise<void> {
  const api = createWorkbenchRuntimeApi(ctx.runtimeTarget, ctx.workspaceId)
  const disposables: Disposable[] = []
  let workspaceChanges: WorkbenchWorkspaceChangeSubscription | undefined
  let scm: ScmController | undefined
  try {
    // Bind before initialize so startup readdir/stat requests target this workspace.
    state.platformFsProvider.bind(api)
    const hasWorkbenchHost = Boolean(container.querySelector('.termbridge-code-workbench-host'))
    if (
      shouldReloadWorkbenchPage({
        initPromise: state.initPromise,
        servicesInitialized,
        hasWorkbenchHost,
      })
    ) {
      window.location.reload()
      return
    }
    if (!state.initPromise) {
      const root = createWorkbenchMountRoot(container)
      try {
        await startWorkbenchInitialization(
          state,
          async () => {
            await initializeWorkbench(root)
          },
          () => servicesInitialized,
        )
      } catch (error) {
        if (isServicesAlreadyInitializedError(error) || servicesInitialized) {
          window.location.reload()
          return
        }
        throw error
      }
    } else {
      await state.initPromise
    }
    if (!isCurrentMountAttempt(attempt)) {
      return
    }

    if (state.mounted && state.mounted.workspaceKey === attempt.key) {
      if (state.mounted.scm) {
        state.platformFsProvider.bind(api, () => {
          void state.mounted?.scm?.refresh()
        })
        void state.mounted.scm.refresh()
      }
      return
    }

    if (!state.vscodeApiPromise) {
      throw new Error('Workbench VS Code API is not initialized.')
    }
    const vscode = await state.vscodeApiPromise
    const rootUri = await waitForWorkspaceFolderUri(vscode)
    if (!isCurrentMountAttempt(attempt)) {
      return
    }

    scm = await registerTermBridgeScm(vscode, api, rootUri)
    disposables.push(scm)
    if (!isCurrentMountAttempt(attempt)) {
      return
    }

    state.platformFsProvider.bind(api, () => {
      void scm?.refresh()
    })

    // Refresh when user opens Source Control (activity bar / view switch).
    // Without this, status is only fetched once at register time.
    try {
      const viewsService = await getService(IViewsService)
      disposables.push(
        viewsService.onDidChangeViewContainerVisibility((e) => {
          if (e.visible && e.id === 'workbench.view.scm') {
            void scm?.refresh()
          }
        }),
        viewsService.onDidChangeViewVisibility((e) => {
          if (e.visible && (e.id === 'workbench.scm' || e.id === 'workbench.view.scm')) {
            void scm?.refresh()
          }
        }),
      )
    } catch (error) {
      console.warn('[termbridge-workbench] SCM view visibility hook unavailable', error)
    }
    if (!isCurrentMountAttempt(attempt)) {
      return
    }

    workspaceChanges = subscribeWorkspaceChanges(ctx.runtimeTarget, ctx.workspaceId, {
      onChange(change) {
        state.platformFsProvider.applyWorkspaceChange(change)
        void scm?.refresh()
      },
      onRescanRequired() {
        state.platformFsProvider.rescanRequired()
        void scm?.refresh()
      },
      onDisconnected() {
        // The subscription reconnects itself. This callback is observability for
        // consumers; the conservative FileService refresh already happened.
      },
    })
    if (!isCurrentMountAttempt(attempt)) {
      return
    }

    // External resume only (tab/window back). Do NOT use vscode.window.onDidChangeWindowState:
    // Workbench internal focus moves (Explorer click, extension host iframe) can report focused:true
    // and would spam /scm/status + /scm/repository, and also couple with upstream Explorer host-focus refresh.
    disposables.push(
      bindExternalResume(() => {
        state.platformFsProvider.rescanRequired()
        void scm?.refresh()
      }),
    )

    try {
      await vscode.commands.executeCommand('workbench.view.explorer')
    } catch {
      // optional
    }
    if (!isCurrentMountAttempt(attempt)) {
      return
    }

    window.dispatchEvent(new window.Event('resize'))
    state.mounted = {
      workspaceKey: attempt.key,
      disposables,
      scm,
      workspaceChanges,
    }
    workspaceChanges = undefined
    scm = undefined
  } finally {
    if (!isCurrentMountAttempt(attempt)) {
      disposeMountResources(disposables, workspaceChanges)
    }
  }
}

export function disposeMountedWorkbench(): void {
  state.mountGeneration += 1
  state.mountAttempt = null
  if (!state.mounted) {
    return
  }
  disposeMountResources(state.mounted.disposables, state.mounted.workspaceChanges)
  state.mounted = null
}
