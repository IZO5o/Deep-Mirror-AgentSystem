<template>
  <section class="page">
    <div class="page-header">
      <div>
        <span class="page-kicker">Overview</span>
        <h1>Dashboard</h1>
        <p class="muted">Persistent workbench state across interviews, memory, training activity, and trace evidence.</p>
      </div>
      <button class="secondary" type="button" @click="load">Refresh</button>
    </div>

    <div class="metric-grid">
      <article class="metric">
        <strong>{{ recentInterviews.length }}</strong>
        <span>Recent interviews</span>
      </article>
      <article class="metric">
        <strong>{{ summary?.pending_memory_candidate_count || 0 }}</strong>
        <span>Pending memory candidates</span>
      </article>
      <article class="metric">
        <strong>{{ activeCoachingSessions.length }}</strong>
        <span>Active coaching</span>
      </article>
      <article class="metric">
        <strong>{{ activeMockInterviews.length }}</strong>
        <span>Active mocks</span>
      </article>
    </div>

    <div class="dashboard-grid">
      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Recent Interviews</h2>
            <p>Open a detail page without starting coaching or mock flow.</p>
          </div>
        </div>
        <EmptyState
          v-if="!recentInterviews.length"
          title="No interviews"
          message="Create an interview from the Interviews page."
        />
        <div v-else class="list">
          <RouterLink
            v-for="item in recentInterviews"
            :key="item.interview_id"
            class="row-card link-card"
            :to="`/interviews/${item.interview_id}`"
          >
            <div>
              <strong>{{ item.company_name || item.interview_id }}</strong>
              <span>{{ item.job_title || 'No job title' }} · {{ item.interview_round || 'round unset' }}</span>
            </div>
            <StatusBadge :status="item.status" />
          </RouterLink>
        </div>
      </section>

      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Memory Inbox</h2>
            <p>Pending items are candidates only until user review.</p>
          </div>
        </div>
        <div class="status-grid">
          <div class="status-item">
            <span>Pending</span>
            <strong>{{ summary?.pending_memory_candidate_count || 0 }}</strong>
          </div>
          <div class="status-item">
            <span>Recent shown</span>
            <strong>{{ pendingCandidates.length }}</strong>
          </div>
          <div class="status-item">
            <span>Boundary</span>
            <strong>accept/reject</strong>
          </div>
        </div>
        <RouterLink class="text-link" to="/memory">Review memory candidates</RouterLink>
      </section>

      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Practice Summary</h2>
            <p>Read-only aggregate from current practice states.</p>
          </div>
        </div>
        <dl class="summary-list">
          <div>
            <dt>Total states</dt>
            <dd>{{ practiceSummary.total_states || 0 }}</dd>
          </div>
          <div>
            <dt>Average mastery</dt>
            <dd>{{ practiceSummary.average_mastery_score || 0 }}</dd>
          </div>
          <div>
            <dt>Weak states</dt>
            <dd>{{ practiceSummary.weak_state_count || 0 }}</dd>
          </div>
          <div>
            <dt>Recent attempts</dt>
            <dd>{{ practiceSummary.recent_attempt_count || 0 }}</dd>
          </div>
        </dl>
      </section>

      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Trace Evaluation</h2>
            <p>Engineering evidence from recent agent traces.</p>
          </div>
          <RouterLink class="text-link" to="/trace">Open trace</RouterLink>
        </div>
        <dl class="summary-list">
          <div>
            <dt>Total traces</dt>
            <dd>{{ evaluationSummary.total_traces || 0 }}</dd>
          </div>
          <div>
            <dt>Passed</dt>
            <dd>{{ evaluationSummary.passed_traces || 0 }}</dd>
          </div>
          <div>
            <dt>Failed</dt>
            <dd>{{ evaluationSummary.failed_traces || 0 }}</dd>
          </div>
        </dl>
      </section>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { api } from '../api'
import EmptyState from '../components/EmptyState.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { runWithStatus, workbenchState as state } from '../workbenchState'

const summary = ref(null)

const recentInterviews = computed(() => summary.value?.recent_interviews || [])
const pendingCandidates = computed(() => summary.value?.recent_pending_candidates || [])
const activeCoachingSessions = computed(() => summary.value?.active_coaching_sessions || [])
const activeMockInterviews = computed(() => summary.value?.active_mock_interviews || [])
const practiceSummary = computed(() => summary.value?.practice_state_summary || {})
const evaluationSummary = computed(() => summary.value?.evaluation_summary || {})

async function load() {
  summary.value = await runWithStatus('dashboard', () => api.dashboardSummary(state.userId), 'Dashboard refreshed')
}

onMounted(load)
</script>

<style scoped>
.metric-grid,
.dashboard-grid {
  display: grid;
  gap: 12px;
}

.metric-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.metric {
  background: #f7f9fc;
  border: 1px solid #e4e9f0;
  border-radius: 8px;
  display: grid;
  gap: 4px;
  padding: 12px;
}

.metric strong {
  font-size: 24px;
}

.metric span {
  color: #667286;
  font-size: 12px;
  font-weight: 700;
}

.dashboard-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.summary-list {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin: 0;
}

.summary-list div {
  background: #f7f9fc;
  border: 1px solid #e4e9f0;
  border-radius: 6px;
  padding: 8px;
}

.summary-list dt {
  color: #667286;
  font-size: 12px;
  font-weight: 700;
}

.summary-list dd {
  font-size: 18px;
  font-weight: 750;
  margin: 2px 0 0;
}

.link-card,
.text-link {
  color: inherit;
  text-decoration: none;
}

.text-link {
  color: #1f5fd1;
  font-weight: 700;
}

@media (max-width: 900px) {
  .metric-grid,
  .dashboard-grid,
  .summary-list {
    grid-template-columns: 1fr;
  }
}
</style>
