<template>
  <div class="flex flex-col gap-1.5 text-sm text-[var(--color-text)]">
    <span>{{ t('dialog.command') }}</span>

    <div ref="fieldRef" class="session-command-field">
      <ToggleGroupRoot
        type="single"
        class="session-command-source"
        :model-value="commandSource"
        :disabled="disabled"
        :aria-label="t('dialog.commandSource')"
        orientation="horizontal"
        @update:model-value="onCommandSourceChange"
      >
        <ToggleGroupItem
          value="shortcut"
          class="session-command-source-item"
          :aria-label="t('dialog.shortcut')"
          :title="t('dialog.shortcut')"
        >
          <Keyboard class="session-command-source-icon" aria-hidden="true" />
        </ToggleGroupItem>
        <ToggleGroupItem
          value="command"
          class="session-command-source-item"
          :aria-label="t('dialog.directCommand')"
          :title="t('dialog.directCommand')"
        >
          <Command class="session-command-source-icon" aria-hidden="true" />
        </ToggleGroupItem>
      </ToggleGroupRoot>

      <div class="session-command-body">
        <ComboboxRoot
          v-if="commandSource === 'shortcut'"
          v-model:open="shortcutPickerOpen"
          class="session-command-body-fill"
          :model-value="selectedShortcutId"
          :disabled="disabled || shortcuts.length === 0"
          @update:model-value="emit('update:selectedShortcutId', $event ?? null)"
        >
          <ComboboxAnchor class="session-command-body-fill relative flex items-center">
            <ComboboxInput
              class="session-command-input"
              :placeholder="
                shortcuts.length ? t('dialog.searchShortcuts') : t('dialog.noShortcuts')
              "
              :display-value="shortcutDisplayValue"
            />
            <ComboboxTrigger
              class="session-command-trigger"
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
                  <span v-if="shortcut.tags?.length" class="session-command-item-tags">
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
          class="session-command-input"
          :placeholder="t('dialog.commandPlaceholder')"
          :disabled="disabled"
          @input="emit('update:command', ($event.target as HTMLInputElement).value)"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
  import { Check, ChevronDown, Command, Keyboard } from '@lucide/vue'
  import { nextTick, ref, watch } from 'vue'
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
