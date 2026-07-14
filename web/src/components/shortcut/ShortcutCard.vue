<template>
  <article
    class="shortcut-card"
    :class="{ 'shortcut-card-disabled': shortcut.enabled === false }"
    tabindex="0"
    @dblclick="emit('edit', shortcut)"
    @keydown.enter.prevent="emit('edit', shortcut)"
  >
    <div class="shortcut-card-top">
      <div class="shortcut-card-tags">
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

      <DropdownMenuRoot>
        <DropdownMenuTrigger
          class="shortcut-card-more"
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
            <DropdownMenuItem class="shortcut-card-menu-item" @select="emit('edit', shortcut)">
              <Pencil class="size-3.5" aria-hidden="true" />
              {{ t('common.edit') }}
            </DropdownMenuItem>
            <DropdownMenuItem
              class="shortcut-card-menu-item"
              @select="emit('toggle-enabled', shortcut)"
            >
              <Power class="size-3.5" aria-hidden="true" />
              {{ shortcut.enabled === false ? t('shortcut.enable') : t('shortcut.disable') }}
            </DropdownMenuItem>
            <DropdownMenuSeparator class="my-1 h-px bg-[var(--color-border)]" />
            <DropdownMenuItem
              class="shortcut-card-menu-item shortcut-card-menu-item-danger"
              @select="emit('delete', shortcut)"
            >
              <Trash2 class="size-3.5" aria-hidden="true" />
              {{ t('common.delete') }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenuPortal>
      </DropdownMenuRoot>
    </div>

    <div class="shortcut-card-body">
      <h3 class="shortcut-card-name" :title="shortcut.name">
        {{ shortcut.name }}
      </h3>
      <p
        v-if="shortcut.description"
        class="shortcut-card-secondary"
        :title="shortcut.description"
      >
        {{ shortcut.description }}
      </p>
      <div class="shortcut-command-row" data-no-drag>
        <pre class="shortcut-command" :title="shortcut.command">{{ shortcut.command }}</pre>
        <button
          type="button"
          class="shortcut-command-copy"
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

    <div class="shortcut-card-footer">
      <span
        v-if="shortcut.enabled === false"
        class="shortcut-card-status shortcut-card-status-disabled"
      >
        <CircleOff class="size-3" aria-hidden="true" />
        {{ t('shortcut.disabled') }}
      </span>
      <span v-else class="shortcut-card-status" :title="t('shortcut.updatedAtHint')">
        <Clock3 class="size-3" aria-hidden="true" />
        {{ updatedLabel }}
      </span>
    </div>
  </article>
</template>

<script setup lang="ts">
  import { CircleOff, Clock3, Copy, Ellipsis, Pencil, Power, Trash2 } from '@lucide/vue'
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

  const props = defineProps<{ shortcut: Shortcut }>()
  const emit = defineEmits<{
    edit: [shortcut: Shortcut]
    delete: [shortcut: Shortcut]
    copy: [shortcut: Shortcut]
    'toggle-enabled': [shortcut: Shortcut]
  }>()
  const { t, locale } = useI18n()

  const MAX_VISIBLE_TAGS = 2
  const visibleTags = computed(() => (props.shortcut.tags ?? []).slice(0, MAX_VISIBLE_TAGS))
  const hiddenTagCount = computed(() =>
    Math.max(0, (props.shortcut.tags?.length ?? 0) - MAX_VISIBLE_TAGS),
  )

  function tagStyle(tag: string) {
    return shortcutTagStyle(tag)
  }

  function parseTimestamp(value: string | undefined) {
    if (!value) {
      return null
    }
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? null : date
  }

  const updatedLabel = computed(() => {
    const date =
      parseTimestamp(props.shortcut.updated_at) ?? parseTimestamp(props.shortcut.created_at)
    if (!date) {
      return t('shortcut.updatedUnknown')
    }
    const formatted = new Intl.DateTimeFormat(locale.value === 'zh-CN' ? 'zh-CN' : 'en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    }).format(date)
    return t('shortcut.updatedAt', { time: formatted })
  })
</script>
