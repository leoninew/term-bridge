<template>
  <DrawerRoot :open="open" swipe-direction="right" @update:open="handleOpenChange">
    <DrawerPortal>
      <DrawerOverlay class="dialog-overlay" />
      <DrawerContent
        class="background-sessions-drawer fixed inset-y-0 right-0 z-50 flex w-[min(680px,calc(100vw-32px))] flex-col border-l border-[var(--color-border)] bg-[var(--color-surface)] text-sm leading-normal text-[var(--color-text)] shadow-[-20px_0_48px_rgb(0_0_0/0.28)]"
        :class="{ 'background-sessions-drawer-closing': closing }"
      >
        <header class="flex items-center justify-between gap-3 border-b border-[var(--color-border)] p-5">
          <DrawerTitle class="dialog-title">
            {{ t('dialog.closeBackgroundSessionsTitle') }}
          </DrawerTitle>
          <DrawerClose as-child>
            <button
              type="button"
              class="button button-secondary button-icon !min-h-[30px] !w-[30px] !min-w-[30px] !border-transparent !bg-transparent hover:!border-[var(--color-border)] hover:!bg-[var(--color-control-hover)]"
              :aria-label="t('common.close')"
              :title="t('common.close')"
              :disabled="closing"
            >
              <X class="size-4" aria-hidden="true" />
            </button>
          </DrawerClose>
        </header>

        <section class="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          <div v-if="runningWorkspaceTree.length > 0" class="flex flex-col gap-2">
            <section
              v-for="workspace in runningWorkspaceTree"
              :key="workspace.id"
              class="min-w-0"
            >
              <div class="flex h-7 min-w-0 items-center gap-1 px-1 py-0.5 text-[var(--color-text)]">
                <FolderOpen
                  class="size-4 shrink-0 text-[var(--color-text-subtle)]"
                  aria-hidden="true"
                />
                <span class="min-w-0 flex-1 truncate text-sm font-semibold leading-5">
                  {{ workspace.name }}
                </span>
              </div>

              <ul class="flex flex-col">
                <li
                  v-for="session in workspace.children"
                  :key="sessionIdentityKey(session.workspace_id, session.id)"
                  class="flex h-7 min-w-0 items-center gap-1.5 rounded-md border border-transparent py-0.5 pl-6 pr-1 text-[var(--color-text-muted)] transition-colors hover:border-[var(--color-border)] hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)]"
                >
                  <CheckboxRoot
                    :id="checkboxId(session)"
                    :model-value="selectedSessionKeys.has(sessionKey(session))"
                    :disabled="closing"
                    :aria-label="sessionAccessibleLabel(session)"
                    class="inline-flex size-[18px] shrink-0 items-center justify-center rounded border border-[var(--color-border-strong)] bg-[var(--color-control-bg)] text-[var(--color-primary-text)] outline-none focus-visible:shadow-[0_0_0_3px_var(--color-surface-muted)] data-[state=checked]:border-[var(--color-primary-border)] data-[state=checked]:bg-[var(--color-primary-bg)]"
                    @update:model-value="updateSelection(session, $event === true)"
                  >
                    <CheckboxIndicator class="inline-flex items-center justify-center">
                      <Check class="size-3.5" aria-hidden="true" />
                    </CheckboxIndicator>
                  </CheckboxRoot>
                  <label
                    :for="checkboxId(session)"
                    class="flex min-w-0 flex-1 items-center gap-1.5 cursor-pointer"
                  >
                    <SquareTerminal
                      class="size-4 shrink-0 text-[var(--color-text-subtle)]"
                      aria-hidden="true"
                    />
                    <span
                      class="min-w-0 basis-2/5 shrink truncate text-sm leading-5 text-[var(--color-text)]"
                    >
                      {{ sessionLabel(session) }}
                    </span>
                    <span
                      class="min-w-0 flex-[1_1_60%] truncate text-xs leading-4 text-[var(--color-text-muted)]"
                      :class="{ 'font-mono text-xs': session.command_source !== 'shortcut' }"
                    >
                      {{ sessionLaunchSource(session).value }}
                    </span>
                  </label>
                </li>
              </ul>
            </section>
          </div>
          <p v-else class="py-2 text-sm leading-5 text-[var(--color-text-muted)]">
            {{ t('dialog.noBackgroundSessions') }}
          </p>
        </section>

        <footer class="flex justify-end gap-2 border-t border-[var(--color-border)] px-5 py-4">
          <DrawerClose as-child>
            <button type="button" class="button button-secondary" :disabled="closing">
              {{ t('common.cancel') }}
            </button>
          </DrawerClose>
          <button
            type="button"
            class="button button-primary"
            :disabled="closing || selectedSessions.length === 0"
            @click="emit('confirm', selectedSessions)"
          >
            {{ closing ? t('common.closing') : t('common.confirm') }}
          </button>
        </footer>
      </DrawerContent>
    </DrawerPortal>
  </DrawerRoot>
</template>

<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { Check, FolderOpen, SquareTerminal, X } from '@lucide/vue'
  import {
    CheckboxIndicator,
    CheckboxRoot,
    DrawerClose,
    DrawerContent,
    DrawerOverlay,
    DrawerPortal,
    DrawerRoot,
    DrawerTitle,
  } from 'reka-ui'
    import {
    defaultBackgroundSessionSelectionKeys,
    sessionIdentityKey,
    type SessionIdentity,
  } from '../../features/sessions/tabManagement'
  import type {
    SessionSummary,
    WorkspaceTreeNode,
  } from '../../gen/proto/termbridge/agent/v1/workspace'
  import type { OpenSessionTab } from '../../store/workbench'

  const props = defineProps<{
    open: boolean
    workspaceTree: WorkspaceTreeNode[]
    openedTabs: OpenSessionTab[]
    closing: boolean
  }>()

  const emit = defineEmits<{
    'update:open': [open: boolean]
    confirm: [sessions: SessionIdentity[]]
  }>()

  const { t } = useI18n()
  const selectedSessionKeys = ref(new Set<string>())
  const runningWorkspaceTree = computed(() =>
    props.workspaceTree.flatMap((workspace) => {
      const children = workspace.children.filter((session) => session.lifecycle_state === 'running')
      return children.length > 0 ? [{ ...workspace, children }] : []
    }),
  )
  const sessions = computed(() =>
    runningWorkspaceTree.value.flatMap((workspace) => workspace.children),
  )
  const selectedSessions = computed(() =>
    sessions.value
      .filter((session) => selectedSessionKeys.value.has(sessionKey(session)))
      .map((session) => ({ workspaceId: session.workspace_id, sessionId: session.id })),
  )

  watch(
    () => props.open,
    (open, wasOpen) => {
      if (open && !wasOpen) {
        selectedSessionKeys.value = defaultBackgroundSessionSelectionKeys(
          props.workspaceTree,
          props.openedTabs,
        )
      }
    },
  )

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen && props.closing) {
      return
    }
    emit('update:open', nextOpen)
  }

  function updateSelection(session: SessionSummary, selected: boolean) {
    if (props.closing) {
      return
    }
    const key = sessionKey(session)
    const nextSelectedSessionKeys = new Set(selectedSessionKeys.value)
    if (selected) {
      nextSelectedSessionKeys.add(key)
    } else {
      nextSelectedSessionKeys.delete(key)
    }
    selectedSessionKeys.value = nextSelectedSessionKeys
  }

  function checkboxId(session: SessionSummary) {
    return `background-session-${sessions.value.indexOf(session)}`
  }

  function sessionLabel(session: SessionSummary) {
    return session.name || sessionLaunchSource(session).value
  }

  function sessionLaunchSource(session: SessionSummary) {
    return session.command_source === 'shortcut'
      ? { label: t('dialog.shortcut'), value: session.shortcut_name_snapshot }
      : { label: t('dialog.directCommand'), value: session.command }
  }

  function sessionAccessibleLabel(session: SessionSummary) {
    const source = sessionLaunchSource(session)
    return [sessionLabel(session), source.label, source.value].filter(Boolean).join(' · ')
  }

  function sessionKey(session: SessionSummary) {
    return sessionIdentityKey(session.workspace_id, session.id)
  }
</script>
