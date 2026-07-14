import { ref } from 'vue'
import { defineStore } from 'pinia'
import type { Shortcut } from '../gen/proto/termbridge/agent/v1/shortcut'

export const useShortcutTagsStore = defineStore('shortcutTags', () => {
  const tags = ref<string[]>([])

  function setShortcuts(values: Shortcut[]) {
    tags.value = [...new Set(values.flatMap((shortcut) => shortcut.tags ?? []))].sort(
      (left, right) => left.localeCompare(right),
    )
  }

  return { tags, setShortcuts }
})
