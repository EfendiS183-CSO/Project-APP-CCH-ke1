import { useContext } from 'react'
import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { AuthContext } from '../../features/auth/auth-context.js'

/** Redirects unauthenticated visitors to the staff login page. */
export function RequireAuth({ children }) {
  const { token } = useContext(AuthContext)
  const location = useLocation()

  if (!token) {
    return <Navigate to="/login" state={{ from: location.pathname }} replace />
  }
  return children ?? <Outlet />
}