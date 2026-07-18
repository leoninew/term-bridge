<template>
  <ToastRoot
    v-for="toast in notifications.toasts"
    :key="toast.id"
    class="toast-root"
    :class="`toast-${toast.kind}`"
    :duration="5000"
    @update:open="(open) => !open && notifications.dismissToast(toast.id)"
  >
    <ToastTitle class="toast-title">{{ toast.title }}</ToastTitle>
    <ToastDescription v-if="toast.description" class="toast-description">
      {{ toast.description }}
    </ToastDescription>
    <ToastClose class="toast-close" :aria-label="t('common.close')">
      <X class="size-3" aria-hidden="true" />
    </ToastClose>
  </ToastRoot>
  <ToastViewport class="toast-viewport" />
</template>

<script setup lang="ts">
  import { X } from '@lucide/vue'
  import { useI18n } from 'vue-i18n'
  import { ToastClose, ToastDescription, ToastRoot, ToastTitle, ToastViewport } from 'reka-ui'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const notifications = useNotificationsStore()
</script>
