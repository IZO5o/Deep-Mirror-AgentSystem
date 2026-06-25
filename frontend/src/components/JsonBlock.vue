<template>
  <details class="json-block" :open="open">
    <summary>{{ title }}</summary>
    <pre>{{ formattedValue }}</pre>
  </details>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  title: { type: String, required: true },
  value: { type: null, default: null },
  open: { type: Boolean, default: false },
})

const formattedValue = computed(() => {
  if (props.value === null || props.value === undefined || props.value === '') {
    return '-'
  }

  if (typeof props.value !== 'string') {
    return JSON.stringify(props.value, null, 2)
  }

  try {
    return JSON.stringify(JSON.parse(props.value), null, 2)
  } catch {
    return props.value
  }
})
</script>
