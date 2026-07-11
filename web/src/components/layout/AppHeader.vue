<template>
  <header
    class="flex h-16 items-center justify-between border-b border-[var(--color-border)] bg-[var(--color-panel-header)] px-6"
  >
    <RouterLink to="/" class="flex items-center gap-3">
      <img :src="logoDataUrl" alt="TermBridge" class="size-10 rounded-xl shadow-lg" />
      <p class="text-lg font-semibold text-[var(--color-text-strong)]">TermBridge</p>
    </RouterLink>

    <div class="flex items-center gap-3">
      <DisplayControls />
      <slot name="actions" />
    </div>
  </header>
</template>

<script setup lang="ts">
  import { RouterLink } from 'vue-router'
  import DisplayControls from '../dashboard/DisplayControls.vue'

  const logoDataUrl =
    'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI0MCIgaGVpZ2h0PSI0MCIgdmlld0JveD0iMCAwIDQwIDQwIj48cmVjdCB3aWR0aD0iNDAiIGhlaWdodD0iNDAiIHJ4PSIxMCIgZmlsbD0iIzI1NjNlYiIvPjx0ZXh0IHg9IjIwIiB5PSIyNSIgZm9udC1zaXplPSIxNCIgZm9udC1mYW1pbHk9IkFyaWFsLCBzYW5zLXNlcmlmIiBmaWxsPSJ3aGl0ZSIgZm9udC13ZWlnaHQ9IjcwMCIgdGV4dC1hbmNob3I9Im1pZGRsZSI+VEI8L3RleHQ+PC9zdmc+Cg=='
</script>
