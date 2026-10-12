import { apiRequest } from '../../../shared/api/apiRequest.js'

/**
 * Call Center follow-up API client following the CCH-BE v1 contract.
 * All reads require a staff JWT via X-Authorization; office scope is
 * revalidated server-side and never supplied by the client.
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

export const followUpsApi = {
  listForTicket(token, id, { limit = 25, offset = 0 } = {}) {
    return readPaged(`/api/v1/complaints/call-center/${id}/follow-ups`, token, { limit, offset })
  },

  listForOffice(token, { limit = 25, offset = 0 } = {}) {
    return readPaged('/api/v1/follow-ups/call-center/office', token, { limit, offset })
  },

  listDueToday(token, { limit = 25, offset = 0 } = {}) {
    return readPaged('/api/v1/follow-ups/call-center/office/due-today', token, { limit, offset })
  },
}