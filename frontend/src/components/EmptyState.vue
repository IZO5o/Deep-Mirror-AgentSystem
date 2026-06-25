<template>
  <div class="empty-state">
    <strong>{{ title }}</strong>
    <span>{{ message }}</span>
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, required: true },
  message: { type: String, required: true },
})
</script>

<style scoped>
.empty-state {
  background: #f7f9fc;
  border: 1px dashed #c8d0da;
  border-radius: 8px;
  color: #667286;
  display: grid;
  gap: 5px;
  padding: 18px;
}

.empty-state strong {
  color: #172033;
  font-size: 15px;
}
</style>
