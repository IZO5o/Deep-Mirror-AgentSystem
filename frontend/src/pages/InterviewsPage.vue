<template>
  <section class="page">
    <div class="page-header">
      <div>
        <span class="page-kicker">Interview sessions</span>
        <h1>Interviews</h1>
        <p class="muted">Create interview material records and open detail pages without forcing a training flow.</p>
      </div>
      <button class="secondary" type="button" @click="load">Refresh</button>
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Create Interview</h2>
          <p>Company is required by the API; other fields can be refined later.</p>
        </div>
      </div>
      <div class="form-grid">
        <label>
          Company
          <input v-model.trim="form.company_name" autocomplete="off" placeholder="Acme" />
        </label>
        <label>
          Job Title
          <input v-model.trim="form.job_title" autocomplete="off" placeholder="Backend Engineer" />
        </label>
        <label>
          Round
          <input v-model.trim="form.interview_round" autocomplete="off" placeholder="second_round" />
        </label>
        <label>
          Type
          <input v-model.trim="form.interview_type" autocomplete="off" placeholder="technical" />
        </label>
      </div>
      <div class="actions">
        <button class="primary" type="button" :disabled="!canCreate" @click="create">Create</button>
      </div>
    </section>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Interview List</h2>
          <p>Opening a row stores the selected interview id in shared workbench state.</p>
        </div>
      </div>
      <EmptyState
        v-if="!interviews.length"
        title="No interview records"
        message="Create one above or seed demo data."
      />
      <div v-else class="list">
        <RouterLink
          v-for="item in interviews"
          :key="item.interview_id"
          class="row-card link-card"
          :to="`/interviews/${item.interview_id}`"
          @click="selectInterview(item.interview_id)"
        >
          <div>
            <strong>{{ item.company_name || item.interview_id }}</strong>
            <span>{{ item.job_title || 'No job title' }} · {{ item.interview_round || 'round unset' }} · {{ item.interview_type || 'type unset' }}</span>
          </div>
          <StatusBadge :status="item.status" />
        </RouterLink>
      </div>
    </section>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { api } from '../api'
import EmptyState from '../components/EmptyState.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { rememberSelection, runWithStatus, workbenchState as state } from '../workbenchState'

const interviews = ref([])
const form = reactive({
  company_name: '',
  job_title: '',
  interview_round: 'second_round',
  interview_type: 'technical',
})

const canCreate = computed(() => form.company_name.trim().length > 0)

async function load() {
  interviews.value = await runWithStatus('interviews', () => api.listInterviews(state.userId), 'Interviews refreshed')
}

async function create() {
  const created = await runWithStatus(
    'createInterview',
    () => api.createInterview({ ...form, user_id: state.userId }),
    'Interview created',
  )
  rememberSelection('selectedInterviewId', created.interview_id)
  form.company_name = ''
  await load()
}

function selectInterview(interviewId) {
  rememberSelection('selectedInterviewId', interviewId)
}

onMounted(load)
</script>

<style scoped>
.link-card {
  color: inherit;
  text-decoration: none;
}
</style>
