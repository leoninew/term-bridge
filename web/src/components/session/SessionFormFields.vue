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
      <label class="flex shrink-0 items-center gap-1.5 whitespace-nowrap"
        ><input
          type="radio"
          value="shortcut"
          :checked="commandSource === 'shortcut'"
          @change="emit('update:commandSource', 'shortcut')"
        />{{ t('dialog.shortcut') }}</label
      >
      <label class="flex shrink-0 items-center gap-1.5 whitespace-nowrap"
        ><input
          type="radio"
          value="command"
          :checked="commandSource === 'command'"
          @change="emit('update:commandSource', 'command')"
        />{{ t('dialog.directCommand') }}</label
      >
    </div>
  </fieldset>
  <label
    v-if="commandSource === 'shortcut'"
    class="flex flex-col gap-1.5 text-sm text-[var(--color-text)]"
  >
    <span>{{ t('dialog.shortcut') }}</span>
    <ComboboxRoot
      :model-value="selectedShortcutId"
      :disabled="disabled || shortcuts.length === 0"
      @update:model-value="emit('update:selectedShortcutId', $event ?? null)"
    >
      <ComboboxAnchor class="relative flex w-full items-center">
        <ComboboxInput
          class="h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] py-2 pl-2 pr-9 text-[var(--color-text)] outline-none placeholder:text-[var(--color-text-subtle)] focus:border-[var(--color-border-strong)] disabled:opacity-70"
          :placeholder="shortcuts.length ? t('dialog.searchShortcuts') : t('dialog.noShortcuts')"
          :display-value="shortcutDisplayValue"
        />
        <ComboboxTrigger
          class="absolute right-0 inline-flex size-9 items-center justify-center rounded-r-md text-[var(--color-text-subtle)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)] disabled:cursor-not-allowed disabled:opacity-70"
          :aria-label="t('dialog.selectShortcut')"
        >
          <ChevronDown class="size-4" aria-hidden="true" />
        </ComboboxTrigger>
      </ComboboxAnchor>

      <ComboboxPortal>
        <ComboboxContent
          position="popper"
          :side-offset="6"
          class="z-[55] w-[var(--reka-combobox-trigger-width)] overflow-hidden rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] p-1 text-sm text-[var(--color-text)] shadow-xl"
        >
          <ComboboxViewport class="max-h-48 overflow-y-auto">
            <ComboboxEmpty class="px-2 py-2 text-[var(--color-text-muted)]">
              {{ shortcuts.length ? t('dialog.noShortcutMatches') : t('dialog.noShortcuts') }}
            </ComboboxEmpty>
            <ComboboxItem
              v-for="shortcut in shortcuts"
              :key="shortcut.id"
              :value="shortcut.id"
              :text-value="`${shortcut.name} ${shortcut.command}`"
              class="flex cursor-pointer flex-col gap-0.5 rounded px-2 py-1.5 outline-none data-[highlighted]:bg-[var(--color-control-hover)] data-[state=checked]:bg-[var(--color-control-active)]"
            >
              <span class="truncate font-medium">{{ shortcut.name }}</span>
              <span class="truncate text-[var(--color-text-muted)]">{{ shortcut.command }}</span>
            </ComboboxItem>
          </ComboboxViewport>
        </ComboboxContent>
      </ComboboxPortal>
    </ComboboxRoot>
  </label>
  <label
    v-if="commandSource === 'command'"
    class="flex flex-col gap-1.5 text-sm text-[var(--color-text)]"
  >
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
  import { ChevronDown } from '@lucide/vue'
  import { useI18n } from 'vue-i18n'
  import {
    ComboboxAnchor,
    ComboboxContent,
    ComboboxEmpty,
    ComboboxInput,
    ComboboxItem,
    ComboboxPortal,
    ComboboxRoot,
    ComboboxTrigger,
    ComboboxViewport,
  } from 'reka-ui'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'
  import type { CommandSource } from '../../composable/useCreateSessionDraft'

  const props = defineProps<{
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

  function shortcutDisplayValue(shortcutId: string | null) {
    const shortcut = props.shortcuts.find((value) => value.id === shortcutId)
    return shortcut ? `${shortcut.name} — ${shortcut.command}` : ''
  }
</script>
