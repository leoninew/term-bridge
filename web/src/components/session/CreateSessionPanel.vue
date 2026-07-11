<template>
  <section ref="workbench" class="flex min-h-0 min-w-0 flex-1 items-center justify-center overflow-hidden bg-[var(--color-panel-bg)] p-6">
    <form class="flex w-full max-w-xl flex-col gap-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-xl" @submit.prevent="emit('submit')">
      <div class="flex items-start gap-3 border-b border-[var(--color-border)] pb-3">
        <span class="flex size-9 shrink-0 items-center justify-center rounded-md border border-[var(--color-border)] bg-[var(--color-panel-header)] text-[var(--color-text-muted)]"><SquareTerminal class="size-4" /></span>
        <div><h3 class="text-base font-semibold text-[var(--color-text-strong)]">{{ t('dialog.newSessionTitle') }}</h3><p class="mt-0.5 text-sm text-[var(--color-text-muted)]">{{ t('dialog.newSessionDescription') }}</p></div>
      </div>
      <SessionFormFields
        :cwd="cwd" :name="name" :command="command" :command-source="commandSource" :selected-shortcut-id="selectedShortcutId" :shortcuts="shortcuts" :disabled="creating"
        @update:cwd="emit('update:cwd', $event)" @update:name="emit('update:name', $event)" @update:command="emit('update:command', $event)" @update:command-source="emit('update:commandSource', $event)" @update:selected-shortcut-id="emit('update:selectedShortcutId', $event)"
      />
      <div class="mt-1 flex items-center justify-end gap-2"><button type="button" class="button button-secondary" :disabled="creating" @click="emit('cancel')">{{ t('common.cancel') }}</button><button type="submit" class="button button-primary" :disabled="creating">{{ creating ? t('common.creating') : t('common.create') }}</button></div>
    </form>
  </section>
</template>
<script setup lang="ts">
  import { useTemplateRef, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { SquareTerminal } from '@lucide/vue'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'
  import type { CommandSource } from '../../composable/useCreateSessionDraft'
  import SessionFormFields from './SessionFormFields.vue'
  defineProps<{ cwd: string; name: string; command: string; commandSource: CommandSource; selectedShortcutId: string | null; shortcuts: Shortcut[]; creating: boolean }>()
  const emit = defineEmits<{ 'update:cwd': [value: string]; 'update:name': [value: string]; 'update:command': [value: string]; 'update:commandSource': [value: CommandSource]; 'update:selectedShortcutId': [value: string | null]; workbench: [element: HTMLElement | null]; submit: []; cancel: [] }>()
  const { t } = useI18n(); const workbench = useTemplateRef<HTMLElement>('workbench')
  watch(workbench, (element) => emit('workbench', element), { immediate: true })
</script>
