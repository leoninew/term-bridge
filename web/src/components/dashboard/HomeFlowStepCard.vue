<template>
  <article
    class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-xl sm:p-5"
  >
    <div class="inline-flex max-w-full items-center gap-2">
      <span
        class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-blue-500/10 text-blue-500 sm:size-10"
      >
        <component :is="icon" class="size-4 sm:size-5" />
      </span>
      <div class="min-w-0">
        <h2 class="text-sm font-semibold text-[var(--color-text-strong)] sm:text-base">
          {{ title }}
        </h2>
        <p class="mt-1.5 text-sm leading-6 text-[var(--color-text-muted)]">
          {{ copy }}
        </p>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
  import type { Component } from 'vue'

  defineProps<{
    icon: Component
    title: string
    copy: string
  }>()
</script>
