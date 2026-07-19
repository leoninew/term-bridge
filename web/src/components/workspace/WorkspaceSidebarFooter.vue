<template>
  <footer
    class="flex h-8 shrink-0 items-center gap-2 border-t border-[var(--color-border)] bg-[var(--color-panel-header)] px-2 text-sm text-[var(--color-text-muted)]"
  >
    <div class="flex min-w-0 flex-1 items-center gap-2 overflow-hidden">
      <DropdownMenuRoot :modal="false">
        <DropdownMenuTrigger
          class="flex h-6 items-center gap-1.5 rounded px-1.5 text-[var(--color-text-muted)] outline-none hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)] focus:bg-[var(--color-control-hover)] focus:text-[var(--color-text)]"
          :aria-label="t('common.settings')"
          :title="t('common.settings')"
        >
          <Settings class="size-4" />
          <span>{{ t('common.settings') }}</span>
        </DropdownMenuTrigger>
        <DropdownMenuPortal>
          <DropdownMenuContent
            side="top"
            align="start"
            :side-offset="8"
            :class="sessionDropdownContentClass"
          >
            <DropdownMenuSub>
              <DropdownMenuSubTrigger
                class="flex cursor-pointer items-center justify-between rounded px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
              >
                <span class="inline-flex items-center gap-2">
                  <Languages class="size-4 text-[var(--color-text-subtle)]" />
                  {{ t('common.language') }}
                </span>
                <ChevronRight class="size-4 text-[var(--color-text-subtle)]" />
              </DropdownMenuSubTrigger>
              <DropdownMenuPortal>
                <DropdownMenuSubContent :side-offset="8" :class="sessionDropdownContentClass">
                  <DropdownMenuRadioGroup :model-value="locale" @update:model-value="changeLocale">
                    <DropdownMenuRadioItem
                      v-for="item in localeOptions"
                      :key="item.value"
                      :value="item.value"
                      :class="sessionMenuItemClass"
                    >
                      <Check
                        :class="locale === item.value ? 'opacity-100' : 'opacity-0'"
                        class="size-4 text-blue-500"
                      />
                      {{ item.label }}
                    </DropdownMenuRadioItem>
                  </DropdownMenuRadioGroup>
                </DropdownMenuSubContent>
              </DropdownMenuPortal>
            </DropdownMenuSub>
            <DropdownMenuSub>
              <DropdownMenuSubTrigger
                class="flex cursor-pointer items-center justify-between rounded px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
              >
                <span class="inline-flex items-center gap-2">
                  <Sun class="size-4 text-[var(--color-text-subtle)]" />
                  {{ t('common.theme') }}
                </span>
                <ChevronRight class="size-4 text-[var(--color-text-subtle)]" />
              </DropdownMenuSubTrigger>
              <DropdownMenuPortal>
                <DropdownMenuSubContent :side-offset="8" :class="sessionDropdownContentClass">
                  <DropdownMenuRadioGroup
                    :model-value="themeStore.theme"
                    @update:model-value="changeTheme"
                  >
                    <DropdownMenuRadioItem
                      v-for="item in themeOptions"
                      :key="item.value"
                      :value="item.value"
                      :class="sessionMenuItemClass"
                    >
                      <Check
                        :class="themeStore.theme === item.value ? 'opacity-100' : 'opacity-0'"
                        class="size-4 text-blue-500"
                      />
                      {{ item.label }}
                    </DropdownMenuRadioItem>
                  </DropdownMenuRadioGroup>
                </DropdownMenuSubContent>
              </DropdownMenuPortal>
            </DropdownMenuSub>
          </DropdownMenuContent>
        </DropdownMenuPortal>
      </DropdownMenuRoot>
    </div>

    <button
      type="button"
      class="flex size-6 shrink-0 items-center justify-center rounded text-[var(--color-text-muted)] outline-none hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)] focus-visible:bg-[var(--color-control-hover)] focus-visible:text-[var(--color-text)] disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-[var(--color-text-muted)]"
      :disabled="!activeWorkspace"
      :aria-label="switchToFilesAria"
      :title="switchToFilesAria"
      @click="emit('switchToFiles')"
    >
      <FileCode2 class="size-4" />
    </button>
  </footer>
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { Check, ChevronRight, FileCode2, Languages, Settings, Sun } from '@lucide/vue'
  import {
    DropdownMenuContent,
    DropdownMenuPortal,
    DropdownMenuRadioGroup,
    DropdownMenuRadioItem,
    DropdownMenuRoot,
    DropdownMenuSub,
    DropdownMenuSubContent,
    DropdownMenuSubTrigger,
    DropdownMenuTrigger,
  } from 'reka-ui'
  import { localeLabels, locales, setLocale, type AppLocale } from '../../i18n'
  import { themes, useThemeStore, type AppTheme } from '../../store/theme'
  import type { Workspace as WorkspaceSummary } from '../../gen/proto/termbridge/agent/v1/workspace'
  import {
    sessionDropdownContentClass,
    sessionMenuItemClass,
  } from '../session/sessionUi'

  defineProps<{
    activeWorkspace: WorkspaceSummary | null
    switchToFilesAria: string
  }>()

  const emit = defineEmits<{
    switchToFiles: []
  }>()

  const { t, locale } = useI18n()
  const themeStore = useThemeStore()

  const localeOptions = computed(() =>
    locales.map((value) => ({ value, label: localeLabels[value] })),
  )
  const themeOptions = computed(() =>
    themes.map((value) => ({
      value,
      label: t(`theme.${value}`),
    })),
  )

  function changeLocale(value: unknown) {
    if (typeof value === 'string' && locales.includes(value as AppLocale)) {
      setLocale(value as AppLocale)
    }
  }

  function changeTheme(value: unknown) {
    if (typeof value === 'string' && themes.includes(value as AppTheme)) {
      themeStore.setTheme(value as AppTheme)
    }
  }
</script>
