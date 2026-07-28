<template>
  <article class="min-w-0 max-w-3xl pb-16" aria-label="Documentation content">
    <section v-for="page in pages" :id="page.id" :key="page.id" class="doc-page scroll-mt-20 py-10 first:pt-2">
      <header class="mb-7 border-b border-[var(--color-border)] pb-6">
        <p class="text-xs font-semibold uppercase tracking-[0.18em] text-[var(--color-primary-text)]">
          {{ page.id === 'index' ? 'TermBridge' : page.order.toString().padStart(2, '0') }}
        </p>
        <p class="mt-3 text-base leading-7 text-[var(--color-text-muted)]">{{ page.description }}</p>
      </header>

      <template v-for="(block, index) in page.blocks" :key="`${page.id}-${index}`">
        <!-- eslint-disable-next-line vue/no-v-html -- Markdown is repository-controlled, raw HTML is disabled, and the rendered result is sanitized in features/docs/markdown.ts. -->
        <div v-if="block.kind === 'html'" class="doc-markdown" v-html="block.value" />
        <DocScreenshotPlaceholder
          v-else
          :id="toScreenshotId(block.value)"
          :language="language"
        />
      </template>

      <footer v-if="page.next" class="mt-10 border-t border-[var(--color-border)] pt-6">
        <button
          type="button"
          class="group flex w-full items-center justify-between gap-4 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-4 text-left transition-colors hover:border-[var(--color-border-strong)] hover:bg-[var(--color-control-hover)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary-border)]"
          @click="emit('navigate', page.next)"
        >
          <span>
            <span class="block text-xs font-medium text-[var(--color-text-subtle)]">
              {{ language === 'zh-CN' ? '下一篇' : 'Next' }}
            </span>
            <span class="mt-1 block text-sm font-semibold text-[var(--color-text-strong)]">
              {{ titleFor(page.next) }}
            </span>
          </span>
          <span class="text-lg text-[var(--color-primary-text)] transition-transform group-hover:translate-x-1" aria-hidden="true">→</span>
        </button>
      </footer>
    </section>
  </article>
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import type { DocLanguage, DocPageId, DocScreenshotId } from '../../features/docs/catalog'
  import { DOC_SCREENSHOT_IDS } from '../../features/docs/catalog'
  import type { DocPage } from '../../features/docs/content'
  import DocScreenshotPlaceholder from './DocScreenshotPlaceholder.vue'

  const props = defineProps<{ language: DocLanguage; pages: DocPage[] }>()
  const emit = defineEmits<{ navigate: [id: DocPageId] }>()
  const pageTitles = computed(() => new Map(props.pages.map((page) => [page.id, page.title])))

  function titleFor(id: DocPageId): string {
    return pageTitles.value.get(id) ?? id
  }

  function toScreenshotId(value: string): DocScreenshotId {
    if (!DOC_SCREENSHOT_IDS.includes(value as DocScreenshotId)) {
      throw new Error(`Unknown documentation screenshot: ${value}`)
    }
    return value as DocScreenshotId
  }
</script>
