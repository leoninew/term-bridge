<template>
  <footer
    class="flex h-8 shrink-0 items-center gap-1.5 overflow-hidden border-t border-[var(--color-border)] bg-[var(--color-panel-header)] px-3 text-sm text-[var(--color-text-muted)]"
  >
    <template v-if="session">
      <span>{{ t('workbench.status') }}</span>
      <span :class="lifecycleStateClassName(session.lifecycle_state)">
        {{ session.lifecycle_state }}
      </span>
      <SessionSourceIcon
        :command-source="session.command_source"
        size-class="size-3.5"
        :label="commandSourceLabel"
      />
      <span
        v-if="session.command_source === 'shortcut'"
        class="min-w-0 truncate text-[var(--color-text)]"
      >
        {{ session.shortcut_name_snapshot }}
      </span>
      <span v-else class="min-w-0 truncate font-mono text-xs text-[var(--color-text)]">
        {{ session.command }}
      </span>
    </template>
    <span v-else>{{ t('workbench.noActiveSession') }}</span>

    <div class="ml-auto flex min-w-0 shrink-0 items-center gap-3">
      <span v-if="deviceLabel" class="min-w-0 truncate text-[var(--color-text)]">
        {{ deviceLabel }}
      </span>
      <span
        v-if="showCloudConnection"
        class="inline-flex shrink-0 items-center gap-1.5"
        :title="cloudConnectionLabel"
      >
        <span
          class="size-2 shrink-0 rounded-full"
          :class="cloudConnected ? 'bg-green-500' : 'bg-[var(--color-text-subtle)]'"
        />
        <span class="truncate text-[var(--color-text)]">{{ cloudConnectionLabel }}</span>
      </span>
    </div>
  </footer>
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { lifecycleStateClassName } from '../../features/sessions/lifecycleState'
  import { useCloudSessionStore } from '../../store/cloudSession'
  import SessionSourceIcon from './SessionSourceIcon.vue'
  import type { SessionSummary } from '../../gen/proto/termbridge/agent/v1/workspace'
  import type { CloudSessionSummary } from '../../gen/proto/termbridge/cloud/v1/session'
  import type { DeviceSummary } from '../../gen/proto/termbridge/cloud/v1/device'

  const props = withDefaults(
    defineProps<{
      session: SessionSummary | null
      device: DeviceSummary | CloudSessionSummary | null
      showCloudConnection?: boolean
    }>(),
    {
      showCloudConnection: false,
    },
  )

  const { t } = useI18n()
  const cloudSession = useCloudSessionStore()

  const commandSourceLabel = computed(() =>
    props.session?.command_source === 'shortcut'
      ? t('dialog.shortcut')
      : t('workbench.launchCommand'),
  )
  const deviceLabel = computed(() => {
    if (!props.device) {
      return ''
    }
    return 'name' in props.device ? props.device.name : props.device.device_name
  })
  const cloudConnected = computed(() => !!cloudSession.cloudSession)
  const cloudConnectionLabel = computed(() =>
    cloudConnected.value
      ? t('dashboard.localCloudConnected')
      : t('dashboard.localCloudDisconnected'),
  )
</script>
