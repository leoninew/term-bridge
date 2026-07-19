<template>
  <AppPageShell :main-class="homePageMainClass">
    <template #actions>
      <CloudAccountMenu
        :authenticated="cloudAuth.authenticated"
        :user-display-name="cloudUserDisplayName"
        :user-email="cloudAuth.user?.email ?? ''"
        :can-change-password="cloudAuth.user?.provider === 'email'"
        @login="openCloudLogin"
        @logout="logoutCloud"
        @change-password="openChangePassword"
      />
    </template>

    <div :class="homePageContentClass">
      <HomeHeroBanner
        :background-url="cloudHomeBgUrl"
        :version="projectVersionLabel"
        content-class="flex items-center"
      >
        <div class="relative flex max-w-md flex-col items-start gap-5 text-left">
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

          <div class="flex flex-wrap items-center gap-2">
            <button
              v-if="!cloudAuth.authenticated"
              type="button"
              :class="[homeCtaClass, 'button-primary']"
              :disabled="cloudAuthLoading"
              @click="openCloudLogin"
            >
              {{ cloudAuthLoading ? t('cloud.checkingAuth') : t('dashboard.cloudSignInCta') }}
              <ArrowRight class="size-3.5" />
            </button>
            <RouterLink
              v-else
              :to="{ name: 'cloud-dashboard' }"
              :class="[homeCtaClass, 'button-primary']"
            >
              {{ t('dashboard.viewDeviceStatus') }}
              <ArrowRight class="size-3.5" />
            </RouterLink>
            <button
              v-if="runtimeConfig.config.local.mode === 'hybrid'"
              type="button"
              :class="[homeCtaClass, 'button-secondary']"
              @click="openLocalEntry"
            >
              {{ t('dashboard.openLocalPage') }}
            </button>
          </div>
        </div>
      </HomeHeroBanner>

      <section class="grid grid-cols-1 gap-3 sm:grid-cols-3 sm:gap-4">
        <HomeFlowStepCard
          v-for="step in flowSteps"
          :key="step.title"
          :icon="step.icon"
          :title="step.title"
          :copy="step.copy"
        />
      </section>
    </div>

    <footer class="mt-6 shrink-0 pb-1 text-center text-xs text-[var(--color-text-subtle)] sm:mt-8">
      <div class="flex flex-wrap items-center justify-center gap-x-3 gap-y-1">
        <span>{{ t('dashboard.cloudFooterCopyright', { year: 2026 }) }}</span>
        <a
          href="https://beian.miit.gov.cn/"
          target="_blank"
          rel="noopener noreferrer"
          :class="homeFooterLinkClass"
        >
          ?ICP?2026011519?-2
        </a>
        <a href="mailto:support@preflite.cn" :class="homeFooterLinkClass">
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
  import cloudHomeBgUrl from '../../assets/cloud-home-bg.jpg'
  import CloudAccountMenu from './CloudAccountMenu.vue'
  import HomeFlowStepCard from './HomeFlowStepCard.vue'
  import HomeHeroBanner from './HomeHeroBanner.vue'
  import {
    homeCtaClass,
    homeFooterLinkClass,
    homePageContentClass,
    homePageMainClass,
  } from './homeUi'
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
    cloudAuth.authenticated ? t('dashboard.cloudLandingWelcome') : t('dashboard.cloudLandingTitle'),
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
