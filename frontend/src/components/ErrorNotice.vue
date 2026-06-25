<template>
  <div class="notice error" role="alert">{{ message }}</div>
</template>

<script setup>
defineProps({
  message: { type: String, required: true },
})
</script>
