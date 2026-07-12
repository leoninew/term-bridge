<template>
  <article
    class="shortcut-card flex min-w-0 flex-col border border-[var(--color-border)] bg-[var(--color-surface)]"
  >
    <div class="shortcut-card-header flex min-w-0 items-start justify-between">
      <div class="min-w-0">
        <h3 class="truncate text-sm font-semibold text-[var(--color-text-strong)]">
          {{ shortcut.name }}
        </h3>
        <p
          v-if="shortcut.description"
          class="shortcut-card-description max-h-10 overflow-hidden text-sm text-[var(--color-text-muted)]"
        >
          {{ shortcut.description }}
        </p>
      </div>

      <div class="shortcut-card-actions flex shrink-0 items-center">
        <button
          type="button"
          class="button button-secondary button-icon shortcut-card-action text-[var(--color-text-muted)]"
          :aria-label="t('common.edit')"
          :title="t('common.edit')"
          @click="emit('edit', shortcut)"
        >
          <Pencil class="size-4" aria-hidden="true" />
        </button>
        <button
          type="button"
          class="button button-danger button-icon shortcut-card-action"
          :aria-label="t('common.delete')"
          :title="t('common.delete')"
          @click="emit('delete', shortcut)"
        >
          <Trash2 class="size-4" aria-hidden="true" />
        </button>
      </div>
    </div>

    <p class="shortcut-command" :title="shortcut.command">
      {{ shortcut.command }}
    </p>
  </article>
</template>

<script setup lang="ts">
  import { Pencil, Trash2 } from '@lucide/vue'
  import { useI18n } from 'vue-i18n'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'

  defineProps<{ shortcut: Shortcut }>()
  const emit = defineEmits<{ edit: [shortcut: Shortcut]; delete: [shortcut: Shortcut] }>()
  const { t } = useI18n()
</script>
