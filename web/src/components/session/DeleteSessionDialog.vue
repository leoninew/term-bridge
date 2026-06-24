<template>
  <AlertDialogRoot :open="open" @update:open="emit('update:open', $event)">
    <AlertDialogPortal>
      <AlertDialogOverlay class="dialog-overlay" />
      <AlertDialogContent class="dialog-content">
        <AlertDialogTitle class="dialog-title">{{
          t('dialog.deleteSessionTitle')
        }}</AlertDialogTitle>
        <AlertDialogDescription class="dialog-description">
          <template v-if="session && session.lifecycle_state === 'running'">
            {{ t('dialog.deleteActiveSessionDescription') }}
          </template>
          <template v-else>
            {{
              t('dialog.deleteSessionDescription', {
                name: session ? session.name : t('dialog.fallbackSession'),
              })
            }}
          </template>
        </AlertDialogDescription>
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
              :disabled="!session || session.lifecycle_state === 'running' || deleting"
              @click="emit('confirm')"
            >
              {{ deleting ? t('common.deleting') : t('common.delete') }}
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
  import type { SessionSummary } from '../../protocol/terminal'

  defineProps<{
    open: boolean
    session: SessionSummary | null
    deleting: boolean
  }>()

  const emit = defineEmits<{
    'update:open': [open: boolean]
    confirm: []
  }>()

  const { t } = useI18n()
</script>
