<template>
  <div
    role="button"
    tabindex="0"
    class="session-sortable-item group flex min-h-10 w-full min-w-0 cursor-pointer items-center gap-1 border-0 px-1 py-1 text-left transition sm:min-h-7 sm:py-0.5"
    :class="
      active
        ? 'bg-[var(--color-control-active)] text-[var(--color-text-strong)]'
        : 'bg-transparent text-[var(--color-text-muted)] hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)]'
    "
    :style="{ paddingLeft: '24px' }"
    @click="emit('activate', $event)"
    @keydown.enter="emit('activateKey', $event)"
    @keydown.space="emit('activateKey', $event)"
  >
    <SquareTerminal class="size-4 shrink-0 text-[var(--color-text-subtle)]" aria-hidden="true" />
    <span class="min-w-0 flex-1 truncate text-sm">{{ session.name || session.command }}</span>

    <span v-if="session.lifecycle_state === 'running'" :class="treeNodeActionsClass">
      <button
        type="button"
        :class="treeNodeActionClass"
        :aria-label="t('sidebar.copySessionAria')"
        :title="t('sidebar.copySessionAria')"
        @click.stop="emit('copy')"
      >
        <Copy class="size-3.5" />
      </button>
    </span>

    <span
      v-if="session.lifecycle_state === 'running'"
      class="flex size-5 shrink-0 items-center justify-center"
    >
      <CircleDot
        class="size-3.5"
        :class="lifecycleStateClassName(session.lifecycle_state)"
        aria-hidden="true"
      />
    </span>

    <span v-if="isEditable" :class="treeNodeActionsClass">
      <button
        type="button"
        :class="treeNodeActionClass"
        :aria-label="t('sidebar.editSessionAria')"
        :title="t('sidebar.editSessionAria')"
        @click.stop="emit('edit')"
      >
        <Pencil class="size-3.5" />
      </button>
    </span>

    <button
      v-if="isEditable"
      type="button"
      :disabled="rerunning"
      :class="treeNodeActionClass"
      :aria-label="t('sidebar.rerunSessionAria')"
      :title="t('sidebar.rerunSessionAria')"
      @click.stop="emit('rerun')"
    >
      <Loader2 v-if="rerunning" class="size-3.5 animate-spin" />
      <RotateCcw v-else class="size-3.5" />
    </button>

    <button
      v-if="session.lifecycle_state === 'running'"
      type="button"
      :disabled="stopping"
      :class="treeNodeActionClass"
      :aria-label="t('sidebar.stopSessionAria')"
      :title="t('sidebar.stopSessionAria')"
      @click.stop="emit('stop')"
    >
      <Loader2 v-if="stopping" class="size-3.5 animate-spin" />
      <CircleStop v-else class="size-3.5" />
    </button>

    <button
      v-if="isEditable"
      type="button"
      :disabled="deleting"
      :class="treeNodeActionClass"
      :aria-label="t('sidebar.deleteSessionAria')"
      :title="t('sidebar.deleteSessionAria')"
      @click.stop="emit('delete')"
    >
      <Loader2 v-if="deleting" class="size-3.5 animate-spin" />
      <Trash2 v-else class="size-3.5" />
    </button>
  </div>
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import {
    CircleDot,
    CircleStop,
    Copy,
    Loader2,
    Pencil,
    RotateCcw,
    SquareTerminal,
    Trash2,
  } from '@lucide/vue'
  import { lifecycleStateClassName } from '../../features/sessions/lifecycleState'
  import type { SessionSummary } from '../../gen/proto/termbridge/agent/v1/workspace'
  import { treeNodeActionClass, treeNodeActionsClass } from '../session/sessionUi'

  const props = defineProps<{
    session: SessionSummary
    active: boolean
    stopping?: boolean
    rerunning?: boolean
    deleting?: boolean
  }>()

  const emit = defineEmits<{
    activate: [event: Event]
    activateKey: [event: Event]
    copy: []
    edit: []
    stop: []
    rerun: []
    delete: []
  }>()

  const { t } = useI18n()

  const isEditable = computed(() => ['stopped', 'failed'].includes(props.session.lifecycle_state))
</script>
