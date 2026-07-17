<template>
  <button
    v-if="!authenticated"
    type="button"
    class="inline-flex size-9 items-center justify-center rounded-md text-[var(--color-text)] outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
    :title="t('cloud.signIn')"
    :aria-label="t('cloud.signIn')"
    @click="emit('login')"
  >
    <LogIn class="size-4 text-[var(--color-text-subtle)]" />
  </button>

  <DropdownMenuRoot v-else>
    <DropdownMenuTrigger
      class="inline-flex h-9 max-w-[9.5rem] items-center gap-1.5 rounded-md px-2 text-sm text-[var(--color-text)] outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)] sm:max-w-56 sm:px-3"
      :title="userTitle"
      :aria-label="userTitle"
    >
      <User class="size-4 shrink-0 text-[var(--color-text-subtle)]" />
      <span class="truncate">{{ userDisplayName }}</span>
      <ChevronDown class="size-4 shrink-0 text-[var(--color-text-subtle)]" />
    </DropdownMenuTrigger>
    <DropdownMenuPortal>
      <DropdownMenuContent
        align="end"
        :side-offset="8"
        class="z-50 min-w-40 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] p-1 text-sm text-[var(--color-text)] shadow-xl"
      >
        <DropdownMenuItem
          v-if="showChangePassword"
          class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
          @select="emit('changePassword')"
        >
          <KeyRound class="size-4 text-[var(--color-text-subtle)]" />
          {{ t('cloud.changePassword') }}
        </DropdownMenuItem>
        <DropdownMenuItem
          class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
          @select="emit('logout')"
        >
          <LogOut class="size-4 text-[var(--color-text-subtle)]" />
          {{ t('cloud.logout') }}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ChevronDown, KeyRound, LogIn, LogOut, User } from '@lucide/vue'
  import {
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuPortal,
    DropdownMenuRoot,
    DropdownMenuTrigger,
  } from 'reka-ui'

  const props = defineProps<{
    authenticated: boolean
    userDisplayName: string
    userEmail: string
    showChangePassword?: boolean
  }>()

  const emit = defineEmits<{
    login: []
    logout: []
    changePassword: []
  }>()

  const { t } = useI18n()

  const userTitle = computed(() => props.userEmail || props.userDisplayName)
</script>
