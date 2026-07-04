<template>
  <div
    v-if="appMode.isHybrid"
    class="fixed bottom-4 right-4 z-50 inline-flex rounded-full border border-[var(--color-border)] bg-[var(--color-surface)] p-1 text-xs font-medium text-[var(--color-text)] shadow-xl"
  >
    <button
      type="button"
      class="rounded-full px-3 py-1.5 transition"
      :class="buttonClass('agent')"
      @click="switchMode('agent')"
    >
      Agent
    </button>
    <button
      type="button"
      class="rounded-full px-3 py-1.5 transition"
      :class="buttonClass('cloud')"
      @click="switchMode('cloud')"
    >
      Cloud
    </button>
  </div>
</template>

<script setup lang="ts">
  import { useRouter } from 'vue-router'
  import { useAppModeStore, type ActiveMode } from '../store/appMode'
  import { useGatewayStore } from '../store/gateway'

  const router = useRouter()
  const appMode = useAppModeStore()
  const gateway = useGatewayStore()

  function buttonClass(mode: ActiveMode) {
    return appMode.effectiveMode === mode
      ? 'bg-blue-600 text-white shadow'
      : 'text-[var(--color-text-muted)] hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)]'
  }

  async function switchMode(mode: ActiveMode) {
    if (!appMode.setActiveMode(mode)) {
      return
    }
    await gateway.initializeAuth({ force: true })
    if (mode === 'agent') {
      await router.push({ name: 'agent-dashboard' })
      return
    }
    await router.push(gateway.authenticated ? { name: 'cloud-dashboard' } : { name: 'login' })
  }
</script>
