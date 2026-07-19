<template>
  <section
    class="overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl"
  >
    <div
      class="flex flex-col gap-2 border-b border-[var(--color-border)] px-3 py-3 sm:flex-row sm:items-center sm:justify-between sm:gap-3 sm:px-4 sm:py-3.5"
    >
      <component
        :is="heading"
        class="text-base font-semibold text-[var(--color-text-strong)] sm:text-lg"
      >
        {{ title }}
      </component>
      <slot name="action" />
    </div>

    <PageStatus
      class="min-w-0"
      :loading="loading"
      :error="error"
      :empty="empty"
      :loading-text="loadingText"
      :empty-text="emptyText"
    >
      <template #loading>
        <div class="px-3 py-4 text-sm text-[var(--color-text-muted)] sm:px-4 sm:py-5">
          {{ loadingText }}
        </div>
      </template>
      <template #error>
        <div class="px-3 py-4 text-sm text-[var(--color-danger-text)] sm:px-4 sm:py-5">
          {{ error }}
        </div>
      </template>
      <template #empty>
        <div class="px-3 py-4 text-sm text-[var(--color-text-muted)] sm:px-4 sm:py-5">
          {{ emptyText }}
        </div>
      </template>
      <slot />
    </PageStatus>
  </section>
</template>

<script setup lang="ts">
  import PageStatus from '../layout/PageStatus.vue'

  withDefaults(
    defineProps<{
      title: string
      heading?: 'h1' | 'h2'
      loading?: boolean
      error?: string | null
      empty?: boolean
      loadingText?: string
      emptyText?: string
    }>(),
    {
      heading: 'h2',
      loading: false,
      error: null,
      empty: false,
      loadingText: '',
      emptyText: '',
    },
  )
</script>
