<template>
  <main class="page practice-goals">
    <section class="panel">
      <h1>Practice Goals</h1>
      <form class="goal-form" @submit.prevent="createGoal">
        <label>
          User ID
          <input v-model.trim="form.user_id" required />
        </label>
        <label>
          Company
          <input v-model.trim="form.company_name" required />
        </label>
        <label>
          Job Title
          <input v-model.trim="form.job_title" />
        </label>
        <label>
          Target Round
          <input v-model.trim="form.target_round" />
        </label>
        <label>
          Remaining Days
          <input v-model.number="form.remaining_days" min="1" type="number" />
        </label>
        <label class="wide">
          Focus Topics
          <input v-model.trim="focusTopicsText" placeholder="缓存一致性, Redlock 争议点" />
        </label>
        <label class="wide">
          Job Description
          <textarea v-model.trim="form.job_description" rows="4" />
        </label>
        <button class="primary" type="submit" :disabled="loading">Create Goal</button>
        <button class="secondary" type="button" :disabled="loading || !form.user_id" @click="loadGoals">Refresh</button>
      </form>
      <p v-if="message" class="message">{{ message }}</p>
      <p v-if="error" class="error">{{ error }}</p>
    </section>

    <section class="goal-list">
      <article v-for="goal in goals" :key="goal.goal_id" class="goal-card">
        <div>
          <h2>{{ goal.company_name || 'Practice goal' }}</h2>
          <p>{{ goal.job_title }} · {{ goal.target_round }} · {{ goal.remaining_days }} days</p>
          <p v-if="goal.focus_topics?.length" class="topics">{{ goal.focus_topics.join(', ') }}</p>
        </div>
        <div class="actions">
          <button type="button" :disabled="loading" @click="generatePlan(goal)">Generate Plan</button>
          <button type="button" :disabled="loading" @click="startMock(goal)">Start Mock</button>
          <button type="button" :disabled="loading" @click="archiveGoal(goal)">Archive</button>
        </div>
      </article>
      <p v-if="!goals.length" class="empty">No active practice goals.</p>
    </section>
  </main>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '../api.js'

const loading = ref(false)
const message = ref('')
const error = ref('')
const goals = ref([])
const focusTopicsText = ref('')
const form = reactive({
  user_id: 'user_001',
  company_name: '',
  job_title: '',
  target_round: 'second_round',
  job_description: '',
  remaining_days: 3,
})

const focusTopics = computed(() => focusTopicsText.value.split(',').map((topic) => topic.trim()).filter(Boolean))

async function run(action, successMessage) {
  loading.value = true
  message.value = ''
  error.value = ''
  try {
    const result = await action()
    message.value = successMessage
    return result
  } catch (err) {
    error.value = err.message
    return null
  } finally {
    loading.value = false
  }
}

async function loadGoals() {
  const result = await run(
    () => api.listPracticeGoals({ user_id: form.user_id, status: 'active' }),
    'Goals loaded',
  )
  if (result) goals.value = result
}

async function createGoal() {
  const result = await run(
    () => api.createPracticeGoal({ ...form, focus_topics: focusTopics.value }),
    'Goal created',
  )
  if (result) {
    form.company_name = ''
    form.job_title = ''
    form.job_description = ''
    focusTopicsText.value = ''
    await loadGoals()
  }
}

async function generatePlan(goal) {
  await run(
    () => api.generatePracticeGoalCoachingPlan(goal.goal_id, {
      user_id: goal.user_id,
      target_round: goal.target_round,
      remaining_days: goal.remaining_days,
    }),
    'Coaching plan generated',
  )
}

async function startMock(goal) {
  await run(
    () => api.startPracticeGoalMock(goal.goal_id, {
      user_id: goal.user_id,
      target_round: goal.target_round,
    }),
    'Mock interview started',
  )
}

async function archiveGoal(goal) {
  await run(() => api.archivePracticeGoal(goal.goal_id), 'Goal archived')
  await loadGoals()
}

onMounted(loadGoals)
</script>
