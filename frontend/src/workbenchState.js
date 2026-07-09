import { reactive } from 'vue'

const storageKeys = {
  userId: 'workbench.user_id',
  selectedInterviewId: 'workbench.interview_id',
  selectedPlanId: 'workbench.plan_id',
  selectedSessionId: 'workbench.session_id',
  selectedMockId: 'workbench.mock_id',
  focusTopic: 'workbench.focus_topic',
}

function readStorage(key, fallback = '') {
  return window.localStorage.getItem(key) || fallback
}

export const workbenchState = reactive({
  userId: readStorage(storageKeys.userId, 'user_001'),
  selectedInterviewId: readStorage(storageKeys.selectedInterviewId),
  selectedPlanId: readStorage(storageKeys.selectedPlanId),
  selectedSessionId: readStorage(storageKeys.selectedSessionId),
  selectedMockId: readStorage(storageKeys.selectedMockId),
  focusTopic: readStorage(storageKeys.focusTopic),
  lastResult: '',
  globalError: '',
  loadingKeys: new Set(),
})

export function setUserId(userId) {
  workbenchState.userId = userId.trim() || 'user_001'
  window.localStorage.setItem(storageKeys.userId, workbenchState.userId)
}

export function rememberSelection(key, value) {
  if (!Object.prototype.hasOwnProperty.call(storageKeys, key)) {
    return
  }
  workbenchState[key] = value || ''
  window.localStorage.setItem(storageKeys[key], workbenchState[key])
}

export function beginLoading(key) {
  workbenchState.loadingKeys.add(key)
}

export function endLoading(key) {
  workbenchState.loadingKeys.delete(key)
}

export function clearGlobalNotice() {
  workbenchState.globalError = ''
  workbenchState.lastResult = ''
}

export async function runWithStatus(key, action, successMessage) {
  beginLoading(key)
  workbenchState.globalError = ''
  try {
    const result = await action()
    workbenchState.lastResult = successMessage || `${key} completed`
    return result
  } catch (error) {
    workbenchState.globalError = error.message || String(error)
    throw error
  } finally {
    endLoading(key)
  }
}
