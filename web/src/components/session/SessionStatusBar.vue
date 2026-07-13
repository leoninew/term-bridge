<template>
  <footer
    class="flex h-8 shrink-0 items-center gap-1.5 overflow-hidden border-t border-[var(--color-border)] bg-[var(--color-panel-header)] px-3 text-sm text-[var(--color-text-muted)]"
  >
    <template v-if="session">
      <span>{{ t('workbench.status') }}</span>
      <span :class="lifecycleStateClassName(session.lifecycle_state)">{{
        session.lifecycle_state
      }}</span>
      <span class="text-[var(--color-border-strong)]">·</span>
      <span>{{ commandSourceLabel }}</span>
      <span
        v-if="session.command_source === 'shortcut'"
        class="min-w-0 truncate text-[var(--color-text)]"
      >
        {{ session.shortcut_name_snapshot }}
      </span>
      <span v-if="session.command_source === 'shortcut'" class="text-[var(--color-border-strong)]"
        >·</span
      >
      <span
        v-if="session.command_source !== 'shortcut'"
        class="min-w-0 truncate font-mono text-xs text-[var(--color-text)]"
      >
        {{ session.command }}
      </span>
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
  import { lifecycleStateClassName } from '../../features/sessions/lifecycleState'
  import type { SessionSummary } from '../../gen/proto/termbridge/agent/v1/workspace'
  import type { CloudSessionSummary } from '../../gen/proto/termbridge/cloud/v1/session'
  import type { DeviceSummary } from '../../gen/proto/termbridge/cloud/v1/device'

  const props = defineProps<{
    session: SessionSummary | null
    device: DeviceSummary | CloudSessionSummary | null
  }>()

  const { t } = useI18n()
  const commandSourceLabel = computed(() => {
    if (!props.session) return ''
    if (props.session.command_source === 'shortcut') return t('dialog.shortcut')
    if (props.session.command_source === 'command') return t('dialog.directCommand')
    return t('workbench.launchCommand')
  })
  const deviceLabel = computed(() => {
    if (!props.device) {
      return ''
    }
    return 'name' in props.device ? props.device.name : props.device.device_name
  })
</script>
