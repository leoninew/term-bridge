<template>
  <section class="min-h-screen bg-[var(--color-app-bg)] text-sm text-[var(--color-text)]">
    <AppHeader>
      <template #actions>
        <CloudAccountMenu
          :authenticated="cloudAuth.authenticated"
          :user-display-name="cloudUserDisplayName"
          :user-email="cloudAuth.user?.email ?? ''"
          @login="openCloudLogin"
          @logout="logoutCloud"
          @change-password="changePasswordDialogOpen = true"
        />
      </template>
    </AppHeader>

    <main class="min-h-[calc(100vh-4rem)] p-6 pt-10">
      <div class="mx-auto flex w-full max-w-[1200px] flex-col gap-5">
        <section
          class="grid min-h-[680px] content-center gap-10 lg:grid-cols-[minmax(0,440px)_minmax(0,1fr)]"
        >
          <div class="flex flex-col justify-center gap-6">
            <div>
              <h1 class="mt-3 text-lg font-semibold text-[var(--color-text-strong)]">
                {{ t('dashboard.cloudLandingTitle') }}
              </h1>
              <p class="mt-3 max-w-md text-sm text-[var(--color-text-muted)]">
                {{ t('dashboard.cloudLandingCopy') }}
              </p>
            </div>

            <div class="flex flex-wrap items-center gap-2">
              <button
                v-if="!cloudAuth.authenticated"
                type="button"
                class="inline-flex h-8 items-center gap-1.5 rounded-md border border-blue-700 bg-blue-600 px-3 text-sm text-slate-50 outline-none hover:bg-blue-500 focus:bg-blue-500 disabled:cursor-not-allowed disabled:border-[var(--color-border)] disabled:bg-[var(--color-control-bg)] disabled:text-[var(--color-text-muted)]"
                :disabled="cloudAuthLoading"
                @click="openCloudLogin"
              >
                {{ cloudAuthLoading ? t('cloud.checkingAuth') : t('dashboard.cloudSignInCta') }}
                <ArrowRight class="size-3.5 text-slate-100" />
              </button>
              <RouterLink
                v-else
                :to="{ name: 'cloud-dashboard' }"
                class="inline-flex h-8 items-center gap-1.5 rounded-md border border-blue-700 bg-blue-600 px-3 text-sm text-slate-50 outline-none hover:bg-blue-500 focus:bg-blue-500"
              >
                {{ t('dashboard.viewDeviceStatus') }}
                <ArrowRight class="size-3.5 text-slate-100" />
              </RouterLink>
              <button
                type="button"
                class="inline-flex h-8 items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-3 text-sm text-[var(--color-text)] outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
                @click="openLocalEntry"
              >
                {{ localEntryLabel }}
              </button>
            </div>
          </div>

          <div
            class="flex min-h-[520px] items-center"
            :aria-label="t('dashboard.cloudHeroImageAlt')"
          >
            <img
              :src="cloudHomeHeroUrl"
              :alt="t('dashboard.cloudHeroImageAlt')"
              class="w-full rounded-2xl object-cover shadow-2xl shadow-blue-950/30"
            />
          </div>
        </section>
      </div>
    </main>

    <DialogRoot :open="changePasswordDialogOpen" @update:open="changePasswordDialogOpen = $event">
      <DialogPortal>
        <DialogOverlay class="dialog-overlay" />
        <DialogContent class="dialog-content">
          <DialogTitle class="dialog-title">{{ t('cloud.changePassword') }}</DialogTitle>
          <form class="mt-4 space-y-3" @submit.prevent="submitChangePassword">
            <input
              v-model="currentPassword"
              type="password"
              class="h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-sm text-[var(--color-text)] outline-none"
              :placeholder="t('cloud.currentPassword')"
            />
            <input
              v-model="newPassword"
              type="password"
              class="h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-sm text-[var(--color-text)] outline-none"
              :placeholder="t('cloud.newPassword')"
            />
            <div class="flex justify-end gap-2 pt-2">
              <DialogClose as-child>
                <button
                  type="button"
                  class="h-8 rounded-md border border-[var(--color-border)] px-3 text-sm text-[var(--color-text)]"
                >
                  {{ t('common.cancel') }}
                </button>
              </DialogClose>
              <button
                type="submit"
                class="h-8 rounded-md border border-blue-700 bg-blue-600 px-3 text-sm text-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="changingPassword"
              >
                {{ changingPassword ? t('cloud.changingPassword') : t('cloud.changePassword') }}
              </button>
            </div>
          </form>
        </DialogContent>
      </DialogPortal>
    </DialogRoot>
  </section>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ArrowRight } from '@lucide/vue'
  import { RouterLink, useRouter } from 'vue-router'
  import AppHeader from '../layout/AppHeader.vue'
  import cloudHomeHeroUrl from '../../assets/cloud-home-hero-candidate.png'
  import {
    DialogClose,
    DialogContent,
    DialogOverlay,
    DialogPortal,
    DialogRoot,
    DialogTitle,
  } from 'reka-ui'
  import CloudAccountMenu from './CloudAccountMenu.vue'
  import { authChangePassword, authLogout } from '../../features/cloud/api'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useRuntimeConfigStore } from '../../store/runtimeConfig'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const router = useRouter()
  const runtimeConfig = useRuntimeConfigStore()
  const cloudAuth = useCloudAuthStore()
  const notifications = useNotificationsStore()
  const cloudAuthLoading = ref(false)
  const changePasswordDialogOpen = ref(false)
  const changingPassword = ref(false)
  const currentPassword = ref('')
  const newPassword = ref('')

  const cloudUserDisplayName = computed(
    () => cloudAuth.user?.display_name || cloudAuth.user?.email || t('dashboard.signedIn'),
  )
  const localEntryLabel = computed(() =>
    runtimeConfig.config.local.mode === 'cloud'
      ? t('dashboard.openLocalPage')
      : t('dashboard.switchToLocalMode'),
  )
  async function loadCloudHome() {
    cloudAuthLoading.value = true
    try {
      await cloudAuth.initializeAuth({ force: true })
    } catch (err) {
      notifications.notifyError(t('cloud.loginFailed'), err)
      return
    } finally {
      cloudAuthLoading.value = false
    }
  }

  async function openLocalEntry() {
    if (runtimeConfig.config.local.mode === 'cloud') {
      window.open(runtimeConfig.config.local.publicUrl, '_blank', 'noopener,noreferrer')
      return
    }
    if (!runtimeConfig.switchMode('local')) {
      return
    }
    await router.push({ name: 'home' })
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

  async function submitChangePassword() {
    if (changingPassword.value) {
      return
    }
    if (!currentPassword.value) {
      notifications.pushToast(
        'error',
        t('cloud.changePasswordFailed'),
        t('message.currentPasswordRequired'),
      )
      return
    }
    if (!newPassword.value) {
      notifications.pushToast(
        'error',
        t('cloud.changePasswordFailed'),
        t('message.newPasswordRequired'),
      )
      return
    }
    changingPassword.value = true
    try {
      await authChangePassword(currentPassword.value, newPassword.value)
      currentPassword.value = ''
      newPassword.value = ''
      changePasswordDialogOpen.value = false
      notifications.pushToast('success', t('cloud.changePasswordSucceeded'), '')
    } catch (err) {
      notifications.notifyError(t('cloud.changePasswordFailed'), err)
    } finally {
      changingPassword.value = false
    }
  }

  onMounted(() => {
    void loadCloudHome()
  })
</script>
