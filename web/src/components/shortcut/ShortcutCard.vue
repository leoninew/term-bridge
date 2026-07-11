<template>
  <article
    class="flex min-w-0 flex-col rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-sm"
  >
    <div class="flex min-w-0 items-start justify-between gap-3">
      <div class="min-w-0">
        <h3 class="truncate font-semibold text-[var(--color-text-strong)]">
          {{ shortcut.name }}
        </h3>
        <p v-if="shortcut.description" class="mt-1 max-h-10 overflow-hidden text-sm text-[var(--color-text-muted)]">
          {{ shortcut.description }}
        </p>
      </div>

      <div class="flex shrink-0 items-center gap-1">
        <button
          type="button"
          class="inline-flex size-8 items-center justify-center rounded-md text-sm text-[var(--color-text-muted)] outline-none hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)] focus:bg-[var(--color-control-hover)] focus:text-[var(--color-text)]"
          :aria-label="t('common.edit')"
          :title="t('common.edit')"
          @click="emit('edit', shortcut)"
        >
          {{ t('common.edit') }}
        </button>
        <button
          type="button"
          class="inline-flex size-8 items-center justify-center rounded-md text-sm text-[var(--color-danger-text)] outline-none hover:bg-[var(--color-danger-bg)] focus:bg-[var(--color-danger-bg)]"
          :aria-label="t('common.delete')"
          :title="t('common.delete')"
          @click="emit('delete', shortcut)"
        >
          {{ t('common.delete') }}
        </button>
      </div>
    </div>

    <code
      class="mt-4 block max-h-36 min-h-16 overflow-auto whitespace-pre-wrap break-words rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-muted)] p-3 font-mono text-xs leading-5 text-[var(--color-text)]"
    >{{ shortcut.command }}</code>
  </article>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'

  defineProps<{ shortcut: Shortcut }>()
  const emit = defineEmits<{ edit: [shortcut: Shortcut]; delete: [shortcut: Shortcut] }>()
  const { t } = useI18n()
</script>
