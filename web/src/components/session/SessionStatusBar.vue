<template>
  <footer
    class="flex h-8 shrink-0 items-center gap-1.5 overflow-hidden border-t border-[var(--color-border)] bg-[var(--color-panel-header)] px-3 text-sm text-[var(--color-text-muted)]"
  >
    <template v-if="session">
      <span>{{ t('workbench.status') }}</span>
      <span class="text-[var(--color-text)]">{{ session.lifecycle_state }}</span>
      <span class="text-[var(--color-border-strong)]">·</span>
      <span>{{ t('workbench.command') }}</span>
      <span class="min-w-0 truncate text-[var(--color-text)]">{{ session.command }}</span>
    </template>
    <span v-else>{{ t('workbench.noActiveSession') }}</span>
    <span v-if="deviceLabel" class="ml-auto min-w-0 truncate text-[var(--color-text)]">
      {{ deviceLabel }}
    </span>
  </footer>
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import type { SessionSummary } from '../../gen/proto/termbridge/agent/v1/workspace'
  import type { CloudSessionSummary } from '../../gen/proto/termbridge/cloud/v1/session'
  import type { DeviceSummary } from '../../gen/proto/termbridge/cloud/v1/device'

  const props = defineProps<{
    session: SessionSummary | null
    device: DeviceSummary | CloudSessionSummary | null
  }>()

  const { t } = useI18n()
  const deviceLabel = computed(() => {
    if (!props.device) {
      return ''
    }
    return 'name' in props.device ? props.device.name : props.device.device_name
  })
</script>
