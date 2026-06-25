import assert from 'node:assert/strict'

import { api, buildQuery } from './api.js'

const originalFetch = globalThis.fetch
const calls = []

function installFetch(responseFactory = defaultResponse) {
  calls.length = 0
  globalThis.fetch = async (path, init = {}) => {
    calls.push({ path, init })
    return responseFactory(path, init)
  }
}

function defaultResponse() {
  return {
    ok: true,
    status: 200,
    text: async () => JSON.stringify({ code: 0, data: { ok: true } }),
  }
}

function latestCall() {
  assert.equal(calls.length, 1)
  return calls[0]
}

function assertJsonBody(init, body) {
  assert.equal(init.headers['Content-Type'], 'application/json')
  assert.deepEqual(JSON.parse(init.body), body)
}

async function assertPostBody(name, action, expectedPath, body) {
  installFetch()
  await action()
  const call = latestCall()
  assert.equal(call.path, expectedPath, name)
  assert.equal(call.init.method, 'POST')
  assertJsonBody(call.init, body)
}

async function main() {
  assert.equal(buildQuery({
    user_id: 'user_001',
    empty: '',
    blank: '   ',
    nil: null,
    missing: undefined,
    zero: 0,
    bool: false,
  }), '?user_id=user_001&zero=0&bool=false')

  await assertPostBody(
    'generateCoachingPlan path/body',
    () => api.generateCoachingPlan('interview_1', { user_id: 'user_001', target_round: 'second_round', remaining_days: 3 }),
    '/api/interviews/interview_1/coaching-plan',
    { user_id: 'user_001', target_round: 'second_round', remaining_days: 3 },
  )

  installFetch()
  await api.startOrResumeCoachingSession('plan_1', 'user_001')
  assert.equal(latestCall().path, '/api/coaching-plans/plan_1/sessions?user_id=user_001')
  assert.equal(latestCall().init.method, 'POST')

  await assertPostBody(
    'submitCoachingTurn path/body',
    () => api.submitCoachingTurn('session_1', { user_input: 'answer', input_type: 'formal_answer' }),
    '/api/coaching-sessions/session_1/turns',
    { user_input: 'answer', input_type: 'formal_answer' },
  )

  await assertPostBody(
    'startMockInterview path/body',
    () => api.startMockInterview('interview_1', { user_id: 'user_001', plan_id: 'plan_1', target_round: 'second_round' }),
    '/api/interviews/interview_1/mock-interviews',
    { user_id: 'user_001', plan_id: 'plan_1', target_round: 'second_round' },
  )

  await assertPostBody(
    'submitMockTurn path/body',
    () => api.submitMockTurn('mock_1', { answer: 'my answer' }),
    '/api/mock-interviews/mock_1/turns',
    { answer: 'my answer' },
  )

  installFetch()
  await api.listAgentDecisionTraces({
    user_id: 'user_001',
    interview_id: 'interview_1',
    source_type: '',
    step_name: 'mock_start',
    limit: 20,
  })
  assert.equal(latestCall().path, '/api/agent-decision-traces?user_id=user_001&interview_id=interview_1&step_name=mock_start&limit=20')

  installFetch()
  await api.listAgentEvaluations({
    user_id: 'user_001',
    agent_type: 'mock_interviewer',
    status: 'passed',
    limit: 10,
  })
  assert.equal(latestCall().path, '/api/agent-evaluations?user_id=user_001&agent_type=mock_interviewer&status=passed&limit=10')

  installFetch()
  await api.listPracticeStates({
    user_id: 'user_001',
    topic: 'Redis consistency',
    dimension: '',
  })
  assert.equal(latestCall().path, '/api/practice-states?user_id=user_001&topic=Redis+consistency')

  installFetch(() => ({
    ok: false,
    status: 500,
    text: async () => JSON.stringify({ code: 500, msg: 'session missing' }),
  }))
  await assert.rejects(
    () => api.getCoachingSession('missing_session'),
    /session missing/,
  )

  console.log('api helper tests passed')
}

try {
  await main()
} finally {
  globalThis.fetch = originalFetch
}
