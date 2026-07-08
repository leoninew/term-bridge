<template>
  <div class="flex items-center gap-2">
    <button
      type="button"
      class="inline-flex size-9 items-center justify-center rounded-md text-[var(--color-text-subtle)] outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
      :aria-label="languageLabel"
      :title="languageLabel"
      @click="toggleLocale"
    >
      <component :is="languageIcon" class="size-4" />
    </button>

    <button
      type="button"
      class="inline-flex size-9 items-center justify-center rounded-md text-[var(--color-text-subtle)] outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
      :aria-label="themeLabel"
      :title="themeLabel"
      @click="toggleTheme"
    >
      <component :is="themeIcon" class="size-4" />
    </button>
  </div>
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { Globe, Languages, Moon, Sun } from '@lucide/vue'
  import { localeLabels, locales, setLocale, type AppLocale } from '../../i18n'
  import { themes, useThemeStore, type AppTheme } from '../../store/theme'

  const { t, locale } = useI18n()
  const themeStore = useThemeStore()

  const languageLabel = computed(
    () => `${t('common.language')}: ${localeLabels[locale.value as AppLocale] ?? String(locale.value)}`,
  )
  const themeLabel = computed(() => `${t('common.theme')}: ${t(`theme.${themeStore.theme}`)}`)
  const languageIcon = computed(() => (locale.value === 'zh-CN' ? Languages : Globe))
  const themeIcon = computed(() => (themeStore.theme === 'dark' ? Moon : Sun))

  function toggleLocale() {
    const currentIndex = locales.indexOf(locale.value as AppLocale)
    const nextIndex = currentIndex === -1 ? 0 : (currentIndex + 1) % locales.length
    setLocale(locales[nextIndex])
  }

  function toggleTheme() {
    const currentIndex = themes.indexOf(themeStore.theme as AppTheme)
    const nextIndex = currentIndex === -1 ? 0 : (currentIndex + 1) % themes.length
    themeStore.setTheme(themes[nextIndex])
  }
</script>
