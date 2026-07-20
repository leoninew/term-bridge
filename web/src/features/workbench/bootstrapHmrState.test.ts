import { describe, expect, it, vi } from 'vitest'
import {
  isServicesAlreadyInitializedError,
  retainHmrState,
  shouldReloadWorkbenchPage,
  startWorkbenchInitialization,
  type HmrContext,
  type WorkbenchInitializationState,
} from './bootstrapHmrState'

function createHot(data: Record<string, unknown> = {}) {
  let disposeCallback: ((data: Record<string, unknown>) => void) | undefined
  const hot: HmrContext = {
    data,
    dispose(callback) {
      disposeCallback = callback
    },
  }
  return {
    hot,
    dispose(nextData: Record<string, unknown>) {
      disposeCallback?.(nextData)
    },
  }
}

function initializationState(): WorkbenchInitializationState {
  return {
    initPromise: null,
    monacoInitializationStarted: false,
    terminalInitializationError: null,
  }
}

describe('Workbench HMR state', () => {
  it('retains one bootstrap state object and its live resources across HMR replacement', () => {
    const first = createHot()
    const initPromise = Promise.resolve()
    const vscodeApiPromise = Promise.resolve({})
    const provider = {}
    const mounted = {}
    const created = vi.fn(() => ({ provider, initPromise, vscodeApiPromise, mounted }))
    const state = retainHmrState(first.hot, 'workbench', isBootstrapState, created)
    const replacementData: Record<string, unknown> = {}

    first.dispose(replacementData)
    const second = createHot(replacementData)
    const restored = retainHmrState(second.hot, 'workbench', isBootstrapState, created)

    expect(restored).toBe(state)
    expect(restored.provider).toBe(provider)
    expect(restored.initPromise).toBe(initPromise)
    expect(restored.vscodeApiPromise).toBe(vscodeApiPromise)
    expect(restored.mounted).toBe(mounted)
    expect(created).toHaveBeenCalledTimes(1)
  })

  it('allows retry when initialization fails before Monaco services start', async () => {
    const state = initializationState()
    const transientFailure = new Error('IndexedDB unavailable')
    const servicesInitialized = vi.fn(() => false)

    await expect(
      startWorkbenchInitialization(
        state,
        async () => {
          throw transientFailure
        },
        servicesInitialized,
      ),
    ).rejects.toBe(transientFailure)
    expect(state.initPromise).toBeNull()
    expect(state.terminalInitializationError).toBeNull()

    await expect(
      startWorkbenchInitialization(state, async () => undefined, servicesInitialized),
    ).resolves.toBeUndefined()
  })

  it('keeps a post-Monaco initialization failure terminal for this page', async () => {
    const state = initializationState()
    const terminalFailure = new Error('extension host failed')
    const initialize = vi.fn(async () => {
      state.monacoInitializationStarted = true
      throw terminalFailure
    })

    await expect(startWorkbenchInitialization(state, initialize)).rejects.toBe(terminalFailure)
    await expect(startWorkbenchInitialization(state, initialize)).rejects.toBe(terminalFailure)

    expect(initialize).toHaveBeenCalledTimes(1)
    expect(state.terminalInitializationError).toBe(terminalFailure)
  })

  it('treats library servicesInitialized as terminal even without monacoInitializationStarted', async () => {
    const state = initializationState()
    let libraryReady = false
    const servicesInitialized = () => libraryReady
    const initialize = vi.fn(async () => {
      libraryReady = true
      throw new Error('failed after StandaloneServices started')
    })

    await expect(
      startWorkbenchInitialization(state, initialize, servicesInitialized),
    ).rejects.toThrow('failed after StandaloneServices started')
    await expect(
      startWorkbenchInitialization(state, initialize, servicesInitialized),
    ).rejects.toThrow('failed after StandaloneServices started')
    expect(initialize).toHaveBeenCalledTimes(1)
    expect(state.monacoInitializationStarted).toBe(true)
    expect(state.terminalInitializationError).toBeInstanceOf(Error)
  })

  it('rejects immediately when bootstrap state was lost but services already initialized', async () => {
    const state = initializationState()
    const initialize = vi.fn(async () => undefined)

    await expect(startWorkbenchInitialization(state, initialize, () => true)).rejects.toThrow(
      /requires a full reload/,
    )
    expect(initialize).not.toHaveBeenCalled()
    expect(state.terminalInitializationError).toBeInstanceOf(Error)
    expect(state.monacoInitializationStarted).toBe(true)
  })

  it('detects page reload conditions for lost state or missing host', () => {
    expect(
      shouldReloadWorkbenchPage({
        initPromise: null,
        servicesInitialized: true,
        hasWorkbenchHost: false,
      }),
    ).toBe(true)
    expect(
      shouldReloadWorkbenchPage({
        initPromise: Promise.resolve(),
        servicesInitialized: true,
        hasWorkbenchHost: false,
      }),
    ).toBe(true)
    expect(
      shouldReloadWorkbenchPage({
        initPromise: Promise.resolve(),
        servicesInitialized: true,
        hasWorkbenchHost: true,
      }),
    ).toBe(false)
    expect(
      shouldReloadWorkbenchPage({
        initPromise: null,
        servicesInitialized: false,
        hasWorkbenchHost: false,
      }),
    ).toBe(false)
  })

  it('recognizes Services are already initialized errors', () => {
    expect(isServicesAlreadyInitializedError(new Error('Services are already initialized'))).toBe(
      true,
    )
    expect(isServicesAlreadyInitializedError(new Error('other'))).toBe(false)
    expect(isServicesAlreadyInitializedError('Services are already initialized')).toBe(false)
  })
})

type TestBootstrapState = {
  provider: object
  initPromise: Promise<void>
  vscodeApiPromise: Promise<object>
  mounted: object
}

function isBootstrapState(value: unknown): value is TestBootstrapState {
  return value !== null && typeof value === 'object' && 'provider' in value
}
