export type RuntimeTarget = { mode: 'local' } | { mode: 'cloud'; deviceId: string }

export function runtimePath(target: RuntimeTarget, path: string): string {
  if (target.mode === 'local') {
    return path
  }
  return `/devices/${encodeURIComponent(target.deviceId)}${path}`
}
