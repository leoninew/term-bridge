<template>
  <div class="termbridge-code-page">
    <header class="termbridge-code-page-header">
      <div class="termbridge-code-page-title">
        <span class="termbridge-code-page-eyebrow">Code</span>
        <strong class="termbridge-code-page-id">{{ workspaceId }}</strong>
      </div>
      <RouterLink class="termbridge-code-page-link" :to="sessionsRoute">Sessions</RouterLink>
    </header>

    <div v-if="error" class="termbridge-code-page-error" role="alert">{{ error }}</div>
    <div v-else-if="loading" class="termbridge-code-page-loading">Starting workbench…</div>
    <div ref="hostEl" class="termbridge-code-page-host" />
  </div>
</template>

<script setup lang="ts">
  import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import { RouterLink } from 'vue-router'
  import type { RouteLocationRaw } from 'vue-router'
  import type { RuntimeTarget } from '../../features/runtimeTarget'
  import { disposeMountedWorkbench, mountCodeWorkbench } from '../../features/workbench/bootstrap'

  const props = defineProps<{
    runtimeTarget: RuntimeTarget
    workspaceId: string
    sessionsRoute: RouteLocationRaw
  }>()

  const hostEl = ref<HTMLElement | null>(null)
  const loading = ref(true)
  const error = ref<string | null>(null)
  let resizeObserver: ResizeObserver | null = null

  function notifyLayout() {
    window.dispatchEvent(new window.Event('resize'))
  }

  async function mount() {
    if (!hostEl.value || !props.workspaceId) {
      return
    }
    loading.value = true
    error.value = null
    try {
      await mountCodeWorkbench(hostEl.value, {
        runtimeTarget: props.runtimeTarget,
        workspaceId: props.workspaceId,
      })
      await nextTick()
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

  onBeforeUnmount(() => {
    resizeObserver?.disconnect()
    resizeObserver = null
    // Workbench services are process-global; release only this workspace's
    // subscription and disposable hooks when its route unmounts.
    disposeMountedWorkbench()
  })
</script>

<style scoped>
  .termbridge-code-page {
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

  .termbridge-code-page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    height: 28px;
    padding: 0 0.6rem;
    border-bottom: 1px solid #2b2b2b;
    background: #252526;
    flex: 0 0 auto;
  }

  .termbridge-code-page-title {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-width: 0;
  }

  .termbridge-code-page-eyebrow {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: #9d9d9d;
    flex: 0 0 auto;
  }

  .termbridge-code-page-id {
    font-size: 12px;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .termbridge-code-page-link {
    color: #3794ff;
    text-decoration: none;
    font-size: 12px;
    flex: 0 0 auto;
  }

  .termbridge-code-page-link:hover {
    text-decoration: underline;
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
    padding: 0.5rem 0.75rem;
    font-size: 12px;
    flex: 0 0 auto;
  }

  .termbridge-code-page-error {
    color: #f48771;
    background: #3a1d1d;
  }
</style>
