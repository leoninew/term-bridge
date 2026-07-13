<template>
  <ToastProvider>
    <RouterView />
    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { ToastProvider } from 'reka-ui'
  import { RouterView } from 'vue-router'
  import ToastHost from './components/session/ToastHost.vue'
</script>
