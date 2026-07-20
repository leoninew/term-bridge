<template>
  <li class="flex min-w-0 items-center justify-between gap-3 px-3 py-3 sm:gap-4 sm:px-4 sm:py-3.5">
    <div class="min-w-0 flex-1">
      <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
        <span
          class="size-2 shrink-0 rounded-full"
          :class="device.online ? 'bg-green-500' : 'bg-[var(--color-text-subtle)]'"
        />
        <Monitor class="size-4 shrink-0 text-[var(--color-text-subtle)]" />
        <span class="min-w-0 truncate text-sm text-[var(--color-text-strong)]">
          {{ device.name }}
        </span>
        <span class="shrink-0 text-xs text-[var(--color-text-muted)] sm:text-sm">
          {{ device.online ? t('cloud.online') : t('cloud.offline') }}
        </span>
      </div>
      <p class="mt-1 truncate pl-6 text-xs text-[var(--color-text-muted)] sm:pl-8 sm:text-sm">
        {{ activityLabel }}
      </p>
    </div>

    <div class="flex shrink-0 items-center gap-1">
      <button
        v-if="!device.online"
        type="button"
        :class="listIconDangerActionClass"
        :disabled="deleting"
        :aria-label="t('dashboard.deleteDeviceAria', { name: device.name })"
        :title="t('dashboard.deleteOfflineDevice')"
        @click.stop="emit('delete', device)"
      >
        <Trash2 class="size-4" />
      </button>
      <button
        type="button"
        :class="listIconActionClass"
        :disabled="!device.online"
        :aria-label="t('dashboard.openWorkbench')"
        @click="emit('open', device)"
      >
        <ArrowRight
          class="size-5"
          :class="
            device.online ? 'text-[var(--color-text-subtle)]' : 'text-[var(--color-text-muted)]'
          "
        />
      </button>
    </div>
  </li>
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ArrowRight, Monitor, Trash2 } from '@lucide/vue'
  import type { DeviceSummary } from '../../gen/proto/termbridge/cloud/v1/device'
  import { listIconActionClass, listIconDangerActionClass } from './homeUi'

  const props = defineProps<{
    device: DeviceSummary
    deleting?: boolean
  }>()

  const emit = defineEmits<{
    delete: [device: DeviceSummary]
    open: [device: DeviceSummary]
  }>()

  const { t } = useI18n()

  const activityLabel = computed(() => {
    const device = props.device
    if (device.online && device.connected_at) {
      return t('dashboard.connectedAt', { value: formatTime(device.connected_at) })
    }
    if (device.last_seen) {
      return t('dashboard.lastSeenAt', { value: formatTime(device.last_seen) })
    }
    return t('dashboard.noActivity')
  })

  function formatTime(value: string) {
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
  }
</script>
