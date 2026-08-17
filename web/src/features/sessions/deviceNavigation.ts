export const localDeviceIdQueryKey = 'local_device_id'

export function localDeviceIdFromQuery(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

export function cloudSessionsQuery(localDeviceId: string): Record<string, string> {
  const deviceId = localDeviceId.trim()
  return deviceId ? { [localDeviceIdQueryKey]: deviceId } : {}
}

export function sessionsPageUrl(baseUrl: string, deviceId = ''): URL {
  const base = new URL(`${baseUrl.replace(/\/+$/, '')}/`)
  const path = deviceId ? `devices/${encodeURIComponent(deviceId)}/sessions` : 'sessions'
  return new URL(path, base)
}
