export class ApiError extends Error {
  constructor(code, message, requestId, status) {
    super(message || code || 'Permintaan gagal')
    this.name = 'ApiError'
    this.code = code || 'internal_error'
    this.requestId = requestId || null
    this.status = status || 0
  }
}

/**
 * Thin fetch wrapper following the CCH-BE HTTP contract.
 * Staff endpoints require `X-Authorization: Bearer <token>`; guest intake
 * endpoints require an `X-Intake-Session` conversation token.
 */
export async function apiRequest(path, { method = 'GET', headers = {}, body, token, sessionToken } = {}) {
  const requestHeaders = { Accept: 'application/json', ...headers }

  if (body !== undefined) {
    requestHeaders['Content-Type'] = 'application/json'
  }
  if (token) requestHeaders['X-Authorization'] = `Bearer ${token}`
  if (sessionToken) requestHeaders['X-Intake-Session'] = sessionToken

  let response
  try {
    response = await fetch(path, {
      method,
      headers: requestHeaders,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })
  } catch {
    throw new ApiError('network_error', 'Tidak dapat terhubung ke server. Periksa koneksi jaringan Anda.')
  }

  const requestId = response.headers.get('X-Request-ID') || undefined
  const contentType = response.headers.get('Content-Type') || ''
  let payload = null
  if (contentType.includes('application/json')) {
    try {
      payload = await response.json()
    } catch {
      payload = null
    }
  }

  if (!response.ok) {
    const errorBody = payload?.error ?? {}
    throw new ApiError(
      errorBody.code || 'request_failed',
      errorBody.message || `Permintaan gagal (${response.status})`,
      requestId,
      response.status,
    )
  }

  return payload
}