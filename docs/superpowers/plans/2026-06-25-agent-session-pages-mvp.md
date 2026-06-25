# Agent Session Pages MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the stub Coaching, Mock Interview, and Engineering Trace pages into a demonstrable MVP using existing backend APIs and preserving the `memory_candidates -> accept/reject -> memory_items` boundary.

**Architecture:** Keep all business flow semantics in the existing Go backend. Add frontend API helpers, small read-only display components, and page-level Vue state that calls existing coaching, mock, practice-state, trace, and evaluation endpoints. Trace links pass `source_type` and `source_id` through route query params so Coaching and Mock pages can deep-link into Engineering Trace without adding backend routes.

**Tech Stack:** Vue 3 Composition API, Vue Router 4, Vite, existing Gin/Go APIs, existing `workbenchState` localStorage selections, Node built-in `assert` for lightweight API helper tests, `go test ./...`, `npm run build`.

---

## Scope And Constraints

This plan intentionally does not add business Agents, ReAct, MCP, function calling, scoring logic, prompt changes, or backend state transitions. It does not call `POST /api/coaching-sessions/:session_id/memory-candidates` or `POST /api/mock-interviews/:mock_id/memory-candidates` from the MVP pages. Formal memory writes remain available only through `memory_candidates -> accept/reject -> memory_items`.

The existing `frontend/src/workbenchState.js` already persists:

- `selectedInterviewId`
- `selectedPlanId`
- `selectedSessionId`
- `selectedMockId`

Use those fields directly. Do not add new global state unless a later task proves the page cannot be restored with the existing IDs.

No backend change is expected. If implementation later proves resume/restore is unusable without list endpoints, stop and ask for a plan update before adding backend APIs.

## File Structure

- Modify `frontend/package.json`: add a focused `test:api` script for helper-level tests.
- Modify `frontend/src/api.js`: add helpers for coaching, mock interview, practice states, traces, and evaluations.
- Create `frontend/src/api.test.mjs`: lightweight Node tests for URL construction, query filtering, JSON body encoding, and error handling for the new helper methods.
- Create `frontend/src/components/JsonBlock.vue`: scrollable/collapsible JSON/text renderer for trace snapshots and raw output.
- Create `frontend/src/components/TraceEvaluationSummary.vue`: compact evaluation report renderer shared by Trace, Coaching, and Mock pages.
- Create `frontend/src/components/TurnTimelineItem.vue`: display one coaching or mock turn consistently.
- Modify `frontend/src/components/StatusBadge.vue`: style failed/cancelled/error states so failed traces and sessions are visible.
- Modify `frontend/src/pages/EngineeringTracePage.vue`: replace the stub with filters, trace list, selected trace detail, and evaluation report.
- Modify `frontend/src/pages/CoachingPage.vue`: replace the stub with interview context, plan/task/session controls, composer, practice evidence, and trace link.
- Modify `frontend/src/pages/MockInterviewPage.vue`: replace the stub with interview context, mock controls, turn timeline, composer, practice evidence, and trace link.
- Modify `frontend/src/styles.css`: add shared layout utilities for dense page panels, list rows, JSON blocks, timeline rows, and trace check states.

## Existing API Shapes To Use

Use the response fields already exposed from `vo/vo.go`:

- `CoachingPlanVO`: `plan_id`, `user_id`, `interview_id`, `target_round`, `remaining_days`, `company_name`, `job_title`, `overall_strategy`, `focus_summary`, `status`, `created_at`, `updated_at`.
- `CoachingTaskVO`: `task_id`, `sequence`, `day_index`, `task_type`, `title`, `description`, `priority`, `status`, `related_memory_ids`.
- `CoachingSessionDetailVO`: `session`, `current_task`, `tasks`, `turns`, `attempts`.
- `CoachingSessionVO`: `session_id`, `status`, `current_task_id`, `progress_summary`, `last_agent_message`, `error_message`, `last_active_at`.
- `CoachingSessionTurnVO`: `turn_id`, `role`, `turn_type`, `content`, `agent_action`, `score`, `feedback`, `error_message`, `created_at`.
- `CoachingTaskAttemptVO`: `attempt_id`, `coaching_task_id`, `user_answer`, `score`, `feedback`, `passed`, `attempt_index`, `error_message`, `created_at`.
- `MockInterviewVO`: `mock_id`, `interview_id`, `plan_id`, `target_round`, `status`, `current_turn`, `current_topic`, `overall_goal`, `first_question`, `last_feedback`, `error_message`, `final_summary`.
- `MockTurnVO`: `turn_id`, `turn_index`, `role`, `turn_type`, `phase`, `agent_action`, `content`, `interviewer_question`, `user_answer`, `feedback`, `score`, `follow_up_reason`, `topic_tags`, `next_question`, `error_message`.
- `PracticeStateVO`: `state_id`, `topic`, `dimension`, `mastery_score`, `attempt_count`, `last_score`, `last_feedback`, `source_type`, `source_id`, `last_practiced_at`.
- `AgentDecisionTraceVO`: `trace_id`, `agent_type`, `source_type`, `source_id`, `step_name`, `selected_context_snapshot`, `input_snapshot`, `raw_agent_output`, `parsed_decision`, `service_actions`, `status`, `error_message`, `created_at`.
- `AgentEvaluationReportVO`: `total_traces`, `passed_traces`, `failed_traces`, `results`.
- `AgentEvaluationResultVO`: `trace_id`, `agent_type`, `source_type`, `source_id`, `step_name`, `trace_status`, `passed`, `score`, `checks`.
- `AgentEvaluationCheckVO`: `name`, `passed`, `reason`.

### Task 1: Add Frontend API Helpers And Tests

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/src/api.js`
- Create: `frontend/src/api.test.mjs`

- [ ] **Step 1: Add the API test script**

Modify `frontend/package.json` so the `scripts` object becomes:

```json
{
  "dev": "vite --host 127.0.0.1 --port 5173",
  "build": "vite build",
  "preview": "vite preview --host 127.0.0.1 --port 4173",
  "test:api": "node src/api.test.mjs"
}
```

- [ ] **Step 2: Write failing API helper tests**

Create `frontend/src/api.test.mjs`:

```js
import assert from 'node:assert/strict'
import { api, buildQuery } from './api.js'

const calls = []

function installFetch(responseFactory) {
  calls.length = 0
  globalThis.fetch = async (path, init = {}) => {
    calls.push({ path, init })
    return responseFactory(path, init)
  }
}

function ok(data) {
  return {
    ok: true,
    status: 200,
    text: async () => JSON.stringify({ code: 0, data }),
  }
}

function fail(status, payload) {
  return {
    ok: false,
    status,
    text: async () => JSON.stringify(payload),
  }
}

assert.equal(buildQuery({ user_id: 'user_001', empty: '', missing: undefined, limit: 25 }), '?user_id=user_001&limit=25')

installFetch(() => ok({ plan_id: 'plan_1' }))
await api.generateCoachingPlan('int_1', { user_id: 'user_001', target_round: 'second_round', remaining_days: 7 })
assert.equal(calls[0].path, '/api/interviews/int_1/coaching-plan')
assert.equal(calls[0].init.method, 'POST')
assert.deepEqual(JSON.parse(calls[0].init.body), {
  user_id: 'user_001',
  target_round: 'second_round',
  remaining_days: 7,
})

installFetch(() => ok({ session: { session_id: 'sess_1' } }))
await api.startOrResumeCoachingSession('plan_1', 'user_001')
assert.equal(calls[0].path, '/api/coaching-plans/plan_1/sessions?user_id=user_001')
assert.equal(calls[0].init.method, 'POST')

installFetch(() => ok({ turn_id: 'turn_1' }))
await api.submitCoachingTurn('sess_1', 'formal answer')
assert.equal(calls[0].path, '/api/coaching-sessions/sess_1/turns')
assert.deepEqual(JSON.parse(calls[0].init.body), { user_input: 'formal answer' })

installFetch(() => ok({ mock_id: 'mock_1' }))
await api.startMockInterview('int_1', { user_id: 'user_001', plan_id: 'plan_1', target_round: 'second_round' })
assert.equal(calls[0].path, '/api/interviews/int_1/mock-interviews')
assert.deepEqual(JSON.parse(calls[0].init.body), {
  user_id: 'user_001',
  plan_id: 'plan_1',
  target_round: 'second_round',
})

installFetch(() => ok({ turn_id: 'turn_2' }))
await api.submitMockTurn('mock_1', 'mock answer')
assert.equal(calls[0].path, '/api/mock-interviews/mock_1/turns')
assert.deepEqual(JSON.parse(calls[0].init.body), { answer: 'mock answer' })

installFetch(() => ok([{ trace_id: 'trace_1' }]))
await api.listAgentDecisionTraces({
  user_id: 'user_001',
  source_type: 'coaching_session',
  source_id: 'sess_1',
  status: '',
  limit: 20,
})
assert.equal(
  calls[0].path,
  '/api/agent-decision-traces?user_id=user_001&source_type=coaching_session&source_id=sess_1&limit=20',
)

installFetch(() => ok({ total_traces: 1, results: [] }))
await api.listAgentEvaluations({ source_type: 'mock_interview', source_id: 'mock_1', limit: 20 })
assert.equal(calls[0].path, '/api/agent-evaluations?source_type=mock_interview&source_id=mock_1&limit=20')

installFetch(() => ok([{ state_id: 'state_1' }]))
await api.listPracticeStates({ user_id: 'user_001', topic: '', dimension: undefined })
assert.equal(calls[0].path, '/api/practice-states?user_id=user_001')

installFetch(() => fail(500, { code: 500, msg: 'backend exploded' }))
await assert.rejects(() => api.getCoachingSession('sess_404'), /backend exploded/)

console.log('api helper tests passed')
```

- [ ] **Step 3: Run the API test and verify it fails**

Run:

```bash
cd frontend
npm run test:api
```

Expected: FAIL with an error like `TypeError: api.generateCoachingPlan is not a function`.

- [ ] **Step 4: Add the API helper implementation**

Modify `frontend/src/api.js` by adding these methods inside the exported `api` object after `listMemoryItems(userId)`:

```js
  generateCoachingPlan(interviewId, body) {
    return apiRequest(`/api/interviews/${interviewId}/coaching-plan`, { method: 'POST', body })
  },
  getCoachingPlan(interviewId) {
    return apiRequest(`/api/interviews/${interviewId}/coaching-plan`)
  },
  listCoachingTasks(planId) {
    return apiRequest(`/api/coaching-plans/${planId}/tasks`)
  },
  startOrResumeCoachingSession(planId, userId) {
    return apiRequest(`/api/coaching-plans/${planId}/sessions${buildQuery({ user_id: userId })}`, { method: 'POST' })
  },
  getCoachingSession(sessionId) {
    return apiRequest(`/api/coaching-sessions/${sessionId}`)
  },
  submitCoachingTurn(sessionId, userInput) {
    return apiRequest(`/api/coaching-sessions/${sessionId}/turns`, {
      method: 'POST',
      body: { user_input: userInput },
    })
  },
  pauseCoachingSession(sessionId) {
    return apiRequest(`/api/coaching-sessions/${sessionId}/pause`, { method: 'POST' })
  },
  cancelCoachingSession(sessionId) {
    return apiRequest(`/api/coaching-sessions/${sessionId}/cancel`, { method: 'POST' })
  },
  startMockInterview(interviewId, body) {
    return apiRequest(`/api/interviews/${interviewId}/mock-interviews`, { method: 'POST', body })
  },
  getMockInterview(mockId) {
    return apiRequest(`/api/mock-interviews/${mockId}`)
  },
  listMockTurns(mockId) {
    return apiRequest(`/api/mock-interviews/${mockId}/turns`)
  },
  submitMockTurn(mockId, answer) {
    return apiRequest(`/api/mock-interviews/${mockId}/turns`, {
      method: 'POST',
      body: { answer },
    })
  },
  completeMockInterview(mockId) {
    return apiRequest(`/api/mock-interviews/${mockId}/complete`, { method: 'POST' })
  },
  cancelMockInterview(mockId) {
    return apiRequest(`/api/mock-interviews/${mockId}/cancel`, { method: 'POST' })
  },
  listPracticeStates(filters) {
    return apiRequest(`/api/practice-states${buildQuery(filters)}`)
  },
  listAgentDecisionTraces(filters) {
    return apiRequest(`/api/agent-decision-traces${buildQuery(filters)}`)
  },
  listAgentEvaluations(filters) {
    return apiRequest(`/api/agent-evaluations${buildQuery(filters)}`)
  },
```

Keep the existing memory helpers unchanged.

- [ ] **Step 5: Run API helper tests**

Run:

```bash
cd frontend
npm run test:api
```

Expected: PASS and prints `api helper tests passed`.

- [ ] **Step 6: Verify the frontend still builds**

Run:

```bash
cd frontend
npm run build
```

Expected: PASS with Vite build output and no syntax errors.

- [ ] **Step 7: Commit**

Run:

```bash
git add frontend/package.json frontend/src/api.js frontend/src/api.test.mjs
git commit -m "test: cover agent session api helpers"
```

### Task 2: Add Shared Display Components

**Files:**
- Create: `frontend/src/components/JsonBlock.vue`
- Create: `frontend/src/components/TraceEvaluationSummary.vue`
- Create: `frontend/src/components/TurnTimelineItem.vue`
- Modify: `frontend/src/components/StatusBadge.vue`
- Modify: `frontend/src/styles.css`
- Test: `frontend/src/components/JsonBlock.vue`, `frontend/src/components/TraceEvaluationSummary.vue`, `frontend/src/components/TurnTimelineItem.vue` through `npm run build`

- [ ] **Step 1: Create a JSON/text block component**

Create `frontend/src/components/JsonBlock.vue`:

```vue
<template>
  <details class="json-block" :open="open">
    <summary>{{ title }}</summary>
    <pre>{{ formattedValue }}</pre>
  </details>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  title: { type: String, required: true },
  value: { type: [String, Object, Array, Number, Boolean], default: '' },
  open: { type: Boolean, default: false },
})

const formattedValue = computed(() => {
  if (props.value === null || props.value === undefined || props.value === '') {
    return '-'
  }
  if (typeof props.value !== 'string') {
    return JSON.stringify(props.value, null, 2)
  }
  try {
    return JSON.stringify(JSON.parse(props.value), null, 2)
  } catch {
    return props.value
  }
})
</script>
```

- [ ] **Step 2: Create the evaluation summary component**

Create `frontend/src/components/TraceEvaluationSummary.vue`:

```vue
<template>
  <section class="evaluation-summary">
    <div class="metric-row">
      <div>
        <span>Total traces</span>
        <strong>{{ report?.total_traces ?? 0 }}</strong>
      </div>
      <div>
        <span>Passed</span>
        <strong>{{ report?.passed_traces ?? 0 }}</strong>
      </div>
      <div>
        <span>Failed</span>
        <strong>{{ report?.failed_traces ?? 0 }}</strong>
      </div>
    </div>

    <EmptyState
      v-if="!results.length"
      title="No evaluation results"
      message="Run or filter traces to see rule checks."
    />

    <div v-else class="evaluation-list">
      <article
        v-for="result in results"
        :key="result.trace_id"
        class="evaluation-card"
        :class="{ failed: !result.passed }"
      >
        <div class="evaluation-head">
          <div>
            <strong>{{ result.step_name || result.trace_id }}</strong>
            <p>{{ result.agent_type }} · {{ result.source_type }} · {{ result.source_id }}</p>
          </div>
          <StatusBadge :status="result.passed ? 'passed' : 'failed'" />
        </div>
        <div class="check-list">
          <div
            v-for="check in result.checks || []"
            :key="`${result.trace_id}-${check.name}`"
            class="check-row"
            :class="{ failed: !check.passed }"
          >
            <strong>{{ check.passed ? 'PASS' : 'FAIL' }}</strong>
            <span>{{ check.name }}</span>
            <small>{{ check.reason || '-' }}</small>
          </div>
        </div>
      </article>
    </div>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import EmptyState from './EmptyState.vue'
import StatusBadge from './StatusBadge.vue'

const props = defineProps({
  report: { type: Object, default: null },
})

const results = computed(() => props.report?.results || [])
</script>
```

- [ ] **Step 3: Create the turn timeline component**

Create `frontend/src/components/TurnTimelineItem.vue`:

```vue
<template>
  <article class="timeline-item" :class="{ failed: Boolean(turn.error_message) }">
    <div class="timeline-head">
      <div>
        <strong>{{ title }}</strong>
        <p>{{ subtitle }}</p>
      </div>
      <StatusBadge :status="status" />
    </div>

    <p v-if="primaryText" class="timeline-content">{{ primaryText }}</p>

    <dl class="timeline-meta">
      <div v-if="turn.score">
        <dt>score</dt>
        <dd>{{ turn.score }}</dd>
      </div>
      <div v-if="turn.agent_action">
        <dt>action</dt>
        <dd>{{ turn.agent_action }}</dd>
      </div>
      <div v-if="turn.feedback">
        <dt>feedback</dt>
        <dd>{{ turn.feedback }}</dd>
      </div>
      <div v-if="turn.next_question">
        <dt>next</dt>
        <dd>{{ turn.next_question }}</dd>
      </div>
      <div v-if="turn.error_message">
        <dt>error</dt>
        <dd>{{ turn.error_message }}</dd>
      </div>
    </dl>
  </article>
</template>

<script setup>
import { computed } from 'vue'
import StatusBadge from './StatusBadge.vue'

const props = defineProps({
  turn: { type: Object, required: true },
  kind: { type: String, default: 'coaching' },
})

const title = computed(() => {
  if (props.kind === 'mock') {
    return `Turn ${props.turn.turn_index ?? '-'} · ${props.turn.phase || props.turn.turn_type || props.turn.role || 'mock'}`
  }
  return `${props.turn.role || 'agent'} · ${props.turn.turn_type || 'turn'}`
})

const subtitle = computed(() => {
  const created = props.turn.created_at ? new Date(props.turn.created_at * 1000).toLocaleString() : ''
  return [props.turn.turn_id, created].filter(Boolean).join(' · ')
})

const status = computed(() => {
  if (props.turn.error_message) {
    return 'failed'
  }
  return props.turn.agent_action || props.turn.turn_type || props.turn.phase || props.turn.role || 'turn'
})

const primaryText = computed(() => {
  return props.turn.content || props.turn.user_answer || props.turn.interviewer_question || props.turn.feedback || props.turn.next_question || ''
})
</script>
```

- [ ] **Step 4: Extend status badge failure styling**

Modify `frontend/src/components/StatusBadge.vue` by appending this CSS inside the existing `<style scoped>` block:

```css
.status-badge[data-status="failed"],
.status-badge[data-status="error"],
.status-badge[data-status="cancelled"],
.status-badge[data-status="canceled"] {
  background: #fff1f0;
  border-color: #ffc7c2;
  color: #9d1c16;
}

.status-badge[data-status="paused"],
.status-badge[data-status="passed"] {
  background: #eef9f1;
  border-color: #b9e4c4;
  color: #176331;
}
```

- [ ] **Step 5: Add shared styles**

Append to `frontend/src/styles.css`:

```css
.dense-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.field-row {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.list-row,
.timeline-item,
.evaluation-card {
  background: #ffffff;
  border: 1px solid #e2e7ee;
  border-radius: 6px;
  display: grid;
  gap: 8px;
  padding: 10px;
}

.list-row.failed,
.timeline-item.failed,
.evaluation-card.failed {
  border-color: #ffc7c2;
  background: #fffafa;
}

.timeline-head,
.evaluation-head,
.row-head {
  align-items: flex-start;
  display: flex;
  gap: 10px;
  justify-content: space-between;
}

.timeline-content,
.summary-box {
  background: #f7f9fc;
  border: 1px solid #e4e9f0;
  border-radius: 6px;
  color: #405069;
  overflow-wrap: anywhere;
  padding: 9px;
}

.timeline-meta,
.mini-meta {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin: 0;
}

.timeline-meta div,
.mini-meta div {
  min-width: 0;
}

.timeline-meta dt,
.mini-meta dt {
  color: #667286;
  font-size: 12px;
  font-weight: 700;
}

.timeline-meta dd,
.mini-meta dd {
  margin: 2px 0 0;
  overflow-wrap: anywhere;
}

.json-block {
  background: #0f172a;
  border-radius: 6px;
  color: #e5e7eb;
  overflow: hidden;
}

.json-block summary {
  background: #1e293b;
  cursor: pointer;
  font-weight: 700;
  padding: 8px 10px;
}

.json-block pre {
  max-height: 280px;
  overflow: auto;
  padding: 10px;
}

.metric-row {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.metric-row div {
  background: #f7f9fc;
  border: 1px solid #e4e9f0;
  border-radius: 6px;
  display: grid;
  gap: 4px;
  padding: 9px;
}

.metric-row span,
.evaluation-head p,
.timeline-head p {
  color: #667286;
  font-size: 12px;
}

.check-list {
  display: grid;
  gap: 6px;
}

.check-row {
  align-items: start;
  background: #f7fbf8;
  border: 1px solid #d8eadc;
  border-radius: 6px;
  display: grid;
  gap: 8px;
  grid-template-columns: 52px minmax(160px, 0.4fr) minmax(0, 1fr);
  padding: 8px;
}

.check-row.failed {
  background: #fff1f0;
  border-color: #ffc7c2;
}

@media (max-width: 900px) {
  .dense-grid,
  .field-row,
  .timeline-meta,
  .mini-meta,
  .metric-row,
  .check-row {
    grid-template-columns: 1fr;
  }
}
```

- [ ] **Step 6: Build to verify components compile**

Run:

```bash
cd frontend
npm run build
```

Expected: PASS with Vite build output.

- [ ] **Step 7: Commit**

Run:

```bash
git add frontend/src/components/JsonBlock.vue frontend/src/components/TraceEvaluationSummary.vue frontend/src/components/TurnTimelineItem.vue frontend/src/components/StatusBadge.vue frontend/src/styles.css
git commit -m "feat: add agent trace display components"
```

### Task 3: Implement Engineering Trace Page

**Files:**
- Modify: `frontend/src/pages/EngineeringTracePage.vue`
- Test: `frontend/src/pages/EngineeringTracePage.vue` through `npm run build` and manual query-param checks

- [ ] **Step 1: Replace the Trace page stub**

Replace `frontend/src/pages/EngineeringTracePage.vue` with:

```vue
<template>
  <section class="page">
    <div class="page-header">
      <div>
        <span class="page-kicker">Agent observability</span>
        <h1>Engineering Trace</h1>
        <p class="muted">selected context -> raw output -> parsed decision -> service actions -> evaluation checks</p>
      </div>
      <button class="secondary" type="button" @click="load">Refresh</button>
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Trace Filters</h2>
          <p>Deep links from Coaching and Mock Interview prefill source filters.</p>
        </div>
        <StatusBadge :status="traceStatus" />
      </div>
      <div class="field-row">
        <label>user_id <input v-model="filters.user_id" /></label>
        <label>interview_id <input v-model="filters.interview_id" /></label>
        <label>source_type <input v-model="filters.source_type" placeholder="coaching_session or mock_interview" /></label>
        <label>source_id <input v-model="filters.source_id" /></label>
        <label>agent_type <input v-model="filters.agent_type" /></label>
        <label>step_name <input v-model="filters.step_name" /></label>
        <label>status <input v-model="filters.status" placeholder="success or failed" /></label>
        <label>limit <input v-model.number="filters.limit" type="number" min="1" max="100" /></label>
      </div>
      <div class="actions">
        <button class="primary" type="button" @click="applyFilters">Apply Filters</button>
        <button class="secondary" type="button" @click="resetFilters">Reset</button>
      </div>
      <ErrorNotice v-if="loadError" :message="loadError" />
    </section>

    <div class="dense-grid">
      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Trace List</h2>
            <p>{{ traces.length }} traces loaded.</p>
          </div>
        </div>
        <EmptyState v-if="!traces.length" title="No traces" message="Run a coaching or mock flow, then refresh this page." />
        <div v-else class="list compact">
          <button
            v-for="trace in traces"
            :key="trace.trace_id"
            class="list-row id-button"
            :class="{ failed: trace.status !== 'success' }"
            type="button"
            @click="selectedTraceId = trace.trace_id"
          >
            <span class="row-head">
              <strong>{{ trace.step_name || trace.trace_id }}</strong>
              <StatusBadge :status="trace.status" />
            </span>
            <small>{{ formatTime(trace.created_at) }} · {{ trace.agent_type }} · {{ trace.source_type }} · {{ trace.source_id }}</small>
            <small v-if="trace.error_message" class="blocked">{{ trace.error_message }}</small>
          </button>
        </div>
      </section>

      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Evaluation Report</h2>
            <p>Rule checks highlight failed traces and memory boundary evidence.</p>
          </div>
        </div>
        <TraceEvaluationSummary :report="evaluationReport" />
        <ErrorNotice v-if="evaluationError" :message="evaluationError" />
      </section>
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Trace Detail</h2>
          <p>{{ selectedTrace?.trace_id || 'Select a trace from the list.' }}</p>
        </div>
        <StatusBadge v-if="selectedTrace" :status="selectedTrace.status" />
      </div>
      <EmptyState v-if="!selectedTrace" title="No trace selected" message="Select a row to inspect context, model output, parsed decision, and service actions." />
      <div v-else class="trace-detail-grid">
        <JsonBlock title="Selected context snapshot" :value="selectedTrace.selected_context_snapshot" open />
        <JsonBlock title="Input snapshot" :value="selectedTrace.input_snapshot" />
        <JsonBlock title="Raw agent output" :value="selectedTrace.raw_agent_output" />
        <JsonBlock title="Parsed decision" :value="selectedTrace.parsed_decision" open />
        <JsonBlock title="Service actions" :value="selectedTrace.service_actions" open />
        <ErrorNotice v-if="selectedTrace.error_message" :message="selectedTrace.error_message" />
      </div>
    </section>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import EmptyState from '../components/EmptyState.vue'
import ErrorNotice from '../components/ErrorNotice.vue'
import JsonBlock from '../components/JsonBlock.vue'
import StatusBadge from '../components/StatusBadge.vue'
import TraceEvaluationSummary from '../components/TraceEvaluationSummary.vue'
import { runWithStatus, workbenchState as state } from '../workbenchState'

const route = useRoute()
const router = useRouter()

const filters = reactive({
  user_id: '',
  interview_id: '',
  source_type: '',
  source_id: '',
  agent_type: '',
  step_name: '',
  status: '',
  limit: 25,
})

const traces = ref([])
const evaluationReport = ref(null)
const selectedTraceId = ref('')
const loadError = ref('')
const evaluationError = ref('')

const selectedTrace = computed(() => traces.value.find((trace) => trace.trace_id === selectedTraceId.value) || traces.value[0] || null)
const traceStatus = computed(() => (loadError.value ? 'failed' : traces.value.length ? 'loaded' : 'empty'))

function syncFiltersFromRoute() {
  filters.user_id = String(route.query.user_id || state.userId || '')
  filters.interview_id = String(route.query.interview_id || '')
  filters.source_type = String(route.query.source_type || '')
  filters.source_id = String(route.query.source_id || '')
  filters.agent_type = String(route.query.agent_type || '')
  filters.step_name = String(route.query.step_name || '')
  filters.status = String(route.query.status || '')
  filters.limit = Number(route.query.limit || 25)
}

function cleanFilters() {
  return {
    user_id: filters.user_id,
    interview_id: filters.interview_id,
    source_type: filters.source_type,
    source_id: filters.source_id,
    agent_type: filters.agent_type,
    step_name: filters.step_name,
    status: filters.status,
    limit: filters.limit || 25,
  }
}

function formatTime(value) {
  return value ? new Date(value * 1000).toLocaleString() : '-'
}

async function load() {
  loadError.value = ''
  evaluationError.value = ''
  const query = cleanFilters()
  try {
    traces.value = await runWithStatus('loadTraces', () => api.listAgentDecisionTraces(query), 'Traces loaded')
    selectedTraceId.value = traces.value[0]?.trace_id || ''
  } catch (error) {
    traces.value = []
    loadError.value = error.message || String(error)
  }
  try {
    evaluationReport.value = await api.listAgentEvaluations(query)
  } catch (error) {
    evaluationReport.value = null
    evaluationError.value = error.message || String(error)
  }
}

async function applyFilters() {
  await router.replace({ path: '/trace', query: cleanFilters() })
  await load()
}

async function resetFilters() {
  filters.user_id = state.userId
  filters.interview_id = ''
  filters.source_type = ''
  filters.source_id = ''
  filters.agent_type = ''
  filters.step_name = ''
  filters.status = ''
  filters.limit = 25
  await applyFilters()
}

watch(() => route.query, syncFiltersFromRoute)

onMounted(async () => {
  syncFiltersFromRoute()
  await load()
})
</script>

<style scoped>
.trace-detail-grid {
  display: grid;
  gap: 10px;
}
</style>
```

- [ ] **Step 2: Build the frontend**

Run:

```bash
cd frontend
npm run build
```

Expected: PASS with Vite build output.

- [ ] **Step 3: Manually verify query-param behavior**

Run the frontend after the backend is available:

```bash
cd frontend
npm run dev
```

Open:

```text
http://127.0.0.1:5173/trace?source_type=coaching_session&source_id=sess_demo&limit=20
```

Expected:

- `source_type` input shows `coaching_session`.
- `source_id` input shows `sess_demo`.
- `limit` input shows `20`.
- Page remains usable if trace or evaluation loading returns an empty list.

- [ ] **Step 4: Commit**

Run:

```bash
git add frontend/src/pages/EngineeringTracePage.vue
git commit -m "feat: expose agent engineering traces"
```

### Task 4: Implement Coaching Page MVP

**Files:**
- Modify: `frontend/src/pages/CoachingPage.vue`
- Test: `frontend/src/pages/CoachingPage.vue` through `npm run build` and manual coaching flow checks

- [ ] **Step 1: Replace the Coaching page stub**

Replace `frontend/src/pages/CoachingPage.vue` with:

```vue
<template>
  <section class="page">
    <div class="page-header">
      <div>
        <span class="page-kicker">Second-round preparation</span>
        <h1>Coaching</h1>
        <p class="muted">Task-oriented coaching plan, session state, turns, practice updates, and trace evidence.</p>
      </div>
      <button class="secondary" type="button" @click="refreshAll">Refresh</button>
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Interview Context</h2>
          <p>Use a reviewed interview so coaching can reuse review context.</p>
        </div>
        <StatusBadge :status="interview.status || 'unloaded'" />
      </div>
      <div class="field-row">
        <label>interview_id <input v-model="interviewIdInput" @change="rememberInterview" /></label>
        <label>target_round <input v-model="targetRound" /></label>
        <label>remaining_days <input v-model.number="remainingDays" type="number" min="1" max="60" /></label>
        <label>user_id <input v-model="state.userId" disabled /></label>
      </div>
      <div class="actions">
        <button class="secondary" type="button" :disabled="!interviewIdInput" @click="loadInterview">Load Interview</button>
        <button class="primary" type="button" :disabled="!interviewIdInput" @click="generatePlan">Generate Plan</button>
        <button class="secondary" type="button" :disabled="!interviewIdInput" @click="loadPlan">Get Plan</button>
      </div>
      <ErrorNotice v-if="loadError" :message="loadError" />
      <EmptyState v-if="!interviewIdInput" title="No interview selected" message="Open Interviews or enter an interview_id to start coaching." />
      <dl v-else class="mini-meta">
        <div><dt>company</dt><dd>{{ interview.company_name || '-' }}</dd></div>
        <div><dt>job title</dt><dd>{{ interview.job_title || '-' }}</dd></div>
        <div><dt>review readiness</dt><dd>{{ reviewReadiness }}</dd></div>
        <div><dt>plan_id</dt><dd>{{ plan?.plan_id || state.selectedPlanId || '-' }}</dd></div>
      </dl>
    </section>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Coaching Plan</h2>
          <p>{{ plan?.focus_summary || 'Generate or load a plan for this interview.' }}</p>
        </div>
        <StatusBadge :status="plan?.status || 'none'" />
      </div>
      <p class="summary-box">{{ plan?.overall_strategy || 'No strategy loaded.' }}</p>
      <div class="actions">
        <button class="primary" type="button" :disabled="!plan?.plan_id" @click="startOrResume">Start/Resume Session</button>
        <button class="secondary" type="button" :disabled="!state.selectedSessionId" @click="loadSession">Refresh Session</button>
        <button class="secondary" type="button" :disabled="!canPause" @click="pauseSession">Pause</button>
        <button class="danger" type="button" :disabled="!canCancel" @click="cancelSession">Cancel</button>
        <RouterLink v-if="session?.session_id" class="text-link" :to="traceLink">Open Trace Evidence</RouterLink>
      </div>
    </section>

    <div class="dense-grid">
      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Coaching Tasks</h2>
            <p>Current task is highlighted from the session state.</p>
          </div>
        </div>
        <EmptyState v-if="!tasks.length" title="No tasks" message="Generate or load a coaching plan first." />
        <div v-else class="list compact">
          <article
            v-for="task in tasks"
            :key="task.task_id"
            class="list-row"
            :class="{ active: task.task_id === session?.current_task_id }"
          >
            <div class="row-head">
              <strong>#{{ task.sequence }} {{ task.title }}</strong>
              <StatusBadge :status="task.status" />
            </div>
            <p>{{ task.description }}</p>
            <small>{{ task.task_type }} · day {{ task.day_index }} · {{ task.priority || 'priority unset' }}</small>
            <small>success criteria: complete formal answer attempts until passed by backend state machine</small>
          </article>
        </div>
      </section>

      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Session State</h2>
            <p>{{ session?.progress_summary || 'No active session loaded.' }}</p>
          </div>
          <StatusBadge :status="session?.status || 'none'" />
        </div>
        <dl class="mini-meta">
          <div><dt>session_id</dt><dd>{{ session?.session_id || '-' }}</dd></div>
          <div><dt>current_task</dt><dd>{{ currentTask?.title || session?.current_task_id || '-' }}</dd></div>
          <div><dt>last active</dt><dd>{{ formatTime(session?.last_active_at) }}</dd></div>
          <div><dt>last agent message</dt><dd>{{ session?.last_agent_message || '-' }}</dd></div>
        </dl>
        <ErrorNotice v-if="session?.error_message" :message="session.error_message" />

        <label>User input
          <textarea v-model="userInput" rows="6" :disabled="!canSubmit" placeholder="Submit a coaching answer, hint request, skip request, or clarification." />
        </label>
        <div class="actions">
          <button class="primary" type="button" :disabled="!canSubmit || !userInput.trim()" @click="submitTurn">Submit Turn</button>
          <button class="secondary" type="button" :disabled="!userInput" @click="userInput = ''">Clear</button>
        </div>
      </section>
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Latest Result</h2>
          <p>Assistant response, score, feedback, action, and recorded attempt evidence.</p>
        </div>
      </div>
      <EmptyState v-if="!latestTurn && !latestAttempt" title="No result yet" message="Submit a turn after starting a session." />
      <div v-else class="dense-grid">
        <TurnTimelineItem v-if="latestTurn" :turn="latestTurn" />
        <article v-if="latestAttempt" class="list-row">
          <div class="row-head">
            <strong>Attempt {{ latestAttempt.attempt_index }}</strong>
            <StatusBadge :status="latestAttempt.passed ? 'passed' : 'failed'" />
          </div>
          <p>{{ latestAttempt.feedback }}</p>
          <dl class="mini-meta">
            <div><dt>score</dt><dd>{{ latestAttempt.score }}</dd></div>
            <div><dt>recorded</dt><dd>{{ latestAttempt.attempt_id ? 'yes' : 'no' }}</dd></div>
          </dl>
        </article>
      </div>
    </section>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Practice Evidence</h2>
          <p>Read-only practice_states changed by formal training answers.</p>
        </div>
        <button class="secondary" type="button" @click="loadPracticeStates">Refresh Practice States</button>
      </div>
      <EmptyState v-if="!practiceStates.length" title="No practice states" message="Formal answers may update practice state after the backend evaluates them." />
      <div v-else class="list compact">
        <article v-for="item in practiceStates" :key="item.state_id" class="list-row">
          <div class="row-head">
            <strong>{{ item.topic }} · {{ item.dimension }}</strong>
            <StatusBadge :status="item.source_type" />
          </div>
          <small>mastery {{ item.mastery_score }} · attempts {{ item.attempt_count }} · last score {{ item.last_score }}</small>
          <p>{{ item.last_feedback || '-' }}</p>
        </article>
      </div>
    </section>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Turn Timeline</h2>
          <p>{{ turns.length }} session turns loaded.</p>
        </div>
      </div>
      <EmptyState v-if="!turns.length" title="No turns" message="Start a session or refresh an existing session." />
      <div v-else class="list compact">
        <TurnTimelineItem v-for="turn in turns" :key="turn.turn_id" :turn="turn" />
      </div>
    </section>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { api } from '../api'
import EmptyState from '../components/EmptyState.vue'
import ErrorNotice from '../components/ErrorNotice.vue'
import StatusBadge from '../components/StatusBadge.vue'
import TurnTimelineItem from '../components/TurnTimelineItem.vue'
import { rememberSelection, runWithStatus, workbenchState as state } from '../workbenchState'

const interviewIdInput = ref(state.selectedInterviewId || '')
const targetRound = ref('second_round')
const remainingDays = ref(7)
const detail = ref(null)
const plan = ref(null)
const sessionDetail = ref(null)
const practiceStates = ref([])
const userInput = ref('')
const loadError = ref('')

const interview = computed(() => detail.value?.interview || {})
const session = computed(() => sessionDetail.value?.session || null)
const tasks = computed(() => sessionDetail.value?.tasks || loadedTasks.value)
const turns = computed(() => sessionDetail.value?.turns || [])
const attempts = computed(() => sessionDetail.value?.attempts || [])
const currentTask = computed(() => sessionDetail.value?.current_task || tasks.value.find((task) => task.task_id === session.value?.current_task_id))
const latestTurn = computed(() => turns.value[turns.value.length - 1] || null)
const latestAttempt = computed(() => attempts.value[attempts.value.length - 1] || null)
const loadedTasks = ref([])
const reviewReadiness = computed(() => {
  if (!detail.value) return 'not loaded'
  if (detail.value.review_report?.status) return detail.value.review_report.status
  return interview.value.status === 'reviewed' ? 'reviewed' : 'review first'
})
const canSubmit = computed(() => Boolean(session.value?.session_id && !['failed', 'completed', 'cancelled', 'canceled'].includes(session.value.status)))
const canPause = computed(() => Boolean(session.value?.session_id && ['active', 'running'].includes(session.value.status)))
const canCancel = computed(() => Boolean(session.value?.session_id && !['completed', 'cancelled', 'canceled'].includes(session.value.status)))
const traceLink = computed(() => ({
  path: '/trace',
  query: { source_type: 'coaching_session', source_id: session.value?.session_id || '', user_id: state.userId, limit: 25 },
}))

function formatTime(value) {
  return value ? new Date(value * 1000).toLocaleString() : '-'
}

function rememberInterview() {
  rememberSelection('selectedInterviewId', interviewIdInput.value)
}

async function loadInterview() {
  if (!interviewIdInput.value) return
  loadError.value = ''
  try {
    detail.value = await runWithStatus('coachingInterviewDetail', () => api.interviewDetail(interviewIdInput.value, state.userId), 'Interview context loaded')
    rememberSelection('selectedInterviewId', interviewIdInput.value)
    if (detail.value?.coaching_plan?.plan_id) {
      plan.value = detail.value.coaching_plan
      rememberSelection('selectedPlanId', plan.value.plan_id)
      await loadTasks(plan.value.plan_id)
    }
  } catch (error) {
    loadError.value = error.message || String(error)
  }
}

async function generatePlan() {
  plan.value = await runWithStatus(
    'generateCoachingPlan',
    () => api.generateCoachingPlan(interviewIdInput.value, {
      user_id: state.userId,
      target_round: targetRound.value,
      remaining_days: remainingDays.value,
    }),
    'Coaching plan generated',
  )
  rememberSelection('selectedPlanId', plan.value.plan_id)
  await loadTasks(plan.value.plan_id)
}

async function loadPlan() {
  plan.value = await runWithStatus('loadCoachingPlan', () => api.getCoachingPlan(interviewIdInput.value), 'Coaching plan loaded')
  rememberSelection('selectedPlanId', plan.value.plan_id)
  await loadTasks(plan.value.plan_id)
}

async function loadTasks(planId) {
  loadedTasks.value = await runWithStatus('loadCoachingTasks', () => api.listCoachingTasks(planId), 'Coaching tasks loaded')
}

async function startOrResume() {
  sessionDetail.value = await runWithStatus(
    'startCoachingSession',
    () => api.startOrResumeCoachingSession(plan.value.plan_id, state.userId),
    'Coaching session ready',
  )
  rememberSelection('selectedSessionId', session.value.session_id)
  await loadPracticeStates()
}

async function loadSession() {
  if (!state.selectedSessionId) return
  sessionDetail.value = await runWithStatus('loadCoachingSession', () => api.getCoachingSession(state.selectedSessionId), 'Coaching session refreshed')
  rememberSelection('selectedSessionId', session.value.session_id)
}

async function submitTurn() {
  sessionDetail.value = await runWithStatus('submitCoachingTurn', () => api.submitCoachingTurn(session.value.session_id, userInput.value), 'Coaching turn submitted')
  userInput.value = ''
  await loadPracticeStates()
}

async function pauseSession() {
  sessionDetail.value = await runWithStatus('pauseCoachingSession', () => api.pauseCoachingSession(session.value.session_id), 'Coaching session paused')
}

async function cancelSession() {
  sessionDetail.value = await runWithStatus('cancelCoachingSession', () => api.cancelCoachingSession(session.value.session_id), 'Coaching session cancelled')
}

async function loadPracticeStates() {
  practiceStates.value = await api.listPracticeStates({ user_id: state.userId })
}

async function refreshAll() {
  await loadInterview()
  if (state.selectedSessionId) {
    await loadSession()
  }
  await loadPracticeStates()
}

onMounted(refreshAll)
</script>

<style scoped>
.list-row.active {
  border-color: #1f5fd1;
  box-shadow: inset 0 0 0 1px #1f5fd1;
}
</style>
```

- [ ] **Step 2: Build the frontend**

Run:

```bash
cd frontend
npm run build
```

Expected: PASS with Vite build output.

- [ ] **Step 3: Manually verify the coaching MVP**

With backend and frontend running, open:

```text
http://127.0.0.1:5173/coaching
```

Use a reviewed `interview_id`.

Expected:

- Loading interview context shows company, job title, status, and review readiness.
- Generate Plan calls `POST /api/interviews/:interview_id/coaching-plan` and displays plan strategy/tasks.
- Get Plan calls `GET /api/interviews/:interview_id/coaching-plan`.
- Start/Resume calls `POST /api/coaching-plans/:plan_id/sessions?user_id=...` and persists `selectedSessionId`.
- Submit Turn calls `POST /api/coaching-sessions/:session_id/turns`, displays latest response, score/feedback/action, and attempt evidence when returned.
- Failed/completed/cancelled sessions disable the submit composer.
- Open Trace Evidence navigates to `/trace?source_type=coaching_session&source_id=<session_id>`.
- No UI path calls coaching-session memory-candidate generation.

- [ ] **Step 4: Commit**

Run:

```bash
git add frontend/src/pages/CoachingPage.vue
git commit -m "feat: build coaching session mvp"
```

### Task 5: Implement Mock Interview Page MVP

**Files:**
- Modify: `frontend/src/pages/MockInterviewPage.vue`
- Test: `frontend/src/pages/MockInterviewPage.vue` through `npm run build` and manual mock interview flow checks

- [ ] **Step 1: Replace the Mock Interview page stub**

Replace `frontend/src/pages/MockInterviewPage.vue` with:

```vue
<template>
  <section class="page">
    <div class="page-header">
      <div>
        <span class="page-kicker">Practice loop</span>
        <h1>Mock Interview</h1>
        <p class="muted">Independent long-session mock interviewer state, turn timeline, evaluations, and trace evidence.</p>
      </div>
      <button class="secondary" type="button" @click="refreshAll">Refresh</button>
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Interview Context</h2>
          <p>Use a reviewed interview and optional coaching plan context.</p>
        </div>
        <StatusBadge :status="interview.status || 'unloaded'" />
      </div>
      <div class="field-row">
        <label>interview_id <input v-model="interviewIdInput" @change="rememberInterview" /></label>
        <label>plan_id <input v-model="planIdInput" @change="rememberPlan" /></label>
        <label>target_round <input v-model="targetRound" /></label>
        <label>user_id <input v-model="state.userId" disabled /></label>
      </div>
      <div class="actions">
        <button class="secondary" type="button" :disabled="!interviewIdInput" @click="loadInterview">Load Interview</button>
        <button class="primary" type="button" :disabled="!interviewIdInput" @click="startMock">Start/Resume Mock</button>
        <button class="secondary" type="button" :disabled="!state.selectedMockId" @click="loadMock">Refresh Mock</button>
        <button class="secondary" type="button" :disabled="!canComplete" @click="completeMock">Complete</button>
        <button class="danger" type="button" :disabled="!canCancel" @click="cancelMock">Cancel</button>
        <RouterLink v-if="mock?.mock_id" class="text-link" :to="traceLink">Open Trace Evidence</RouterLink>
      </div>
      <ErrorNotice v-if="loadError" :message="loadError" />
      <EmptyState v-if="!interviewIdInput" title="No interview selected" message="Open Interviews or enter an interview_id to start a mock interview." />
      <dl v-else class="mini-meta">
        <div><dt>company</dt><dd>{{ interview.company_name || '-' }}</dd></div>
        <div><dt>job title</dt><dd>{{ interview.job_title || '-' }}</dd></div>
        <div><dt>interview status</dt><dd>{{ interview.status || '-' }}</dd></div>
        <div><dt>latest mock</dt><dd>{{ mock?.mock_id || state.selectedMockId || '-' }}</dd></div>
      </dl>
    </section>

    <div class="dense-grid">
      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Mock State</h2>
            <p>{{ mock?.overall_goal || 'Start or refresh a mock interview.' }}</p>
          </div>
          <StatusBadge :status="mock?.status || 'none'" />
        </div>
        <dl class="mini-meta">
          <div><dt>mock_id</dt><dd>{{ mock?.mock_id || '-' }}</dd></div>
          <div><dt>current topic</dt><dd>{{ mock?.current_topic || '-' }}</dd></div>
          <div><dt>current turn</dt><dd>{{ mock?.current_turn ?? '-' }}</dd></div>
          <div><dt>target round</dt><dd>{{ mock?.target_round || targetRound }}</dd></div>
        </dl>
        <p class="summary-box">{{ currentQuestion }}</p>
        <ErrorNotice v-if="mock?.error_message" :message="mock.error_message" />
      </section>

      <section class="panel">
        <div class="panel-title">
          <div>
            <h2>Answer Composer</h2>
            <p>Submit answers only while the mock session is active.</p>
          </div>
        </div>
        <label>Answer
          <textarea v-model="answer" rows="8" :disabled="!canSubmit" placeholder="Answer the current interviewer question." />
        </label>
        <div class="actions">
          <button class="primary" type="button" :disabled="!canSubmit || !answer.trim()" @click="submitTurn">Submit Answer</button>
          <button class="secondary" type="button" :disabled="!answer" @click="answer = ''">Clear</button>
        </div>
      </section>
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Latest Result</h2>
          <p>Feedback, score, next action, and practice update summary.</p>
        </div>
      </div>
      <EmptyState v-if="!latestTurn && !mock?.last_feedback" title="No result yet" message="Submit an answer to receive feedback." />
      <div v-else class="dense-grid">
        <TurnTimelineItem v-if="latestTurn" :turn="latestTurn" kind="mock" />
        <article class="list-row">
          <div class="row-head">
            <strong>Session Feedback</strong>
            <StatusBadge :status="mock?.status || 'mock'" />
          </div>
          <p>{{ mock?.last_feedback || latestTurn?.feedback || '-' }}</p>
          <dl class="mini-meta">
            <div><dt>score</dt><dd>{{ latestTurn?.score || '-' }}</dd></div>
            <div><dt>next action</dt><dd>{{ latestTurn?.agent_action || latestTurn?.phase || '-' }}</dd></div>
          </dl>
        </article>
      </div>
    </section>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Turn Timeline</h2>
          <p>Opening question, user answers, evaluations, hints, follow-ups, topic switches, and closing turns.</p>
        </div>
      </div>
      <EmptyState v-if="!timeline.length" title="No turns" message="Start a mock interview to see the opening question." />
      <div v-else class="list compact">
        <TurnTimelineItem v-for="turn in timeline" :key="turn.turn_id || `opening-${turn.turn_index}`" :turn="turn" kind="mock" />
      </div>
    </section>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>Practice Evidence</h2>
          <p>Read-only practice_states updated by mock answers.</p>
        </div>
        <button class="secondary" type="button" @click="loadPracticeStates">Refresh Practice States</button>
      </div>
      <EmptyState v-if="!practiceStates.length" title="No practice states" message="Mock turns may update practice state after evaluation." />
      <div v-else class="list compact">
        <article v-for="item in practiceStates" :key="item.state_id" class="list-row">
          <div class="row-head">
            <strong>{{ item.topic }} · {{ item.dimension }}</strong>
            <StatusBadge :status="item.source_type" />
          </div>
          <small>mastery {{ item.mastery_score }} · attempts {{ item.attempt_count }} · last score {{ item.last_score }}</small>
          <p>{{ item.last_feedback || '-' }}</p>
        </article>
      </div>
    </section>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { api } from '../api'
import EmptyState from '../components/EmptyState.vue'
import ErrorNotice from '../components/ErrorNotice.vue'
import StatusBadge from '../components/StatusBadge.vue'
import TurnTimelineItem from '../components/TurnTimelineItem.vue'
import { rememberSelection, runWithStatus, workbenchState as state } from '../workbenchState'

const interviewIdInput = ref(state.selectedInterviewId || '')
const planIdInput = ref(state.selectedPlanId || '')
const targetRound = ref('second_round')
const detail = ref(null)
const mock = ref(null)
const turns = ref([])
const practiceStates = ref([])
const answer = ref('')
const loadError = ref('')

const interview = computed(() => detail.value?.interview || {})
const latestTurn = computed(() => turns.value[turns.value.length - 1] || null)
const currentQuestion = computed(() => latestTurn.value?.next_question || latestTurn.value?.interviewer_question || mock.value?.first_question || 'No current question loaded.')
const canSubmit = computed(() => Boolean(mock.value?.mock_id && !['failed', 'completed', 'cancelled', 'canceled'].includes(mock.value.status)))
const canComplete = computed(() => Boolean(mock.value?.mock_id && !['completed', 'cancelled', 'canceled'].includes(mock.value.status)))
const canCancel = computed(() => Boolean(mock.value?.mock_id && !['completed', 'cancelled', 'canceled'].includes(mock.value.status)))
const traceLink = computed(() => ({
  path: '/trace',
  query: { source_type: 'mock_interview', source_id: mock.value?.mock_id || '', user_id: state.userId, limit: 25 },
}))
const timeline = computed(() => {
  if (!mock.value?.first_question) {
    return turns.value
  }
  const opening = {
    turn_id: 'opening-question',
    turn_index: 0,
    phase: 'opening',
    agent_action: 'ask_question',
    interviewer_question: mock.value.first_question,
    content: mock.value.first_question,
  }
  return [opening, ...turns.value]
})

function rememberInterview() {
  rememberSelection('selectedInterviewId', interviewIdInput.value)
}

function rememberPlan() {
  rememberSelection('selectedPlanId', planIdInput.value)
}

async function loadInterview() {
  if (!interviewIdInput.value) return
  loadError.value = ''
  try {
    detail.value = await runWithStatus('mockInterviewDetail', () => api.interviewDetail(interviewIdInput.value, state.userId), 'Interview context loaded')
    rememberSelection('selectedInterviewId', interviewIdInput.value)
    if (detail.value?.coaching_plan?.plan_id && !planIdInput.value) {
      planIdInput.value = detail.value.coaching_plan.plan_id
      rememberSelection('selectedPlanId', planIdInput.value)
    }
    if (detail.value?.latest_mock_interview?.mock_id && !mock.value) {
      mock.value = detail.value.latest_mock_interview
      rememberSelection('selectedMockId', mock.value.mock_id)
    }
  } catch (error) {
    loadError.value = error.message || String(error)
  }
}

async function startMock() {
  mock.value = await runWithStatus(
    'startMockInterview',
    () => api.startMockInterview(interviewIdInput.value, {
      user_id: state.userId,
      plan_id: planIdInput.value,
      target_round: targetRound.value,
    }),
    'Mock interview ready',
  )
  rememberSelection('selectedMockId', mock.value.mock_id)
  await loadTurns()
  await loadPracticeStates()
}

async function loadMock() {
  if (!state.selectedMockId) return
  mock.value = await runWithStatus('loadMockInterview', () => api.getMockInterview(state.selectedMockId), 'Mock interview refreshed')
  rememberSelection('selectedMockId', mock.value.mock_id)
  await loadTurns()
}

async function loadTurns() {
  if (!mock.value?.mock_id && !state.selectedMockId) return
  turns.value = await runWithStatus('loadMockTurns', () => api.listMockTurns(mock.value?.mock_id || state.selectedMockId), 'Mock turns loaded')
}

async function submitTurn() {
  const turn = await runWithStatus('submitMockTurn', () => api.submitMockTurn(mock.value.mock_id, answer.value), 'Mock answer submitted')
  answer.value = ''
  turns.value = [...turns.value, turn]
  mock.value = await api.getMockInterview(mock.value.mock_id)
  await loadPracticeStates()
}

async function completeMock() {
  mock.value = await runWithStatus('completeMockInterview', () => api.completeMockInterview(mock.value.mock_id), 'Mock interview completed')
  await loadTurns()
}

async function cancelMock() {
  mock.value = await runWithStatus('cancelMockInterview', () => api.cancelMockInterview(mock.value.mock_id), 'Mock interview cancelled')
}

async function loadPracticeStates() {
  practiceStates.value = await api.listPracticeStates({ user_id: state.userId })
}

async function refreshAll() {
  await loadInterview()
  if (state.selectedMockId) {
    await loadMock()
  }
  await loadPracticeStates()
}

onMounted(refreshAll)
</script>
```

- [ ] **Step 2: Build the frontend**

Run:

```bash
cd frontend
npm run build
```

Expected: PASS with Vite build output.

- [ ] **Step 3: Manually verify the mock MVP**

With backend and frontend running, open:

```text
http://127.0.0.1:5173/mock
```

Use a reviewed `interview_id`.

Expected:

- Loading interview context shows company, job title, status, and latest mock if present.
- Start/Resume Mock calls `POST /api/interviews/:interview_id/mock-interviews`, displays first question, status, current topic, and goal.
- Refresh Mock calls `GET /api/mock-interviews/:mock_id` and `GET /api/mock-interviews/:mock_id/turns`.
- Submit Answer calls `POST /api/mock-interviews/:mock_id/turns`, appends the returned turn, refreshes mock state, and displays feedback/score/next question.
- Complete and Cancel call the existing endpoints and disable answer submission when the state is terminal.
- Open Trace Evidence navigates to `/trace?source_type=mock_interview&source_id=<mock_id>`.
- No UI path calls mock-interview memory-candidate generation.

- [ ] **Step 4: Commit**

Run:

```bash
git add frontend/src/pages/MockInterviewPage.vue
git commit -m "feat: build mock interview mvp"
```

### Task 6: Final Regression, Backend Boundary Check, And Demo Verification

**Files:**
- Read-only verification of frontend and backend files.
- No implementation files should change in this task unless a previous task failed verification.

- [ ] **Step 1: Search for forbidden memory-generation calls in the new pages**

Run:

```bash
rg "memory-candidates|generateMemoryCandidates|coaching-sessions/.*/memory|mock-interviews/.*/memory" frontend/src/pages frontend/src/api.js
```

Expected: only existing memory inbox/interview detail helper references appear, and there are no new calls from `CoachingPage.vue` or `MockInterviewPage.vue` to session/mock memory-candidate generation endpoints.

- [ ] **Step 2: Search for forbidden architecture additions**

Run:

```bash
rg "ReAct|MCP|function calling|tool calling|new Agent|AgentType|ResolveAgentType" frontend server agent
```

Expected: no new frontend code introduces ReAct/MCP/function calling or new business Agent registration. Existing backend references may appear; do not modify them for this MVP.

- [ ] **Step 3: Run API helper tests**

Run:

```bash
cd frontend
npm run test:api
```

Expected: PASS and prints `api helper tests passed`.

- [ ] **Step 4: Build frontend**

Run:

```bash
cd frontend
npm run build
```

Expected: PASS with Vite build output.

- [ ] **Step 5: Run backend tests**

Run:

```bash
go test ./...
```

Expected: PASS. If tests fail because of pre-existing real LLM/network-dependent tests, record the exact failing package/test name and rerun the smallest relevant local package tests that do not require network. Do not claim backend verification passed unless `go test ./...` actually passes.

- [ ] **Step 6: Manual demo script**

Run backend:

```bash
go run ./cmd/server
```

Run frontend in a second shell:

```bash
cd frontend
npm run dev
```

Manual acceptance:

- Dashboard, Interviews, Interview Detail, and Memory Inbox still render.
- Interview Detail still saves transcripts, triggers review, displays candidates, and links Memory Inbox without formal memory writes outside accept/reject.
- Coaching page loads a reviewed interview, generates/loads a plan, displays tasks, starts/resumes a session, submits a turn, shows latest result, shows practice state evidence, and links filtered trace view.
- Mock page loads a reviewed interview, starts/resumes a mock, shows first/current question, submits an answer, shows feedback/score/next question, shows timeline and practice state evidence, and links filtered trace view.
- Trace page lists traces, filters by `source_type=coaching_session` and `source_type=mock_interview`, displays selected context/input/raw output/parsed decision/service actions, and displays evaluation checks with failed checks highlighted.

- [ ] **Step 7: Commit final verification notes if any docs were added**

If no files changed during verification, do not create a commit.

If a short verification note is added to `docs/DEMO_GUIDE.md`, commit it:

```bash
git add docs/DEMO_GUIDE.md
git commit -m "docs: add agent session page demo checks"
```

## Self-Review

- Spec coverage: Coaching page requirements map to Task 4; Mock Interview page requirements map to Task 5; Engineering Trace page requirements map to Task 3; shared API/component work maps to Tasks 1 and 2; verification and memory boundary checks map to Task 6.
- Backend policy: plan uses only existing backend APIs and includes no backend mutations beyond existing coaching/mock state endpoints. It excludes session/mock memory-candidate generation from UI.
- Placeholder scan: the plan contains concrete file paths, exact code blocks for new/modified frontend files, exact commands, and expected outcomes.
- Type consistency: API helper names used by pages are defined in Task 1; route query names match backend controller filters; response field names match `vo/vo.go`.
- Independent acceptance: each task has its own build/test/manual verification and commit point.
