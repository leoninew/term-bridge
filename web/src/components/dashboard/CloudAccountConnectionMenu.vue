<template>
  <DropdownMenuRoot>
    <DropdownMenuTrigger
      class="inline-flex h-9 items-center justify-center rounded-md border px-3 text-sm outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
      :class="
        connection
          ? 'border-green-500/30 bg-green-500/10 text-green-600'
          : 'border-[var(--color-border)] bg-[var(--color-control-bg)] text-[var(--color-text)]'
      "
      :aria-label="statusLabel"
      :title="statusLabel"
    >
      {{ statusLabel }}
    </DropdownMenuTrigger>
    <DropdownMenuPortal>
      <DropdownMenuContent
        side="bottom"
        align="end"
        :side-offset="8"
        class="z-50 min-w-72 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] p-2 text-sm text-[var(--color-text)] shadow-xl"
      >
        <div class="px-2 py-1.5">
          <p class="font-medium text-[var(--color-text-strong)]">
            {{ t('dashboard.cloudConnectionDetails') }}
          </p>
          <p class="mt-1 text-xs text-[var(--color-text-muted)]">
            {{
              connection
                ? t('dashboard.cloudConnectionConnectedDescription')
                : t('dashboard.cloudConnectionNotConnectedDescription')
            }}
          </p>
        </div>

        <div v-if="connection" class="space-y-2 px-2 py-2 text-xs text-[var(--color-text-muted)]">
          <div>
            <p class="font-medium text-[var(--color-text)]">
              {{ t('dashboard.cloudConnectionGate') }}
            </p>
            <p class="mt-0.5 break-all">{{ connection.gate_url }}</p>
          </div>
          <div>
            <p class="font-medium text-[var(--color-text)]">
              {{ t('dashboard.cloudConnectionDevice') }}
            </p>
            <p class="mt-0.5">{{ connection.device_name || connection.device_id }}</p>
          </div>
          <div>
            <p class="font-medium text-[var(--color-text)]">
              {{ t('dashboard.cloudConnectionConnectedAt') }}
            </p>
            <p class="mt-0.5">{{ connectedAtLabel }}</p>
          </div>
        </div>

        <DropdownMenuItem
          v-if="cloudConnectEnabled"
          class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
          @select="emit('connect')"
        >
          {{
            connection ? t('dashboard.reconnectCloudAccount') : t('dashboard.connectCloudAccount')
          }}
        </DropdownMenuItem>
        <div v-else class="px-2 py-1.5 text-xs text-[var(--color-text-muted)]">
          {{ t('dashboard.cloudGateNotConfigured') }}
        </div>
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import {
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuPortal,
    DropdownMenuRoot,
    DropdownMenuTrigger,
  } from 'reka-ui'
  import type { CloudSessionSummary } from '../../features/gateway/api'

  const props = defineProps<{
    connection: CloudSessionSummary | null
    cloudConnectEnabled: boolean
  }>()

  const emit = defineEmits<{
    connect: []
  }>()

  const { t } = useI18n()

  const statusLabel = computed(() =>
    props.connection
      ? t('dashboard.cloudAccountConnected')
      : t('dashboard.cloudAccountNotConnected'),
  )

  const connectedAtLabel = computed(() => {
    if (!props.connection?.connected_at) {
      return t('dashboard.noActivity')
    }
    const date = new Date(props.connection.connected_at)
    return Number.isNaN(date.getTime()) ? props.connection.connected_at : date.toLocaleString()
  })
</script>
