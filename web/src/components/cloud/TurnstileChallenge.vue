<template>
  <section class="mt-3">
    <label class="block" for="turnstile-widget">
      <span class="text-[var(--color-text)]">{{ t('cloud.humanVerification') }}</span>
    </label>
    <div
      id="turnstile-widget"
      ref="widget"
      class="mt-1 flex min-h-[65px] w-full justify-center overflow-hidden"
    />
    <p v-if="error" class="mt-2 text-xs text-[var(--color-danger-text)]" role="alert">
      {{ error }}
    </p>
  </section>
</template>

<script setup lang="ts">
  import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'

  const turnstileScriptSelector = 'script[data-termbridge-turnstile]'
  const turnstileLoadTimeoutMs = 10_000

  type TurnstileApi = {
    render: (container: HTMLElement, options: Record<string, unknown>) => string
    remove: (widgetId: string) => void
    reset: (widgetId: string) => void
  }

  declare global {
    interface Window {
      turnstile?: TurnstileApi
    }
  }

  const props = defineProps<{ siteKey: string }>()
  const emit = defineEmits<{ token: [value: string]; reset: [] }>()
  const { t } = useI18n()
  const widget = ref<HTMLElement | null>(null)
  const error = ref('')
  let widgetId = ''
  let disposed = false

  onMounted(async () => {
    if (!props.siteKey) {
      error.value = t('cloud.humanVerificationUnavailable')
      return
    }
    try {
      await loadTurnstile()
      renderWidget()
    } catch {
      error.value = t('cloud.humanVerificationUnavailable')
    }
  })

  onBeforeUnmount(() => {
    disposed = true
    if (widgetId && window.turnstile) {
      window.turnstile.remove(widgetId)
    }
  })

  watch(
    () => props.siteKey,
    () => reset(),
  )

  function renderWidget() {
    if (disposed || !widget.value || !window.turnstile || widgetId) {
      return
    }
    widgetId = window.turnstile.render(widget.value, {
      sitekey: props.siteKey,
      size: 'flexible',
      callback: (value: string) => {
        error.value = ''
        emit('token', value)
      },
      'expired-callback': reset,
      'error-callback': () => {
        error.value = t('cloud.humanVerificationUnavailable')
        reset()
      },
      'timeout-callback': reset,
    })
  }

  function reset() {
    emit('reset')
    if (widgetId && window.turnstile) {
      window.turnstile.reset(widgetId)
    }
  }

  async function loadTurnstile(): Promise<void> {
    if (window.turnstile) {
      return
    }
    document.querySelector(turnstileScriptSelector)?.remove()
    const script = document.createElement('script')
    script.setAttribute(
      'src',
      'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit',
    )
    script.setAttribute('async', '')
    script.setAttribute('defer', '')
    script.setAttribute('data-termbridge-turnstile', 'true')
    document.head.appendChild(script)

    await new Promise<void>((resolve, reject) => {
      const timeout = window.setTimeout(
        () => reject(new Error('load timed out')),
        turnstileLoadTimeoutMs,
      )
      const complete = (callback: () => void) => {
        window.clearTimeout(timeout)
        callback()
      }
      script.addEventListener(
        'load',
        () =>
          complete(() => (window.turnstile ? resolve() : reject(new Error('missing turnstile')))),
        { once: true },
      )
      script.addEventListener('error', () => complete(() => reject(new Error('load failed'))), {
        once: true,
      })
    })
  }

  defineExpose({ reset })
</script>
