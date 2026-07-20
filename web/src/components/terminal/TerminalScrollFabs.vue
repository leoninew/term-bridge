<template>
  <TooltipProvider v-if="showTop || showBottom" :delay-duration="400" :disable-hoverable-content="true">
    <div
      class="terminal-scroll-fabs pointer-events-none absolute right-2 bottom-[max(0.75rem,env(safe-area-inset-bottom,0px))] z-20 flex flex-col gap-2"
      :class="dimmed ? 'opacity-40' : ''"
    >
      <TooltipRoot v-if="showTop">
        <TooltipTrigger as-child>
          <button
            type="button"
            class="terminal-scroll-fab pointer-events-auto"
            :aria-label="t('workbench.scrollToTopAria')"
            :title="t('workbench.scrollToTopAria')"
            @click.stop="emit('scrollToTop')"
          >
            <ChevronsUp class="size-4" aria-hidden="true" />
          </button>
        </TooltipTrigger>
        <TooltipPortal>
          <TooltipContent
            side="left"
            :side-offset="8"
            class="z-50 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-2 py-1 text-xs text-[var(--color-text)] shadow-lg"
          >
            {{ t('workbench.scrollToTopAria') }}
          </TooltipContent>
        </TooltipPortal>
      </TooltipRoot>
      <TooltipRoot v-if="showBottom">
        <TooltipTrigger as-child>
          <button
            type="button"
            class="terminal-scroll-fab pointer-events-auto"
            :aria-label="t('workbench.scrollToBottomAria')"
            :title="t('workbench.scrollToBottomAria')"
            @click.stop="emit('scrollToBottom')"
          >
            <ChevronsDown class="size-4" aria-hidden="true" />
          </button>
        </TooltipTrigger>
        <TooltipPortal>
          <TooltipContent
            side="left"
            :side-offset="8"
            class="z-50 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-2 py-1 text-xs text-[var(--color-text)] shadow-lg"
          >
            {{ t('workbench.scrollToBottomAria') }}
          </TooltipContent>
        </TooltipPortal>
      </TooltipRoot>
    </div>
  </TooltipProvider>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import { ChevronsDown, ChevronsUp } from '@lucide/vue'
  import {
    TooltipContent,
    TooltipPortal,
    TooltipProvider,
    TooltipRoot,
    TooltipTrigger,
  } from 'reka-ui'

  withDefaults(
    defineProps<{
      dimmed?: boolean
      showTop?: boolean
      showBottom?: boolean
    }>(),
    {
      dimmed: false,
      showTop: true,
      showBottom: true,
    },
  )

  const emit = defineEmits<{
    scrollToTop: []
    scrollToBottom: []
  }>()

  const { t } = useI18n()
</script>
