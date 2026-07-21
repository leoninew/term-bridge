<template>
  <TooltipProvider :delay-duration="400" :disable-hoverable-content="true">
    <!--
      Sit left of xterm's vertical scrollbar (~14px) so the rail never covers the thumb.
      Order: page up → fit → page down (browse, then adapt, then continue).
    -->
    <div
      class="terminal-scroll-fabs pointer-events-none absolute z-20"
      :class="dimmed ? 'is-dimmed' : ''"
    >
      <div class="terminal-scroll-fab-rail pointer-events-auto">
        <TooltipRoot v-if="showTop">
          <TooltipTrigger as-child>
            <button
              type="button"
              class="terminal-scroll-fab"
              :aria-label="t('workbench.scrollPageUpAria')"
              :title="t('workbench.scrollPageUpAria')"
              @click.stop="emit('scrollPageUp')"
            >
              <ChevronUp class="size-3.5" aria-hidden="true" />
            </button>
          </TooltipTrigger>
          <TooltipPortal>
            <TooltipContent
              side="left"
              :side-offset="10"
              class="z-50 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-2 py-1 text-xs text-[var(--color-text)] shadow-lg"
            >
              {{ t('workbench.scrollPageUpAria') }}
            </TooltipContent>
          </TooltipPortal>
        </TooltipRoot>

        <TooltipRoot>
          <TooltipTrigger as-child>
            <button
              type="button"
              class="terminal-scroll-fab terminal-scroll-fab-fit"
              :aria-label="t('workbench.fitTerminalAria')"
              :title="t('workbench.fitTerminalAria')"
              @click.stop="emit('fit')"
            >
              <Expand class="size-3.5" aria-hidden="true" />
            </button>
          </TooltipTrigger>
          <TooltipPortal>
            <TooltipContent
              side="left"
              :side-offset="10"
              class="z-50 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-2 py-1 text-xs text-[var(--color-text)] shadow-lg"
            >
              {{ t('workbench.fitTerminalAria') }}
            </TooltipContent>
          </TooltipPortal>
        </TooltipRoot>

        <TooltipRoot v-if="showBottom">
          <TooltipTrigger as-child>
            <button
              type="button"
              class="terminal-scroll-fab"
              :aria-label="t('workbench.scrollPageDownAria')"
              :title="t('workbench.scrollPageDownAria')"
              @click.stop="emit('scrollPageDown')"
            >
              <ChevronDown class="size-3.5" aria-hidden="true" />
            </button>
          </TooltipTrigger>
          <TooltipPortal>
            <TooltipContent
              side="left"
              :side-offset="10"
              class="z-50 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-2 py-1 text-xs text-[var(--color-text)] shadow-lg"
            >
              {{ t('workbench.scrollPageDownAria') }}
            </TooltipContent>
          </TooltipPortal>
        </TooltipRoot>
      </div>
    </div>
  </TooltipProvider>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import { ChevronDown, ChevronUp, Expand } from '@lucide/vue'
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
    scrollPageUp: []
    scrollPageDown: []
    fit: []
  }>()

  const { t } = useI18n()
</script>
