<template>
  <AlertDialogRoot :open="open" @update:open="emit('update:open', $event)">
    <AlertDialogPortal>
      <AlertDialogOverlay class="dialog-overlay" />
      <AlertDialogContent class="dialog-content shortcut-delete-dialog">
        <div class="dialog-header">
          <AlertDialogTitle class="dialog-title">{{ t('shortcut.deleteTitle') }}</AlertDialogTitle>
          <AlertDialogDescription class="dialog-description">
            {{ t('shortcut.deleteDescription', { name: shortcut?.name ?? '' }) }}
          </AlertDialogDescription>
        </div>

        <div class="dialog-actions">
          <AlertDialogCancel class="button button-secondary" :disabled="deleting">
            {{ t('common.cancel') }}
          </AlertDialogCancel>
          <AlertDialogAction class="button button-danger" :disabled="deleting" @click="emit('confirm')">
            {{ deleting ? t('common.deleting') : t('common.delete') }}
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
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'

  defineProps<{ open: boolean; shortcut: Shortcut | null; deleting: boolean }>()
  const emit = defineEmits<{ 'update:open': [open: boolean]; confirm: [] }>()
  const { t } = useI18n()
</script>
