<template>
  <section class="page">
    <div class="page-header">
      <div>
        <span class="page-kicker">Practice goals</span>
        <h1>Practice Goals</h1>
        <p class="muted">User-level coaching and mock entry point without an interview record.</p>
      </div>
      <button class="secondary" type="button" :disabled="isLoading('practiceGoals')" @click="loadGoals">Refresh</button>
    </div>

    <section class="panel goal-create-panel">
      <form class="goal-form" @submit.prevent="createGoal">
        <label>
          user_id
          <input v-model.trim="form.user_id" autocomplete="off" />
        </label>
        <label>
          company
          <input v-model.trim="form.company_name" autocomplete="off" />
        </label>
        <label>
          job
          <input v-model.trim="form.job_title" autocomplete="off" />
        </label>
        <label>
          round
          <input v-model.trim="form.target_round" autocomplete="off" />
        </label>
        <label>
          days
          <input v-model.number="form.remaining_days" min="1" type="number" />
        </label>
        <label class="wide">
          focus topics
          <input v-model.trim="focusTopicsText" autocomplete="off" placeholder="缓存一致性, Redlock 争议点" />
        </label>
        <label class="wide">
          job description
          <textarea v-model="form.job_description" rows="4" />
        </label>
        <div class="actions wide">
          <button class="primary" type="submit" :disabled="!canCreate || isLoading('createPracticeGoal')">Create Goal</button>
        </div>
      </form>
    </section>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Active Goals</h2>
          <p>{{ goals.length }} goal{{ goals.length === 1 ? '' : 's' }}</p>
        </div>
      </div>
      <EmptyState v-if="!goals.length" title="No practice goals" message="Create a goal to start coaching or mock without an interview." />
      <div v-else class="goal-list">
        <article v-for="goal in goals" :key="goal.goal_id" class="goal-card">
          <header class="task-head">
            <div>
              <strong>{{ goal.company_name }} · {{ goal.job_title || 'Role unset' }}</strong>
              <small>{{ goal.target_round || '-' }} · {{ goal.remaining_days || 1 }} days · {{ goal.goal_id }}</small>
            </div>
            <StatusBadge :status="goal.status" />
          </header>
          <p>{{ goal.focus_topics?.join(', ') || 'No focus topics.' }}</p>
          <div class="actions">
            <button class="primary" type="button" :disabled="isLoading(`goalPlan:${goal.goal_id}`)" @click="generatePlan(goal)">Generate Plan</button>
            <button class="secondary" type="button" :disabled="isLoading(`goalMock:${goal.goal_id}`)" @click="startMock(goal)">Start Mock</button>
            <button class="secondary" type="button" :disabled="isLoading(`archiveGoal:${goal.goal_id}`)" @click="archiveGoal(goal)">Archive</button>
          </div>
        </article>
      </div>
    </section>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'
import EmptyState from '../components/EmptyState.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { rememberSelection, runWithStatus, workbenchState as state } from '../workbenchState'

const router = useRouter()
const goals = ref([])
const focusTopicsText = ref('')
const form = reactive({
  user_id: state.userId || 'user_001',
  company_name: '',
  job_title: '',
  target_round: 'second_round',
  remaining_days: 3,
  job_description: '',
})

const focusTopics = computed(() => focusTopicsText.value.split(',').map((topic) => topic.trim()).filter(Boolean))
const canCreate = computed(() => Boolean(form.user_id && form.company_name))

function isLoading(key) {
  return state.loadingKeys.has(key)
}

async function loadGoals() {
  const loaded = await runWithStatus(
    'practiceGoals',
    () => api.listPracticeGoals({ user_id: form.user_id, status: 'active' }),
    'Practice goals loaded',
  )
  goals.value = Array.isArray(loaded) ? loaded : []
}

async function createGoal() {
  const created = await runWithStatus(
    'createPracticeGoal',
    () => api.createPracticeGoal({ ...form, focus_topics: focusTopics.value }),
    'Practice goal created',
  )
  goals.value = [created, ...goals.value.filter((goal) => goal.goal_id !== created.goal_id)]
}

async function generatePlan(goal) {
  const plan = await runWithStatus(
    `goalPlan:${goal.goal_id}`,
    () => api.generatePracticeGoalCoachingPlan(goal.goal_id, {
      user_id: goal.user_id,
      target_round: goal.target_round,
      remaining_days: goal.remaining_days,
    }),
    'Coaching plan generated',
  )
  rememberSelection('selectedPlanId', plan.plan_id)
  rememberSelection('selectedSessionId', '')
  rememberSelection('selectedInterviewId', '')
  router.push('/coaching')
}

async function startMock(goal) {
  const mock = await runWithStatus(
    `goalMock:${goal.goal_id}`,
    () => api.startPracticeGoalMock(goal.goal_id, {
      user_id: goal.user_id,
      target_round: goal.target_round,
      focus_topic: goal.focus_topics?.[0] || '',
    }),
    'Mock interview ready',
  )
  rememberSelection('selectedMockId', mock.mock_id)
  rememberSelection('selectedSessionId', '')
  rememberSelection('selectedInterviewId', '')
  router.push('/mock')
}

async function archiveGoal(goal) {
  await runWithStatus(`archiveGoal:${goal.goal_id}`, () => api.archivePracticeGoal(goal.goal_id), 'Practice goal archived')
  goals.value = goals.value.filter((item) => item.goal_id !== goal.goal_id)
}

watch(() => state.userId, (userId) => {
  if (userId && form.user_id !== userId) {
    form.user_id = userId
    loadGoals()
  }
})

onMounted(loadGoals)
</script>

<style scoped>
.goal-create-panel {
  margin-bottom: 14px;
}

.goal-form {
  align-items: end;
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(5, minmax(0, 1fr));
}

.goal-form .wide {
  grid-column: 1 / -1;
}

.goal-list {
  display: grid;
  gap: 10px;
}

.goal-card {
  background: #fff;
  border: 1px solid #e2e7ee;
  border-radius: 6px;
  display: grid;
  gap: 10px;
  padding: 12px;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

@media (max-width: 900px) {
  .goal-form {
    grid-template-columns: 1fr;
  }
}
</style>
