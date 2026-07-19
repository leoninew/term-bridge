<template>
  <label class="flex flex-col gap-2">
    <span :class="['flex items-center justify-between gap-3', authFieldLabelClass]">
      <slot name="label">{{ label }}</slot>
    </span>
    <input
      :class="[authInputClass, inputClass]"
      :value="modelValue"
      v-bind="inputAttrs"
      @input="onInput"
    />
  </label>
</template>

<script setup lang="ts">
  import { computed, useAttrs } from 'vue'
  import { authFieldLabelClass, authInputClass } from './authUi'

  defineOptions({ inheritAttrs: false })

  withDefaults(
    defineProps<{
      modelValue: string
      label?: string
      inputClass?: string
    }>(),
    {
      label: '',
      inputClass: '',
    },
  )

  const emit = defineEmits<{
    'update:modelValue': [value: string]
  }>()

  const attrs = useAttrs()
  const inputAttrs = computed(() => {
    const rest = { ...attrs } as Record<string, unknown>
    delete rest.class
    return rest
  })

  function onInput(event: Event) {
    emit('update:modelValue', (event.target as HTMLInputElement).value)
  }
</script>
