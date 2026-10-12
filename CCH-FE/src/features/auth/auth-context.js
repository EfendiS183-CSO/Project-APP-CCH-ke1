import { createContext } from 'react'

/** Shared auth state; consumers read this context, the provider lives in AuthContext.jsx. */
export const AuthContext = createContext(null)