import { computed, onMounted, onUnmounted, ref } from 'vue'

const NARROW_QUERY = '(max-width: 768px)'
const COARSE_POINTER_QUERY = '(pointer: coarse)'

function matchQuery(query: string): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return false
  }
  return window.matchMedia(query).matches
}

/**
 * Layout mode for the sessions shell.
 * Narrow viewports get an overlay sidebar; coarse pointers disable drag reorder.
 */
export function useSessionsLayoutMode() {
  const isNarrow = ref(matchQuery(NARROW_QUERY))
  const isCoarsePointer = ref(matchQuery(COARSE_POINTER_QUERY))

  let narrowMedia: MediaQueryList | null = null
  let coarseMedia: MediaQueryList | null = null

  function syncNarrow() {
    isNarrow.value = matchQuery(NARROW_QUERY)
  }

  function syncCoarse() {
    isCoarsePointer.value = matchQuery(COARSE_POINTER_QUERY)
  }

  onMounted(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
      return
    }
    narrowMedia = window.matchMedia(NARROW_QUERY)
    coarseMedia = window.matchMedia(COARSE_POINTER_QUERY)
    syncNarrow()
    syncCoarse()
    narrowMedia.addEventListener('change', syncNarrow)
    coarseMedia.addEventListener('change', syncCoarse)
  })

  onUnmounted(() => {
    narrowMedia?.removeEventListener('change', syncNarrow)
    coarseMedia?.removeEventListener('change', syncCoarse)
    narrowMedia = null
    coarseMedia = null
  })

  const disableReorder = computed(() => isNarrow.value || isCoarsePointer.value)

  return {
    isNarrow,
    isCoarsePointer,
    disableReorder,
  }
}
