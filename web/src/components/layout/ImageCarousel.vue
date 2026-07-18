<template>
  <div
    class="relative"
    role="region"
    :aria-roledescription="t('common.carousel')"
    :aria-label="ariaLabel"
    @mouseenter="pause"
    @mouseleave="resume"
    @focusin="pause"
    @focusout="resume"
  >
    <div
      class="relative overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-surface-muted)]"
    >
      <div :class="frameClass">
        <img
          v-for="(slide, index) in slides"
          :key="`${slide.src}-${index}`"
          :src="slide.src"
          :alt="slide.alt || ''"
          class="absolute inset-0 h-full w-full object-cover object-center transition-opacity duration-500 ease-out"
          :class="index === activeIndex ? 'opacity-100' : 'pointer-events-none opacity-0'"
          :aria-hidden="index === activeIndex ? undefined : 'true'"
        />
      </div>

      <div
        v-if="slides.length > 1"
        class="absolute inset-x-0 bottom-3 z-10 flex items-center justify-center gap-1.5"
      >
        <button
          v-for="(_, index) in slides"
          :key="index"
          type="button"
          class="size-2 rounded-full transition-colors outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60"
          :class="index === activeIndex ? 'bg-white/90' : 'bg-white/35 hover:bg-white/55'"
          :aria-label="t('common.carouselGoTo', { index: index + 1 })"
          :aria-current="index === activeIndex ? 'true' : undefined"
          @click="goTo(index)"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
  import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'

  export type ImageCarouselSlide = {
    src: string
    alt?: string
  }

  const props = withDefaults(
    defineProps<{
      slides: ImageCarouselSlide[]
      ariaLabel?: string
      intervalMs?: number
      autoplay?: boolean
      frameClass?: string
    }>(),
    {
      ariaLabel: '',
      intervalMs: 5000,
      autoplay: true,
      frameClass: 'relative h-[200px] w-full sm:h-[240px] lg:h-[260px]',
    },
  )

  const { t } = useI18n()
  const activeIndex = ref(0)
  const paused = ref(false)
  let timer: ReturnType<typeof window.setInterval> | null = null

  const slideCount = computed(() => props.slides.length)

  function clearTimer() {
    if (timer) {
      window.clearInterval(timer)
      timer = null
    }
  }

  function startTimer() {
    clearTimer()
    if (!props.autoplay || props.intervalMs <= 0 || slideCount.value <= 1 || paused.value) {
      return
    }
    timer = window.setInterval(() => {
      activeIndex.value = (activeIndex.value + 1) % slideCount.value
    }, props.intervalMs)
  }

  function goTo(index: number) {
    if (slideCount.value === 0) {
      return
    }
    activeIndex.value = ((index % slideCount.value) + slideCount.value) % slideCount.value
    startTimer()
  }

  function pause() {
    paused.value = true
    clearTimer()
  }

  function resume() {
    paused.value = false
    startTimer()
  }

  watch(
    () => [props.autoplay, props.intervalMs, slideCount.value] as const,
    () => {
      if (activeIndex.value >= slideCount.value) {
        activeIndex.value = 0
      }
      startTimer()
    },
  )

  onMounted(() => {
    startTimer()
  })

  onBeforeUnmount(() => {
    clearTimer()
  })
</script>
