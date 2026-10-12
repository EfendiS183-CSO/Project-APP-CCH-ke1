import { apiRequest } from '../../../shared/api/apiRequest.js'

/**
 * Call Center document metadata client following the CCH-BE v1 contract.
 * Only metadata is exposed; file names, paths, and bytes stay server-side.
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

export const documentApi = {
  listForTicket(token, id, { limit = 25, offset = 0 } = {}) {
    return readPaged(`/api/v1/complaints/call-center/${id}/documents`, token, { limit, offset })
  },
}