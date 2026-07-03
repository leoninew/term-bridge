import type { ApiTarget } from '../config'

export type RuntimeTarget = { mode: 'local' } | { mode: 'cloud'; deviceId: string }

export function runtimeApiTarget(target: RuntimeTarget): ApiTarget {
  return target.mode === 'local' ? 'agent' : 'cloud'
}

export function runtimePath(target: RuntimeTarget, path: string): string {
  if (target.mode === 'local') {
    return `/api${path}`
  }
  return `/api/devices/${encodeURIComponent(target.deviceId)}${path}`
}
