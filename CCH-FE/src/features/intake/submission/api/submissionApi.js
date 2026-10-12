import { apiRequest } from '../../../../shared/api/apiRequest.js'

/** Starts an allowlisted guest conversation and returns its X-Intake-Session token. */
export function createIntakeSession() {
  return apiRequest('/api/v1/intake/sessions', { method: 'POST' })
}

/** Submits an explicitly consented complaint using the guest conversation token. */
export function submitIntakeSubmission(submission, sessionToken) {
  return apiRequest('/api/v1/intake/submissions', {
    method: 'POST',
    body: submission,
    sessionToken,
  })
}
