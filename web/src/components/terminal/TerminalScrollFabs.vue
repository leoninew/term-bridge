<template>
  <TooltipProvider :delay-duration="400" :disable-hoverable-content="true">
    <!--
      Sit left of xterm's vertical scrollbar (~14px) so the rail never covers the thumb.
      Order: page up → fit → page down (browse, then adapt, then continue).
      Whole rail is draggable; click actions still work under a small move threshold.
    -->
    <div
      ref="rootEl"
      class="terminal-scroll-fabs pointer-events-none absolute z-20"
      :class="{
        'is-dimmed': dimmed,
        'is-custom': customPos !== null,
        'is-dragging': dragging,
      }"
      :style="rootStyle"
    >
      <div
        class="terminal-scroll-fab-rail pointer-events-auto"
        :class="{ 'is-dragging': dragging }"
        :aria-grabbed="dragging ? 'true' : 'false'"
        @pointerdown="onPointerDown"
      >
        <TooltipRoot v-if="showTop">
          <TooltipTrigger as-child>
            <button
              type="button"
              class="terminal-scroll-fab"
              :aria-label="t('workbench.scrollPageUpAria')"
              :title="t('workbench.scrollPageUpAria')"
              @click.stop="onAction(() => emit('scrollPageUp'))"
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
              @click.stop="onAction(() => emit('fit'))"
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
              @click.stop="onAction(() => emit('scrollPageDown'))"
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
  import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ChevronDown, ChevronUp, Expand } from '@lucide/vue'
  import {
    TooltipContent,
    TooltipPortal,
    TooltipProvider,
    TooltipRoot,
    TooltipTrigger,
  } from 'reka-ui'
  import {
    clampFabPosition,
    isDragPastThreshold,
    loadFabPosition,
    saveFabPosition,
    type TerminalScrollFabPosition,
  } from './terminalScrollFabPosition'

  const props = withDefaults(
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

  const rootEl = ref<HTMLElement | null>(null)
  const customPos = ref<TerminalScrollFabPosition | null>(loadFabPosition())
  const dragging = ref(false)

  type DragState = {
    pointerId: number
    startClientX: number
    startClientY: number
    originLeft: number
    originTop: number
    moved: boolean
  }

  let dragState: DragState | null = null
  /** Swallow the click that browsers fire after a completed drag. */
  let suppressClick = false
  let shellObserver: ResizeObserver | null = null
  let observedShell: HTMLElement | null = null
  let windowListenersBound = false

  const rootStyle = computed(() => {
    if (!customPos.value) {
      return undefined
    }
    return {
      left: `${customPos.value.left}px`,
      top: `${customPos.value.top}px`,
    }
  })

  function shellElement(): HTMLElement | null {
    return (rootEl.value?.closest('.terminal-shell') as HTMLElement | null) ?? null
  }

  function hasMeasurableLayout(): boolean {
    const shell = shellElement()
    const root = rootEl.value
    return Boolean(
      shell &&
        root &&
        shell.clientWidth > 0 &&
        shell.clientHeight > 0 &&
        root.offsetWidth > 0 &&
        root.offsetHeight > 0,
    )
  }

  function measureAndClamp(position: TerminalScrollFabPosition): TerminalScrollFabPosition {
    const shell = shellElement()
    const root = rootEl.value
    if (!shell || !root || !hasMeasurableLayout()) {
      return position
    }
    return clampFabPosition(
      position,
      { width: shell.clientWidth, height: shell.clientHeight },
      { width: root.offsetWidth, height: root.offsetHeight },
    )
  }

  function reclampStoredPosition() {
    if (!customPos.value || !hasMeasurableLayout()) {
      return
    }
    const next = measureAndClamp(customPos.value)
    if (next.left !== customPos.value.left || next.top !== customPos.value.top) {
      customPos.value = next
      saveFabPosition(next)
    }
  }

  function captureCurrentOffset(): TerminalScrollFabPosition | null {
    const shell = shellElement()
    const root = rootEl.value
    if (!shell || !root) {
      return null
    }
    const shellRect = shell.getBoundingClientRect()
    const rootRect = root.getBoundingClientRect()
    return {
      left: rootRect.left - shellRect.left,
      top: rootRect.top - shellRect.top,
    }
  }

  function bindWindowListeners() {
    if (windowListenersBound) {
      return
    }
    window.addEventListener('pointermove', onWindowPointerMove, { passive: false })
    window.addEventListener('pointerup', onWindowPointerUp)
    window.addEventListener('pointercancel', onWindowPointerUp)
    windowListenersBound = true
  }

  function unbindWindowListeners() {
    if (!windowListenersBound) {
      return
    }
    window.removeEventListener('pointermove', onWindowPointerMove)
    window.removeEventListener('pointerup', onWindowPointerUp)
    window.removeEventListener('pointercancel', onWindowPointerUp)
    windowListenersBound = false
  }

  function onPointerDown(event: PointerEvent) {
    // Primary button only; ignore secondary / multi-touch extras.
    if (event.button !== 0 || event.isPrimary === false) {
      return
    }
    const origin = captureCurrentOffset()
    if (!origin) {
      return
    }
    dragState = {
      pointerId: event.pointerId,
      startClientX: event.clientX,
      startClientY: event.clientY,
      originLeft: origin.left,
      originTop: origin.top,
      moved: false,
    }
    // Do NOT setPointerCapture here — that steals the target and breaks button clicks.
    bindWindowListeners()
  }

  function onWindowPointerMove(event: PointerEvent) {
    if (!dragState || event.pointerId !== dragState.pointerId) {
      return
    }
    const dx = event.clientX - dragState.startClientX
    const dy = event.clientY - dragState.startClientY
    if (!dragState.moved) {
      if (!isDragPastThreshold(dx, dy)) {
        return
      }
      dragState.moved = true
      dragging.value = true
      suppressClick = true
      // Switch from CSS right/bottom default to explicit left/top at current visual spot.
      customPos.value = { left: dragState.originLeft, top: dragState.originTop }
    }
    customPos.value = measureAndClamp({
      left: dragState.originLeft + dx,
      top: dragState.originTop + dy,
    })
    event.preventDefault()
  }

  function onWindowPointerUp(event: PointerEvent) {
    if (!dragState || event.pointerId !== dragState.pointerId) {
      return
    }
    const moved = dragState.moved
    dragState = null
    dragging.value = false
    unbindWindowListeners()
    if (moved && customPos.value && hasMeasurableLayout()) {
      const clamped = measureAndClamp(customPos.value)
      customPos.value = clamped
      saveFabPosition(clamped)
    }
  }

  function onAction(action: () => void) {
    if (suppressClick) {
      // Consume the post-drag click once, then allow normal clicks again.
      suppressClick = false
      return
    }
    if (dragging.value || dragState?.moved) {
      return
    }
    action()
  }

  function bindShellObserver() {
    const shell = shellElement()
    if (shell === observedShell) {
      return
    }
    shellObserver?.disconnect()
    observedShell = shell
    if (!shell) {
      return
    }
    shellObserver = new ResizeObserver(() => {
      reclampStoredPosition()
    })
    shellObserver.observe(shell)
  }

  onMounted(() => {
    void nextTick(() => {
      bindShellObserver()
      reclampStoredPosition()
    })
  })

  watch(
    () => [props.showTop, props.showBottom] as const,
    () => {
      void nextTick(() => {
        bindShellObserver()
        reclampStoredPosition()
      })
    },
  )

  onBeforeUnmount(() => {
    unbindWindowListeners()
    shellObserver?.disconnect()
    shellObserver = null
    observedShell = null
    dragState = null
    dragging.value = false
  })
</script>
