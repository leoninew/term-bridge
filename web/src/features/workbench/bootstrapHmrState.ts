export type HmrData = Record<string, unknown>

export type HmrContext = {
  data: HmrData
  dispose: (callback: (data: HmrData) => void) => void
  accept?: (callback: () => void) => void
}

export type WorkbenchInitializationState = {
  initPromise: Promise<void> | null
  monacoInitializationStarted: boolean
  terminalInitializationError: unknown | null
}

export type ServicesInitializedProbe = () => boolean

const LOST_STATE_RELOAD_MESSAGE =
  'Services are already initialized; workbench bootstrap state was lost and requires a full reload'

export function retainHmrState<T>(
  hot: HmrContext | undefined,
  key: string,
  isState: (value: unknown) => value is T,
  create: () => T,
): T {
  const retained = hot?.data[key]
  const state = isState(retained) ? retained : create()
  hot?.dispose((data) => {
    data[key] = state
  })
  return state
}

/**
 * Decide whether the page must hard-reload before mounting workbench again.
 * Monaco services are process-global and cannot be re-initialized.
 */
export function shouldReloadWorkbenchPage(input: {
  initPromise: Promise<void> | null
  servicesInitialized: boolean
  hasWorkbenchHost: boolean
}): boolean {
  if (input.servicesInitialized && input.initPromise === null) {
    return true
  }
  if (input.initPromise !== null && !input.hasWorkbenchHost) {
    return true
  }
  return false
}

export function isServicesAlreadyInitializedError(error: unknown): boolean {
  return error instanceof Error && /Services are already initialized/i.test(error.message)
}

export function startWorkbenchInitialization(
  state: WorkbenchInitializationState,
  initialize: () => Promise<void>,
  servicesInitialized: ServicesInitializedProbe = () => false,
): Promise<void> {
  if (state.terminalInitializationError) {
    return Promise.reject(state.terminalInitializationError)
  }
  if (state.initPromise) {
    return state.initPromise
  }

  // Library already initialized but our HMR-retained state was lost.
  // Never call initialize()/registerCustomProvider again on this page.
  if (servicesInitialized()) {
    const error = new Error(LOST_STATE_RELOAD_MESSAGE)
    state.monacoInitializationStarted = true
    state.terminalInitializationError = error
    return Promise.reject(error)
  }

  const promise = Promise.resolve().then(initialize)
  state.initPromise = promise
  void promise.catch((error: unknown) => {
    // Align terminal boundary with the library: once services are up (or we
    // crossed monacoInitializationStarted), retries are illegal.
    if (state.monacoInitializationStarted || servicesInitialized()) {
      state.monacoInitializationStarted = true
      state.terminalInitializationError = error
      return
    }
    if (state.initPromise === promise) {
      state.initPromise = null
    }
  })
  return promise
}
