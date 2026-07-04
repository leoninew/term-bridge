<template>
  <section
    class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text-muted)]"
  >
    {{ t('gateway.checkingAuth') }}
  </section>
</template>

<script setup lang="ts">
  import { onMounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { useAppModeStore } from '../store/appMode'
  import { useGatewayStore } from '../store/gateway'

  const { t } = useI18n()
  const router = useRouter()
  const appMode = useAppModeStore()
  const gateway = useGatewayStore()

  onMounted(async () => {
    if (appMode.effectiveMode === 'agent') {
      await gateway.initializeAuth()
      await router.replace({ name: 'agent-dashboard' })
      return
    }
    await gateway.initializeAuth()
    if (gateway.authenticated) {
      await router.replace({ name: 'cloud-dashboard' })
      return
    }
    await router.replace({ name: 'login' })
  })
</script>
