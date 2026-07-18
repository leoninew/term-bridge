<template>
  <div class="termbridge-code-page">
    <div v-if="error" class="termbridge-code-page-error" role="alert">
      <span>{{ error }}</span>
      <RouterLink class="termbridge-code-page-inline-link" :to="sessionsRoute">
        {{ t('workbench.backToSessions') }}
      </RouterLink>
    </div>
    <div v-else-if="loading" class="termbridge-code-page-loading">
      {{ t('workbench.startingWorkbench') }}
    </div>
    <div ref="hostEl" class="termbridge-code-page-host" />
  </div>
</template>

<script setup lang="ts">
  import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { RouterLink, useRouter } from 'vue-router'
  import type { RouteLocationRaw } from 'vue-router'
  import type { RuntimeTarget } from '../../features/runtimeTarget'
  import { bindWorkbenchAppIconNavigation } from '../../features/workbench/titlebarAppIcon'

  const props = defineProps<{
    runtimeTarget: RuntimeTarget
    workspaceId: string
    sessionsRoute: RouteLocationRaw
  }>()


  const { t } = useI18n()
  const router = useRouter()
  const hostEl = ref<HTMLElement | null>(null)
  const loading = ref(true)
  const error = ref<string | null>(null)
  let resizeObserver: ResizeObserver | null = null
  let unbindAppIcon: (() => void) | null = null
  let disposeMountedWorkbench: (() => void) | null = null
  let previousDocumentTitle: string | null = null

  function notifyLayout() {
    window.dispatchEvent(new window.Event('resize'))
  }

  function wireTitlebarAppIcon() {
    unbindAppIcon?.()
    unbindAppIcon = null
    if (!hostEl.value) {
      return
    }
    // Native titlebar slot: <a class="window-appicon"> — no extra top chrome.
    unbindAppIcon = bindWorkbenchAppIconNavigation(hostEl.value, {
      ariaLabel: t('workbench.backToSessionsAria'),
      onNavigate: () => {
        void router.push(props.sessionsRoute)
      },
    })
  }

  async function mount() {
    if (!hostEl.value || !props.workspaceId) {
      return
    }
    if (previousDocumentTitle === null) {
      previousDocumentTitle = document.title
    }
    loading.value = true
    error.value = null
    try {
      // Keep the route shell tiny; pull the full monaco-vscode graph only on mount.
      const workbench = await import('../../features/workbench/bootstrap')
      disposeMountedWorkbench = workbench.disposeMountedWorkbench
      await workbench.mountCodeWorkbench(hostEl.value, {
        runtimeTarget: props.runtimeTarget,
        workspaceId: props.workspaceId,
      })
      await nextTick()
      wireTitlebarAppIcon()
      notifyLayout()
      // Observe host size so workbench reflows when shell chrome changes.
      resizeObserver?.disconnect()
      resizeObserver = new ResizeObserver(() => notifyLayout())
      resizeObserver.observe(hostEl.value)
    } catch (err) {
      error.value = err instanceof Error ? err.message : String(err)
    } finally {
      loading.value = false
      await nextTick()
      notifyLayout()
    }
  }

  onMounted(() => {
    void mount()
  })

  watch(
    () => [props.workspaceId, props.runtimeTarget] as const,
    () => {
      void mount()
    },
  )

  watch(
    () => props.sessionsRoute,
    () => {
      if (!loading.value && !error.value) {
        wireTitlebarAppIcon()
      }
    },
  )

  onBeforeUnmount(() => {
    unbindAppIcon?.()
    unbindAppIcon = null
    resizeObserver?.disconnect()
    resizeObserver = null
    // Workbench services are process-global; release only this workspace's
    // subscription and disposable hooks when its route unmounts.
    disposeMountedWorkbench?.()
    // Monaco workbench rewrites document.title via window.title; restore pre-route title.
    if (previousDocumentTitle !== null) {
      document.title = previousDocumentTitle
      previousDocumentTitle = null
    }
  })
</script>

<style scoped>
  .termbridge-code-page {
    position: relative;
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100%;
    max-height: 100vh;
    overflow: hidden;
    /* Match VS Code chrome; avoid app theme vars leaking into workbench metrics */
    background: #1e1e1e;
    color: #cccccc;
    font-family: 'Segoe WPC', 'Segoe UI', system-ui, sans-serif;
    font-size: 13px;
    line-height: 1.4;
  }

  .termbridge-code-page-host {
    position: relative;
    flex: 1 1 auto;
    min-height: 0;
    width: 100%;
    height: 100%;
    overflow: hidden;
  }

  .termbridge-code-page-loading,
  .termbridge-code-page-error {
    position: absolute;
    top: 8px;
    left: 8px;
    right: 8px;
    z-index: 15;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.5rem 0.75rem;
    border-radius: 6px;
    padding: 0.5rem 0.75rem;
    font-size: 12px;
    pointer-events: auto;
  }

  .termbridge-code-page-loading {
    background: rgb(37 37 38 / 0.92);
    color: #cccccc;
  }

  .termbridge-code-page-error {
    color: #f48771;
    background: rgb(58 29 29 / 0.95);
  }

  .termbridge-code-page-inline-link {
    color: #3794ff;
    text-decoration: none;
    flex: 0 0 auto;
  }

  .termbridge-code-page-inline-link:hover {
    text-decoration: underline;
  }
</style>
