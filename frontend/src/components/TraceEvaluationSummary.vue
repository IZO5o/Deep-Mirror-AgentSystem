<template>
  <section class="evaluation-summary">
    <div class="metric-grid dense-grid">
      <article class="metric mini-metric">
        <span>Total</span>
        <strong>{{ totals.total }}</strong>
      </article>
      <article class="metric mini-metric">
        <span>Passed</span>
        <strong>{{ totals.passed }}</strong>
      </article>
      <article class="metric mini-metric">
        <span>Failed</span>
        <strong>{{ totals.failed }}</strong>
      </article>
    </div>

    <EmptyState
      v-if="!results.length"
      title="No evaluation results"
      message="No trace evaluation results are available for this run."
    />

    <div v-else class="evaluation-list">
      <article
        v-for="(result, resultIndex) in results"
        :key="result.trace_id || result.step_name || resultIndex"
        class="evaluation-result list-row"
        :class="{ failed: isFailed(result) }"
      >
        <div class="list-row-main">
          <div class="list-row-title">
            <strong>{{ result.step_name || result.trace_id || `Result ${resultIndex + 1}` }}</strong>
            <StatusBadge :status="resultStatus(result)" />
          </div>
          <p v-if="result.message || result.reason">{{ result.message || result.reason }}</p>
        </div>

        <div v-if="result.checks?.length" class="check-list">
          <div
            v-for="(check, checkIndex) in result.checks"
            :key="check.id || check.name || check.check || checkIndex"
            class="check-row"
            :class="{ failed: isFailed(check) }"
          >
            <span>{{ check.reason || '-' }}</span>
            <StatusBadge :status="resultStatus(check)" />
          </div>
        </div>
      </article>
    </div>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import EmptyState from './EmptyState.vue'
import StatusBadge from './StatusBadge.vue'

const props = defineProps({
  report: { type: Object, default: null },
})

const results = computed(() => (Array.isArray(props.report?.results) ? props.report.results : []))

const isFailed = (item) => {
  if (!item) return false
  if (item.passed === false) return true
  return ['failed', 'error'].includes(String(item.status || item.result || '').toLowerCase())
}

const resultStatus = (item) => {
  if (!item) return 'unknown'
  if (item.status) return String(item.status)
  if (typeof item.passed === 'boolean') return item.passed ? 'passed' : 'failed'
  return String(item.result || 'unknown')
}

const totals = computed(() => {
  return {
    total: props.report?.total_traces ?? 0,
    passed: props.report?.passed_traces ?? 0,
    failed: props.report?.failed_traces ?? 0,
  }
})
</script>
