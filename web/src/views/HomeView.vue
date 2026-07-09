<template>
  <ToastProvider>
    <LocalHome v-if="runtimeConfig.view.mode === 'local'" />
    <CloudHome v-else />
    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { ToastProvider } from 'reka-ui'
  import LocalHome from '../components/dashboard/LocalHome.vue'
  import CloudHome from '../components/dashboard/CloudHome.vue'
  import ToastHost from '../components/session/ToastHost.vue'
  import { useRuntimeConfigStore } from '../store/runtimeConfig'

  const runtimeConfig = useRuntimeConfigStore()
</script>
