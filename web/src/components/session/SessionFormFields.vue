<template>
  <label class="flex flex-col gap-1.5 text-sm text-[var(--color-text)]">
    <span>{{ t('dialog.cwd') }}</span>
    <input
      :value="cwd"
      class="h-9 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-[var(--color-text)] outline-none placeholder:text-[var(--color-text-subtle)] focus:border-[var(--color-border-strong)] disabled:opacity-70"
      :placeholder="t('dialog.workingDirectoryPlaceholder')"
      :readonly="cwdReadonly"
      :disabled="disabled"
      @input="emit('update:cwd', ($event.target as HTMLInputElement).value)"
    />
  </label>
  <label class="flex flex-col gap-1.5 text-sm text-[var(--color-text)]">
    <span>{{ t('dialog.name') }}</span>
    <input
      :value="name"
      class="h-9 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-[var(--color-text)] outline-none placeholder:text-[var(--color-text-subtle)] focus:border-[var(--color-border-strong)]"
      :placeholder="t('dialog.sessionNamePlaceholder')"
      :disabled="disabled"
      @input="emit('update:name', ($event.target as HTMLInputElement).value)"
    />
  </label>
  <fieldset class="flex flex-col gap-2 text-sm text-[var(--color-text)]" :disabled="disabled">
    <legend>{{ t('dialog.commandSource') }}</legend>
    <div class="flex gap-3">
      <label class="flex items-center gap-1.5"><input type="radio" value="shortcut" :checked="commandSource === 'shortcut'" @change="emit('update:commandSource', 'shortcut')" />{{ t('dialog.shortcut') }}</label>
      <label class="flex items-center gap-1.5"><input type="radio" value="command" :checked="commandSource === 'command'" @change="emit('update:commandSource', 'command')" />{{ t('dialog.directCommand') }}</label>
    </div>
  </fieldset>
  <label v-if="commandSource === 'shortcut'" class="flex flex-col gap-1.5 text-sm text-[var(--color-text)]">
    <span>{{ t('dialog.shortcut') }}</span>
    <select
      :value="selectedShortcutId ?? ''"
      class="h-9 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-[var(--color-text)] outline-none focus:border-[var(--color-border-strong)]"
      :disabled="disabled"
      @change="emit('update:selectedShortcutId', ($event.target as HTMLSelectElement).value || null)"
    >
      <option value="">{{ shortcuts.length ? t('dialog.selectShortcut') : t('dialog.noShortcuts') }}</option>
      <option v-for="shortcut in shortcuts" :key="shortcut.id" :value="shortcut.id">{{ shortcut.name }} — {{ shortcut.command }}</option>
    </select>
  </label>
  <label v-if="commandSource === 'command'" class="flex flex-col gap-1.5 text-sm text-[var(--color-text)]">
    <span>{{ t('dialog.command') }}</span>
    <input
      :value="command"
      class="h-9 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-[var(--color-text)] outline-none placeholder:text-[var(--color-text-subtle)] focus:border-[var(--color-border-strong)]"
      :placeholder="t('dialog.commandPlaceholder')"
      :disabled="disabled"
      @input="emit('update:command', ($event.target as HTMLInputElement).value)"
    />
  </label>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'
  import type { CommandSource } from '../../composable/useCreateSessionDraft'

  defineProps<{
    cwd: string
    cwdReadonly?: boolean
    name: string
    command: string
    commandSource: CommandSource
    selectedShortcutId: string | null
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
