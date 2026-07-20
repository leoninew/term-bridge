import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { useSessionsLayoutMode } from './useSessionsLayoutMode'

function mountHook() {
  let api: ReturnType<typeof useSessionsLayoutMode> | null = null
  const Comp = defineComponent({
    setup() {
      api = useSessionsLayoutMode()
      return () => null
    },
  })
  const wrapper = mount(Comp)
  return {
    api: api!,
    unmount: () => wrapper.unmount(),
  }
}

describe('useSessionsLayoutMode', () => {
  const listeners = new Map<string, Set<() => void>>()

  beforeEach(() => {
    listeners.clear()
    vi.stubGlobal(
      'matchMedia',
      vi.fn((query: string) => {
        const matches = query.includes('max-width: 768px')
          ? false
          : query.includes('pointer: coarse')
            ? false
            : false
        return {
          matches,
          media: query,
          addEventListener: (_event: string, cb: () => void) => {
            const set = listeners.get(query) ?? new Set()
            set.add(cb)
            listeners.set(query, set)
          },
          removeEventListener: (_event: string, cb: () => void) => {
            listeners.get(query)?.delete(cb)
          },
        }
      }),
    )
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('reports desktop defaults and allows reorder', async () => {
    const { api, unmount } = mountHook()
    await nextTick()
    expect(api.isNarrow.value).toBe(false)
    expect(api.isCoarsePointer.value).toBe(false)
    expect(api.disableReorder.value).toBe(false)
    unmount()
  })

  it('disables reorder when the viewport is narrow', async () => {
    vi.stubGlobal(
      'matchMedia',
      vi.fn((query: string) => ({
        matches: query.includes('max-width: 768px'),
        media: query,
        addEventListener: () => undefined,
        removeEventListener: () => undefined,
      })),
    )
    const { api, unmount } = mountHook()
    await nextTick()
    expect(api.isNarrow.value).toBe(true)
    expect(api.disableReorder.value).toBe(true)
    unmount()
  })
})
