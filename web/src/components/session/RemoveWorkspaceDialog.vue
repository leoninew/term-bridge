<template>
  <AlertDialogRoot :open="open" @update:open="emit('update:open', $event)">
    <AlertDialogPortal>
      <AlertDialogOverlay class="dialog-overlay" />
      <AlertDialogContent class="dialog-content">
        <div class="dialog-header">
          <AlertDialogTitle class="dialog-title">{{
            t('dialog.removeWorkspaceTitle')
          }}</AlertDialogTitle>
          <AlertDialogDescription class="dialog-description">
            <template v-if="workspace">
              {{ t('dialog.removeWorkspaceDescription', { name: workspace.name }) }}
            </template>
            <template v-else>
              {{ t('dialog.removeWorkspaceDescription', { name: t('dialog.fallbackSession') }) }}
            </template>
          </AlertDialogDescription>
        </div>
        <div class="dialog-actions">
          <AlertDialogCancel as-child>
            <button type="button" class="button button-secondary">
              {{ t('common.cancel') }}
            </button>
          </AlertDialogCancel>
          <AlertDialogAction as-child>
            <button
              type="button"
              class="button button-danger"
              :disabled="!workspace || removing"
              @click="emit('confirm')"
            >
              {{ removing ? t('common.removing') : t('common.removeWorkspace') }}
            </button>
          </AlertDialogAction>
        </div>
      </AlertDialogContent>
    </AlertDialogPortal>
  </AlertDialogRoot>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import {
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogOverlay,
    AlertDialogPortal,
    AlertDialogRoot,
    AlertDialogTitle,
  } from 'reka-ui'
  import type { Workspace as WorkspaceSummary } from '../../gen/proto/termbridge/agent/v1/workspace'

  defineProps<{
    open: boolean
    workspace: WorkspaceSummary | null
    removing: boolean
  }>()

  const emit = defineEmits<{
    'update:open': [open: boolean]
    confirm: []
  }>()

  const { t } = useI18n()
</script>
