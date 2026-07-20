<template>
  <AuthStatusScreen
    status="loading"
    :title="t('cloud.checkingAuth')"
    :description="t('cloud.externalSigningInHint')"
  />
</template>

<script setup lang="ts">
  import { onMounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import AuthStatusScreen from '../../components/cloud/AuthStatusScreen.vue'

  const { t } = useI18n()
  const router = useRouter()

  onMounted(async () => {
    await router.replace({ name: 'home' })
  })
</script>
