<template>
  <div class="termbridge-code-redirect">Redirecting to code workbench…</div>
</template>

<script setup lang="ts">
  import { onMounted } from 'vue'
  import { useRoute, useRouter } from 'vue-router'

  const route = useRoute()
  const router = useRouter()

  onMounted(async () => {
    await router.replace({
      name: 'cloud-workspace-code',
      params: {
        deviceId: route.params.deviceId,
        workspaceId: route.params.workspaceId,
      },
    })
  })
</script>
