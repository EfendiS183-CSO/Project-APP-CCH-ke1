import { apiRequest } from '../../../../shared/api/apiRequest.js'

/** Staff intake-review API. The backend revalidates JWT role, office, and ACL. */
export const reviewApi = {
  listDrafts(token, { limit = 25, offset = 0 } = {}) {
    const query = new URLSearchParams({ limit: String(limit), offset: String(offset) })
    return apiRequest(`/api/v1/intake/review/drafts?${query}`, { token })
  },

  getDraft(token, id) {
    return apiRequest(`/api/v1/intake/review/drafts/${id}`, { token })
  },

  searchCustomers(token, q) {
    const query = new URLSearchParams({ q })
    return apiRequest(`/api/v1/intake/review/customers/search?${query}`, { token })
  },

  createTicket(token, id, body) {
    return apiRequest(`/api/v1/intake/review/drafts/${id}/create-ticket`, {
      method: 'POST',
      body,
      token,
    })
  },
}
