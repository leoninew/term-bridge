<template>
  <header
    class="flex h-14 shrink-0 items-center justify-between gap-3 border-b border-[var(--color-border)] bg-[var(--color-panel-header)] px-4 sm:h-16 sm:px-6 lg:px-8 2xl:px-12"
  >
    <RouterLink to="/" class="flex min-w-0 items-center gap-2 sm:gap-3">
      <BrandMark class="size-8 shrink-0 rounded-xl shadow-lg sm:size-10" />
      <p class="truncate text-base font-semibold text-[var(--color-text-strong)] sm:text-lg">
        TermBridge
      </p>
    </RouterLink>

    <div class="flex shrink-0 items-center gap-1.5 sm:gap-3">
      <a
        href="https://github.com/leoninew/TermBridge-go"
        target="_blank"
        rel="noopener noreferrer"
        class="inline-flex size-9 items-center justify-center rounded-md text-[var(--color-text-subtle)] outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
        :aria-label="t('dashboard.footerGitHub')"
        :title="t('dashboard.footerGitHub')"
      >
        <GithubIcon class="size-4" />
      </a>
      <DisplayControls />
      <slot name="actions" />
    </div>
  </header>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import { RouterLink } from 'vue-router'
  import BrandMark from '../branding/BrandMark.vue'
  import GithubIcon from '../branding/GithubIcon.vue'
  import DisplayControls from '../dashboard/DisplayControls.vue'

  const { t } = useI18n()
</script>
