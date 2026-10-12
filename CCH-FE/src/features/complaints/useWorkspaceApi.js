import { useReducer } from 'react'

const initialState = { data: null, loading: true, error: '' }

function reducer(state, action) {
  switch (action.type) {
    case 'loading':
      return { ...state, loading: true, error: '' }
    case 'success':
      return { data: action.data, loading: false, error: '' }
    case 'error':
      return { data: null, loading: false, error: action.error }
    default:
      return state
  }
}

/**
 * Loader wrapper that returns data/loading/error plus a stable refresh
 * callback. The loader is expected to be memoized by the caller.
 */
export function useWorkspaceApi(loader) {
  const [state, dispatch] = useReducer(reducer, initialState)

  function refresh() {
    dispatch({ type: 'loading' })
    Promise.resolve(loader())
      .then((data) => dispatch({ type: 'success', data }))
      .catch((error) => dispatch({ type: 'error', error: error.message || 'Gagal memuat data.' }))
  }

  return { data: state.data, loading: state.loading, error: state.error, refresh }
}