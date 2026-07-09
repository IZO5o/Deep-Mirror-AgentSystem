<template>
  <section class="page conversation-page">
    <div class="conversation-topbar">
      <div>
        <span class="page-kicker">模拟面试</span>
        <h1>Mock Interview</h1>
        <p class="muted">{{ mockSummaryLine }}</p>
      </div>
      <div class="topbar-actions">
        <StatusBadge :status="pageStatus" />
        <button class="secondary" type="button" @click="evidenceOpen = !evidenceOpen">
          {{ evidenceOpen ? '收起证据' : '展开证据' }}
        </button>
      </div>
    </div>

    <section class="conversation-setup panel">
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
          轮次
          <input v-model.trim="context.target_round" autocomplete="off" placeholder="second_round" />
        </label>
        <label>
          user_id
          <input v-model.trim="context.user_id" autocomplete="off" placeholder="user_001" />
        </label>
        <label>
          focus_topic
          <input v-model.trim="context.focus_topic" autocomplete="off" placeholder="optional focus topic" @change="rememberFocusTopic" />
        </label>
        <div class="context-actions">
          <button class="secondary" type="submit" :disabled="!canUseInterview || isLoading('mockInterviewDetail')">加载面试</button>
          <button class="primary" type="button" :disabled="!canUseInterview || isLoading('startMockInterview')" @click="startOrResumeMock">
            开始/继续 Mock
          </button>
          <button class="secondary" type="button" :disabled="!selectedMockId || isLoading('getMockInterview')" @click="refreshMock">
            刷新
          </button>
        </div>
      </form>
    </section>

    <div class="conversation-shell" :class="{ 'evidence-collapsed': !evidenceOpen }">
      <main class="chat-main panel">
        <div class="chat-summary-line">
          <strong>当前题目：{{ currentQuestion }}</strong>
          <span>状态：{{ mock?.status || '未开始' }}</span>
          <span v-if="timerState.active" :class="['mock-timer', { expired: timerState.expired }]">
            {{ timerLabel }} · {{ timerState.style }}
          </span>
        </div>

        <div class="long-chat-stream">
          <EmptyState v-if="!chatMessages.length" title="还没有对话" message="开始 Mock 后，这里会显示面试官和你的长对话。" />
          <article v-for="message in chatMessages" :key="message.id" class="chat-bubble" :class="message.role">
            <span>{{ message.role === 'user' ? '我' : '面试官' }}</span>
            <p>{{ message.content }}</p>
          </article>
        </div>

        <form class="fixed-composer" @submit.prevent="submitFormalAnswer">
          <textarea v-model="answerInput" rows="4" :disabled="!canAnswerMock" placeholder="默认作为正式回答提交。需要提示或澄清时，用场外提问。" />
          <div class="composer-actions">
            <button class="primary" type="button" :disabled="!canSubmitTurn || isLoading('submitMockTurn')" @click="submitFormalAnswer">
              回答
            </button>
            <button class="secondary" type="button" :disabled="!canSubmitTurn || isLoading('submitMockTurn')" @click="submitOffRecordQuestion">
              场外提问
            </button>
          </div>
        </form>
      </main>

      <aside v-show="evidenceOpen" class="evidence-panel">
        <details open class="evidence-section">
          <summary>面试状态</summary>
          <div class="secondary-actions compact-actions">
            <button class="secondary" type="button" :disabled="!canControlMock || isLoading('completeMockInterview')" @click="completeMock">完成</button>
            <button class="danger" type="button" :disabled="!canControlMock || isLoading('cancelMockInterview')" @click="cancelMock">取消</button>
            <button class="secondary" type="button" v-if="canResumeMock" :disabled="isLoading('resumeMockInterview')" @click="resumeFailedMock">重试上一轮</button>
          </div>
          <dl class="field-stack">
            <div class="field-row">
              <dt>mock_id</dt>
              <dd>{{ mock?.mock_id || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>company</dt>
              <dd>{{ interview.company_name || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>plan_id</dt>
              <dd>{{ context.plan_id || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>当前 topic</dt>
              <dd>{{ mock?.current_topic || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>正式轮次</dt>
              <dd>{{ mock?.current_turn ?? '-' }}</dd>
            </div>
          </dl>
        </details>

        <details class="evidence-section">
          <summary>最近反馈</summary>
          <EmptyState v-if="!latestResultTurn && !mock?.last_feedback && !mock?.final_summary" title="还没有反馈" message="提交回答后会显示最近反馈。" />
          <article v-else class="result-card" :class="{ 'has-error': Boolean(latestResultTurn?.error_message || mock?.error_message) }">
            <StatusBadge :status="latestResultStatus" />
            <p>{{ latestFeedback }}</p>
            <small>score {{ latestResultTurn?.score ?? '-' }} · {{ latestNextAction }}</small>
            <p v-if="practiceUpdateSummary" class="muted">{{ practiceUpdateSummary }}</p>
          </article>
        </details>

        <details class="evidence-section">
          <summary>Practice States</summary>
          <div class="secondary-actions compact-actions">
            <button class="secondary" type="button" :disabled="isLoading('mockPracticeStates')" @click="loadPracticeStates">刷新 Practice</button>
          </div>
          <ErrorNotice v-if="practiceError" :message="practiceError" />
          <EmptyState v-else-if="!practiceStates.length" title="还没有练习状态" message="正式回答可能更新 practice state。" />
          <div v-else class="practice-list">
            <article v-for="item in practiceStates" :key="item.state_id || `${item.topic}-${item.dimension}`" class="practice-card">
              <strong>{{ item.topic || '未命名 topic' }}</strong>
              <p>{{ item.last_feedback || '暂无反馈。' }}</p>
              <small>{{ item.source_type || '-' }} · {{ item.source_id || '-' }}</small>
            </article>
          </div>
        </details>

        <details class="evidence-section">
          <summary>Trace / Timeline</summary>
          <RouterLink v-if="selectedMockId" class="text-link" :to="traceLink">打开 Trace</RouterLink>
          <div class="timeline compact">
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
        </details>
      </aside>
    </div>
  </section>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
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
  focus_topic: state.focusTopic || '',
})

const detail = ref(null)
const mock = ref(null)
const turns = ref([])
const practiceStates = ref([])
const practiceError = ref('')
const answerInput = ref('')
const evidenceOpen = ref(false)
const nowSeconds = ref(Math.floor(Date.now() / 1000))
const countdownInterval = ref(null)
const checkInInterval = ref(null)
const lastTimerCheckInTurnId = ref('')

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
const canResumeMock = computed(() => ['failed', 'retriable_failed'].includes(normalizeStatus(mock.value?.status)))
const latestTimedQuestionTurn = computed(() => {
  const reversed = [...turns.value].reverse()
  return reversed.find((turn) => {
    return String(turn.role || '').toLowerCase() === 'assistant' && Number(turn.time_limit_seconds || 0) > 0
  }) || null
})
const timerState = computed(() => {
  const turn = latestTimedQuestionTurn.value
  if (!turn) return { active: false }
  const total = Number(turn.time_limit_seconds || 0)
  const startedAt = Number(turn.created_at || 0)
  const elapsed = Math.max(0, nowSeconds.value - startedAt)
  const remaining = total - elapsed
  return {
    active: total > 0,
    total,
    elapsed,
    remaining,
    expired: remaining <= 0,
    warnAt: Number(turn.warn_at_seconds || 300),
    style: turn.time_pressure_style || 'none',
  }
})
const timerLabel = computed(() => {
  if (!timerState.value.active) return ''
  const seconds = Math.max(0, timerState.value.remaining)
  const minutes = Math.floor(seconds / 60)
  const rest = seconds % 60
  return `${minutes}:${String(rest).padStart(2, '0')}`
})
const mockSummaryLine = computed(() => {
  const question = currentQuestion.value && currentQuestion.value !== '-' ? currentQuestion.value : '尚未开始'
  const status = mock.value?.status || '未开始'
  return `当前题目：${question} / 状态：${status}`
})
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
const chatMessages = computed(() =>
  timeline.value
    .filter((turn) => ['user', 'assistant'].includes(String(turn.role || '').toLowerCase()))
    .map((turn) => ({
      id: turn.turn_id || `turn-${turn.turn_index}-${turn.role}`,
      role: String(turn.role || '').toLowerCase() === 'user' ? 'user' : 'assistant',
      content: turn.content || turn.next_question || turn.feedback || turn.interviewer_question || '',
    }))
    .filter((message) => message.content.trim()),
)

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

function rememberFocusTopic() {
  rememberSelection('focusTopic', context.focus_topic)
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
        focus_topic: context.focus_topic,
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

async function submitWithMode(submitMode) {
  if (!canSubmitTurn.value) return
  await runWithStatus(
    'submitMockTurn',
    () => api.submitMockTurn(mock.value.mock_id, { answer: answerInput.value.trim(), submit_mode: submitMode }),
    submitMode === 'chat' ? '场外提问已发送' : '回答已提交',
  )
  answerInput.value = ''
  await refreshMock()
}

async function submitFormalAnswer() {
  await submitWithMode('formal_answer')
}

async function submitOffRecordQuestion() {
  await submitWithMode('chat')
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

async function resumeFailedMock() {
  if (!selectedMockId.value) return
  const loaded = await runWithStatus(
    'resumeMockInterview',
    () => api.resumeMockInterview(selectedMockId.value),
    'Mock interview resumed',
  )
  syncMock(loaded)
  await refreshTurns()
  await loadPracticeStates()
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

watch(
  () => state.focusTopic,
  (focusTopic) => {
    if (focusTopic && !context.focus_topic) {
      context.focus_topic = focusTopic
    }
  },
)

watch([() => mock.value?.status, timerState], ([status, timer]) => {
  if (checkInInterval.value) {
    window.clearInterval(checkInInterval.value)
    checkInInterval.value = null
  }
  if (status === 'waiting_answer' && timer.active && !timer.expired && timer.elapsed >= timer.warnAt) {
    checkInInterval.value = window.setInterval(async () => {
      if (!mock.value?.mock_id || answerInput.value.trim() !== '') return
      const timedTurn = latestTimedQuestionTurn.value
      const timedTurnId = timedTurn?.turn_id || `${timedTurn?.turn_index || ''}-${timedTurn?.created_at || ''}`
      if (!timedTurnId || lastTimerCheckInTurnId.value === timedTurnId) return
      try {
        await api.submitMockTurn(mock.value.mock_id, {
          answer: '',
          submit_mode: 'chat',
          trigger: 'silence_timeout',
        })
        lastTimerCheckInTurnId.value = timedTurnId
        await refreshTurns()
      } catch {
        // Silent timer check-in is best-effort; manual submit remains available.
      }
    }, 30000)
  }
}, { immediate: true })

onMounted(async () => {
  countdownInterval.value = window.setInterval(() => {
    nowSeconds.value = Math.floor(Date.now() / 1000)
  }, 1000)
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

onBeforeUnmount(() => {
  if (countdownInterval.value) window.clearInterval(countdownInterval.value)
  if (checkInInterval.value) window.clearInterval(checkInInterval.value)
})
</script>

<style scoped>
.mock-context-grid {
  align-items: end;
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(5, minmax(0, 1fr));
}

.context-actions,
.secondary-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.mock-timer {
  color: #526070;
  font-variant-numeric: tabular-nums;
  font-weight: 700;
}

.mock-timer.expired {
  color: #b42318;
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
