<template>
  <div class="flex flex-col gap-1.5 text-sm text-[var(--color-text)]">
    <span>{{ t('dialog.command') }}</span>
    <div
      class="flex h-9 overflow-hidden rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] focus-within:border-[var(--color-border-strong)]"
    >
      <ComboboxRoot
        v-model:open="sourcePickerOpen"
        :model-value="commandSource"
        :disabled="disabled"
        @update:model-value="selectCommandSource"
      >
        <ComboboxAnchor
          class="relative flex w-32 shrink-0 items-center border-r border-[var(--color-border)]"
        >
          <ComboboxInput
            class="h-full w-full !border-0 bg-transparent py-2 pl-2 pr-8 text-[var(--color-text)] !shadow-none outline-none focus:!border-0 focus:!shadow-none disabled:opacity-70"
            :display-value="commandSourceDisplayValue"
            readonly
          />
          <ComboboxTrigger
            class="absolute right-0 inline-flex size-9 items-center justify-center text-[var(--color-text-subtle)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)] disabled:cursor-not-allowed disabled:opacity-70"
            :aria-label="t('dialog.commandSource')"
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
            <ComboboxViewport>
              <ComboboxItem
                value="shortcut"
                class="cursor-pointer rounded px-2 py-1.5 outline-none data-[highlighted]:bg-[var(--color-control-hover)] data-[state=checked]:bg-[var(--color-control-active)]"
              >
                {{ t('dialog.shortcut') }}
              </ComboboxItem>
              <ComboboxItem
                value="command"
                class="cursor-pointer rounded px-2 py-1.5 outline-none data-[highlighted]:bg-[var(--color-control-hover)] data-[state=checked]:bg-[var(--color-control-active)]"
              >
                {{ t('dialog.directCommand') }}
              </ComboboxItem>
            </ComboboxViewport>
          </ComboboxContent>
        </ComboboxPortal>
      </ComboboxRoot>

      <ComboboxRoot
        v-if="commandSource === 'shortcut'"
        v-model:open="shortcutPickerOpen"
        class="min-w-0 flex-1"
        :model-value="selectedShortcutId"
        :disabled="disabled || shortcuts.length === 0"
        @update:model-value="emit('update:selectedShortcutId', $event ?? null)"
      >
        <ComboboxAnchor class="relative flex min-w-0 flex-1 items-center">
          <ComboboxInput
            class="h-full w-full !border-0 bg-transparent py-2 pl-2 pr-9 text-[var(--color-text)] !shadow-none outline-none placeholder:text-[var(--color-text-subtle)] focus:!border-0 focus:!shadow-none disabled:opacity-70"
            :placeholder="shortcuts.length ? t('dialog.searchShortcuts') : t('dialog.noShortcuts')"
            :display-value="shortcutDisplayValue"
          />
          <ComboboxTrigger
            class="absolute right-0 inline-flex size-9 items-center justify-center text-[var(--color-text-subtle)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)] disabled:cursor-not-allowed disabled:opacity-70"
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
                :text-value="shortcutLabel(shortcut)"
                class="block cursor-pointer truncate rounded px-2 py-1.5 outline-none data-[highlighted]:bg-[var(--color-control-hover)] data-[state=checked]:bg-[var(--color-control-active)]"
                :title="shortcutLabel(shortcut)"
              >
                {{ shortcutLabel(shortcut) }}
              </ComboboxItem>
            </ComboboxViewport>
          </ComboboxContent>
        </ComboboxPortal>
      </ComboboxRoot>

      <input
        v-else
        :value="command"
        class="min-w-0 flex-1 !border-0 bg-transparent px-2 text-[var(--color-text)] !shadow-none outline-none placeholder:text-[var(--color-text-subtle)] focus:!border-0 focus:!shadow-none disabled:opacity-70"
        :placeholder="t('dialog.commandPlaceholder')"
        :disabled="disabled"
        @input="emit('update:command', ($event.target as HTMLInputElement).value)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
  import { ChevronDown } from '@lucide/vue'
  import { ref } from 'vue'
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
  import type { CommandSource } from '../../composable/useCreateSessionDraft'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'

  const props = defineProps<{
    command: string
    commandSource: CommandSource
    selectedShortcutId: string | null
    shortcuts: Shortcut[]
    disabled: boolean
  }>()
  const emit = defineEmits<{
    'update:command': [value: string]
    'update:commandSource': [value: CommandSource]
    'update:selectedShortcutId': [value: string | null]
  }>()
  const { t } = useI18n()
  const sourcePickerOpen = ref(false)
  const shortcutPickerOpen = ref(false)

  function commandSourceDisplayValue(source: CommandSource) {
    return source === 'shortcut' ? t('dialog.shortcut') : t('dialog.directCommand')
  }

  function selectCommandSource(source: CommandSource) {
    sourcePickerOpen.value = false
    emit('update:commandSource', source)
    if (source === 'command') {
      emit('update:selectedShortcutId', null)
    }
  }

  function shortcutDisplayValue(shortcutId: string | null) {
    const shortcut = props.shortcuts.find((value) => value.id === shortcutId)
    return shortcut ? shortcutLabel(shortcut) : ''
  }

  function shortcutLabel(shortcut: Shortcut) {
    return `${shortcut.name}(${shortcut.command})`
  }
</script>
