<template>
  <footer
    class="flex h-8 shrink-0 items-center gap-2 overflow-hidden border-t border-[var(--color-border)] bg-[var(--color-panel-header)] px-2.5 text-xs leading-4 text-[var(--color-text-muted)]"
  >
    <template v-if="session">
      <div class="flex min-w-0 items-center gap-1">
        <span class="shrink-0">{{ t('dialog.cwd') }}</span>
        <button
          type="button"
          class="max-w-full truncate rounded px-0.5 text-left text-[var(--color-text)] outline-none hover:bg-[var(--color-control-hover)] focus-visible:bg-[var(--color-control-hover)] max-sm:max-w-[12rem]"
          :title="cwdCopyTitle"
          :aria-label="cwdCopyTitle"
          @click="copyCwd"
        >
          {{ cwdDisplayName }}
        </button>
      </div>

      <span class="shrink-0 text-[var(--color-border-strong)]" aria-hidden="true">|</span>

      <div class="flex min-w-0 items-center gap-1">
        <span class="shrink-0">{{ t('dialog.commandSource') }}</span>
        <LaunchMethodIcon
          :command-source="session.command_source"
          size-class="size-3.5 text-[var(--color-text)]"
          :label="launchMethodLabel"
        />
        <span
          v-if="isShortcutLaunch"
          class="min-w-0 truncate text-[var(--color-text)]"
          :title="session.shortcut_name_snapshot"
        >
          {{ session.shortcut_name_snapshot }}
        </span>
        <span
          v-else
          class="min-w-0 truncate font-mono text-[var(--color-text)]"
          :title="session.command"
        >
          {{ session.command }}
        </span>
      </div>

      <span class="shrink-0 text-[var(--color-border-strong)]" aria-hidden="true">|</span>

      <div class="flex min-w-0 items-center gap-1">
        <span class="shrink-0">{{ t('workbench.status') }}</span>
        <span :class="lifecycleStateClassName(session.lifecycle_state)">
          {{ session.lifecycle_state }}
        </span>
      </div>
    </template>
    <span v-else>{{ t('workbench.noActiveSession') }}</span>

    <div class="ml-auto flex min-w-0 shrink-0 items-center gap-2">
      <span v-if="deviceLabel" class="min-w-0 truncate text-[var(--color-text)]">
        {{ deviceLabel }}
      </span>
      <span
        v-if="deviceLabel && showCloudConnection"
        class="shrink-0 text-[var(--color-border-strong)]"
        aria-hidden="true"
        >|</span
      >
      <span
        v-if="showCloudConnection"
        class="inline-flex shrink-0 items-center gap-1"
        :title="cloudConnectionLabel"
      >
        <span
          class="size-1.5 shrink-0 rounded-full"
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
  import { useNotificationsStore } from '../../store/notifications'
  import LaunchMethodIcon from './LaunchMethodIcon.vue'
  import { isShortcutLaunchMethod, launchMethodLabelKey } from './launchMethod'
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
  const notifications = useNotificationsStore()

  const launchMethodLabel = computed(() =>
    props.session ? t(launchMethodLabelKey(props.session.command_source)) : '',
  )
  const isShortcutLaunch = computed(() => isShortcutLaunchMethod(props.session?.command_source))

  const cwdFullPath = computed(() => props.session?.cwd?.trim() ?? '')
  const cwdDisplayName = computed(() => pathBaseName(cwdFullPath.value))
  const cwdCopyTitle = computed(() => {
    if (!cwdFullPath.value) {
      return t('workbench.copyCwd')
    }
    return `${cwdFullPath.value}
${t('workbench.copyCwd')}`
  })

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

  async function copyCwd() {
    const path = cwdFullPath.value
    if (!path) {
      return
    }
    try {
      await window.navigator.clipboard.writeText(path)
      notifications.pushToast('success', t('toast.cwdCopied'), path)
    } catch (err) {
      notifications.notifyError(t('toast.copyCwdFailed'), err)
    }
  }

  function pathBaseName(path: string): string {
    const backslash = String.fromCharCode(92)
    const trimmed = path.trim()
    if (!trimmed) {
      return '.'
    }
    let core = trimmed
    while (core.endsWith('/') || core.endsWith(backslash)) {
      core = core.slice(0, -1)
    }
    if (!core) {
      return trimmed
    }
    const sep = Math.max(core.lastIndexOf('/'), core.lastIndexOf(backslash))
    return sep >= 0 ? core.slice(sep + 1) : core
  }
</script>
