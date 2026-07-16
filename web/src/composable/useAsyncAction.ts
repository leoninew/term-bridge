import { proxyRefs, ref } from 'vue'
import { errorMessage } from '../store/notifications'

export type AsyncActionResult<T> =
  | { ok: true; value: T }
  | { ok: false; error: string; cause: unknown }

export type UseAsyncActionOptions = {
  onError?: (err: unknown, message: string) => void
}

export type RunAsyncActionOptions = {
  onError?: (err: unknown, message: string) => void
  rethrow?: boolean
}

export function useAsyncAction(options: UseAsyncActionOptions = {}) {
  const running = ref(false)
  const error = ref<string | null>(null)

  async function run<T>(
    task: () => Promise<T>,
    runOptions: RunAsyncActionOptions = {},
  ): Promise<AsyncActionResult<T>> {
    if (running.value) {
      return {
        ok: false,
        error: error.value ?? 'busy',
        cause: new Error('async action already running'),
      }
    }

    running.value = true
    error.value = null
    try {
      const value = await task()
      return { ok: true, value }
    } catch (err) {
      const message = errorMessage(err)
      error.value = message
      const handleError = runOptions.onError ?? options.onError
      handleError?.(err, message)
      if (runOptions.rethrow) {
        throw err
      }
      return { ok: false, error: message, cause: err }
    } finally {
      running.value = false
    }
  }

  function clearError() {
    error.value = null
  }

  return proxyRefs({
    running,
    error,
    run,
    clearError,
  })
}
