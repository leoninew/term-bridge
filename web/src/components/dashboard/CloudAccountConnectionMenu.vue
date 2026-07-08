<template>
  <button
    type="button"
    class="inline-flex size-9 items-center justify-center rounded-md outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
    :title="title"
    :aria-label="title"
  >
    <component :is="statusIcon" class="size-4" :class="statusIconClass" />
  </button>
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { Cloud, CloudOff } from '@lucide/vue'
  import type { CloudSessionSummary } from '../../gen/proto/termbridge/cloud/v1/session'

  const props = defineProps<{
    connection: CloudSessionSummary | null
  }>()

  const { t } = useI18n()

  const statusIcon = computed(() => (props.connection ? Cloud : CloudOff))
  const statusIconClass = computed(() =>
    props.connection ? 'text-green-500' : 'text-[var(--color-danger-text)]',
  )

  const connectedAtLabel = computed(() => {
    if (!props.connection?.connected_at) {
      return ''
    }
    const date = new Date(props.connection.connected_at)
    return Number.isNaN(date.getTime()) ? props.connection.connected_at : date.toLocaleString()
  })

  const title = computed(() => {
    if (!props.connection) {
      return t('dashboard.cloudAccountNotConnected')
    }
    const at = connectedAtLabel.value
    return at
      ? `${t('dashboard.cloudAccountConnected')} · ${at}`
      : t('dashboard.cloudAccountConnected')
  })
</script>
