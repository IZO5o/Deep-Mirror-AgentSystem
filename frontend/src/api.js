export async function apiRequest(path, options = {}) {
  const init = {
    method: options.method || 'GET',
    headers: options.body ? { 'Content-Type': 'application/json', ...(options.headers || {}) } : options.headers,
    ...(options.signal ? { signal: options.signal } : {}),
  }

  if (options.body) {
    init.body = JSON.stringify(options.body)
  }

  const response = await fetch(path, init)
  const text = await response.text()
  let payload = null
  if (text) {
    try {
      payload = JSON.parse(text)
    } catch (error) {
      throw new Error(`HTTP ${response.status}: ${text}`)
    }
  }

  if (!response.ok) {
    throw new Error(payload?.msg || `HTTP ${response.status}`)
  }
  if (payload && payload.code !== 0) {
    throw new Error(payload.msg || `API error code ${payload.code}`)
  }
  return payload?.data
}

export function buildQuery(params) {
  const search = new URLSearchParams()
  Object.entries(params || {}).forEach(([key, value]) => {
    if (value !== undefined && value !== null && String(value).trim() !== '') {
      search.set(key, value)
    }
  })
  const query = search.toString()
  return query ? `?${query}` : ''
}

export const api = {
  dashboardSummary(userId) {
    return apiRequest(`/api/dashboard-summary${buildQuery({ user_id: userId })}`)
  },
  listInterviews(userId) {
    return apiRequest(`/api/interviews${buildQuery({ user_id: userId })}`)
  },
  createInterview(body) {
    return apiRequest('/api/interviews', { method: 'POST', body })
  },
  upsertTranscript(interviewId, body) {
    return apiRequest(`/api/interviews/${interviewId}/transcript`, { method: 'PUT', body })
  },
  triggerReview(interviewId) {
    return apiRequest(`/api/interviews/${interviewId}/review`, { method: 'POST' })
  },
  interviewDetail(interviewId, userId) {
    return apiRequest(`/api/interviews/${interviewId}/detail${buildQuery({ user_id: userId })}`)
  },
  listMemoryCandidates(filters) {
    return apiRequest(`/api/memory-candidates${buildQuery(filters)}`)
  },
  acceptMemoryCandidate(candidateId) {
    return apiRequest(`/api/memory-candidates/${candidateId}/accept`, { method: 'POST' })
  },
  rejectMemoryCandidate(candidateId) {
    return apiRequest(`/api/memory-candidates/${candidateId}/reject`, { method: 'POST' })
  },
  listMemoryItems(userId) {
    return apiRequest(`/api/memory-items${buildQuery({ user_id: userId })}`)
  },
  createPracticeGoal(body) {
    return apiRequest('/api/practice-goals', { method: 'POST', body })
  },
  listPracticeGoals(filters) {
    return apiRequest(`/api/practice-goals${buildQuery(filters)}`)
  },
  getPracticeGoal(goalId) {
    return apiRequest(`/api/practice-goals/${goalId}`)
  },
  updatePracticeGoal(goalId, body) {
    return apiRequest(`/api/practice-goals/${goalId}`, { method: 'PATCH', body })
  },
  archivePracticeGoal(goalId) {
    return apiRequest(`/api/practice-goals/${goalId}/archive`, { method: 'POST' })
  },
  generatePracticeGoalCoachingPlan(goalId, body) {
    return apiRequest(`/api/practice-goals/${goalId}/coaching-plan`, { method: 'POST', body })
  },
  startPracticeGoalMock(goalId, body) {
    return apiRequest(`/api/practice-goals/${goalId}/mock`, { method: 'POST', body })
  },
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
  submitCoachingTurn(sessionId, body) {
    return apiRequest(`/api/coaching-sessions/${sessionId}/turns`, { method: 'POST', body })
  },
  pauseCoachingSession(sessionId) {
    return apiRequest(`/api/coaching-sessions/${sessionId}/pause`, { method: 'POST' })
  },
  cancelCoachingSession(sessionId) {
    return apiRequest(`/api/coaching-sessions/${sessionId}/cancel`, { method: 'POST' })
  },
  resumeCoachingSession(sessionId) {
    return apiRequest(`/api/coaching-sessions/${sessionId}/resume`, { method: 'POST' })
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
  submitMockTurn(mockId, body, options = {}) {
    return apiRequest(`/api/mock-interviews/${mockId}/turns`, { method: 'POST', body, ...options })
  },
  completeMockInterview(mockId) {
    return apiRequest(`/api/mock-interviews/${mockId}/complete`, { method: 'POST' })
  },
  cancelMockInterview(mockId) {
    return apiRequest(`/api/mock-interviews/${mockId}/cancel`, { method: 'POST' })
  },
  resumeMockInterview(mockId) {
    return apiRequest(`/api/mock-interviews/${mockId}/resume`, { method: 'POST' })
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
}
