<template>
  <ToastProvider>
    <AgentHome v-if="runtimeConfig.view.mode === 'agent'" />
    <CloudHome v-else />
    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { ToastProvider } from 'reka-ui'
  import AgentHome from '../components/dashboard/AgentHome.vue'
  import CloudHome from '../components/dashboard/CloudHome.vue'
  import ToastHost from '../components/session/ToastHost.vue'
  import { useRuntimeConfigStore } from '../store/runtimeConfig'

  const runtimeConfig = useRuntimeConfigStore()
</script>
