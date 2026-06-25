<template>
  <article class="timeline-item" :class="{ 'has-error': Boolean(errorMessage) }">
    <div class="timeline-marker" aria-hidden="true"></div>
    <div class="timeline-card">
      <header class="timeline-head">
        <div>
          <strong>{{ title }}</strong>
          <div class="mini-meta">{{ subtitle }}</div>
        </div>
        <StatusBadge :status="status" />
      </header>

      <p v-if="primaryText" class="timeline-primary">{{ primaryText }}</p>

      <dl class="field-stack">
        <div v-if="turn?.score !== undefined && turn?.score !== null" class="field-row">
          <dt>Score</dt>
          <dd>{{ turn.score }}</dd>
        </div>
        <div v-if="actionText" class="field-row">
          <dt>Action</dt>
          <dd>{{ actionText }}</dd>
        </div>
        <div v-if="turn?.feedback" class="field-row">
          <dt>Feedback</dt>
          <dd>{{ turn.feedback }}</dd>
        </div>
        <div v-if="turn?.next_question" class="field-row">
          <dt>Next</dt>
          <dd>{{ turn.next_question }}</dd>
        </div>
        <div v-if="errorMessage" class="field-row has-error">
          <dt>Error</dt>
          <dd>{{ errorMessage }}</dd>
        </div>
      </dl>
    </div>
  </article>
</template>

<script setup>
import { computed } from 'vue'
import StatusBadge from './StatusBadge.vue'

const props = defineProps({
  turn: { type: Object, required: true },
  kind: { type: String, default: 'mock' },
})

const title = computed(() => (props.kind === 'coaching' ? 'Coaching turn' : 'Mock interview turn'))

const subtitle = computed(() => {
  const parts = []
  if (props.turn?.turn_id) parts.push(`turn_id: ${props.turn.turn_id}`)
  if (props.turn?.created_at) parts.push(`created_at: ${props.turn.created_at}`)
  return parts.length ? parts.join(' | ') : '-'
})

const errorMessage = computed(() => props.turn?.error_message || '')

const status = computed(() => {
  if (errorMessage.value) return 'error'
  const value = props.turn?.agent_action || props.turn?.turn_type || props.turn?.phase || props.turn?.role || 'unknown'
  return typeof value === 'string' ? value : JSON.stringify(value)
})

const primaryText = computed(() => {
  const fields = ['content', 'user_answer', 'interviewer_question', 'feedback', 'next_question']
  return fields.map((field) => props.turn?.[field]).find((value) => value !== undefined && value !== null && value !== '') || ''
})

const actionText = computed(() => {
  if (!props.turn?.agent_action) return ''
  if (typeof props.turn.agent_action === 'string') return props.turn.agent_action
  return JSON.stringify(props.turn.agent_action)
})
</script>
