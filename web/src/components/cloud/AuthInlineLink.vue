<template>
  <RouterLink :to="to" :class="[authInlineLinkClass, linkClass]">
    <slot />
  </RouterLink>
</template>

<script setup lang="ts">
  import { RouterLink, type RouteLocationRaw } from 'vue-router'
  import { authInlineLinkClass } from './authUi'

  withDefaults(
    defineProps<{
      to: RouteLocationRaw
      linkClass?: string
    }>(),
    { linkClass: '' },
  )
</script>
