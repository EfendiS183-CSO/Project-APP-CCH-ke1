import { apiRequest } from '../../../shared/api/apiRequest.js'

/**
 * Call Center complaint API client following the CCH-BE v1 contract.
 * The backend revalidates JWT role, office, ACL, and routing on every call;
 * this client only shapes requests and normalizes the JSON responses.
 */
async function readPaged(path, token, params = {}) {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      query.set(key, String(value))
    }
  }
  const suffix = query.toString() ? `?${query}` : ''
  const payload = await apiRequest(`${path}${suffix}`, { token })
  return payload?.data ?? []
}

export const complaintApi = {
  count(token, path) {
    return apiRequest(path, { token }).then((payload) => payload?.count ?? 0)
  },

  listOpen(token, { limit = 25, offset = 0 } = {}) {
    return readPaged('/api/v1/complaints/call-center/open', token, { limit, offset })
  },

  listCompleted(token, { limit = 25, offset = 0 } = {}) {
    return readPaged('/api/v1/complaints/call-center/completed', token, { limit, offset })
  },

  listReceivedOpen(token, { from, to, limit = 25, offset = 0 } = {}) {
    return readPaged('/api/v1/complaints/dashboard/received-open', token, { from, to, limit, offset })
  },

  listSentOpen(token, { from, to, limit = 25, offset = 0 } = {}) {
    return readPaged('/api/v1/complaints/dashboard/sent-open', token, { from, to, limit, offset })
  },

  getOpen(token, id) {
    return apiRequest(`/api/v1/complaints/call-center/${id}`, { token }).then((payload) => payload?.data ?? null)
  },

  getCompleted(token, id) {
    return apiRequest(`/api/v1/complaints/call-center/completed/${id}`, { token }).then((payload) => payload?.data ?? null)
  },

  listOpenHistory(token, id, { limit = 50, offset = 0 } = {}) {
    return readPaged(`/api/v1/complaints/call-center/${id}/history`, token, { limit, offset })
  },

  listCompletedHistory(token, id, { limit = 50, offset = 0 } = {}) {
    return readPaged(`/api/v1/complaints/call-center/completed/${id}/history`, token, { limit, offset })
  },

  listStatusOptions(token, id) {
    return readPaged(`/api/v1/complaints/call-center/${id}/status-options`, token)
  },

  updateStatus(token, { id, expectedStatusCode, statusCode, description, idempotencyKey }) {
    return apiRequest(`/api/v1/complaints/call-center/${id}/status`, {
      method: 'POST',
      token,
      headers: { 'Idempotency-Key': idempotencyKey },
      body: {
        expected_status_code: expectedStatusCode,
        status_code: statusCode,
        description,
      },
    })
  },

  countOpen(token) {
    return complaintApi.count(token, '/api/v1/complaints/call-center/open/count')
  },

  countReceivedOpen(token) {
    return complaintApi.count(token, '/api/v1/complaints/call-center/received-open/count')
  },

  countSentOpen(token) {
    return complaintApi.count(token, '/api/v1/complaints/call-center/sent-open/count')
  },

  countCompleted(token) {
    return complaintApi.count(token, '/api/v1/complaints/call-center/completed/count')
  },
}