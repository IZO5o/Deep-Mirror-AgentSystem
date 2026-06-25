<template>
  <section class="page">
    <div class="page-header">
      <div>
        <span class="page-kicker">Agent observability</span>
        <h1>Engineering Trace</h1>
        <p class="muted">Read-only trace and evaluation evidence for agent decisions.</p>
      </div>
      <button class="secondary" type="button" :disabled="isTraceLoading" @click="loadAll">Refresh</button>
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Trace Filters</h2>
          <p>Query existing agent_decision_traces without changing business state.</p>
        </div>
        <StatusBadge :status="activeFilters.status || 'all'" />
      </div>

      <form class="trace-filter-grid" @submit.prevent="applyFilters">
        <label>
          user_id
          <input v-model.trim="filters.user_id" autocomplete="off" placeholder="user_001" />
        </label>
        <label>
          interview_id
          <input v-model.trim="filters.interview_id" autocomplete="off" placeholder="interview_id" />
        </label>
        <label>
          source_type
          <select v-model="filters.source_type">
            <option value="">all</option>
            <option value="coaching_plan">coaching_plan</option>
            <option value="coaching_session">coaching_session</option>
            <option value="mock_interview">mock_interview</option>
            <option value="memory_candidate_generation">memory_candidate_generation</option>
            <option value="review_report">review_report</option>
          </select>
        </label>
        <label>
          source_id
          <input v-model.trim="filters.source_id" autocomplete="off" placeholder="source_id" />
        </label>
        <label>
          agent_type
          <input v-model.trim="filters.agent_type" autocomplete="off" placeholder="agent_type" />
        </label>
        <label>
          step_name
          <input v-model.trim="filters.step_name" autocomplete="off" placeholder="step_name" />
        </label>
        <label>
          status
          <select v-model="filters.status">
            <option value="">all</option>
            <option value="succeeded">succeeded</option>
            <option value="failed">failed</option>
          </select>
        </label>
        <label>
          limit
          <input v-model.number="filters.limit" min="1" max="200" type="number" />
        </label>
        <div class="trace-filter-actions">
          <button class="primary" type="submit" :disabled="isTraceLoading">Apply Filters</button>
          <button class="secondary" type="button" :disabled="isTraceLoading" @click="resetFilters">Reset</button>
        </div>
      </form>
    </section>

    <ErrorNotice v-if="traceError" :message="traceError" />

    <div class="trace-layout">
      <section class="panel trace-list-panel">
        <div class="panel-title">
          <div>
            <h2>Trace List</h2>
            <p>{{ traces.length }} records loaded.</p>
          </div>
          <StatusBadge :status="isTraceLoading ? 'running' : 'ready'" />
        </div>

        <EmptyState
          v-if="!traceError && !traces.length"
          title="No traces"
          message="Apply filters or generate coaching/mock activity to produce trace records."
        />

        <div v-else class="trace-list">
          <button
            v-for="(trace, index) in traces"
            :key="trace.trace_id || `${trace.created_at}-${index}`"
            class="trace-row"
            :class="{ active: isSelected(trace), failed: isFailed(trace) }"
            type="button"
            @click="selectTrace(trace)"
          >
            <div class="trace-row-main">
              <span class="trace-time">{{ formatTime(trace.created_at) }}</span>
              <strong>{{ trace.agent_type || '-' }}</strong>
              <small>{{ trace.source_type || '-' }} · {{ trace.source_id || '-' }}</small>
              <span>{{ trace.step_name || '-' }}</span>
              <p v-if="trace.error_summary || trace.error_message">
                {{ trace.error_summary || trace.error_message }}
              </p>
            </div>
            <StatusBadge :status="trace.status" />
          </button>
        </div>
      </section>

      <section class="panel trace-detail-panel">
        <div class="panel-title">
          <div>
            <h2>Trace Detail</h2>
            <p>{{ selectedTrace?.trace_id || 'Select a trace row.' }}</p>
          </div>
          <StatusBadge :status="selectedTrace?.status || 'none'" />
        </div>

        <EmptyState
          v-if="!selectedTrace"
          title="No trace selected"
          message="Select a trace from the list to inspect snapshots and service actions."
        />

        <div v-else class="detail-stack">
          <dl class="field-stack">
            <div class="field-row">
              <dt>user_id</dt>
              <dd>{{ selectedTrace.user_id || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>interview_id</dt>
              <dd>{{ selectedTrace.interview_id || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>source</dt>
              <dd>{{ selectedTrace.source_type || '-' }} / {{ selectedTrace.source_id || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>step</dt>
              <dd>{{ selectedTrace.step_name || '-' }}</dd>
            </div>
          </dl>

          <ErrorNotice v-if="selectedTrace.error_message" :message="selectedTrace.error_message" />
          <JsonBlock title="Selected context snapshot" :value="selectedTrace.selected_context_snapshot" open />
          <JsonBlock title="Input snapshot" :value="selectedTrace.input_snapshot" />
          <JsonBlock title="Raw agent output" :value="selectedTrace.raw_agent_output" />
          <JsonBlock title="Parsed decision" :value="selectedTrace.parsed_decision" open />
          <JsonBlock title="Service actions" :value="selectedTrace.service_actions" open />
        </div>
      </section>
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Evaluation Report</h2>
          <p>Rule checks computed from the same trace filters.</p>
        </div>
        <StatusBadge :status="evaluationError ? 'failed' : 'ready'" />
      </div>
      <ErrorNotice v-if="evaluationError" :message="evaluationError" />
      <TraceEvaluationSummary v-else :report="evaluationReport" />
    </section>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import EmptyState from '../components/EmptyState.vue'
import ErrorNotice from '../components/ErrorNotice.vue'
import JsonBlock from '../components/JsonBlock.vue'
import StatusBadge from '../components/StatusBadge.vue'
import TraceEvaluationSummary from '../components/TraceEvaluationSummary.vue'
import { runWithStatus, workbenchState as state } from '../workbenchState'

const route = useRoute()
const router = useRouter()

const emptyFilters = () => ({
  user_id: state.userId,
  interview_id: '',
  source_type: '',
  source_id: '',
  agent_type: '',
  step_name: '',
  status: '',
  limit: 25,
})

const filters = reactive(emptyFilters())
const activeFilters = ref(emptyFilters())
const traces = ref([])
const selectedTraceId = ref('')
const traceError = ref('')
const evaluationError = ref('')
const evaluationReport = ref(null)

const isTraceLoading = computed(() => state.loadingKeys.has('engineeringTraces'))
const selectedTrace = computed(() => traces.value.find((trace) => traceKey(trace) === selectedTraceId.value) || null)

function firstQueryValue(value) {
  return Array.isArray(value) ? value[0] : value
}

function queryString(query, key) {
  return String(firstQueryValue(query[key]) || '').trim()
}

function queryLimit(query) {
  const parsed = Number.parseInt(firstQueryValue(query.limit), 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 25
}

function syncFiltersFromRoute() {
  const nextFilters = emptyFilters()
  Object.assign(nextFilters, {
    user_id: queryString(route.query, 'user_id') || state.userId,
    interview_id: queryString(route.query, 'interview_id'),
    source_type: queryString(route.query, 'source_type'),
    source_id: queryString(route.query, 'source_id'),
    agent_type: queryString(route.query, 'agent_type'),
    step_name: queryString(route.query, 'step_name'),
    status: queryString(route.query, 'status'),
    limit: queryLimit(route.query),
  })
  Object.assign(filters, nextFilters)
  activeFilters.value = { ...nextFilters }
}

function requestFilters() {
  return {
    ...activeFilters.value,
    limit: Number.parseInt(activeFilters.value.limit, 10) || 25,
  }
}

function queryFromFilters(nextFilters) {
  const query = {}
  Object.entries(nextFilters).forEach(([key, value]) => {
    const normalized = key === 'limit' ? Number.parseInt(value, 10) || 25 : String(value || '').trim()
    if (normalized !== '') {
      query[key] = normalized
    }
  })
  return query
}

async function loadTraces() {
  traceError.value = ''
  try {
    const loaded = await runWithStatus(
      'engineeringTraces',
      () => api.listAgentDecisionTraces(requestFilters()),
      'Engineering traces refreshed',
    )
    traces.value = Array.isArray(loaded) ? loaded : []
    keepSelection()
  } catch (error) {
    traces.value = []
    selectedTraceId.value = ''
    traceError.value = error.message || String(error)
  }
}

async function loadEvaluation() {
  evaluationError.value = ''
  try {
    evaluationReport.value = await api.listAgentEvaluations(requestFilters())
  } catch (error) {
    evaluationReport.value = null
    evaluationError.value = error.message || String(error)
  }
}

async function loadAll() {
  await loadTraces()
  await loadEvaluation()
}

async function applyFilters() {
  await updateQueryAndLoad(queryFromFilters(filters))
}

async function resetFilters() {
  Object.assign(filters, emptyFilters())
  await updateQueryAndLoad(queryFromFilters(filters))
}

async function updateQueryAndLoad(query) {
  const target = router.resolve({ path: '/trace', query })
  if (target.fullPath === route.fullPath) {
    syncFiltersFromRoute()
    await loadAll()
    return
  }
  await router.push({ path: '/trace', query })
}

function keepSelection() {
  if (traces.value.some((trace) => traceKey(trace) === selectedTraceId.value)) {
    return
  }
  selectedTraceId.value = traces.value.length ? traceKey(traces.value[0]) : ''
}

function selectTrace(trace) {
  selectedTraceId.value = traceKey(trace)
}

function isSelected(trace) {
  return traceKey(trace) === selectedTraceId.value
}

function traceKey(trace) {
  return trace?.trace_id || `${trace?.created_at || ''}:${trace?.agent_type || ''}:${trace?.step_name || ''}`
}

function isFailed(trace) {
  return String(trace?.status || '').toLowerCase() === 'failed'
}

function formatTime(value) {
  if (!value) return '-'
  const numeric = Number(value)
  const date = Number.isFinite(numeric) ? new Date(numeric < 100000000000 ? numeric * 1000 : numeric) : new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return date.toLocaleString()
}

watch(
  () => route.query,
  async () => {
    syncFiltersFromRoute()
    await loadAll()
  },
)

onMounted(async () => {
  syncFiltersFromRoute()
  await loadAll()
})
</script>

<style scoped>
.trace-filter-grid {
  align-items: end;
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.trace-filter-grid select {
  background: #ffffff;
  border: 1px solid #c8d0da;
  border-radius: 6px;
  color: #172033;
  min-height: 34px;
  padding: 8px 10px;
  width: 100%;
}

.trace-filter-actions {
  display: flex;
  gap: 8px;
}

.trace-layout {
  display: grid;
  gap: 14px;
  grid-template-columns: minmax(340px, 0.9fr) minmax(0, 1.4fr);
}

.trace-list-panel,
.trace-detail-panel {
  align-content: start;
}

.trace-list {
  display: grid;
  gap: 8px;
  max-height: 680px;
  overflow: auto;
  padding-right: 4px;
}

.trace-row {
  align-items: flex-start;
  background: #ffffff;
  border: 1px solid #e4e9f0;
  border-radius: 8px;
  display: flex;
  gap: 10px;
  justify-content: space-between;
  min-height: 118px;
  padding: 10px;
  text-align: left;
  width: 100%;
}

.trace-row:hover,
.trace-row.active {
  border-color: #1f5fd1;
  box-shadow: inset 3px 0 0 #1f5fd1;
}

.trace-row.failed {
  background: #fff8f7;
  border-color: #ffc7c2;
}

.trace-row.failed.active {
  box-shadow: inset 3px 0 0 #d64b42;
}

.trace-row-main {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.trace-row-main strong,
.trace-row-main span,
.trace-row-main small,
.trace-row-main p {
  overflow-wrap: anywhere;
}

.trace-row-main small,
.trace-time {
  color: #667286;
}

.trace-row-main p {
  color: #9d1c16;
}

.detail-stack {
  display: grid;
  gap: 10px;
}

@media (max-width: 980px) {
  .trace-filter-grid,
  .trace-layout {
    grid-template-columns: 1fr;
  }
}
</style>
