<template>
  <AppPageShell
    main-class="flex flex-col overflow-y-auto px-3 py-4 text-sm sm:px-5 sm:py-6 md:px-6 md:py-8"
  >
    <template #actions>
      <CloudAccountMenu
        :authenticated="cloudAuth.authenticated"
        :user-display-name="cloudUserDisplayName"
        :user-email="cloudAuth.user?.email ?? ''"
        :show-change-password="true"
        @login="openCloudLogin"
        @logout="logoutCloud"
        @change-password="openChangePassword"
      />
    </template>

    <div class="mx-auto flex w-full max-w-5xl flex-1 flex-col justify-center gap-4 sm:gap-5">
      <section
        class="relative overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl"
      >
        <div
          class="pointer-events-none absolute -right-16 -top-20 size-64 rounded-full bg-blue-500/10 blur-3xl"
          aria-hidden="true"
        />
        <div
          class="pointer-events-none absolute -bottom-24 -left-10 size-56 rounded-full bg-cyan-400/5 blur-3xl"
          aria-hidden="true"
        />

        <div
          class="relative grid items-stretch gap-0 lg:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]"
        >
          <div
            class="relative flex min-h-[220px] items-center justify-center self-stretch p-4 sm:min-h-[260px] sm:p-6 lg:min-h-full lg:p-8"
          >
            <p
              v-if="projectVersionLabel"
              class="absolute right-4 top-4 text-xs text-[var(--color-text-subtle)] sm:right-6 sm:top-6 lg:right-8 lg:top-8"
            >
              {{ t('dashboard.cloudCurrentVersion', { version: projectVersionLabel }) }}
            </p>

            <div class="flex w-full max-w-sm flex-col items-start gap-5 text-left">
              <div class="w-full">
                <h1
                  class="text-2xl font-semibold leading-tight tracking-tight text-[var(--color-text-strong)] sm:text-3xl"
                >
                  {{ landingTitle }}
                </h1>
                <p
                  class="mt-3 text-sm leading-6 text-[var(--color-text-muted)] sm:text-[15px] sm:leading-7"
                >
                  {{ landingCopy }}
                </p>
              </div>

              <div class="flex flex-row flex-wrap items-center justify-start gap-2">
                <button
                  v-if="!cloudAuth.authenticated"
                  type="button"
                  class="button button-primary !rounded-xl h-11 justify-center gap-1.5 px-4 text-sm sm:h-9 sm:px-3"
                  :disabled="cloudAuthLoading"
                  @click="openCloudLogin"
                >
                  {{ cloudAuthLoading ? t('cloud.checkingAuth') : t('dashboard.cloudSignInCta') }}
                  <ArrowRight class="size-3.5" />
                </button>
                <RouterLink
                  v-else
                  :to="{ name: 'cloud-dashboard' }"
                  class="button button-primary !rounded-xl h-11 justify-center gap-1.5 px-4 text-sm sm:h-9 sm:px-3"
                >
                  {{ t('dashboard.viewDeviceStatus') }}
                  <ArrowRight class="size-3.5" />
                </RouterLink>
                <button
                  v-if="runtimeConfig.config.local.mode === 'hybrid'"
                  type="button"
                  class="button button-secondary !rounded-xl h-11 justify-center gap-1.5 px-4 text-sm sm:h-9 sm:px-3"
                  @click="openLocalEntry"
                >
                  {{ t('dashboard.openLocalPage') }}
                </button>
              </div>
            </div>
          </div>

          <div
            class="relative border-t border-[var(--color-border)] p-3 sm:p-4 lg:border-t-0 lg:p-5"
            :aria-label="t('dashboard.cloudHeroImageAlt')"
          >
            <div
              class="overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-surface-muted)]"
            >
              <img
                :src="cloudHomeHeroUrl"
                :alt="t('dashboard.cloudHeroImageAlt')"
                class="block h-[200px] w-full object-cover object-center sm:h-[240px] lg:h-[260px]"
              />
            </div>
          </div>
        </div>
      </section>

      <section class="grid grid-cols-1 gap-3 sm:grid-cols-3 sm:gap-4">
        <article
          v-for="step in flowSteps"
          :key="step.title"
          class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-xl sm:p-5"
        >
          <div class="flex items-start gap-3">
            <span
              class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-blue-500/10 text-blue-500 sm:size-10"
            >
              <component :is="step.icon" class="size-4 sm:size-5" />
            </span>
            <div class="min-w-0">
              <h2 class="text-sm font-semibold text-[var(--color-text-strong)] sm:text-base">
                {{ step.title }}
              </h2>
              <p class="mt-1.5 text-sm leading-6 text-[var(--color-text-muted)]">
                {{ step.copy }}
              </p>
            </div>
          </div>
        </article>
      </section>
    </div>

    <footer
      class="mt-6 shrink-0 pb-1 text-center text-xs text-[var(--color-text-subtle)] sm:mt-8"
    >
      <div class="flex flex-wrap items-center justify-center gap-x-3 gap-y-1">
        <span>{{ t('dashboard.cloudFooterCopyright', { year: 2026 }) }}</span>
        <a
          href="https://beian.miit.gov.cn/"
          target="_blank"
          rel="noopener noreferrer"
          class="outline-none transition-colors hover:text-[var(--color-text-muted)] focus-visible:text-[var(--color-text-muted)]"
        >
          鄂ICP备2026011519号-2
        </a>
        <a
          href="mailto:support@preflite.cn"
          class="outline-none transition-colors hover:text-[var(--color-text-muted)] focus-visible:text-[var(--color-text-muted)]"
        >
          {{ t('dashboard.cloudFooterSupport') }}
        </a>
      </div>
    </footer>
  </AppPageShell>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ArrowRight, Cloud, Monitor, SquareTerminal } from '@lucide/vue'
  import { RouterLink, useRouter } from 'vue-router'
  import AppPageShell from '../layout/AppPageShell.vue'
  import cloudHomeHeroUrl from '../../assets/cloud-home-hero-candidate.png'
  import CloudAccountMenu from './CloudAccountMenu.vue'
  import { authLogout } from '../../features/cloud/api'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useRuntimeConfigStore } from '../../store/runtimeConfig'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const router = useRouter()
  const runtimeConfig = useRuntimeConfigStore()
  const cloudAuth = useCloudAuthStore()
  const notifications = useNotificationsStore()
  const cloudAuthLoading = ref(false)

  const cloudUserDisplayName = computed(
    () => cloudAuth.user?.display_name || cloudAuth.user?.email || t('dashboard.signedIn'),
  )

  const projectVersionLabel = computed(() => runtimeConfig.config.version.trim())

  const landingTitle = computed(() =>
    cloudAuth.authenticated
      ? t('dashboard.cloudLandingWelcome')
      : t('dashboard.cloudLandingTitle'),
  )

  const landingCopy = computed(() =>
    cloudAuth.authenticated
      ? t('dashboard.cloudLandingSignedInCopy')
      : t('dashboard.cloudLandingCopy'),
  )

  const flowSteps = computed(() => [
    {
      icon: Cloud,
      title: t('dashboard.cloudFlowAccountTitle'),
      copy: t('dashboard.cloudFlowAccountCopy'),
    },
    {
      icon: Monitor,
      title: t('dashboard.cloudFlowDevicesTitle'),
      copy: t('dashboard.cloudFlowDevicesCopy'),
    },
    {
      icon: SquareTerminal,
      title: t('dashboard.cloudFlowWorkbenchTitle'),
      copy: t('dashboard.cloudFlowWorkbenchCopy'),
    },
  ])

  async function loadCloudHome() {
    cloudAuthLoading.value = true
    try {
      await cloudAuth.initialize()
    } catch (err) {
      notifications.notifyError(t('cloud.loginFailed'), err)
      return
    } finally {
      cloudAuthLoading.value = false
    }
  }

  function openLocalEntry() {
    window.open(runtimeConfig.config.local.publicUrl, '_blank', 'noopener,noreferrer')
  }

  async function openCloudLogin() {
    await router.push({ name: 'cloud-login', query: { redirect: '/dashboard' } })
  }

  async function logoutCloud() {
    try {
      await authLogout()
    } catch {
      // ignore logout API errors; clear local state anyway
    }
    cloudAuth.clearToken()
    await router.replace({ name: 'home' })
  }

  async function openChangePassword() {
    await router.push({ name: 'cloud-change-password' })
  }

  onMounted(() => {
    void loadCloudHome()
  })
</script>
