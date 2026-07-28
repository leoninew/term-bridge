<template>
  <nav aria-label="Documentation sections" class="space-y-1">
    <button
      v-for="page in pages"
      :key="page.id"
      type="button"
      class="w-full rounded-md px-3 py-2 text-left text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary-border)]"
      :class="activeId === page.id ? 'bg-[var(--color-control-active)] font-semibold text-[var(--color-text-strong)]' : 'text-[var(--color-text-muted)] hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)]'"
      :aria-current="activeId === page.id ? 'page' : undefined"
      @click="emit('navigate', page.id)"
    >
      <span class="mr-2 text-xs text-[var(--color-text-subtle)]">{{ page.order === 0 ? '•' : page.order }}</span>
      {{ page.title }}
    </button>
  </nav>
</template>

<script setup lang="ts">
  import type { DocPageId } from '../../features/docs/catalog'
  import type { DocPage } from '../../features/docs/content'

  defineProps<{ pages: DocPage[]; activeId: DocPageId }>()
  const emit = defineEmits<{ navigate: [id: DocPageId] }>()
</script>
