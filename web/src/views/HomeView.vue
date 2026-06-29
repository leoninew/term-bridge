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
  import { useGatewayStore } from '../store/gateway'

  const { t } = useI18n()
  const router = useRouter()
  const gateway = useGatewayStore()

  onMounted(async () => {
    await gateway.initializeAuth()
    if (gateway.capabilities?.mode === 'local' || gateway.authenticated) {
      await router.replace({ name: 'dashboard' })
      return
    }
    await router.replace({ name: 'login' })
  })
</script>
