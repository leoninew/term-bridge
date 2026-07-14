import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { Shortcut } from '../gen/proto/termbridge/agent/v1/shortcut'
import { useShortcutTagsStore } from './shortcutTags'

function shortcut(id: string, tags: string[]): Shortcut {
  return {
    id,
    name: id,
    command: 'go test ./...',
    description: undefined,
    icon: undefined,
    enabled: true,
    tags,
    last_used_at: undefined,
    created_at: undefined,
    updated_at: undefined,
  }
}

describe('useShortcutTagsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('derives a unique, sorted tag list from loaded shortcuts', () => {
    const store = useShortcutTagsStore()

    store.setShortcuts([
      shortcut('review', ['release', 'review']),
      shortcut('build', ['build', 'review']),
      shortcut('shell', []),
    ])

    expect(store.tags).toEqual(['build', 'release', 'review'])
  })

  it('replaces tags when the loaded shortcut collection changes', () => {
    const store = useShortcutTagsStore()
    store.setShortcuts([shortcut('review', ['review'])])

    store.setShortcuts([shortcut('deploy', ['release'])])

    expect(store.tags).toEqual(['release'])
  })
})
