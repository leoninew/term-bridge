<template>
  <figure class="my-7 overflow-hidden rounded-xl border border-dashed border-[var(--color-border-strong)] bg-[var(--color-surface-muted)]">
    <div class="flex min-h-40 items-center justify-center px-5 py-8 text-center">
      <div class="max-w-lg">
        <p class="text-xs font-semibold uppercase tracking-[0.18em] text-[var(--color-text-subtle)]">
          {{ language === 'zh-CN' ? '截图待补充' : 'Screenshot planned' }}
        </p>
        <h3 class="mt-2 text-base font-semibold text-[var(--color-text-strong)]">{{ copy.title }}</h3>
        <p class="mt-2 text-sm leading-6 text-[var(--color-text-muted)]">{{ copy.planned }}</p>
      </div>
    </div>
    <figcaption class="border-t border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-3 text-sm leading-6 text-[var(--color-text-muted)]">
      {{ copy.caption }}
    </figcaption>
  </figure>
</template>

<script setup lang="ts">
  import type { DocLanguage, DocScreenshotId } from '../../features/docs/catalog'
  import { getScreenshotCopy } from '../../features/docs/screenshots'
  import { computed } from 'vue'

  const props = defineProps<{ language: DocLanguage; id: DocScreenshotId }>()
  const copy = computed(() => getScreenshotCopy(props.language, props.id))
</script>
