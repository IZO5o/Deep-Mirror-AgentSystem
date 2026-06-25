<template>
  <section class="page">
    <div class="page-header">
      <div>
        <span class="page-kicker">Practice loop</span>
        <h1>Mock Interview</h1>
        <p class="muted">Run and inspect the mock interviewer state machine for a selected interview.</p>
      </div>
      <StatusBadge :status="pageStatus" />
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Interview Context</h2>
          <p>Load an interview, choose plan context, then start or resume its mock session.</p>
        </div>
        <StatusBadge :status="interviewStatus" />
      </div>

      <form class="mock-context-grid" @submit.prevent="loadInterview">
        <label>
          interview_id
          <input v-model.trim="context.interview_id" autocomplete="off" placeholder="interview_id" @change="rememberInterview" />
        </label>
        <label>
          plan_id
          <input v-model.trim="context.plan_id" autocomplete="off" placeholder="optional plan_id" @change="rememberPlan" />
        </label>
        <label>
          target_round
          <input v-model.trim="context.target_round" autocomplete="off" placeholder="second_round" />
        </label>
        <label>
          user_id
          <input v-model.trim="context.user_id" autocomplete="off" placeholder="user_001" />
        </label>
        <div class="context-actions">
          <button class="secondary" type="submit" :disabled="!canUseInterview || isLoading('mockInterviewDetail')">Load Interview</button>
          <button class="primary" type="button" :disabled="!canUseInterview || isLoading('startMockInterview')" @click="startOrResumeMock">
            Start/Resume Mock
          </button>
          <button class="secondary" type="button" :disabled="!selectedMockId || isLoading('getMockInterview')" @click="refreshMock">
            Refresh Mock
          </button>
          <button class="secondary" type="button" :disabled="!canControlMock || isLoading('completeMockInterview')" @click="completeMock">
            Complete
          </button>
          <button class="danger" type="button" :disabled="!canControlMock || isLoading('cancelMockInterview')" @click="cancelMock">
            Cancel
          </button>
        </div>
      </form>

      <EmptyState
        v-if="!context.interview_id"
        title="No interview selected"
        message="Enter an interview_id or select one from the workbench, then load the interview context."
      />

      <dl v-else class="mock-detail-grid">
        <div>
          <dt>company</dt>
          <dd>{{ interview.company_name || '-' }}</dd>
        </div>
        <div>
          <dt>job title</dt>
          <dd>{{ interview.job_title || '-' }}</dd>
        </div>
        <div>
          <dt>interview status</dt>
          <dd><StatusBadge :status="interview.status || 'unknown'" /></dd>
        </div>
        <div>
          <dt>plan_id</dt>
          <dd>{{ context.plan_id || '-' }}</dd>
        </div>
        <div>
          <dt>latest mock</dt>
          <dd>{{ selectedMockId || '-' }}</dd>
        </div>
      </dl>
    </section>

    <div class="mock-layout">
      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Mock State</h2>
            <p>{{ mock?.overall_goal || 'Start or refresh a mock interview to inspect state.' }}</p>
          </div>
          <StatusBadge :status="mock?.status || 'none'" />
        </div>

        <EmptyState v-if="!mock" title="No mock loaded" message="Start, resume, or refresh a mock interview." />

        <div v-else class="detail-stack">
          <dl class="field-stack">
            <div class="field-row">
              <dt>mock_id</dt>
              <dd>{{ mock.mock_id || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>status</dt>
              <dd><StatusBadge :status="mock.status || 'unknown'" /></dd>
            </div>
            <div class="field-row">
              <dt>current_topic</dt>
              <dd>{{ mock.current_topic || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>current_turn</dt>
              <dd>{{ mock.current_turn ?? '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>overall_goal</dt>
              <dd>{{ mock.overall_goal || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>current question</dt>
              <dd>{{ currentQuestion }}</dd>
            </div>
            <div v-if="mock.error_message" class="field-row has-error">
              <dt>error_message</dt>
              <dd>{{ mock.error_message }}</dd>
            </div>
          </dl>

          <RouterLink v-if="selectedMockId" class="text-link" :to="traceLink">Open Mock Trace</RouterLink>
        </div>
      </section>

      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Answer Composer</h2>
            <p>Submit an answer to the current interviewer question.</p>
          </div>
          <StatusBadge :status="canSubmitTurn ? 'ready' : 'blocked'" />
        </div>

        <textarea v-model="answerInput" rows="8" :disabled="!canAnswerMock" placeholder="Type your answer for the mock interviewer." />
        <div class="secondary-actions">
          <button class="primary" type="button" :disabled="!canSubmitTurn || isLoading('submitMockTurn')" @click="submitTurn">
            Submit Answer
          </button>
          <button class="secondary" type="button" :disabled="!answerInput" @click="clearAnswer">Clear</button>
        </div>
      </section>
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Latest Result</h2>
          <p>Feedback, score, next action, and practice update summary if the backend returned one.</p>
        </div>
        <StatusBadge :status="latestResultStatus" />
      </div>

      <EmptyState v-if="!latestResultTurn && !mock?.last_feedback && !mock?.final_summary" title="No result yet" message="Submit or refresh a mock turn to see feedback." />

      <div v-else class="mock-result-grid">
        <TurnTimelineItem v-if="latestResultTurn" :turn="latestResultTurn" kind="mock" />

        <article class="result-card" :class="{ 'has-error': Boolean(latestResultTurn?.error_message || mock?.error_message) }">
          <header class="item-head">
            <div>
              <strong>Session result</strong>
              <small>{{ selectedMockId || '-' }}</small>
            </div>
            <StatusBadge :status="mock?.status || latestResultTurn?.agent_action || 'mock'" />
          </header>
          <dl class="field-stack">
            <div class="field-row">
              <dt>feedback</dt>
              <dd>{{ latestFeedback }}</dd>
            </div>
            <div class="field-row">
              <dt>score</dt>
              <dd>{{ latestResultTurn?.score ?? '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>next action</dt>
              <dd>{{ latestNextAction }}</dd>
            </div>
            <div v-if="practiceUpdateSummary" class="field-row">
              <dt>practice update</dt>
              <dd>{{ practiceUpdateSummary }}</dd>
            </div>
            <div v-if="mock?.final_summary" class="field-row">
              <dt>final summary</dt>
              <dd>{{ mock.final_summary }}</dd>
            </div>
          </dl>
        </article>
      </div>
    </section>

    <div class="mock-layout">
      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Practice Evidence</h2>
            <p>Read-only practice states for the selected user.</p>
          </div>
          <StatusBadge :status="practiceError ? 'failed' : 'ready'" />
        </div>

        <div class="secondary-actions">
          <button class="secondary" type="button" :disabled="isLoading('mockPracticeStates')" @click="loadPracticeStates">
            Refresh Practice
          </button>
        </div>

        <ErrorNotice v-if="practiceError" :message="practiceError" />
        <EmptyState v-else-if="!practiceStates.length" title="No practice states" message="Mock answers may update practice state after evaluation." />

        <div v-else class="practice-list">
          <article v-for="item in practiceStates" :key="item.state_id || `${item.topic}-${item.dimension}`" class="practice-card">
            <header class="item-head">
              <div>
                <strong>{{ item.topic || 'Untitled topic' }}</strong>
                <small>{{ item.dimension || '-' }} | attempts {{ item.attempt_count ?? 0 }}</small>
              </div>
              <StatusBadge :status="item.mastery_score ?? 'score'" />
            </header>
            <p>{{ item.last_feedback || 'No feedback.' }}</p>
            <small>{{ item.source_type || '-' }} | {{ item.source_id || '-' }} | last score {{ item.last_score ?? '-' }}</small>
          </article>
        </div>
      </section>

      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Turn Timeline</h2>
            <p>Opening question, answers, evaluations, hints, followups, topic switches, and closing turns.</p>
          </div>
          <StatusBadge :status="timeline.length ? 'ready' : 'empty'" />
        </div>

        <EmptyState v-if="!timeline.length" title="No turns" message="Start or refresh a mock interview to populate the timeline." />

        <div v-else class="timeline">
          <article v-for="turn in timeline" :key="turn.turn_id || `opening-${turn.turn_index}`" class="timeline-entry">
            <TurnTimelineItem :turn="turn" kind="mock" />
            <dl v-if="hasTurnEvidence(turn)" class="timeline-evidence">
              <div v-if="turn.follow_up_reason" class="field-row">
                <dt>follow_up_reason</dt>
                <dd>{{ turn.follow_up_reason }}</dd>
              </div>
              <div v-if="Array.isArray(turn.topic_tags) && turn.topic_tags.length" class="field-row">
                <dt>topic_tags</dt>
                <dd>{{ turn.topic_tags.join(', ') }}</dd>
              </div>
              <div v-if="turn.phase || turn.turn_type" class="field-row">
                <dt>evidence meta</dt>
                <dd>{{ turnEvidenceMeta(turn) }}</dd>
              </div>
            </dl>
          </article>
        </div>
      </section>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { api } from '../api'
import EmptyState from '../components/EmptyState.vue'
import ErrorNotice from '../components/ErrorNotice.vue'
import StatusBadge from '../components/StatusBadge.vue'
import TurnTimelineItem from '../components/TurnTimelineItem.vue'
import { rememberSelection, runWithStatus, workbenchState as state } from '../workbenchState'

const context = reactive({
  interview_id: state.selectedInterviewId || '',
  plan_id: state.selectedPlanId || '',
  target_round: 'second_round',
  user_id: state.userId || 'user_001',
})

const detail = ref(null)
const mock = ref(null)
const turns = ref([])
const practiceStates = ref([])
const practiceError = ref('')
const answerInput = ref('')

const terminalStatuses = new Set(['failed', 'completed', 'cancelled', 'canceled'])

const interview = computed(() => detail.value?.interview || {})
const selectedMockId = computed(() => mock.value?.mock_id || state.selectedMockId || '')
const contextUserId = computed(() => context.user_id || state.userId)
const canUseInterview = computed(() => Boolean(context.interview_id && contextUserId.value))
const interviewStatus = computed(() => interview.value.status || (context.interview_id ? 'selected' : 'none'))
const pageStatus = computed(() => mock.value?.status || interviewStatus.value)
const latestResultTurn = computed(() => {
  const reversed = [...turns.value].reverse()
  return reversed.find((turn) => isResultTurn(turn)) || null
})
const currentQuestion = computed(() => {
  const reversed = [...turns.value].reverse()
  const questionTurn = reversed.find((turn) => turn.next_question || turn.interviewer_question || (turn.role === 'assistant' && turn.content))
  return questionTurn?.next_question || questionTurn?.interviewer_question || questionTurn?.content || mock.value?.first_question || '-'
})
const canControlMock = computed(() => Boolean(mock.value?.mock_id && !terminalStatuses.has(normalizeStatus(mock.value.status))))
const canAnswerMock = computed(() => Boolean(mock.value?.mock_id && !terminalStatuses.has(normalizeStatus(mock.value.status))))
const canSubmitTurn = computed(() => Boolean(canAnswerMock.value && answerInput.value.trim()))
const latestFeedback = computed(() => latestResultTurn.value?.feedback || mock.value?.last_feedback || mock.value?.final_summary || '-')
const latestNextAction = computed(() => latestResultTurn.value?.agent_action || latestResultTurn.value?.phase || latestResultTurn.value?.turn_type || '-')
const latestResultStatus = computed(() => {
  if (latestResultTurn.value?.error_message || mock.value?.error_message) return 'failed'
  if (mock.value?.status) return mock.value.status
  if (latestResultTurn.value) return latestNextAction.value
  return 'none'
})
const practiceUpdateSummary = computed(() => {
  const source = latestResultTurn.value || {}
  const update = source.practice_update_summary || source.practice_updates || source.practice_update || source.practice_state_update
  if (!update) return ''
  if (typeof update === 'string') return update
  return JSON.stringify(update)
})
const traceLink = computed(() => ({
  path: '/trace',
  query: {
    source_type: 'mock_interview',
    source_id: selectedMockId.value,
    user_id: contextUserId.value,
    limit: 25,
  },
}))
const timeline = computed(() => {
  if (!mock.value?.first_question) return turns.value
  const hasOpening = turns.value.some((turn) => turn.phase === 'opening' || turn.turn_type === 'opening' || turn.interviewer_question === mock.value.first_question)
  if (hasOpening) return turns.value
  return [
    {
      turn_id: 'opening-question',
      mock_id: mock.value.mock_id,
      user_id: contextUserId.value,
      interview_id: mock.value.interview_id,
      turn_index: 0,
      role: 'assistant',
      turn_type: 'opening',
      phase: 'opening',
      agent_action: 'ask_question',
      content: mock.value.first_question,
      interviewer_question: mock.value.first_question,
    },
    ...turns.value,
  ]
})

function normalizeStatus(value) {
  return String(value || '').toLowerCase()
}

function isLoading(key) {
  return state.loadingKeys.has(key)
}

function asArray(value) {
  if (Array.isArray(value)) return value
  if (Array.isArray(value?.turns)) return value.turns
  return []
}

function isResultTurn(turn) {
  if (!turn) return false
  const phase = normalizeStatus(turn.phase)
  const turnType = normalizeStatus(turn.turn_type)
  const action = normalizeStatus(turn.agent_action)
  const hasFeedback = Boolean(String(turn.feedback || '').trim())
  const hasError = Boolean(turn.error_message)
  const score = Number(turn.score || 0)
  const hasPositiveScore = score > 0
  if (turn.turn_id === 'opening-question' || turnType === 'opening' || turnType === 'opening_question') return false
  if (phase === 'waiting_answer' && !hasFeedback && !hasError && !hasPositiveScore) return false
  if (action === 'wait_for_input' && !hasFeedback && !hasError && !hasPositiveScore) return false
  if (normalizeStatus(turn.role) === 'user' && !hasFeedback && !hasError && !hasPositiveScore) return false
  const resultPhases = new Set([
    'evaluation',
    'evaluating_answer',
    'action',
    'asking_followup',
    'switching_topic',
    'closing',
    'completed',
    'complete',
  ])
  const resultTypes = new Set(['evaluation', 'action', 'followup', 'follow_up', 'topic_switch', 'closing'])
  return Boolean(
    hasFeedback ||
      hasPositiveScore ||
      hasError ||
      (turn.next_question && Number(turn.turn_index || 0) > 0) ||
      (turn.agent_action && action !== 'ask_question' && phase !== 'opening' && turnType !== 'opening') ||
      resultPhases.has(phase) ||
      resultTypes.has(turnType),
  )
}

function hasTurnEvidence(turn) {
  return Boolean(
    turn?.follow_up_reason ||
      (Array.isArray(turn?.topic_tags) && turn.topic_tags.length) ||
      turn?.phase ||
      turn?.turn_type,
  )
}

function turnEvidenceMeta(turn) {
  return [
    turn?.phase ? `phase: ${turn.phase}` : '',
    turn?.turn_type ? `turn_type: ${turn.turn_type}` : '',
  ]
    .filter(Boolean)
    .join(' | ')
}

function rememberInterview() {
  rememberSelection('selectedInterviewId', context.interview_id)
}

function rememberPlan() {
  rememberSelection('selectedPlanId', context.plan_id)
}

function syncMock(nextMock) {
  if (!nextMock) return
  mock.value = nextMock
  if (nextMock.mock_id) rememberSelection('selectedMockId', nextMock.mock_id)
  if (nextMock.interview_id) {
    context.interview_id = nextMock.interview_id
    rememberSelection('selectedInterviewId', nextMock.interview_id)
  }
  if (nextMock.plan_id) {
    context.plan_id = nextMock.plan_id
    rememberSelection('selectedPlanId', nextMock.plan_id)
  }
  if (nextMock.target_round) context.target_round = nextMock.target_round
}

async function loadInterview() {
  if (!canUseInterview.value) return
  mock.value = null
  turns.value = []
  rememberSelection('selectedMockId', '')
  detail.value = await runWithStatus(
    'mockInterviewDetail',
    () => api.interviewDetail(context.interview_id, contextUserId.value),
    'Interview context loaded',
  )
  rememberSelection('selectedInterviewId', context.interview_id)
  if (detail.value?.coaching_plan?.plan_id) {
    context.plan_id = detail.value.coaching_plan.plan_id
    rememberSelection('selectedPlanId', context.plan_id)
  }
  if (detail.value?.latest_mock_interview) {
    syncMock(detail.value.latest_mock_interview)
    turns.value = []
    await refreshTurns()
  } else {
    turns.value = []
  }
}

async function startOrResumeMock() {
  if (!canUseInterview.value) return
  const loaded = await runWithStatus(
    'startMockInterview',
    () =>
      api.startMockInterview(context.interview_id, {
        user_id: contextUserId.value,
        plan_id: context.plan_id,
        target_round: context.target_round,
      }),
    'Mock interview ready',
  )
  syncMock(loaded)
  await refreshTurns()
  await loadPracticeStates()
}

async function refreshMock() {
  if (!selectedMockId.value) return
  const loaded = await runWithStatus('getMockInterview', () => api.getMockInterview(selectedMockId.value), 'Mock interview refreshed')
  syncMock(loaded)
  await refreshTurns()
  await loadPracticeStates()
}

async function refreshTurns() {
  if (!selectedMockId.value) return
  const loaded = await runWithStatus('listMockTurns', () => api.listMockTurns(selectedMockId.value), 'Mock turns loaded')
  turns.value = asArray(loaded)
}

async function submitTurn() {
  if (!canSubmitTurn.value) return
  await runWithStatus(
    'submitMockTurn',
    () => api.submitMockTurn(mock.value.mock_id, { answer: answerInput.value.trim() }),
    'Mock answer submitted',
  )
  answerInput.value = ''
  await refreshMock()
}

async function completeMock() {
  if (!canControlMock.value) return
  const loaded = await runWithStatus('completeMockInterview', () => api.completeMockInterview(mock.value.mock_id), 'Mock interview completed')
  syncMock(loaded)
  await refreshTurns()
  await loadPracticeStates()
}

async function cancelMock() {
  if (!canControlMock.value) return
  const loaded = await runWithStatus('cancelMockInterview', () => api.cancelMockInterview(mock.value.mock_id), 'Mock interview cancelled')
  syncMock(loaded)
  await refreshTurns()
}

async function loadPracticeStates() {
  practiceError.value = ''
  try {
    const loaded = await runWithStatus(
      'mockPracticeStates',
      () => api.listPracticeStates({ user_id: contextUserId.value }),
      'Practice states refreshed',
    )
    practiceStates.value = Array.isArray(loaded) ? loaded : []
  } catch (error) {
    practiceStates.value = []
    practiceError.value = error.message || String(error)
  }
}

function clearAnswer() {
  answerInput.value = ''
}

watch(
  () => state.selectedInterviewId,
  (interviewId) => {
    if (interviewId && !context.interview_id) {
      context.interview_id = interviewId
    }
  },
)

watch(
  () => state.selectedPlanId,
  (planId) => {
    if (planId && !context.plan_id) {
      context.plan_id = planId
    }
  },
)

watch(
  () => state.userId,
  (userId) => {
    if (userId && context.user_id !== userId) {
      context.user_id = userId
    }
  },
)

onMounted(async () => {
  if (context.interview_id) {
    try {
      await loadInterview()
    } catch {
      await loadPracticeStates()
    }
  } else if (selectedMockId.value) {
    try {
      await refreshMock()
    } catch {
      await loadPracticeStates()
    }
  } else {
    await loadPracticeStates()
  }
})
</script>

<style scoped>
.mock-context-grid {
  align-items: end;
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.context-actions,
.secondary-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.context-actions {
  grid-column: 1 / -1;
}

.mock-detail-grid {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  margin: 0;
}

.mock-detail-grid div,
.practice-card,
.result-card {
  background: #ffffff;
  border: 1px solid #e2e7ee;
  border-radius: 6px;
  display: grid;
  gap: 8px;
  min-width: 0;
  padding: 10px;
}

.mock-detail-grid div {
  background: #f7f9fc;
}

.mock-detail-grid dt {
  color: #667286;
  font-size: 12px;
  font-weight: 700;
}

.mock-detail-grid dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.mock-layout,
.mock-result-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.detail-stack,
.practice-list {
  display: grid;
  gap: 10px;
}

.timeline-entry {
  display: grid;
  gap: 8px;
}

.timeline-evidence {
  background: #f7f9fc;
  border: 1px solid #e2e7ee;
  border-radius: 6px;
  display: grid;
  gap: 6px;
  margin: 0 0 0 18px;
  padding: 8px 10px;
}

.timeline-evidence dt {
  color: #667286;
  font-size: 12px;
  font-weight: 700;
}

.timeline-evidence dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.item-head {
  align-items: flex-start;
  display: flex;
  gap: 10px;
  justify-content: space-between;
}

.item-head div {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.item-head strong,
.item-head small,
.practice-card p {
  overflow-wrap: anywhere;
}

.item-head small,
.practice-card small {
  color: #667286;
}

.practice-card p {
  color: #405069;
}

.result-card.has-error {
  background: #fff8f7;
  border-color: #ffc7c2;
}

.text-link {
  color: #1f5fd1;
  font-weight: 700;
  text-decoration: none;
}

@media (max-width: 900px) {
  .mock-context-grid,
  .mock-detail-grid,
  .mock-layout,
  .mock-result-grid {
    grid-template-columns: 1fr;
  }
}
</style>
