<template>
  <AppPageShell main-class="px-4 py-6 text-sm sm:px-6 sm:py-8 lg:px-8 2xl:px-12">
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
    <div class="mx-auto flex w-full max-w-[1440px] flex-col gap-5">
      <section
        class="grid items-center gap-8 lg:min-h-[min(680px,calc(100vh-8rem))] lg:grid-cols-[minmax(0,480px)_minmax(0,1fr)] lg:gap-12 2xl:gap-16"
      >
        <div class="flex flex-col justify-center gap-5 sm:gap-6">
          <div>
            <h1
              class="text-xl font-semibold leading-tight text-[var(--color-text-strong)] sm:mt-3 sm:text-lg"
            >
              {{ t('dashboard.cloudLandingTitle') }}
            </h1>
            <p class="mt-3 max-w-md text-sm leading-6 text-[var(--color-text-muted)]">
              {{ t('dashboard.cloudLandingCopy') }}
            </p>
          </div>

          <div class="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
            <button
              v-if="!cloudAuth.authenticated"
              type="button"
              class="inline-flex h-11 w-full items-center justify-center gap-1.5 rounded-md border border-blue-700 bg-blue-600 px-4 text-sm text-slate-50 outline-none hover:bg-blue-500 focus:bg-blue-500 disabled:cursor-not-allowed disabled:border-[var(--color-border)] disabled:bg-[var(--color-control-bg)] disabled:text-[var(--color-text-muted)] sm:h-9 sm:w-auto sm:px-3"
              :disabled="cloudAuthLoading"
              @click="openCloudLogin"
            >
              {{ cloudAuthLoading ? t('cloud.checkingAuth') : t('dashboard.cloudSignInCta') }}
              <ArrowRight class="size-3.5 text-slate-100" />
            </button>
            <RouterLink
              v-else
              :to="{ name: 'cloud-dashboard' }"
              class="inline-flex h-11 w-full items-center justify-center gap-1.5 rounded-md border border-blue-700 bg-blue-600 px-4 text-sm text-slate-50 outline-none hover:bg-blue-500 focus:bg-blue-500 sm:h-9 sm:w-auto sm:px-3"
            >
              {{ t('dashboard.viewDeviceStatus') }}
              <ArrowRight class="size-3.5 text-slate-100" />
            </RouterLink>
            <button
              v-if="runtimeConfig.config.local.mode === 'hybrid'"
              type="button"
              class="button button-secondary h-11 w-full justify-center gap-1.5 px-4 text-sm sm:h-9 sm:w-auto sm:px-3"
              @click="openLocalEntry"
            >
              {{ t('dashboard.openLocalPage') }}
            </button>
          </div>
        </div>

        <div
          class="flex min-h-0 items-center lg:min-h-[420px] 2xl:min-h-[540px]"
          :aria-label="t('dashboard.cloudHeroImageAlt')"
        >
          <img
            :src="cloudHomeHeroUrl"
            :alt="t('dashboard.cloudHeroImageAlt')"
            class="h-auto max-h-[42vh] w-full rounded-2xl object-cover shadow-2xl shadow-blue-950/30 sm:max-h-[min(520px,calc(100vh-10rem))] lg:max-h-[calc(100vh-9rem)]"
          />
        </div>
      </section>
    </div>
  </AppPageShell>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ArrowRight } from '@lucide/vue'
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
      // ignore logout API errors — clear local state anyway
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
