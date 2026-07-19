import { ref, type Ref } from 'vue'
import { terminalDebug } from '../../components/terminal/diagnostics'
import {
  clampTerminalKeepAliveConfig,
  type TerminalKeepAliveConfig,
  type TerminalKeepAliveConfigInput,
} from './terminalKeepAliveConfig'

export type KeepAliveActivation = {
  sessionId: string
  lastActivatedAt: number
}

/** Pick hot set: active first (if candidate), then most-recent activation. */
export function computeHotSessionIds(
  candidates: KeepAliveActivation[],
  maxHotTerminals: number,
  activeSessionId: string | null,
): string[] {
  if (maxHotTerminals < 1 || candidates.length === 0) {
    return []
  }

  const byId = new Map(candidates.map((entry) => [entry.sessionId, entry.lastActivatedAt]))
  const hot: string[] = []
  const seen = new Set<string>()

  if (activeSessionId && byId.has(activeSessionId)) {
    hot.push(activeSessionId)
    seen.add(activeSessionId)
  }

  const ranked = [...candidates].sort((left, right) => right.lastActivatedAt - left.lastActivatedAt)
  for (const entry of ranked) {
    if (seen.has(entry.sessionId)) {
      continue
    }
    if (hot.length >= maxHotTerminals) {
      break
    }
    hot.push(entry.sessionId)
    seen.add(entry.sessionId)
  }

  return hot
}

export type TerminalKeepAliveController = {
  limits: Ref<TerminalKeepAliveConfig>
  mountedSessionIds: Ref<string[]>
  hotSessionIds: Ref<string[]>
  isMounted: (sessionId: string) => boolean
  isHot: (sessionId: string) => boolean
  touch: (sessionId: string, at?: number) => void
  reconcile: (openedRunningSessionIds: string[], activeSessionId: string | null) => void
  remove: (sessionId: string) => void
  disposeAll: () => void
}

export function createTerminalKeepAliveController(options?: {
  config?: TerminalKeepAliveConfigInput | null
  now?: () => number
  schedule?: (fn: () => void, ms: number) => number
  cancel?: (id: number) => void
}): TerminalKeepAliveController {
  const now = options?.now ?? (() => Date.now())
  const schedule = options?.schedule ?? ((fn, ms) => window.setTimeout(fn, ms))
  const cancel = options?.cancel ?? ((id) => window.clearTimeout(id))

  const limits = ref(clampTerminalKeepAliveConfig(options?.config))
  const lastActivatedAt = new Map<string, number>()
  const disposeTimers = new Map<string, number>()
  const mountedSessionIds = ref<string[]>([])
  const hotSessionIds = ref<string[]>([])

  function isMounted(sessionId: string): boolean {
    return mountedSessionIds.value.includes(sessionId)
  }

  function isHot(sessionId: string): boolean {
    return hotSessionIds.value.includes(sessionId)
  }

  function setMounted(ids: string[]) {
    mountedSessionIds.value = ids
  }

  function setHot(ids: string[]) {
    hotSessionIds.value = ids
  }

  function clearDisposeTimer(sessionId: string) {
    const timer = disposeTimers.get(sessionId)
    if (timer !== undefined) {
      cancel(timer)
      disposeTimers.delete(sessionId)
    }
  }

  function forceUnmount(sessionId: string, reason: string) {
    clearDisposeTimer(sessionId)
    lastActivatedAt.delete(sessionId)
    const wasMounted = isMounted(sessionId)
    const wasHot = isHot(sessionId)
    if (!wasMounted && !wasHot) {
      return
    }
    setHot(hotSessionIds.value.filter((id) => id !== sessionId))
    setMounted(mountedSessionIds.value.filter((id) => id !== sessionId))
    terminalDebug('keepalive.unmount', { sessionId, reason })
  }

  function scheduleDispose(sessionId: string) {
    if (disposeTimers.has(sessionId)) {
      return
    }
    const delay = limits.value.disposeDelayMs
    terminalDebug('keepalive.schedule-dispose', { sessionId, delayMs: delay })
    if (delay <= 0) {
      forceUnmount(sessionId, 'dispose-delay-zero')
      return
    }
    const timer = schedule(() => {
      disposeTimers.delete(sessionId)
      forceUnmount(sessionId, 'dispose-timeout')
    }, delay)
    disposeTimers.set(sessionId, timer)
  }

  function touch(sessionId: string, at: number = now()) {
    lastActivatedAt.set(sessionId, at)
  }

  function reconcile(openedRunningSessionIds: string[], activeSessionId: string | null) {
    const opened = new Set(openedRunningSessionIds)

    for (const sessionId of [...lastActivatedAt.keys()]) {
      if (!opened.has(sessionId)) {
        forceUnmount(sessionId, 'no-longer-opened-running')
      }
    }

    for (const sessionId of opened) {
      if (!lastActivatedAt.has(sessionId)) {
        lastActivatedAt.set(sessionId, now())
      }
    }

    if (activeSessionId && opened.has(activeSessionId)) {
      touch(activeSessionId)
      clearDisposeTimer(activeSessionId)
    }

    const candidates: KeepAliveActivation[] = openedRunningSessionIds.map((sessionId) => ({
      sessionId,
      lastActivatedAt: lastActivatedAt.get(sessionId) ?? now(),
    }))

    const nextHot = computeHotSessionIds(candidates, limits.value.maxHotTerminals, activeSessionId)
    const nextHotSet = new Set(nextHot)
    const previousHot = new Set(hotSessionIds.value)

    for (const sessionId of previousHot) {
      if (!nextHotSet.has(sessionId) && opened.has(sessionId)) {
        terminalDebug('keepalive.evict', { sessionId, activeSessionId })
        scheduleDispose(sessionId)
      }
    }

    for (const sessionId of nextHot) {
      clearDisposeTimer(sessionId)
    }

    const coldPending = [...disposeTimers.keys()].filter((sessionId) => opened.has(sessionId))
    const nextMounted = uniqueIds([...nextHot, ...coldPending])

    setHot(nextHot)
    setMounted(nextMounted)

    terminalDebug('keepalive.reconcile', {
      activeSessionId,
      openedRunning: openedRunningSessionIds,
      hot: nextHot,
      mounted: nextMounted,
      limits: limits.value,
    })
  }

  function remove(sessionId: string) {
    forceUnmount(sessionId, 'explicit-remove')
  }

  function disposeAll() {
    for (const sessionId of [...disposeTimers.keys()]) {
      clearDisposeTimer(sessionId)
    }
    lastActivatedAt.clear()
    setHot([])
    setMounted([])
    terminalDebug('keepalive.dispose-all', {})
  }

  return {
    limits,
    mountedSessionIds,
    hotSessionIds,
    isMounted,
    isHot,
    touch,
    reconcile,
    remove,
    disposeAll,
  }
}

function uniqueIds(ids: string[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const id of ids) {
    if (seen.has(id)) {
      continue
    }
    seen.add(id)
    out.push(id)
  }
  return out
}