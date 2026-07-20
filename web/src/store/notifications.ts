import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ToastKind = 'success' | 'error' | 'info'

export type AppToast = {
  id: number
  kind: ToastKind
  title: string
  description?: string
}

export function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

export const useNotificationsStore = defineStore('notifications', () => {
  const toasts = ref<AppToast[]>([])
  let toastId = 0

  function pushToast(kind: ToastKind, title: string, description?: string) {
    toasts.value.push({ id: ++toastId, kind, title, description })
  }

  function notifyError(title: string, err: unknown) {
    pushToast('error', title, errorMessage(err))
  }

  function dismissToast(id: number) {
    toasts.value = toasts.value.filter((toast) => toast.id !== id)
  }

  return {
    toasts,
    pushToast,
    notifyError,
    dismissToast,
  }
})
