<template>
  <section
    class="relative min-h-[240px] overflow-hidden rounded-2xl border border-[var(--color-border)] shadow-xl sm:min-h-[280px]"
  >
    <div
      class="absolute inset-0 bg-cover bg-center bg-no-repeat"
      :style="{ backgroundImage: `url(${backgroundUrl})` }"
      aria-hidden="true"
    />
    <div
      class="absolute inset-0 bg-gradient-to-r from-[var(--color-surface)] via-[var(--color-surface)]/88 to-[var(--color-surface)]/35"
      aria-hidden="true"
    />

    <div :class="['relative min-h-[240px] p-4 sm:min-h-[280px] sm:p-6 lg:p-8', contentClass]">
      <span
        v-if="version"
        class="absolute right-4 top-4 inline-flex h-6 items-center gap-1 rounded-full border border-[var(--color-border)] bg-[color-mix(in_srgb,var(--color-control-bg)_88%,transparent)] px-2 text-xs font-medium leading-4 text-[var(--color-text-muted)] sm:right-6 sm:top-6 lg:right-8 lg:top-8"
        :title="versionTitle"
      >
        <Tag class="size-3 shrink-0" aria-hidden="true" />
        <span class="whitespace-nowrap">{{ version }}</span>
      </span>
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { Tag } from '@lucide/vue'

  const props = withDefaults(
    defineProps<{
      backgroundUrl: string
      version?: string
      contentClass?: string
    }>(),
    {
      version: '',
      contentClass: 'flex flex-col justify-center',
    },
  )

  const { t } = useI18n()

  const versionTitle = computed(() =>
    props.version ? t('dashboard.cloudCurrentVersion', { version: props.version }) : '',
  )
</script>
