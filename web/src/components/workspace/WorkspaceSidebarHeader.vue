<template>
  <header
    class="flex h-11 shrink-0 items-center border-b border-[var(--color-border)] bg-[var(--color-panel-header)] px-2"
  >
    <div class="flex w-full items-center gap-1.5">
      <RouterLink
        :to="{ name: homeRouteName }"
        class="flex size-8 shrink-0 items-center justify-center rounded-md bg-blue-600 text-sm font-semibold text-white shadow-sm hover:bg-blue-500 focus:bg-blue-500 focus:outline-none"
        aria-label="TermBridge"
        title="TermBridge"
      >
        TB
      </RouterLink>
      <label class="relative min-w-0 flex-1">
        <span class="sr-only">{{ t('sidebar.searchSessions') }}</span>
        <Search
          class="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-[var(--color-text-subtle)]"
        />
        <input
          :value="searchQuery"
          type="search"
          :placeholder="t('sidebar.searchPlaceholder')"
          class="h-7 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] py-1 pl-8 pr-2 text-sm text-[var(--color-text)] outline-none placeholder:text-[var(--color-text-subtle)]"
          @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
        />
      </label>
      <button
        type="button"
        :class="sidebarHeaderIconButtonClass"
        :aria-label="t('sidebar.newSession')"
        :title="t('sidebar.newSession')"
        @click="emit('newSession')"
      >
        <Plus class="size-3.5" />
      </button>
      <button
        type="button"
        :class="sidebarHeaderIconButtonClass"
        :aria-label="t('workbench.collapseSidebarAria')"
        :title="t('workbench.collapseSidebarAria')"
        @click="emit('collapse')"
      >
        <FoldHorizontal class="size-3.5" />
      </button>
    </div>
  </header>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import { FoldHorizontal, Plus, Search } from '@lucide/vue'
  import { RouterLink } from 'vue-router'
  import { sidebarHeaderIconButtonClass } from '../session/sessionUi'

  defineProps<{
    homeRouteName: string
    searchQuery: string
  }>()

  const emit = defineEmits<{
    'update:searchQuery': [value: string]
    newSession: []
    collapse: []
  }>()

  const { t } = useI18n()
</script>
