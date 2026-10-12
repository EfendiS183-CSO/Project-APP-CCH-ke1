import { useContext, useState } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'
import { KeyRound, LogIn, ShieldCheck } from 'lucide-react'
import { AuthContext } from '../auth-context.js'

export default function LoginPage() {
  const { login, isAuthenticated } = useContext(AuthContext)
  const navigate = useNavigate()
  const location = useLocation()
  const [token, setToken] = useState('')
  const [error, setError] = useState('')

  const from = location.state?.from || '/review'

  if (isAuthenticated) {
    return <Navigate to={from} replace />
  }

  function handleSubmit(event) {
    event.preventDefault()
    const trimmed = token.trim()
    if (!trimmed) {
      setError('Masukkan token JWT staf untuk melanjutkan.')
      return
    }
    login(trimmed)
    void navigate(from, { replace: true })
  }

  return (
    <div className="page-narrow">
      <section className="panel login-panel">
        <div className="login-panel-head">
          <span className="icon-tile">
            <ShieldCheck size={20} />
          </span>
          <div>
            <p className="eyebrow">Sesi staf Call Center</p>
            <h1>Sambungkan sesi staf</h1>
          </div>
        </div>
        <p className="login-hint">
          Backend memvalidasi token <code>X-Authorization</code> dan memuat ulang role/permission dari CCH.
          Tidak ada endpoint login staf pada BE; masukkan token yang diterbitkan oleh CCH.
        </p>
        <form onSubmit={handleSubmit} className="form-stack">
          <label className="field">
            <span>Token JWT staf</span>
            <input
              type="password"
              autoComplete="off"
              value={token}
              onChange={(event) => setToken(event.target.value)}
              placeholder="eyJhbGciOiJIUzUxMiIs..."
            />
          </label>
          {error ? <p className="form-error" role="alert">{error}</p> : null}
          <button type="submit" className="btn btn-primary btn-block">
            <KeyRound size={16} />
            Sambungkan
          </button>
        </form>
        <p className="login-note">
          <LogIn size={14} />
          Token disimpan hanya pada sesi tab browser ini dan dihapus saat keluar.
        </p>
      </section>
    </div>
  )
}
