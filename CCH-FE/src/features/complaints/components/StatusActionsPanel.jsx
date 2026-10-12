import { useContext, useEffect, useState } from 'react'
import { Loader2, Send } from 'lucide-react'
import { AuthContext } from '../../auth/auth-context.js'
import { complaintApi } from '../../complaints/api/complaintApi.js'
import { StatusBadge } from '../../complaints/components/StatusBadge.jsx'
import { statusMeta, waitingForCode } from '../../complaints/format.js'

function newIdempotencyKey() {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) return crypto.randomUUID()
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export function StatusActionsPanel({ complaintId, statusCode, onTransitioned }) {
  const { token } = useContext(AuthContext)
  const [options, setOptions] = useState([])
  const [selected, setSelected] = useState('')
  const [description, setDescription] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    complaintApi
      .listStatusOptions(token, complaintId)
      .then((rows) => {
        if (!cancelled) {
          setOptions(rows)
          setSelected(rows[0]?.code || '')
        }
      })
      .catch((loadError) => {
        if (!cancelled) setError(loadError.message || 'Gagal memuat opsi status.')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [token, complaintId])

  const currentMeta = statusMeta(statusCode)
  const canTransition = waitingForCode(statusCode) && options.some((option) => option.code === selected)

  async function handleSubmit(event) {
    event.preventDefault()
    if (!canTransition || submitting) return
    setSubmitting(true)
    setError('')
    try {
      await complaintApi.updateStatus(token, {
        id: complaintId,
        expectedStatusCode: statusCode,
        statusCode: selected,
        description,
        idempotencyKey: newIdempotencyKey(),
      })
      setDescription('')
      onTransitioned?.()
    } catch (transitionError) {
      setError(transitionError.message || 'Gagal memperbarui status.')
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) {
    return (
      <article className="panel status-actions-panel">
        <div className="queue-state"><Loader2 size={20} className="spin" /><span>Memuat opsi status…</span></div>
      </article>
    )
  }

  if (error && options.length === 0) {
    return (
      <article className="panel status-actions-panel">
        <div className="alert alert-error" role="alert"><span>{error}</span></div>
      </article>
    )
  }

  return (
    <article className="panel status-actions-panel">
      <div className="panel-head">
        <div>
          <p className="section-eyebrow">Perbarui status</p>
          <h3>Transisi tiket</h3>
        </div>
        <StatusBadge code={statusCode} label={currentMeta.label} />
      </div>

      {!waitingForCode(statusCode) ? (
        <p className="status-blocked">
          Tiket ini sudah tidak bisa menerima perubahan status (status akhir <code>{statusCode}</code>).
        </p>
      ) : (
        <form onSubmit={handleSubmit} className="status-form">
          <div className="status-options">
            {options.map((option) => (
              <label key={option.code} className={`status-option ${selected === option.code ? 'is-selected' : ''}`}>
                <input
                  type="radio"
                  name={`status-${complaintId}`}
                  value={option.code}
                  checked={selected === option.code}
                  onChange={(event) => setSelected(event.target.value)}
                />
                <StatusBadge code={option.code} label={option.description} size="sm" />
              </label>
            ))}
          </div>

          <label className="field-group">
            <span className="field-label">Catatan transisi</span>
            <textarea
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              rows={3}
              placeholder="Deskripsi perubahan status (opsional)"
              maxLength={500}
            />
          </label>

          {error ? <div className="alert alert-error" role="alert"><span>{error}</span></div> : null}

          <button type="submit" className="btn btn-primary" disabled={!canTransition || submitting}>
            {submitting ? <Loader2 size={15} className="spin" /> : <Send size={15} />}
            {submitting ? 'Mengirim…' : 'Simpan status'}
          </button>
        </form>
      )}
    </article>
  )
}