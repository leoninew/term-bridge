<template>
  <DrawerRoot :open="open" swipe-direction="right" @update:open="handleOpenChange">
    <DrawerPortal>
      <DrawerOverlay class="dialog-overlay" />
      <DrawerContent
        class="background-sessions-drawer"
        :class="{ 'background-sessions-drawer-closing': closing }"
      >
        <header class="background-sessions-drawer-header">
          <DrawerTitle class="dialog-title">
            {{ t('dialog.closeBackgroundSessionsTitle') }}
          </DrawerTitle>
          <DrawerClose as-child>
            <button
              type="button"
              class="button button-secondary button-icon background-sessions-drawer-close"
              :aria-label="t('common.close')"
              :title="t('common.close')"
              :disabled="closing"
            >
              <X class="size-4" aria-hidden="true" />
            </button>
          </DrawerClose>
        </header>

        <section class="background-sessions-drawer-list">
          <div v-if="runningWorkspaceTree.length > 0" class="background-sessions-tree">
            <section
              v-for="workspace in runningWorkspaceTree"
              :key="workspace.id"
              class="background-sessions-tree-workspace"
            >
              <div class="background-sessions-tree-workspace-node">
                <FolderOpen
                  class="size-4 shrink-0 text-[var(--color-text-subtle)]"
                  aria-hidden="true"
                />
                <span class="min-w-0 flex-1 truncate text-sm font-semibold leading-5">
                  {{ workspace.name }}
                </span>
              </div>

              <ul class="background-sessions-tree-sessions">
                <li
                  v-for="session in workspace.children"
                  :key="sessionIdentityKey(session.workspace_id, session.id)"
                  class="background-sessions-tree-session"
                >
                  <CheckboxRoot
                    :id="checkboxId(session)"
                    :model-value="selectedSessionKeys.has(sessionKey(session))"
                    :disabled="closing"
                    :aria-label="sessionAccessibleLabel(session)"
                    class="background-sessions-checkbox"
                    @update:model-value="updateSelection(session, $event === true)"
                  >
                    <CheckboxIndicator class="background-sessions-checkbox-indicator">
                      <Check class="size-3.5" aria-hidden="true" />
                    </CheckboxIndicator>
                  </CheckboxRoot>
                  <label
                    :for="checkboxId(session)"
                    class="flex min-w-0 flex-1 items-center gap-1.5 cursor-pointer"
                  >
                    <span
                      class="min-w-0 basis-2/5 shrink truncate text-sm leading-5 text-[var(--color-text)]"
                    >
                      {{ sessionLabel(session) }}
                    </span>
                    <span class="background-sessions-tree-session-source">
                      <SessionSourceIcon
                        :command-source="session.command_source"
                        :label="sessionLaunchSource(session).label"
                      />
                      <span
                        class="min-w-0 flex-1 truncate"
                        :class="{ 'font-mono text-xs': session.command_source !== 'shortcut' }"
                      >
                        {{ sessionLaunchSource(session).value }}
                      </span>
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

        <footer class="background-sessions-drawer-actions">
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
  import { Check, FolderOpen, X } from '@lucide/vue'
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
  import SessionSourceIcon from './SessionSourceIcon.vue'
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
      : { label: t('workbench.launchCommand'), value: session.command }
  }

  function sessionAccessibleLabel(session: SessionSummary) {
    const source = sessionLaunchSource(session)
    return [sessionLabel(session), source.label, source.value].filter(Boolean).join(' · ')
  }

  function sessionKey(session: SessionSummary) {
    return sessionIdentityKey(session.workspace_id, session.id)
  }
</script>
