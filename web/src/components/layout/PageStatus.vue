<template>
  <div class="page-status-root">
    <div v-if="loading" class="page-status page-status-loading" role="status">
      <slot name="loading">{{ loadingText }}</slot>
    </div>
    <div
      v-else-if="error"
      class="page-status page-status-error text-[var(--color-danger-text)]"
      role="alert"
    >
      <slot name="error">{{ error }}</slot>
    </div>
    <div v-else-if="empty" class="page-status page-status-empty" role="status">
      <slot name="empty">{{ emptyText }}</slot>
    </div>
    <slot v-else />
  </div>
</template>

<script setup lang="ts">
  withDefaults(
    defineProps<{
      loading?: boolean
      error?: string | null
      empty?: boolean
      loadingText?: string
      emptyText?: string
    }>(),
    {
      loading: false,
      error: null,
      empty: false,
      loadingText: '',
      emptyText: '',
    },
  )
</script>
