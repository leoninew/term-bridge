<template>
  <section
    class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] px-4 py-10 text-sm"
  >
    <div class="w-full max-w-[26rem] text-center">
      <div :class="authFormCardClass">
        <div class="flex flex-col items-center gap-4 text-center">
          <div v-if="$slots.icon" class="flex size-12 items-center justify-center">
            <slot name="icon" />
          </div>
          <Loader2
            v-if="status === 'loading'"
            class="size-8 animate-spin text-blue-500"
            aria-hidden="true"
          />
          <CircleAlert
            v-else-if="status === 'error'"
            class="size-8 text-[var(--color-danger-text)]"
            aria-hidden="true"
          />
          <CheckCircle2
            v-else-if="status === 'success'"
            class="size-8 text-green-500"
            aria-hidden="true"
          />

          <div class="space-y-2">
            <h1
              class="text-xl font-semibold leading-7 tracking-tight text-[var(--color-text-strong)] sm:text-2xl"
            >
              {{ title }}
            </h1>
            <p v-if="description" class="text-sm leading-6 text-[var(--color-text-muted)]">
              {{ description }}
            </p>
          </div>

          <slot />

          <p v-if="footerTo" :class="authFooterClass">
            <AuthInlineLink :to="footerTo">{{ footerLabel }}</AuthInlineLink>
          </p>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
  import { CheckCircle2, CircleAlert, Loader2 } from '@lucide/vue'
  import type { RouteLocationRaw } from 'vue-router'
  import AuthInlineLink from './AuthInlineLink.vue'
  import { authFooterClass, authFormCardClass } from './authUi'

  withDefaults(
    defineProps<{
      status?: 'loading' | 'error' | 'success'
      title: string
      description?: string
      footerTo?: RouteLocationRaw
      footerLabel?: string
    }>(),
    {
      status: 'loading',
      description: '',
      footerTo: undefined,
      footerLabel: '',
    },
  )
</script>
