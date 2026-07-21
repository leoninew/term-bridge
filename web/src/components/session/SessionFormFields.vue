<template>
  <label class="flex flex-col gap-1.5 text-sm font-medium text-[var(--color-text)]">
    <span>{{ t('dialog.name') }}</span>
    <input
      :value="name"
      class="h-9 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-sm font-normal text-[var(--color-text)] outline-none placeholder:text-[var(--color-text-subtle)] focus:border-[var(--color-border-strong)] disabled:opacity-70"
      :placeholder="t('dialog.sessionNamePlaceholder')"
      :disabled="disabled"
      @input="emit('update:name', ($event.target as HTMLInputElement).value)"
    />
  </label>
  <label
    v-if="!cwdHidden"
    class="flex flex-col gap-1.5 text-sm font-medium text-[var(--color-text)]"
  >
    <span>{{ t('dialog.cwd') }}</span>
    <input
      :value="cwd"
      class="h-9 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-sm font-normal text-[var(--color-text)] outline-none placeholder:text-[var(--color-text-subtle)] focus:border-[var(--color-border-strong)] disabled:opacity-70"
      :placeholder="t('dialog.workingDirectoryPlaceholder')"
      :readonly="cwdReadonly"
      :disabled="disabled"
      @input="emit('update:cwd', ($event.target as HTMLInputElement).value)"
    />
  </label>
  <SessionCommandInput
    :command="command"
    :command-source="commandSource"
    :selected-shortcut-id="selectedShortcutId"
    :selected-shortcut-name="selectedShortcutName"
    :shortcuts="shortcuts"
    :disabled="disabled || commandDisabled"
    @update:command="emit('update:command', $event)"
    @update:command-source="emit('update:commandSource', $event)"
    @update:selected-shortcut-id="emit('update:selectedShortcutId', $event)"
  />
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import SessionCommandInput from './SessionCommandInput.vue'
  import type { CommandSource } from '../../composable/useCreateSessionDraft'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'

  defineProps<{
    cwd: string
    /** Hide the directory field; cwd still submits from the draft. */
    cwdHidden?: boolean
    /** Show directory as non-editable text (edit session). */
    cwdReadonly?: boolean
    /** Disable launch method controls while leaving the name editable. */
    commandDisabled?: boolean
    name: string
    command: string
    commandSource: CommandSource
    selectedShortcutId: string | null
    selectedShortcutName: string | null
    shortcuts: Shortcut[]
    disabled: boolean
  }>()
  const emit = defineEmits<{
    'update:cwd': [value: string]
    'update:name': [value: string]
    'update:command': [value: string]
    'update:commandSource': [value: CommandSource]
    'update:selectedShortcutId': [value: string | null]
  }>()
  const { t } = useI18n()
</script>
