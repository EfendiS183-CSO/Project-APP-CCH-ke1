import { useCallback, useMemo, useState } from 'react'
import { AuthContext } from './auth-context.js'

const STAFF_TOKEN_KEY = 'cch_staff_token'

function readStoredToken() {
  try {
    return window.sessionStorage.getItem(STAFF_TOKEN_KEY) || ''
  } catch {
    return ''
  }
}

export function AuthProvider({ children }) {
  const [token, setToken] = useState(readStoredToken)

  const login = useCallback((nextToken) => {
    try {
      window.sessionStorage.setItem(STAFF_TOKEN_KEY, nextToken)
    } catch {
      // Storage can be unavailable; the in-memory session still works.
    }
    setToken(nextToken)
  }, [])

  const logout = useCallback(() => {
    try {
      window.sessionStorage.removeItem(STAFF_TOKEN_KEY)
    } catch {
      // Ignore storage failures; the in-memory session is cleared below.
    }
    setToken('')
  }, [])

  const value = useMemo(
    () => ({ token, isAuthenticated: Boolean(token), login, logout }),
    [token, login, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}