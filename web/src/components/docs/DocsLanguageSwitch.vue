<template>
  <div class="inline-flex rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] p-0.5" role="group" :aria-label="language === 'zh-CN' ? '文档语言' : 'Documentation language'">
    <button
      v-for="item in languages"
      :key="item.id"
      type="button"
      class="rounded px-2.5 py-1.5 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary-border)]"
      :class="item.id === language ? 'bg-[var(--color-control-active)] text-[var(--color-text-strong)]' : 'text-[var(--color-text-muted)] hover:text-[var(--color-text)]'"
      :aria-pressed="item.id === language"
      @click="emit('change', item.id)"
    >
      {{ item.label }}
    </button>
  </div>
</template>

<script setup lang="ts">
  import { DOC_LANGUAGES, type DocLanguage } from '../../features/docs/catalog'

  defineProps<{ language: DocLanguage }>()
  const emit = defineEmits<{ change: [language: DocLanguage] }>()
  const languages = DOC_LANGUAGES.map((id) => ({ id, label: id === 'zh-CN' ? '中文' : 'English' }))
</script>
