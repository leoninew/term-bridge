import type { ApiTarget } from '../config'

export type RuntimeTarget = { mode: 'agent' } | { mode: 'cloud'; deviceId: string }

export function runtimeApiTarget(target: RuntimeTarget): ApiTarget {
  return target.mode
}

export function runtimePath(target: RuntimeTarget, path: string): string {
  if (target.mode === 'agent') {
    return path
  }
  return `/devices/${encodeURIComponent(target.deviceId)}${path}`
}
