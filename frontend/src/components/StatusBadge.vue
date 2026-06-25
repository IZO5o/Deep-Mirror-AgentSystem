<template>
  <span class="status-badge" :data-status="normalizedStatus">{{ displayStatus }}</span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: { type: [String, Number, Boolean], default: '' },
})

const displayStatus = computed(() => (props.status === null || props.status === undefined || props.status === '' ? 'unknown' : String(props.status)))
const normalizedStatus = computed(() => displayStatus.value.toLowerCase().replace(/\s+/g, '_'))
</script>

<style scoped>
.status-badge {
  align-items: center;
  background: #eef2f7;
  border: 1px solid #d7dde6;
  border-radius: 999px;
  color: #2d3a4e;
  display: inline-flex;
  font-size: 12px;
  font-weight: 700;
  line-height: 1;
  min-height: 24px;
  padding: 4px 9px;
}

.status-badge[data-status="active"],
.status-badge[data-status="ready"],
.status-badge[data-status="completed"],
.status-badge[data-status="generated"],
.status-badge[data-status="passed"] {
  background: #eef9f1;
  border-color: #b9e4c4;
  color: #176331;
}

.status-badge[data-status="pending"],
.status-badge[data-status="running"],
.status-badge[data-status="in_progress"],
.status-badge[data-status="paused"] {
  background: #fff8e6;
  border-color: #f0d38a;
  color: #73520a;
}

.status-badge[data-status="failed"],
.status-badge[data-status="error"],
.status-badge[data-status="cancelled"],
.status-badge[data-status="canceled"] {
  background: #fff1f0;
  border-color: #ffc7c2;
  color: #9d1c16;
}
</style>
