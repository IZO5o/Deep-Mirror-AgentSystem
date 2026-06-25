<template>
  <section class="page">
    <div class="page-header">
      <div>
        <span class="page-kicker">Interview detail</span>
        <h1>{{ interview.company_name || 'Interview Detail' }}</h1>
        <p class="muted">{{ interview.job_title || interviewId }}</p>
      </div>
      <button class="secondary" type="button" @click="load">Refresh</button>
    </div>

    <section v-if="detail" class="panel">
      <div class="panel-title">
        <div>
          <h2>Metadata and Status</h2>
          <p>{{ interview.interview_round || 'round unset' }} · {{ interview.interview_type || 'type unset' }}</p>
        </div>
        <StatusBadge :status="interview.status" />
      </div>
      <dl class="detail-grid">
        <div>
          <dt>interview_id</dt>
          <dd>{{ interview.interview_id }}</dd>
        </div>
        <div>
          <dt>user_id</dt>
          <dd>{{ interview.user_id }}</dd>
        </div>
        <div>
          <dt>created_at</dt>
          <dd>{{ formatTime(interview.created_at) }}</dd>
        </div>
        <div>
          <dt>updated_at</dt>
          <dd>{{ formatTime(interview.updated_at) }}</dd>
        </div>
      </dl>
    </section>

    <section v-if="detail" class="panel">
      <div class="panel-title">
        <div>
          <h2>Transcript</h2>
          <p>Manual transcript save updates the interview material and prepares review.</p>
        </div>
        <StatusBadge :status="detail.transcript ? 'saved' : 'missing'" />
      </div>
      <textarea v-model="transcriptContent" rows="10" placeholder="Paste transcript for this interview." />
      <div class="actions">
        <button class="primary" type="button" :disabled="!transcriptContent.trim()" @click="saveTranscript">Save Transcript</button>
        <button class="secondary" type="button" :disabled="!transcriptContent.trim()" @click="review">Run Review</button>
      </div>
    </section>

    <div v-if="detail" class="detail-columns">
      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Review Summary</h2>
            <p>Review can generate questions and memory_candidates.</p>
          </div>
          <StatusBadge :status="detail.review_report?.status || 'none'" />
        </div>
        <p class="summary-box">{{ detail.review_report?.overall_summary || 'No review report yet.' }}</p>
        <div v-if="detail.review_report" class="summary-stack">
          <ListBlock title="Strengths" :items="detail.review_report.strengths" />
          <ListBlock title="Weaknesses" :items="detail.review_report.weaknesses" />
          <ListBlock title="Suggested preparation" :items="detail.review_report.suggested_preparation" />
        </div>
      </section>

      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Questions</h2>
            <p>Structured items extracted from the review report.</p>
          </div>
        </div>
        <EmptyState v-if="!questions.length" title="No questions" message="Run review after transcript is ready." />
        <div v-else class="list compact">
          <article v-for="question in questions" :key="question.question_id" class="question-card">
            <div class="question-head">
              <strong>#{{ question.sequence }} {{ question.question }}</strong>
              <StatusBadge :status="question.answer_quality || question.difficulty || 'question'" />
            </div>
            <p>{{ question.answer }}</p>
            <small>{{ question.weakness_summary || question.improvement_suggestion }}</small>
          </article>
        </div>
      </section>
    </div>

    <section v-if="detail" class="panel">
      <div class="panel-title">
        <div>
          <h2>Memory Candidates</h2>
          <p>These stay in memory_candidates until the user accepts or rejects them in Memory Inbox.</p>
        </div>
        <RouterLink class="text-link" to="/memory">Open Memory Inbox</RouterLink>
      </div>
      <EmptyState
        v-if="!memoryCandidates.length"
        title="No candidates"
        message="Run review or complete training flows that generate candidates; accepted memory_items are created only after user approval."
      />
      <div v-else class="memory-inline-list">
        <article v-for="candidate in memoryCandidates" :key="candidate.candidate_id" class="candidate-card">
          <div class="candidate-head">
            <strong>{{ candidate.memory_type }}</strong>
            <StatusBadge :status="candidate.status" />
          </div>
          <p>{{ candidate.content }}</p>
          <small>{{ candidate.source_ref_type || candidate.source || 'review' }} · {{ candidate.evidence || 'No evidence text' }}</small>
        </article>
      </div>
    </section>

    <section v-if="detail" class="panel">
      <div class="panel-title">
        <div>
          <h2>Training Links</h2>
          <p>Detail view records available coaching/mock context without starting either flow.</p>
        </div>
      </div>
      <dl class="detail-grid">
        <div>
          <dt>coaching_plan</dt>
          <dd>{{ detail.coaching_plan?.plan_id || '-' }}</dd>
        </div>
        <div>
          <dt>coaching_tasks</dt>
          <dd>{{ detail.coaching_tasks?.length || 0 }}</dd>
        </div>
        <div>
          <dt>latest_mock</dt>
          <dd>{{ detail.latest_mock_interview?.mock_id || '-' }}</dd>
        </div>
        <div>
          <dt>mock_status</dt>
          <dd>{{ detail.latest_mock_interview?.status || '-' }}</dd>
        </div>
      </dl>
    </section>

    <section v-else-if="loadError" class="panel">
      <div class="panel-title">
        <div>
          <h2>Interview detail unavailable</h2>
          <p>{{ loadError }}</p>
        </div>
        <button class="secondary" type="button" @click="load">Retry</button>
      </div>
    </section>

    <EmptyState
      v-else
      title="Loading interview detail"
      :message="`Fetching detail for interview_id ${interviewId || '-'}.`"
    />
  </section>
</template>

<script setup>
import { computed, defineComponent, h, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { api } from '../api'
import EmptyState from '../components/EmptyState.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { rememberSelection, runWithStatus, workbenchState as state } from '../workbenchState'

const props = defineProps({
  interviewId: { type: String, required: true },
})

const detail = ref(null)
const transcriptContent = ref('')
const loadError = ref('')

const interview = computed(() => detail.value?.interview || {})
const questions = computed(() => detail.value?.questions || [])
const memoryCandidates = computed(() => detail.value?.memory_candidates || [])

const ListBlock = defineComponent({
  props: {
    title: { type: String, required: true },
    items: { type: Array, default: () => [] },
  },
  setup(componentProps) {
    return () =>
      h('div', { class: 'mini-list' }, [
        h('strong', componentProps.title),
        componentProps.items?.length
          ? h('ul', componentProps.items.map((item) => h('li', item)))
          : h('p', 'No items.'),
      ])
  },
})

function formatTime(value) {
  if (!value) {
    return '-'
  }
  return new Date(value * 1000).toLocaleString()
}

async function load() {
  loadError.value = ''
  try {
    detail.value = await runWithStatus(
      'interviewDetail',
      () => api.interviewDetail(props.interviewId, state.userId),
      'Interview detail refreshed',
    )
    transcriptContent.value = detail.value?.transcript?.content || ''
    rememberSelection('selectedInterviewId', props.interviewId)
    if (detail.value?.coaching_plan?.plan_id) {
      rememberSelection('selectedPlanId', detail.value.coaching_plan.plan_id)
    }
    if (detail.value?.latest_mock_interview?.mock_id) {
      rememberSelection('selectedMockId', detail.value.latest_mock_interview.mock_id)
    }
  } catch (error) {
    detail.value = null
    transcriptContent.value = ''
    loadError.value = error.message || String(error)
  }
}

async function saveTranscript() {
  await runWithStatus(
    'saveTranscript',
    () =>
      api.upsertTranscript(props.interviewId, {
        user_id: state.userId,
        content: transcriptContent.value,
        source_type: 'manual_text',
      }),
    'Transcript saved',
  )
  await load()
}

async function review() {
  await runWithStatus('reviewInterview', () => api.triggerReview(props.interviewId), 'Review requested')
  await load()
}

watch(() => props.interviewId, load)
onMounted(load)
</script>

<style scoped>
.detail-grid {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin: 0;
}

.detail-grid div {
  background: #f7f9fc;
  border: 1px solid #e4e9f0;
  border-radius: 6px;
  min-width: 0;
  padding: 8px;
}

.detail-grid dt {
  color: #667286;
  font-size: 12px;
  font-weight: 700;
}

.detail-grid dd {
  margin: 2px 0 0;
  overflow-wrap: anywhere;
}

.detail-columns {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.summary-stack {
  display: grid;
  gap: 8px;
}

.mini-list {
  background: #ffffff;
  border: 1px solid #e4e9f0;
  border-radius: 6px;
  display: grid;
  gap: 6px;
  padding: 8px;
}

.mini-list ul {
  margin: 0;
  padding-left: 18px;
}

.mini-list p,
.mini-list li,
.question-card p,
.question-card small {
  color: #405069;
  overflow-wrap: anywhere;
}

.question-card {
  background: #ffffff;
  border: 1px solid #e2e7ee;
  border-radius: 6px;
  display: grid;
  gap: 8px;
  padding: 10px;
}

.question-head {
  align-items: flex-start;
  display: flex;
  gap: 10px;
  justify-content: space-between;
}

.text-link {
  color: #1f5fd1;
  font-weight: 700;
  text-decoration: none;
}

@media (max-width: 900px) {
  .detail-grid,
  .detail-columns {
    grid-template-columns: 1fr;
  }
}
</style>
