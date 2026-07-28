<template>
  <DocsShell
    :language="language"
    :pages="pages"
    :active-page-id="activePageId"
    :headings="headings"
    :active-heading="activeHeading"
    @navigate="navigateTo"
    @heading-navigate="navigateToHeading"
    @language-change="changeLanguage"
  />
</template>

<script setup lang="ts">
  import { computed, nextTick, onMounted, ref, watch } from 'vue'
  import { useRoute, useRouter } from 'vue-router'
  import DocsShell from '../components/docs/DocsShell.vue'
  import {
    DOC_DEFAULT_LANGUAGE,
    DOC_PAGES,
    isDocLanguage,
    type DocLanguage,
    type DocPageId,
  } from '../features/docs/catalog'
  import { getDocPages } from '../features/docs/content'

  const route = useRoute()
  const router = useRouter()
  const activeHeading = ref('overview')

  const language = computed<DocLanguage>(() => {
    const queryLanguage = route.query.lang
    if (isDocLanguage(queryLanguage)) return queryLanguage
    const browserLanguage = navigator.language.startsWith('zh') ? 'zh-CN' : 'en-US'
    return browserLanguage === 'zh-CN' ? browserLanguage : DOC_DEFAULT_LANGUAGE
  })
  const pages = computed(() => getDocPages(language.value))
  const activePageId = computed<DocPageId>(() => {
    const hash = route.hash.replace(/^#/, '')
    const page = DOC_PAGES.find((candidate) => candidate.navAnchor === hash)
    if (page) return page.id
    const headingPage = pages.value.find((page) =>
      page.headings.some((heading) => heading.id === hash),
    )
    return headingPage?.id ?? 'index'
  })
  const headings = computed(
    () => pages.value.find((page) => page.id === activePageId.value)?.headings ?? [],
  )

  async function scrollToHash(hash: string) {
    await nextTick()
    const id = hash.replace(/^#/, '') || 'index'
    const target = document.getElementById(id)
    if (target) {
      target.scrollIntoView({ behavior: 'smooth', block: 'start' })
      activeHeading.value = id
    }
  }

  function navigateTo(id: DocPageId) {
    const target = DOC_PAGES.find((page) => page.id === id)
    if (!target) return
    void router.push({
      name: 'docs',
      query: { lang: language.value },
      hash: `#${target.navAnchor}`,
    })
  }

  function navigateToHeading(id: string) {
    void router.push({ name: 'docs', query: { lang: language.value }, hash: `#${id}` })
  }

  function changeLanguage(nextLanguage: DocLanguage) {
    const currentHash = route.hash || '#index'
    void router.push({ name: 'docs', query: { lang: nextLanguage }, hash: currentHash })
  }

  watch(
    () => [route.hash, language.value],
    ([hash]) => {
      void scrollToHash(String(hash))
    },
  )

  onMounted(() => {
    void scrollToHash(route.hash)
  })
</script>
