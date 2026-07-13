<template>
  <LocalHome v-if="runtimeConfig.view.mode === 'local'" />
  <CloudHome v-else />
</template>

<script setup lang="ts">
  import LocalHome from '../components/dashboard/LocalHome.vue'
  import CloudHome from '../components/dashboard/CloudHome.vue'
  import { useRuntimeConfigStore } from '../store/runtimeConfig'

  const runtimeConfig = useRuntimeConfigStore()
</script>
