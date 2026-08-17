import { describe, expect, it } from 'vitest'
import { cloudSessionsQuery, localDeviceIdFromQuery, sessionsPageUrl } from './deviceNavigation'

describe('device navigation', () => {
  it('preserves the configured base path for local and Cloud session pages', () => {
    expect(sessionsPageUrl('http://127.0.0.1:9030/local/').toString()).toBe(
      'http://127.0.0.1:9030/local/sessions',
    )
    expect(sessionsPageUrl('https://cloud.example.test/app', 'device/1').toString()).toBe(
      'https://cloud.example.test/app/devices/device%2F1/sessions',
    )
  })

  it('carries a local device identifier only when available', () => {
    expect(localDeviceIdFromQuery(['device-1'])).toBe('')
    expect(localDeviceIdFromQuery(' device-1 ')).toBe('device-1')
    expect(cloudSessionsQuery('device-1')).toEqual({ local_device_id: 'device-1' })
    expect(cloudSessionsQuery('')).toEqual({})
  })
})
