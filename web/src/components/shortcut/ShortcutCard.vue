<template>
  <article
    class="shortcut-card"
    :class="{
      'shortcut-card-disabled': shortcut.enabled === false,
      'shortcut-card-selected': selected,
      'shortcut-card-selectable': selectionMode,
    }"
    tabindex="0"
    :aria-selected="selectionMode ? selected : undefined"
    @click="onCardClick"
    @keydown.enter.prevent="onCardActivate"
    @keydown.space.prevent="onCardActivate"
  >
    <div class="flex min-w-0 items-center justify-between gap-2">
      <h3
        class="min-w-0 flex-1 truncate text-base leading-tight text-[var(--color-text-strong)]"
        :title="shortcut.name"
      >
        {{ shortcut.name }}
      </h3>

      <DropdownMenuRoot>
        <DropdownMenuTrigger
          class="inline-flex size-6 shrink-0 items-center justify-center rounded-md border-0 bg-transparent text-[var(--color-text-subtle)] outline-none hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)] focus-visible:bg-[var(--color-control-hover)] focus-visible:text-[var(--color-text)] data-[state=open]:bg-[var(--color-control-hover)] data-[state=open]:text-[var(--color-text)]"
          data-no-drag
          :aria-label="t('common.more')"
          :title="t('common.more')"
          @click.stop
          @pointerdown.stop
        >
          <Ellipsis class="size-4" aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuPortal>
          <DropdownMenuContent
            align="end"
            :side-offset="6"
            class="shortcut-card-menu z-[55] min-w-36 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] p-1 text-sm text-[var(--color-text)] shadow-xl"
            data-no-drag
          >
            <DropdownMenuItem
              class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
              @select="emit('edit', shortcut)"
            >
              <Pencil class="size-3.5" aria-hidden="true" />
              {{ t('common.edit') }}
            </DropdownMenuItem>
            <DropdownMenuItem
              class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
              @select="emit('toggle-enabled', shortcut)"
            >
              <Power class="size-3.5" aria-hidden="true" />
              {{ shortcut.enabled === false ? t('shortcut.enable') : t('shortcut.disable') }}
            </DropdownMenuItem>
            <DropdownMenuSeparator class="my-1 h-px bg-[var(--color-border)]" />
            <DropdownMenuItem
              class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-[var(--color-danger-text)] outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
              @select="emit('delete', shortcut)"
            >
              <Trash2 class="size-3.5" aria-hidden="true" />
              {{ t('common.delete') }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenuPortal>
      </DropdownMenuRoot>
    </div>

    <div class="flex min-w-0 flex-1 flex-col gap-0.5 overflow-hidden">
      <p
        v-if="shortcut.description"
        class="truncate text-[13px] leading-snug text-[var(--color-text-muted)]"
        :title="shortcut.description"
      >
        {{ shortcut.description }}
      </p>
      <div class="shortcut-command-row" data-no-drag>
        <pre class="shortcut-command" :title="shortcut.command">{{ shortcut.command }}</pre>
        <button
          type="button"
          class="shortcut-command-copy inline-flex size-6 shrink-0 items-center justify-center rounded border-0 bg-transparent text-[var(--color-text-subtle)] outline-none hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)] focus-visible:bg-[var(--color-control-hover)] focus-visible:text-[var(--color-text)]"
          data-no-drag
          :aria-label="t('shortcut.copyCommand')"
          :title="t('shortcut.copyCommand')"
          @click.stop="emit('copy', shortcut)"
          @pointerdown.stop
        >
          <Copy class="size-3.5" aria-hidden="true" />
        </button>
      </div>
    </div>

    <div class="mt-auto flex min-w-0 items-center justify-between gap-2">
      <div class="flex min-w-0 flex-1 items-center gap-1 overflow-hidden">
        <span
          v-for="tag in visibleTags"
          :key="tag"
          class="shortcut-type-tag"
          :style="tagStyle(tag)"
        >
          {{ tag }}
        </span>
        <span v-if="hiddenTagCount > 0" class="shortcut-type-tag shortcut-type-tag-more">
          +{{ hiddenTagCount }}
        </span>
      </div>

      <span
        v-if="shortcut.enabled === false"
        class="inline-flex min-w-0 shrink-0 items-center gap-1 truncate text-xs leading-none text-[var(--color-text-muted)]"
      >
        <CircleOff class="size-3" aria-hidden="true" />
        {{ t('shortcut.disabled') }}
      </span>
      <!-- timestamp hidden
      <span
        v-else
        class="inline-flex min-w-0 shrink-0 items-center gap-1 truncate text-xs leading-none text-[var(--color-text-subtle)]"
        :title="t('shortcut.updatedAtHint')"
      >
        <Clock3 class="size-3" aria-hidden="true" />
        {{ updatedLabel }}
      </span>
      -->
    </div>
  </article>
</template>

<script setup lang="ts">
  import { CircleOff, /* Clock3, */ Copy, Ellipsis, Pencil, Power, Trash2 } from '@lucide/vue'
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import {
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuPortal,
    DropdownMenuRoot,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
  } from 'reka-ui'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'
  import { shortcutTagStyle } from './tagStyle'

  const props = defineProps<{
    shortcut: Shortcut
    selectionMode?: boolean
    selected?: boolean
  }>()
  const emit = defineEmits<{
    edit: [shortcut: Shortcut]
    delete: [shortcut: Shortcut]
    copy: [shortcut: Shortcut]
    'toggle-enabled': [shortcut: Shortcut]
    toggle: [shortcut: Shortcut]
  }>()
  const { t /*, locale */ } = useI18n()

  const MAX_VISIBLE_TAGS = 2
  const visibleTags = computed(() => (props.shortcut.tags ?? []).slice(0, MAX_VISIBLE_TAGS))
  const hiddenTagCount = computed(() =>
    Math.max(0, (props.shortcut.tags?.length ?? 0) - MAX_VISIBLE_TAGS),
  )

  function tagStyle(tag: string) {
    return shortcutTagStyle(tag)
  }

  function onCardClick() {
    if (!props.selectionMode) {
      return
    }
    emit('toggle', props.shortcut)
  }

  function onCardActivate() {
    if (props.selectionMode) {
      emit('toggle', props.shortcut)
      return
    }
    emit('edit', props.shortcut)
  }

  // timestamp helpers (hidden)
  // function parseTimestamp(value: string | undefined) {
  //   if (!value) {
  //     return null
  //   }
  //   const date = new Date(value)
  //   return Number.isNaN(date.getTime()) ? null : date
  // }
  //
  // const updatedLabel = computed(() => {
  //   const date =
  //     parseTimestamp(props.shortcut.updated_at) ?? parseTimestamp(props.shortcut.created_at)
  //   if (!date) {
  //     return t('shortcut.updatedUnknown')
  //   }
  //   const formatted = new Intl.DateTimeFormat(locale.value === 'zh-CN' ? 'zh-CN' : 'en-US', {
  //     month: 'short',
  //     day: 'numeric',
  //     hour: '2-digit',
  //     minute: '2-digit',
  //   }).format(date)
  //   return t('shortcut.updatedAt', { time: formatted })
  // })
</script>
