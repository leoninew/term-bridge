<template>
  <section
    ref="workbench"
    class="flex min-h-0 min-w-0 flex-1 items-center justify-center overflow-hidden bg-[var(--color-panel-bg)] p-6"
  >
    <form
      class="flex w-full max-w-xl flex-col gap-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-xl"
      @submit.prevent="emit('submit')"
    >
      <div class="flex items-start gap-3 border-b border-[var(--color-border)] pb-3">
        <span
          class="flex size-9 shrink-0 items-center justify-center rounded-md border border-[var(--color-border)] bg-[var(--color-panel-header)] text-[var(--color-text-muted)]"
        >
          <SquareTerminal class="size-4" aria-hidden="true" />
        </span>
        <div class="min-w-0">
          <h3 class="text-base font-semibold text-[var(--color-text-strong)]">
            {{ t('dialog.newSessionTitle') }}
          </h3>
          <p class="mt-0.5 text-sm text-[var(--color-text-muted)]">
            {{ t('dialog.newSessionDescription') }}
          </p>
        </div>
      </div>

      <label class="flex flex-col gap-1.5 text-sm text-[var(--color-text)]">
        <span>{{ t('dialog.cwd') }}</span>
        <input
          :value="cwd"
          class="h-9 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-[var(--color-text)] outline-none placeholder:text-[var(--color-text-subtle)] focus:border-[var(--color-border-strong)]"
          :placeholder="t('dialog.workingDirectoryPlaceholder')"
          @input="emit('update:cwd', ($event.target as HTMLInputElement).value)"
        />
      </label>
      <label class="flex flex-col gap-1.5 text-sm text-[var(--color-text)]">
        <span>{{ t('dialog.name') }}</span>
        <input
          :value="name"
          class="h-9 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-[var(--color-text)] outline-none placeholder:text-[var(--color-text-subtle)] focus:border-[var(--color-border-strong)]"
          :placeholder="t('dialog.sessionNamePlaceholder')"
          @input="emit('update:name', ($event.target as HTMLInputElement).value)"
        />
      </label>
      <label class="flex flex-col gap-1.5 text-sm text-[var(--color-text)]">
        <span>{{ t('dialog.command') }}</span>
        <input
          :value="command"
          class="h-9 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-[var(--color-text)] outline-none placeholder:text-[var(--color-text-subtle)] focus:border-[var(--color-border-strong)]"
          :placeholder="t('dialog.commandPlaceholder')"
          @input="emit('update:command', ($event.target as HTMLInputElement).value)"
        />
      </label>
      <div class="mt-1 flex items-center justify-end gap-2">
        <button
          type="button"
          class="button button-secondary"
          :disabled="creating"
          @click="emit('cancel')"
        >
          {{ t('common.cancel') }}
        </button>
        <button type="submit" class="button button-primary" :disabled="creating">
          {{ creating ? t('common.creating') : t('common.create') }}
        </button>
      </div>
    </form>
  </section>
</template>

<script setup lang="ts">
  import { useTemplateRef, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { SquareTerminal } from '@lucide/vue'

  defineProps<{
    cwd: string
    name: string
    command: string
    creating: boolean
  }>()

  const emit = defineEmits<{
    'update:cwd': [value: string]
    'update:name': [value: string]
    'update:command': [value: string]
    workbench: [element: HTMLElement | null]
    submit: []
    cancel: []
  }>()

  const { t } = useI18n()
  const workbench = useTemplateRef<HTMLElement>('workbench')

  watch(
    workbench,
    (element) => {
      emit('workbench', element)
    },
    { immediate: true },
  )
</script>
