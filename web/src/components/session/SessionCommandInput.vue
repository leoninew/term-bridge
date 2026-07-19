<template>
  <div class="flex flex-col gap-1.5 text-sm text-[var(--color-text)]">
    <span>{{ t('dialog.commandSource') }}</span>

    <div
      ref="fieldRef"
      class="flex h-9 items-stretch overflow-hidden rounded-lg border border-[var(--color-border)] bg-[var(--color-control-bg)] focus-within:border-[var(--color-border-strong)]"
    >
      <ToggleGroupRoot
        type="single"
        class="flex shrink-0 items-stretch divide-x divide-[var(--color-border)] border-r border-[var(--color-border)]"
        :model-value="commandSource"
        :disabled="disabled"
        :aria-label="t('dialog.commandSource')"
        orientation="horizontal"
        @update:model-value="onCommandSourceChange"
      >
        <ToggleGroupItem
          value="shortcut"
          class="inline-flex w-9 min-w-9 items-center justify-center border-0 bg-transparent text-[var(--color-text-muted)] outline-none transition-colors hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)] focus-visible:bg-[var(--color-control-hover)] focus-visible:text-[var(--color-text)] data-[state=on]:bg-[var(--color-control-active)] data-[state=on]:text-[var(--color-text-strong)] disabled:opacity-70"
          :aria-label="t('dialog.shortcut')"
          :title="t('dialog.shortcut')"
        >
          <LaunchMethodIcon command-source="shortcut" />
        </ToggleGroupItem>
        <ToggleGroupItem
          value="command"
          class="inline-flex w-9 min-w-9 items-center justify-center border-0 bg-transparent text-[var(--color-text-muted)] outline-none transition-colors hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)] focus-visible:bg-[var(--color-control-hover)] focus-visible:text-[var(--color-text)] data-[state=on]:bg-[var(--color-control-active)] data-[state=on]:text-[var(--color-text-strong)] disabled:opacity-70"
          :aria-label="t('dialog.directCommand')"
          :title="t('dialog.directCommand')"
        >
          <LaunchMethodIcon command-source="command" />
        </ToggleGroupItem>
      </ToggleGroupRoot>

      <div class="flex min-w-0 flex-1 items-stretch">
        <ComboboxRoot
          v-if="commandSource === 'shortcut'"
          v-model:open="shortcutPickerOpen"
          class="flex min-w-0 flex-1 items-stretch"
          :model-value="selectedShortcutId"
          :disabled="disabled || shortcuts.length === 0"
          @update:model-value="emit('update:selectedShortcutId', $event ?? null)"
        >
          <ComboboxAnchor class="relative flex min-w-0 flex-1 items-center">
            <ComboboxInput
              class="session-command-input h-full w-full min-w-0 rounded-none !border-0 bg-transparent px-3 pr-9 text-sm leading-none text-[var(--color-text)] !shadow-none outline-none placeholder:text-[var(--color-text-subtle)] disabled:opacity-70"
              :placeholder="
                shortcuts.length ? t('dialog.searchShortcuts') : t('dialog.noShortcuts')
              "
              :display-value="shortcutDisplayValue"
            />
            <ComboboxTrigger
              class="absolute inset-y-0 right-0 inline-flex w-9 items-center justify-center border-0 bg-transparent text-[var(--color-text-subtle)] outline-none hover:text-[var(--color-text)] focus-visible:text-[var(--color-text)] disabled:cursor-not-allowed disabled:opacity-70"
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
                  :text-value="shortcutSearchText(shortcut)"
                  class="group flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none data-[highlighted]:bg-[var(--color-control-hover)] data-[state=checked]:bg-[var(--color-control-active)]"
                >
                  <span
                    class="min-w-0 flex-1 truncate text-sm text-[var(--color-text)]"
                    :title="shortcut.name"
                  >
                    {{ shortcut.name }}
                  </span>
                  <span
                    v-if="shortcut.tags?.length"
                    class="inline-flex max-w-[46%] shrink-0 items-center justify-end gap-1 overflow-hidden"
                  >
                    <span
                      v-for="tag in visibleTags(shortcut)"
                      :key="tag"
                      class="shortcut-type-tag"
                      :style="shortcutTagStyle(tag)"
                    >
                      {{ tag }}
                    </span>
                    <span
                      v-if="hiddenTagCount(shortcut) > 0"
                      class="shortcut-type-tag shortcut-type-tag-more"
                    >
                      +{{ hiddenTagCount(shortcut) }}
                    </span>
                  </span>
                  <Check
                    class="size-4 shrink-0 text-[var(--color-text-subtle)] opacity-0 group-data-[state=checked]:opacity-100"
                    aria-hidden="true"
                  />
                </ComboboxItem>
              </ComboboxViewport>
            </ComboboxContent>
          </ComboboxPortal>
        </ComboboxRoot>

        <input
          v-else
          :value="command"
          class="session-command-input h-full w-full min-w-0 rounded-none !border-0 bg-transparent px-3 text-sm leading-none text-[var(--color-text)] !shadow-none outline-none placeholder:text-[var(--color-text-subtle)] disabled:opacity-70"
          :placeholder="t('dialog.commandPlaceholder')"
          :disabled="disabled"
          @input="emit('update:command', ($event.target as HTMLInputElement).value)"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
  import { Check, ChevronDown } from '@lucide/vue'
  import { nextTick, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import LaunchMethodIcon from './LaunchMethodIcon.vue'
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
    ToggleGroupItem,
    ToggleGroupRoot,
  } from 'reka-ui'
  import type { CommandSource } from '../../composable/useCreateSessionDraft'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'
  import { shortcutTagStyle } from '../shortcut/tagStyle'

  const MAX_VISIBLE_TAGS = 2

  const props = defineProps<{
    command: string
    commandSource: CommandSource
    selectedShortcutId: string | null
    selectedShortcutName: string | null
    shortcuts: Shortcut[]
    disabled: boolean
  }>()
  const emit = defineEmits<{
    'update:command': [value: string]
    'update:commandSource': [value: CommandSource]
    'update:selectedShortcutId': [value: string | null]
  }>()
  const { t } = useI18n()
  const fieldRef = ref<HTMLElement | null>(null)
  const shortcutPickerOpen = ref(false)

  function onCommandSourceChange(value: unknown) {
    if (value !== 'shortcut' && value !== 'command') return
    if (value !== 'shortcut') shortcutPickerOpen.value = false
    emit('update:commandSource', value)
  }

  watch(
    () => props.commandSource,
    async (source, previous) => {
      if (previous !== 'shortcut' && previous !== 'command') return
      if (source === previous) return
      await nextTick()
      focusCommandInput()
    },
  )

  function focusCommandInput() {
    if (props.disabled) return
    const input = fieldRef.value?.querySelector('.session-command-input') as {
      disabled: boolean
      focus: () => void
      select: () => void
    } | null
    if (!input || input.disabled) return
    input.focus()
    input.select()
  }

  function visibleTags(shortcut: Shortcut) {
    return (shortcut.tags ?? []).slice(0, MAX_VISIBLE_TAGS)
  }

  function hiddenTagCount(shortcut: Shortcut) {
    return Math.max(0, (shortcut.tags?.length ?? 0) - MAX_VISIBLE_TAGS)
  }

  function shortcutDisplayValue(shortcutId: string | null) {
    return (
      props.shortcuts.find((shortcut) => shortcut.id === shortcutId)?.name ??
      (shortcutId === props.selectedShortcutId ? (props.selectedShortcutName ?? '') : '')
    )
  }

  function shortcutSearchText(shortcut: Shortcut) {
    const tags = (shortcut.tags ?? []).join(' ')
    return `${shortcut.name} ${shortcut.command} ${tags}`
  }
</script>
