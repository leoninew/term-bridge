<template>
  <li>
    <RouterLink
      :to="to"
      class="flex min-w-0 items-center justify-between gap-2 px-3 py-3 outline-none transition-colors hover:bg-[var(--color-control-hover)] focus-visible:bg-[var(--color-control-hover)] sm:gap-3 sm:px-4 sm:py-3.5"
    >
      <div class="min-w-0 flex-1">
        <slot />
      </div>
      <ArrowRight class="size-5 shrink-0 text-[var(--color-text-subtle)]" aria-hidden="true" />
    </RouterLink>
  </li>
</template>

<script setup lang="ts">
  import { ArrowRight } from '@lucide/vue'
  import { RouterLink, type RouteLocationRaw } from 'vue-router'

  defineProps<{
    to: RouteLocationRaw
  }>()
</script>
