<template>
  <section class="page">
    <div class="page-header">
      <div>
        <span class="page-kicker">Long-term memory review</span>
        <h1>Memory Inbox</h1>
        <p class="muted">memory_candidates -> user accept/reject -> memory_items. Agents never write formal memory directly.</p>
      </div>
      <button class="secondary" type="button" @click="loadAll">Refresh</button>
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Candidate Filters</h2>
          <p>Query global memory_candidates by status and source_ref_type for the current user.</p>
        </div>
      </div>
      <div class="filter-grid">
        <label>
          Status
          <select v-model="filters.status">
            <option value="pending">pending</option>
            <option value="accepted">accepted</option>
            <option value="rejected">rejected</option>
            <option value="">all</option>
          </select>
        </label>
        <label>
          Source type
          <select v-model="filters.source_ref_type">
            <option value="">all</option>
            <option value="review_report">review report</option>
            <option value="interview_question">interview question</option>
            <option value="agent_generated">agent generated</option>
            <option value="coaching_session">coaching session</option>
            <option value="mock_interview">mock interview</option>
          </select>
        </label>
        <button class="primary" type="button" @click="loadCandidates">Apply</button>
      </div>
    </section>

    <div class="memory-grid">
      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>memory_candidates</h2>
            <p>Pending candidates are proposals only; accept creates or updates formal memory_items.</p>
          </div>
          <StatusBadge :status="filters.status || 'all'" />
        </div>
        <EmptyState
          v-if="!candidates.length"
          title="No candidates"
          message="Generate memory candidates from review, completed coaching sessions, or completed mock interviews."
        />
        <div v-else class="memory-inline-list">
          <article v-for="candidate in candidates" :key="candidate.candidate_id" class="candidate-card">
            <div class="candidate-head">
              <div>
                <strong>{{ candidate.memory_type }}</strong>
                <small>{{ candidate.subject_key }}</small>
              </div>
              <StatusBadge :status="candidate.status" />
            </div>
            <p>{{ candidate.content }}</p>
            <small>{{ candidate.evidence || 'No evidence text' }}</small>
            <dl class="candidate-meta">
              <div>
                <dt>source_ref_type</dt>
                <dd>{{ candidate.source_ref_type || candidate.source || '-' }}</dd>
              </div>
              <div>
                <dt>source_ref_id</dt>
                <dd>{{ candidate.source_ref_id || candidate.interview_id || '-' }}</dd>
              </div>
              <div>
                <dt>confidence</dt>
                <dd>{{ candidate.confidence || '-' }}</dd>
              </div>
            </dl>
            <div class="actions">
              <button class="primary" type="button" :disabled="candidate.status !== 'pending'" @click="accept(candidate)">Accept</button>
              <button class="danger" type="button" :disabled="candidate.status !== 'pending'" @click="reject(candidate)">Reject</button>
              <RouterLink v-if="candidate.interview_id" class="text-link" :to="`/interviews/${candidate.interview_id}`">
                Interview detail
              </RouterLink>
            </div>
          </article>
        </div>
      </section>

      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>memory_items</h2>
            <p>Only accepted candidates appear here as durable long-term memory.</p>
          </div>
          <StatusBadge status="accepted" />
        </div>
        <EmptyState
          v-if="!items.length"
          title="No accepted memory"
          message="Accept pending candidates to create formal memory_items."
        />
        <div v-else class="memory-inline-list">
          <article v-for="item in items" :key="item.memory_id" class="candidate-card">
            <div class="candidate-head">
              <div>
                <strong>{{ item.memory_type }}</strong>
                <small>{{ item.subject_key }}</small>
              </div>
              <StatusBadge :status="item.status" />
            </div>
            <p>{{ item.content }}</p>
            <small>{{ item.evidence || 'No evidence text' }}</small>
            <dl class="candidate-meta">
              <div>
                <dt>source_candidate_id</dt>
                <dd>{{ item.source_candidate_id || '-' }}</dd>
              </div>
              <div>
                <dt>source_interview_id</dt>
                <dd>{{ item.source_interview_id || '-' }}</dd>
              </div>
              <div>
                <dt>confidence</dt>
                <dd>{{ item.confidence || '-' }}</dd>
              </div>
            </dl>
          </article>
        </div>
      </section>
    </div>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { api } from '../api'
import EmptyState from '../components/EmptyState.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { runWithStatus, workbenchState as state } from '../workbenchState'

const filters = reactive({
  status: 'pending',
  source_ref_type: '',
})
const candidates = ref([])
const items = ref([])

async function loadCandidates() {
  candidates.value = await runWithStatus(
    'memoryCandidates',
    () => api.listMemoryCandidates({ user_id: state.userId, ...filters }),
    'Memory candidates refreshed',
  )
}

async function loadItems() {
  items.value = await runWithStatus('memoryItems', () => api.listMemoryItems(state.userId), 'Memory items refreshed')
}

async function loadAll() {
  await loadCandidates()
  await loadItems()
}

async function accept(candidate) {
  await runWithStatus('acceptMemory', () => api.acceptMemoryCandidate(candidate.candidate_id), 'Candidate accepted')
  await loadAll()
}

async function reject(candidate) {
  await runWithStatus('rejectMemory', () => api.rejectMemoryCandidate(candidate.candidate_id), 'Candidate rejected')
  await loadAll()
}

onMounted(loadAll)
</script>

<style scoped>
.filter-grid {
  align-items: end;
  display: grid;
  gap: 10px;
  grid-template-columns: minmax(180px, 1fr) minmax(180px, 1fr) auto;
}

.filter-grid select {
  background: #ffffff;
  border: 1px solid #c8d0da;
  border-radius: 6px;
  color: #172033;
  min-height: 34px;
  padding: 8px 10px;
  width: 100%;
}

.memory-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.text-link {
  align-items: center;
  color: #1f5fd1;
  display: inline-flex;
  font-weight: 700;
  min-height: 34px;
  text-decoration: none;
}

@media (max-width: 900px) {
  .filter-grid,
  .memory-grid {
    grid-template-columns: 1fr;
  }
}
</style>
