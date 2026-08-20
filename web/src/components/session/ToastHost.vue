<template>
  <ToastRoot
    v-for="toast in notifications.toasts"
    :key="toast.id"
    class="relative rounded-[10px] border border-[var(--color-border-strong)] bg-[var(--color-surface)] p-3.5 pr-10 text-sm text-[var(--color-text)] shadow-[0_8px_24px_rgba(0,0,0,0.12)]"
    :class="{
      'border-[#16a34a]': toast.kind === 'success',
      'border-[var(--color-danger-border)]': toast.kind === 'error',
      'border-[var(--color-primary-border)]': toast.kind === 'info',
    }"
    :duration="toast.durationMs ?? 5000"
    @update:open="(open) => !open && notifications.dismissToast(toast.id)"
  >
    <ToastTitle class="text-sm font-semibold leading-snug text-[var(--color-text-strong)]">
      {{ toast.title }}
    </ToastTitle>
    <ToastDescription
      v-if="toast.description"
      class="mt-1 leading-normal text-[var(--color-text-muted)]"
    >
      {{ toast.description }}
    </ToastDescription>
    <ToastClose
      class="absolute right-2.5 top-2.5 inline-flex size-[18px] min-h-[18px] min-w-[18px] cursor-pointer items-center justify-center rounded-sm border-0 bg-transparent p-0 text-[var(--color-text-muted)] outline-none hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)] focus-visible:bg-[var(--color-control-hover)] focus-visible:text-[var(--color-text)] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-[var(--color-primary-border)]"
      :aria-label="t('common.close')"
    >
      <X class="size-3" aria-hidden="true" />
    </ToastClose>
  </ToastRoot>
  <ToastViewport
    class="fixed bottom-4 right-4 z-[60] m-0 flex w-[360px] max-w-[calc(100vw-32px)] list-none flex-col gap-2.5 p-0"
  />
</template>

<script setup lang="ts">
  import { X } from '@lucide/vue'
  import { useI18n } from 'vue-i18n'
  import { ToastClose, ToastDescription, ToastRoot, ToastTitle, ToastViewport } from 'reka-ui'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const notifications = useNotificationsStore()
</script>
