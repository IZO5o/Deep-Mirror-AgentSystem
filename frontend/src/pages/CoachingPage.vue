<template>
  <section class="page">
    <div class="page-header">
      <div>
        <span class="page-kicker">Second-round preparation</span>
        <h1>Coaching</h1>
        <p class="muted">Plan-level coaching session for a selected interview.</p>
      </div>
      <StatusBadge :status="pageStatus" />
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Interview Context</h2>
          <p>Load an interview, generate or fetch its coaching plan, then start a plan session.</p>
        </div>
        <StatusBadge :status="interviewStatus" />
      </div>

      <form class="coaching-context-grid" @submit.prevent="loadInterview">
        <label>
          interview_id
          <input v-model.trim="context.interview_id" autocomplete="off" placeholder="interview_id" />
        </label>
        <label>
          target_round
          <input v-model.trim="context.target_round" autocomplete="off" placeholder="second_round" />
        </label>
        <label>
          remaining_days
          <input v-model.number="context.remaining_days" min="0" type="number" />
        </label>
        <label>
          user_id
          <input v-model.trim="context.user_id" autocomplete="off" placeholder="user_001" />
        </label>
        <div class="context-actions">
          <button class="secondary" type="submit" :disabled="!canUseInterview || isLoading('coachingInterview')">Load Interview</button>
          <button class="primary" type="button" :disabled="!canUseInterview || isLoading('generateCoachingPlan')" @click="generatePlan">
            Generate Plan
          </button>
          <button class="secondary" type="button" :disabled="!canUseInterview || isLoading('getCoachingPlan')" @click="getPlan">
            Get Plan
          </button>
        </div>
      </form>

      <EmptyState
        v-if="!selectedInterviewId"
        title="No interview selected"
        message="Enter an interview_id or select one from the workbench, then load the interview context."
      />

      <dl v-else class="coaching-detail-grid">
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
          <dt>review readiness</dt>
          <dd><StatusBadge :status="reviewReadiness" /></dd>
        </div>
        <div>
          <dt>plan_id</dt>
          <dd>{{ selectedPlanId || '-' }}</dd>
        </div>
      </dl>
    </section>

    <div class="coaching-layout">
      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Plan</h2>
            <p>{{ selectedPlanId || 'No coaching plan loaded.' }}</p>
          </div>
          <StatusBadge :status="plan?.status || 'none'" />
        </div>

        <EmptyState v-if="!plan" title="No plan" message="Generate or get a coaching plan for the selected interview." />

        <div v-else class="detail-stack">
          <dl class="field-stack">
            <div class="field-row">
              <dt>strategy</dt>
              <dd>{{ plan.overall_strategy || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>focus</dt>
              <dd>{{ plan.focus_summary || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>status</dt>
              <dd><StatusBadge :status="plan.status || 'unknown'" /></dd>
            </div>
          </dl>

          <div class="secondary-actions">
            <button class="secondary" type="button" :disabled="!selectedPlanId || isLoading('coachingTasks')" @click="loadTasks">
              Refresh Tasks
            </button>
          </div>
        </div>
      </section>

      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Session Controls</h2>
            <p>{{ selectedSessionId || 'Start or resume a session from the loaded plan.' }}</p>
          </div>
          <StatusBadge :status="session?.status || 'none'" />
        </div>

        <div class="secondary-actions">
          <button
            class="primary"
            type="button"
            :disabled="!selectedPlanId || !contextUserId || isLoading('startCoachingSession')"
            @click="startOrResumeSession"
          >
            Start/Resume
          </button>
          <button class="secondary" type="button" :disabled="!selectedSessionId || isLoading('getCoachingSession')" @click="refreshSession">
            Refresh Session
          </button>
          <button class="secondary" type="button" :disabled="!canControlSession || isLoading('pauseCoachingSession')" @click="pauseSession">
            Pause
          </button>
          <button class="danger" type="button" :disabled="!canControlSession || isLoading('cancelCoachingSession')" @click="cancelSession">
            Cancel
          </button>
        </div>

        <RouterLink v-if="selectedSessionId" class="text-link" :to="traceLink">Open Coaching Trace</RouterLink>
      </section>
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Tasks</h2>
          <p>{{ tasks.length }} tasks loaded.</p>
        </div>
        <StatusBadge :status="currentTask ? 'current' : 'none'" />
      </div>

      <EmptyState v-if="!tasks.length" title="No tasks" message="Load tasks after a coaching plan is available." />

      <div v-else class="task-list">
        <article
          v-for="task in tasks"
          :key="task.task_id"
          class="task-card"
          :class="{ active: task.task_id && task.task_id === currentTask?.task_id }"
        >
          <header class="task-head">
            <div>
              <strong>#{{ task.sequence || '-' }} {{ task.title || 'Untitled task' }}</strong>
              <small>{{ task.task_type || '-' }} · day {{ task.day_index || '-' }} · {{ task.priority || 'priority unset' }}</small>
            </div>
            <StatusBadge :status="task.status || 'unknown'" />
          </header>
          <p>{{ task.description || 'No description.' }}</p>
          <div class="task-success-criteria">{{ taskSuccessCriteria(task) }}</div>
          <div v-if="task.task_id === currentTask?.task_id" class="current-task-highlight">
            current task
          </div>
        </article>
      </div>
    </section>

    <div class="coaching-layout">
      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Session State</h2>
            <p>{{ session?.session_id || 'No active session.' }}</p>
          </div>
          <StatusBadge :status="session?.status || 'none'" />
        </div>

        <EmptyState v-if="!session" title="No session" message="Start or resume a coaching session to view state." />

        <div v-else class="detail-stack">
          <dl class="field-stack">
            <div class="field-row">
              <dt>current task</dt>
              <dd>{{ currentTaskLabel }}</dd>
            </div>
            <div class="field-row">
              <dt>progress</dt>
              <dd>{{ session.progress_summary || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>last agent</dt>
              <dd>{{ session.last_agent_message || '-' }}</dd>
            </div>
            <div v-if="session.error_message" class="field-row has-error">
              <dt>error</dt>
              <dd>{{ session.error_message }}</dd>
            </div>
          </dl>
        </div>
      </section>

      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Turn Composer</h2>
            <p>Submit user input to the selected coaching session.</p>
          </div>
          <StatusBadge :status="canSubmitTurn ? 'ready' : 'blocked'" />
        </div>

        <textarea v-model="turnInput" rows="6" placeholder="Type a formal answer, hint request, explanation request, pause, or skip instruction." />
        <div class="secondary-actions">
          <button class="primary" type="button" :disabled="!canSubmitTurn || isLoading('submitCoachingTurn')" @click="submitTurn">
            Submit Turn
          </button>
          <button class="secondary" type="button" :disabled="!turnInput" @click="clearTurn">Clear</button>
        </div>
      </section>
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Latest Result</h2>
          <p>Assistant response, score, feedback, action, and latest recorded attempt.</p>
        </div>
        <StatusBadge :status="latestResultStatus" />
      </div>

      <EmptyState v-if="!latestAssistantTurn && !latestAttempt" title="No result yet" message="Submit or refresh a session turn to see the latest coaching result." />

      <div v-else class="coaching-result-grid">
        <TurnTimelineItem v-if="latestAssistantTurn" :turn="latestAssistantTurn" kind="coaching" />

        <article v-if="latestAttempt" class="attempt-card" :class="{ 'has-error': Boolean(latestAttempt.error_message) }">
          <header class="task-head">
            <div>
              <strong>Attempt #{{ latestAttempt.attempt_index || '-' }}</strong>
              <small>{{ latestAttempt.attempt_id || '-' }}</small>
            </div>
            <StatusBadge :status="latestAttempt.passed ? 'passed' : 'needs_revision'" />
          </header>
          <dl class="field-stack">
            <div class="field-row">
              <dt>score</dt>
              <dd>{{ latestAttempt.score ?? '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>feedback</dt>
              <dd>{{ latestAttempt.feedback || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>answer</dt>
              <dd>{{ latestAttempt.user_answer || '-' }}</dd>
            </div>
            <div v-if="latestAttempt.error_message" class="field-row has-error">
              <dt>error</dt>
              <dd>{{ latestAttempt.error_message }}</dd>
            </div>
          </dl>
        </article>
      </div>
    </section>

    <div class="coaching-layout">
      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Practice Evidence</h2>
            <p>Read-only practice states for the selected user.</p>
          </div>
          <StatusBadge :status="practiceError ? 'failed' : 'ready'" />
        </div>

        <ErrorNotice v-if="practiceError" :message="practiceError" />
        <EmptyState v-else-if="!practiceStates.length" title="No practice states" message="Formal coaching answers may update practice states." />

        <div v-else class="practice-list">
          <article v-for="item in practiceStates" :key="item.state_id || `${item.topic}-${item.dimension}`" class="practice-card">
            <header class="task-head">
              <div>
                <strong>{{ item.topic || 'Untitled topic' }}</strong>
                <small>{{ item.dimension || '-' }} · attempts {{ item.attempt_count ?? 0 }}</small>
              </div>
              <StatusBadge :status="item.mastery_score ?? 'score'" />
            </header>
            <p>{{ item.last_feedback || 'No feedback.' }}</p>
            <small>{{ item.source_type || '-' }} · {{ item.source_id || '-' }} · last score {{ item.last_score ?? '-' }}</small>
          </article>
        </div>
      </section>

      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Timeline</h2>
            <p>{{ turns.length }} session turns.</p>
          </div>
          <StatusBadge :status="turns.length ? 'ready' : 'empty'" />
        </div>

        <EmptyState v-if="!turns.length" title="No turns" message="Start a session or submit user input to populate the coaching timeline." />

        <div v-else class="timeline">
          <TurnTimelineItem v-for="turn in turns" :key="turn.turn_id" :turn="turn" kind="coaching" />
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
  target_round: 'second_round',
  remaining_days: 3,
  user_id: state.userId || 'user_001',
})

const detail = ref(null)
const plan = ref(null)
const tasks = ref([])
const sessionDetail = ref(null)
const practiceStates = ref([])
const practiceError = ref('')
const turnInput = ref('')

const selectedInterviewId = computed(() => state.selectedInterviewId || context.interview_id)
const selectedPlanId = computed(() => state.selectedPlanId || plan.value?.plan_id || '')
const selectedSessionId = computed(() => state.selectedSessionId || session.value?.session_id || '')
const contextUserId = computed(() => context.user_id || state.userId)

const interview = computed(() => detail.value?.interview || {})
const reviewReadiness = computed(() => {
  if (!detail.value) return 'not_loaded'
  return detail.value.review_report ? detail.value.review_report.status || 'ready' : 'missing'
})
const interviewStatus = computed(() => interview.value.status || (selectedInterviewId.value ? 'selected' : 'none'))
const pageStatus = computed(() => session.value?.status || plan.value?.status || interviewStatus.value)

const session = computed(() => sessionDetail.value?.session || null)
const currentTask = computed(() => {
  if (sessionDetail.value?.current_task) return sessionDetail.value.current_task
  const currentTaskId = session.value?.current_task_id
  return tasks.value.find((task) => task.task_id === currentTaskId) || null
})
const turns = computed(() => sessionDetail.value?.turns || [])
const attempts = computed(() => sessionDetail.value?.attempts || [])
const latestAssistantTurn = computed(() => {
  const reversed = [...turns.value].reverse()
  return reversed.find((turn) => turn.role === 'assistant' || turn.feedback || turn.agent_action || turn.score || turn.error_message) || null
})
const latestAttempt = computed(() => attempts.value.length ? attempts.value[attempts.value.length - 1] : null)
const latestResultStatus = computed(() => {
  if (latestAssistantTurn.value?.error_message || latestAttempt.value?.error_message) return 'failed'
  if (latestAttempt.value) return latestAttempt.value.passed ? 'passed' : 'needs_revision'
  if (latestAssistantTurn.value) return latestAssistantTurn.value.agent_action || latestAssistantTurn.value.turn_type || 'ready'
  return 'none'
})
const currentTaskLabel = computed(() => {
  if (!currentTask.value) return session.value?.current_task_id || '-'
  return `${currentTask.value.title || currentTask.value.task_id} (${currentTask.value.status || 'unknown'})`
})

const terminalStatuses = new Set(['failed', 'completed', 'cancelled', 'canceled'])
const canUseInterview = computed(() => Boolean(context.interview_id && contextUserId.value))
const canControlSession = computed(() => Boolean(selectedSessionId.value && session.value && !terminalStatuses.has(normalizeStatus(session.value.status))))
const canSubmitTurn = computed(() => Boolean(selectedSessionId.value && turnInput.value.trim() && session.value && !terminalStatuses.has(normalizeStatus(session.value.status))))
const traceLink = computed(() => ({
  path: '/trace',
  query: {
    source_type: 'coaching_session',
    source_id: selectedSessionId.value,
    user_id: contextUserId.value,
    limit: 25,
  },
}))

function normalizeStatus(value) {
  return String(value || '').toLowerCase()
}

function isLoading(key) {
  return state.loadingKeys.has(key)
}

function taskSuccessCriteria(task) {
  const taskType = String(task?.task_type || '').toLowerCase()
  if (['formal_answer', 'practice', 'answer'].includes(taskType)) {
    return 'Success criteria: submit a formal answer, receive backend feedback, and progress this task through the coaching state machine.'
  }
  if (['review', 'weakness', 'gap'].includes(taskType)) {
    return 'Success criteria: address the task description and use feedback to close the identified gap.'
  }
  return 'Success criteria: complete this task through the coaching session and capture backend feedback.'
}

function syncSelectionsFromPlan(nextPlan) {
  if (!nextPlan) return
  plan.value = nextPlan
  if (nextPlan.interview_id) rememberSelection('selectedInterviewId', nextPlan.interview_id)
  if (nextPlan.plan_id) rememberSelection('selectedPlanId', nextPlan.plan_id)
}

function syncSessionDetail(nextDetail) {
  if (!nextDetail) return
  sessionDetail.value = nextDetail
  if (nextDetail.session?.session_id) rememberSelection('selectedSessionId', nextDetail.session.session_id)
  if (nextDetail.session?.coaching_plan_id) rememberSelection('selectedPlanId', nextDetail.session.coaching_plan_id)
  if (Array.isArray(nextDetail.tasks)) tasks.value = nextDetail.tasks
}

async function loadInterview() {
  if (!canUseInterview.value) return
  detail.value = await runWithStatus(
    'coachingInterview',
    () => api.interviewDetail(context.interview_id, contextUserId.value),
    'Interview context loaded',
  )
  rememberSelection('selectedInterviewId', context.interview_id)
  if (detail.value?.coaching_plan) {
    syncSelectionsFromPlan(detail.value.coaching_plan)
  }
  if (Array.isArray(detail.value?.coaching_tasks) && detail.value.coaching_tasks.length) {
    tasks.value = detail.value.coaching_tasks
  }
}

async function generatePlan() {
  const generated = await runWithStatus(
    'generateCoachingPlan',
    () =>
      api.generateCoachingPlan(context.interview_id, {
        user_id: contextUserId.value,
        target_round: context.target_round,
        remaining_days: Number.parseInt(context.remaining_days, 10) || 0,
      }),
    'Coaching plan generated',
  )
  syncSelectionsFromPlan(generated)
  await loadTasks()
  await loadPracticeStates()
}

async function getPlan() {
  const loaded = await runWithStatus('getCoachingPlan', () => api.getCoachingPlan(context.interview_id), 'Coaching plan loaded')
  syncSelectionsFromPlan(loaded)
  await loadTasks()
}

async function loadTasks() {
  if (!selectedPlanId.value) return
  const loaded = await runWithStatus('coachingTasks', () => api.listCoachingTasks(selectedPlanId.value), 'Coaching tasks loaded')
  tasks.value = Array.isArray(loaded) ? loaded : []
}

async function startOrResumeSession() {
  const loaded = await runWithStatus(
    'startCoachingSession',
    () => api.startOrResumeCoachingSession(selectedPlanId.value, contextUserId.value),
    'Coaching session ready',
  )
  syncSessionDetail(loaded)
  await loadPracticeStates()
}

async function refreshSession() {
  const loaded = await runWithStatus('getCoachingSession', () => api.getCoachingSession(selectedSessionId.value), 'Coaching session refreshed')
  syncSessionDetail(loaded)
  await loadPracticeStates()
}

async function pauseSession() {
  const loaded = await runWithStatus('pauseCoachingSession', () => api.pauseCoachingSession(selectedSessionId.value), 'Coaching session paused')
  syncSessionDetail(loaded)
}

async function cancelSession() {
  const loaded = await runWithStatus('cancelCoachingSession', () => api.cancelCoachingSession(selectedSessionId.value), 'Coaching session cancelled')
  syncSessionDetail(loaded)
}

async function submitTurn() {
  const loaded = await runWithStatus(
    'submitCoachingTurn',
    () => api.submitCoachingTurn(selectedSessionId.value, { user_input: turnInput.value.trim() }),
    'Coaching turn submitted',
  )
  turnInput.value = ''
  syncSessionDetail(loaded)
  await loadPracticeStates()
}

function clearTurn() {
  turnInput.value = ''
}

async function loadPracticeStates() {
  practiceError.value = ''
  try {
    const loaded = await api.listPracticeStates({ user_id: contextUserId.value })
    practiceStates.value = Array.isArray(loaded) ? loaded : []
  } catch (error) {
    practiceStates.value = []
    practiceError.value = error.message || String(error)
  }
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
  () => state.userId,
  (userId) => {
    if (userId && context.user_id !== userId) {
      context.user_id = userId
    }
  },
)

onMounted(async () => {
  if (state.selectedInterviewId) {
    context.interview_id = state.selectedInterviewId
  }
  if (state.selectedPlanId) {
    plan.value = { plan_id: state.selectedPlanId, status: 'selected' }
  }
  if (state.selectedSessionId) {
    try {
      await refreshSession()
    } catch {
      await loadPracticeStates()
    }
    return
  }
  await loadPracticeStates()
})
</script>

<style scoped>
.coaching-context-grid {
  align-items: end;
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.context-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  grid-column: 1 / -1;
}

.coaching-detail-grid {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  margin: 0;
}

.coaching-detail-grid div,
.attempt-card,
.practice-card,
.task-card {
  background: #ffffff;
  border: 1px solid #e2e7ee;
  border-radius: 6px;
  display: grid;
  gap: 8px;
  min-width: 0;
  padding: 10px;
}

.coaching-detail-grid div {
  background: #f7f9fc;
}

.coaching-detail-grid dt {
  color: #667286;
  font-size: 12px;
  font-weight: 700;
}

.coaching-detail-grid dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.coaching-layout,
.coaching-result-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.detail-stack,
.task-list,
.practice-list {
  display: grid;
  gap: 10px;
}

.task-card.active {
  border-color: #1f5fd1;
  box-shadow: inset 3px 0 0 #1f5fd1;
}

.task-card p,
.practice-card p {
  color: #405069;
  overflow-wrap: anywhere;
}

.task-head {
  align-items: flex-start;
  display: flex;
  gap: 10px;
  justify-content: space-between;
}

.task-head div {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.task-head strong,
.task-head small {
  overflow-wrap: anywhere;
}

.task-head small,
.practice-card small {
  color: #667286;
}

.current-task-highlight {
  background: #eef4ff;
  border: 1px solid #bfd0f3;
  border-radius: 6px;
  color: #25457c;
  font-size: 12px;
  font-weight: 700;
  overflow-wrap: anywhere;
  padding: 7px 8px;
}

.attempt-card.has-error {
  background: #fff8f7;
  border-color: #ffc7c2;
}

.text-link {
  color: #1f5fd1;
  font-weight: 700;
  text-decoration: none;
}
</style>
