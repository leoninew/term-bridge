<template>
  <RouterView />
  <ModeSwitcher />
</template>

<script setup lang="ts">
  import { RouterView } from 'vue-router'
  import ModeSwitcher from './components/ModeSwitcher.vue'
</script>
