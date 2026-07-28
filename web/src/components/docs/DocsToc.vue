<template>
  <nav v-if="headings.length" aria-label="On this page">
    <p class="mb-3 text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-text-subtle)]">
      {{ language === 'zh-CN' ? '本页内容' : 'On this page' }}
    </p>
    <ul class="space-y-1 border-l border-[var(--color-border)]">
      <li v-for="heading in headings" :key="heading.id">
        <button
          type="button"
          class="w-full px-3 py-1.5 text-left text-sm leading-5 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary-border)]"
          :class="[heading.level === 3 ? 'pl-5' : '', activeHeading === heading.id ? 'font-medium text-[var(--color-primary-text)]' : 'text-[var(--color-text-muted)] hover:text-[var(--color-text)]']"
          @click="emit('navigate', heading.id)"
        >
          {{ heading.text }}
        </button>
      </li>
    </ul>
  </nav>
</template>

<script setup lang="ts">
  import type { DocLanguage } from '../../features/docs/catalog'
  import type { DocHeading } from '../../features/docs/markdown'

  defineProps<{ language: DocLanguage; headings: DocHeading[]; activeHeading: string }>()
  const emit = defineEmits<{ navigate: [id: string] }>()
</script>
