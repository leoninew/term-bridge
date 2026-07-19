<template>
  <AlertDialogRoot :open="open" @update:open="emit('update:open', $event)">
    <AlertDialogPortal>
      <AlertDialogOverlay class="dialog-overlay" />
      <AlertDialogContent class="dialog-content w-[min(448px,calc(100vw-32px))]">
        <div class="dialog-header">
          <AlertDialogTitle class="dialog-title">
            {{ t('shortcut.batchDeleteTitle') }}
          </AlertDialogTitle>
          <AlertDialogDescription class="dialog-description">
            {{ t('shortcut.batchDeleteDescription', { count }) }}
          </AlertDialogDescription>
        </div>

        <div class="dialog-actions">
          <AlertDialogCancel class="button button-secondary" :disabled="deleting">
            {{ t('common.cancel') }}
          </AlertDialogCancel>
          <AlertDialogAction
            class="button button-danger"
            :disabled="deleting || count === 0"
            @click="emit('confirm')"
          >
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

  defineProps<{ open: boolean; count: number; deleting: boolean }>()
  const emit = defineEmits<{ 'update:open': [open: boolean]; confirm: [] }>()
  const { t } = useI18n()
</script>
