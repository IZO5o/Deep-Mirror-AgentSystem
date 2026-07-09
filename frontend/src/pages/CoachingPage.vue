<template>
  <section class="page conversation-page">
    <div class="conversation-topbar">
      <div>
        <span class="page-kicker">二面辅导</span>
        <h1>Coaching</h1>
        <p class="muted">{{ coachingSummaryLine }}</p>
      </div>
      <div class="topbar-actions">
        <StatusBadge :status="pageStatus" />
        <button class="secondary" type="button" @click="evidenceOpen = !evidenceOpen">
          {{ evidenceOpen ? '收起证据' : '展开证据' }}
        </button>
      </div>
    </div>

    <section class="conversation-setup panel">
      <form class="coaching-context-grid" @submit.prevent="loadInterview">
        <label>
          interview_id
          <input v-model.trim="context.interview_id" autocomplete="off" placeholder="interview_id" />
        </label>
        <label>
          轮次
          <input v-model.trim="context.target_round" autocomplete="off" placeholder="second_round" />
        </label>
        <label>
          剩余天数
          <input v-model.number="context.remaining_days" min="0" type="number" />
        </label>
        <label>
          user_id
          <input v-model.trim="context.user_id" autocomplete="off" placeholder="user_001" />
        </label>
        <div class="context-actions">
          <button class="secondary" type="submit" :disabled="!canUseInterview || isLoading('coachingInterview')">加载面试</button>
          <button class="primary" type="button" :disabled="!canUseInterview || isLoading('generateCoachingPlan')" @click="generatePlan">
            生成计划
          </button>
          <button class="secondary" type="button" :disabled="!canUseInterview || isLoading('getCoachingPlan')" @click="getPlan">
            读取计划
          </button>
          <button
            class="primary"
            type="button"
            :disabled="!selectedPlanId || !contextUserId || isLoading('startCoachingSession')"
            @click="startOrResumeSession"
          >
            开始/继续辅导
          </button>
        </div>
      </form>
    </section>

    <div class="conversation-shell" :class="{ 'evidence-collapsed': !evidenceOpen }">
      <main class="chat-main panel">
        <div class="chat-summary-line">
          <strong>当前训练重点：{{ currentTask?.title || '尚未开始' }}</strong>
          <span>状态：{{ session?.status || '未开始' }}</span>
          <button
            v-if="canStartFocusedMock"
            class="primary"
            type="button"
            @click="startFocusedMock"
          >
            Mock this topic
          </button>
        </div>

        <div class="long-chat-stream">
          <EmptyState v-if="!chatMessages.length" title="还没有对话" message="开始辅导后，这里会显示和导师的长对话。" />
          <article v-for="message in chatMessages" :key="message.id" class="chat-bubble" :class="message.role">
            <span>{{ message.role === 'user' ? '我' : '导师' }}</span>
            <p>{{ message.content }}</p>
          </article>
        </div>

        <form class="fixed-composer" @submit.prevent="submitChatTurn">
          <textarea v-model="turnInput" rows="4" :disabled="!canEditTurn" placeholder="可以提问、让导师提示，或先聊清楚思路。" />
          <div class="composer-actions">
            <button class="secondary" type="button" :disabled="!canSubmitTurn || isLoading('submitCoachingTurn')" @click="submitChatTurn">
              发送
            </button>
            <button class="primary" type="button" :disabled="!canSubmitTurn || isLoading('submitCoachingTurn')" @click="submitFormalTurn">
              作为正式回答提交
            </button>
          </div>
        </form>
      </main>

      <aside v-show="evidenceOpen" class="evidence-panel">
        <details open class="evidence-section">
          <summary>训练计划 / 任务</summary>
          <p class="muted">{{ plan?.focus_summary || plan?.overall_strategy || '还没有训练计划。' }}</p>
          <div class="task-list compact">
            <article v-for="task in tasks" :key="task.task_id" class="task-card" :class="{ active: task.task_id === currentTask?.task_id }">
              <header class="task-head">
                <div>
                  <strong>#{{ task.sequence || '-' }} {{ task.title || '未命名任务' }}</strong>
                  <small>{{ task.task_type || '-' }} · day {{ task.day_index || '-' }}</small>
                </div>
                <StatusBadge :status="task.status || 'unknown'" />
              </header>
              <p>{{ task.description || '没有描述。' }}</p>
              <div class="task-success-criteria">{{ taskSuccessCriteria(task) }}</div>
            </article>
          </div>
        </details>

        <details class="evidence-section">
          <summary>会话控制</summary>
          <div class="secondary-actions compact-actions">
            <button class="secondary" type="button" :disabled="!selectedPlanId || isLoading('coachingTasks')" @click="loadTasks">刷新任务</button>
            <button class="secondary" type="button" :disabled="!selectedSessionId || isLoading('getCoachingSession')" @click="refreshSession">刷新会话</button>
            <button class="secondary" type="button" :disabled="!canControlSession || isLoading('pauseCoachingSession')" @click="pauseSession">暂停</button>
            <button class="danger" type="button" :disabled="!canControlSession || isLoading('cancelCoachingSession')" @click="cancelSession">取消</button>
            <button class="secondary" type="button" v-if="canResumeSession" :disabled="isLoading('resumeCoachingSession')" @click="resumeFailedSession">重试上一轮</button>
          </div>
          <dl class="field-stack">
            <div class="field-row">
              <dt>company</dt>
              <dd>{{ interview.company_name || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>job</dt>
              <dd>{{ interview.job_title || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>plan_id</dt>
              <dd>{{ selectedPlanId || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>session</dt>
              <dd>{{ selectedSessionId || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>review</dt>
              <dd><StatusBadge :status="reviewReadiness" /></dd>
            </div>
          </dl>
        </details>

        <details class="evidence-section">
          <summary>Attempts</summary>
          <EmptyState v-if="!attempts.length" title="还没有正式回答" message="点击“作为正式回答提交”后会生成 attempt。" />
          <article v-for="attempt in attempts" :key="attempt.attempt_id" class="attempt-card" :class="{ 'has-error': Boolean(attempt.error_message) }">
            <header class="task-head">
              <div>
                <strong>Attempt #{{ attempt.attempt_index || '-' }}</strong>
                <small>{{ attempt.attempt_id || '-' }}</small>
              </div>
              <StatusBadge :status="attempt.passed ? 'passed' : 'needs_revision'" />
            </header>
            <p>{{ attempt.feedback || '-' }}</p>
            <small>score {{ attempt.score ?? '-' }}</small>
          </article>
        </details>

        <details class="evidence-section">
          <summary>Practice States</summary>
          <ErrorNotice v-if="practiceError" :message="practiceError" />
          <EmptyState v-else-if="!practiceStates.length" title="还没有练习状态" message="正式回答通过服务端规则更新 practice state。" />
          <div v-else class="practice-list">
            <article v-for="item in practiceStates" :key="item.state_id || `${item.topic}-${item.dimension}`" class="practice-card">
              <strong>{{ item.topic || '未命名 topic' }}</strong>
              <p>{{ item.last_feedback || '暂无反馈。' }}</p>
              <small>{{ item.source_type || '-' }} · {{ item.source_id || '-' }} · last score {{ item.last_score ?? '-' }}</small>
            </article>
          </div>
        </details>

        <details class="evidence-section">
          <summary>Trace / Timeline</summary>
          <RouterLink v-if="selectedSessionId" class="text-link" :to="traceLink">打开 Trace</RouterLink>
          <div class="timeline compact">
            <TurnTimelineItem v-for="turn in turns" :key="turn.turn_id" :turn="turn" kind="coaching" />
          </div>
        </details>
      </aside>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
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

const router = useRouter()
const detail = ref(null)
const plan = ref(null)
const tasks = ref([])
const sessionDetail = ref(null)
const practiceStates = ref([])
const practiceError = ref('')
const turnInput = ref('')
const evidenceOpen = ref(false)

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
const coachingSummaryLine = computed(() => {
  const focus = currentTask.value?.title || plan.value?.focus_summary || '尚未加载训练重点'
  const status = session.value?.status || '未开始'
  return `当前训练重点：${focus} / 状态：${status}`
})
const chatMessages = computed(() =>
  turns.value
    .filter((turn) => ['user', 'assistant'].includes(String(turn.role || '').toLowerCase()))
    .map((turn) => ({
      id: turn.turn_id,
      role: String(turn.role || '').toLowerCase() === 'user' ? 'user' : 'assistant',
      content: turn.content || turn.feedback || '',
    }))
    .filter((message) => message.content.trim()),
)
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
const canEditTurn = computed(() => Boolean(selectedSessionId.value && session.value && !terminalStatuses.has(normalizeStatus(session.value.status))))
const canSubmitTurn = computed(() => Boolean(canEditTurn.value && turnInput.value.trim()))
const canResumeSession = computed(() => ['failed', 'retriable_failed'].includes(normalizeStatus(session.value?.status)))
const canStartFocusedMock = computed(() => {
  const status = normalizeStatus(session.value?.status)
  const blocked = ['completed', 'cancelled', 'canceled'].includes(status)
  const interviewId = session.value?.interview_id || selectedInterviewId.value || context.interview_id
  return Boolean(!blocked && interviewId && (currentTask.value || session.value?.coaching_plan_id || selectedPlanId.value))
})
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
    return '验收标准：提交正式回答，获得后端反馈，并推进当前训练任务。'
  }
  if (['review', 'weakness', 'gap'].includes(taskType)) {
    return '验收标准：回应任务描述，并根据反馈补齐对应短板。'
  }
  return '验收标准：在辅导会话中完成任务，并保留后端反馈。'
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

async function resumeFailedSession() {
  if (!selectedSessionId.value) return
  const loaded = await runWithStatus(
    'resumeCoachingSession',
    () => api.resumeCoachingSession(selectedSessionId.value),
    'Coaching session resumed',
  )
  syncSessionDetail(loaded)
  await loadPracticeStates()
}

async function submitWithMode(submitMode) {
  const text = turnInput.value.trim()
  if (!text || !selectedSessionId.value) return
  const loaded = await runWithStatus(
    'submitCoachingTurn',
    () => api.submitCoachingTurn(selectedSessionId.value, { user_input: text, submit_mode: submitMode }),
    submitMode === 'formal_answer' ? '正式回答已提交' : '消息已发送',
  )
  turnInput.value = ''
  syncSessionDetail(loaded)
  await loadPracticeStates()
}

async function submitChatTurn() {
  await submitWithMode('chat')
}

async function submitFormalTurn() {
  await submitWithMode('formal_answer')
}

function clearTurn() {
  turnInput.value = ''
}

function startFocusedMock() {
  const focus = currentTask.value?.title || currentTask.value?.topic || ''
  const interviewId = session.value?.interview_id || selectedInterviewId.value || context.interview_id
  const planId = session.value?.coaching_plan_id || selectedPlanId.value
  if (focus) rememberSelection('focusTopic', focus)
  if (interviewId) rememberSelection('selectedInterviewId', interviewId)
  if (planId) rememberSelection('selectedPlanId', planId)
  router.push('/mock')
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
