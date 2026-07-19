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
      class="inline-flex size-9 items-center justify-center rounded-md text-sm text-[var(--color-text)] outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)] sm:h-9 sm:w-auto sm:max-w-56 sm:gap-1.5 sm:justify-start sm:px-3"
      :title="userTitle"
      :aria-label="userTitle"
    >
      <User class="size-4 shrink-0 text-[var(--color-text-subtle)]" />
      <span class="hidden truncate sm:inline">{{ userDisplayName }}</span>
      <ChevronDown class="hidden size-4 shrink-0 text-[var(--color-text-subtle)] sm:block" />
    </DropdownMenuTrigger>
    <DropdownMenuPortal>
      <DropdownMenuContent
        align="end"
        :side-offset="8"
        class="z-50 min-w-40 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] p-1 text-sm text-[var(--color-text)] shadow-xl"
      >
        <DropdownMenuItem
          :disabled="!canChangePassword"
          class="flex items-center gap-2 rounded px-2 py-1.5 outline-none data-[disabled]:cursor-not-allowed data-[disabled]:opacity-45 data-[highlighted]:bg-[var(--color-control-hover)] data-[disabled]:data-[highlighted]:bg-transparent"
          :class="canChangePassword ? 'cursor-pointer' : 'cursor-not-allowed'"
          :title="changePasswordTitle"
          @select="onChangePasswordSelect"
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

  const props = withDefaults(
    defineProps<{
      authenticated: boolean
      userDisplayName: string
      userEmail: string
      canChangePassword?: boolean
    }>(),
    {
      canChangePassword: false,
    },
  )

  const emit = defineEmits<{
    login: []
    logout: []
    changePassword: []
  }>()

  const { t } = useI18n()

  const userTitle = computed(() => props.userEmail || props.userDisplayName)
  const changePasswordTitle = computed(() =>
    props.canChangePassword ? t('cloud.changePassword') : t('cloud.changePasswordUnavailable'),
  )

  function onChangePasswordSelect(event: { preventDefault: () => void }) {
    if (!props.canChangePassword) {
      event.preventDefault()
      return
    }
    emit('changePassword')
  }
</script>
