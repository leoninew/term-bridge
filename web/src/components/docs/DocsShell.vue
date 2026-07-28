<template>
  <AppPageShell main-class="overflow-y-auto">
    <div class="mx-auto w-full max-w-[1600px] px-4 py-7 sm:px-6 lg:px-8 lg:py-10">
      <header class="mb-8 flex flex-wrap items-end justify-between gap-4 border-b border-[var(--color-border)] pb-5">
        <div>
          <p class="text-xs font-semibold uppercase tracking-[0.2em] text-[var(--color-primary-text)]">TermBridge</p>
          <h1 class="mt-2 text-2xl font-semibold tracking-tight text-[var(--color-text-strong)]">
            {{ language === 'zh-CN' ? '使用文档' : 'Documentation' }}
          </h1>
        </div>
        <DocsLanguageSwitch :language="language" @change="emit('language-change', $event)" />
      </header>

      <div class="lg:grid lg:grid-cols-[228px_minmax(0,1fr)_208px] lg:gap-10 xl:grid-cols-[248px_minmax(0,1fr)_224px]">
        <details class="mb-6 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-2 lg:sticky lg:top-5 lg:mb-0 lg:block lg:h-fit lg:border-0 lg:bg-transparent lg:p-0" open>
          <summary class="cursor-pointer px-2 py-1 text-sm font-semibold text-[var(--color-text-strong)] lg:hidden">
            {{ language === 'zh-CN' ? '浏览章节' : 'Browse sections' }}
          </summary>
          <div class="pt-2 lg:pt-0">
            <DocsSidebar :pages="pages" :active-id="activePageId" @navigate="emit('navigate', $event)" />
          </div>
        </details>

        <DocsArticle :language="language" :pages="pages" @navigate="emit('navigate', $event)" />

        <aside class="hidden lg:block">
          <div class="sticky top-5">
            <DocsToc :language="language" :headings="headings" :active-heading="activeHeading" @navigate="emit('heading-navigate', $event)" />
          </div>
        </aside>
      </div>
    </div>
  </AppPageShell>
</template>

<script setup lang="ts">
  import type { DocLanguage, DocPageId } from '../../features/docs/catalog'
  import type { DocPage } from '../../features/docs/content'
  import './docs.css'
  import type { DocHeading } from '../../features/docs/markdown'
  import AppPageShell from '../layout/AppPageShell.vue'
  import DocsArticle from './DocsArticle.vue'
  import DocsLanguageSwitch from './DocsLanguageSwitch.vue'
  import DocsSidebar from './DocsSidebar.vue'
  import DocsToc from './DocsToc.vue'

  defineProps<{
    language: DocLanguage
    pages: DocPage[]
    activePageId: DocPageId
    headings: DocHeading[]
    activeHeading: string
  }>()
  const emit = defineEmits<{
    navigate: [id: DocPageId]
    'heading-navigate': [id: string]
    'language-change': [language: DocLanguage]
  }>()
</script>
