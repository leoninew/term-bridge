import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useNotificationsStore } from './notifications'

describe('useNotificationsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('preserves an optional toast duration in milliseconds', () => {
    const notifications = useNotificationsStore()

    notifications.pushToast('info', 'Recent terminal output only', undefined, 3000)

    expect(notifications.toasts).toEqual([
      {
        id: 1,
        kind: 'info',
        title: 'Recent terminal output only',
        description: undefined,
        durationMs: 3000,
      },
    ])
  })
})
