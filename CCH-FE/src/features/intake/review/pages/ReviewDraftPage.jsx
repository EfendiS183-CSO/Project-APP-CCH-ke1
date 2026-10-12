import { useContext, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { AlertCircle, ArrowLeft, Loader2 } from 'lucide-react'
import { AuthContext } from '../../../auth/auth-context.js'
import { reviewApi } from '../api/reviewApi.js'
import { CustomerProfilePicker } from '../components/CustomerProfilePicker.jsx'
import { DraftPayloadPanel } from '../components/DraftPayloadPanel.jsx'
import { TicketCreationPanel } from '../components/TicketCreationPanel.jsx'

const EMPTY_CORRECTIONS = {
  Kantor_Tujuan: '',
  Kantor_Asal_Kiriman: '',
  Kantor_Tujuan_Kiriman: '',
}

export default function ReviewDraftPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { token } = useContext(AuthContext)
  const [draft, setDraft] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState(null)
  const [identityVerified, setIdentityVerified] = useState(false)
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState('')
  const [corrections, setCorrections] = useState(EMPTY_CORRECTIONS)

  useEffect(() => {
    let cancelled = false
    reviewApi.getDraft(token, id)
      .then((detail) => {
        if (!cancelled) setDraft(detail)
      })
      .catch((loadError) => {
        if (!cancelled) setError(loadError.message || 'Gagal memuat detail draft.')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [id, token])

  async function handleCreate() {
    if (!selected || !identityVerified || creating || !draft) return
    setCreating(true)
    setCreateError('')
    const correctionsPayload = {}
    for (const key of Object.keys(EMPTY_CORRECTIONS)) {
      correctionsPayload[key] = corrections[key] || draft.payload?.[key] || ''
    }
    try {
      const result = await reviewApi.createTicket(token, id, {
        customer_id: selected.id,
        identity_verified: true,
        corrections: correctionsPayload,
      })
      if (result?.status === 'created') {
        void navigate('/review', { replace: true })
      } else {
        setCreateError(result?.reason
          ? `Pengajuan tetap di antrean: ${result.reason}`
          : 'Pengajuan belum menjadi tiket dan tetap di antrean.')
      }
    } catch (error_) {
      setCreateError(error_.message || 'Gagal membuat tiket.')
    } finally {
      setCreating(false)
    }
  }

  if (loading) {
    return <div className="page-wide queue-state"><Loader2 size={22} className="spin" /><span>Memuat draft…</span></div>
  }

  if (error || !draft) {
    return (
      <div className="page-wide">
        <div className="alert alert-error" role="alert"><AlertCircle size={16} /><span>{error || 'Draft tidak ditemukan.'}</span></div>
        <Link to="/review" className="btn btn-secondary"><ArrowLeft size={16} />Kembali ke antrean</Link>
      </div>
    )
  }

  return (
    <div className="page-wide review-draft-page">
      <div className="page-heading">
        <div>
          <Link to="/review" className="back-link"><ArrowLeft size={14} />Kembali ke antrean</Link>
          <p className="eyebrow">Draft #{draft.id}</p>
          <h1>{draft.summary || 'Draft tanpa ringkasan'}</h1>
          <p className="page-heading-sub">
            Status <code>{draft.status}</code>{draft.review_reason ? ` · alasan ${draft.review_reason}` : ''}{draft.ticket_id ? ` · tiket ${draft.ticket_id}` : ''}
          </p>
        </div>
        <div className="draft-meta">
          <span>Dibuat {new Date(draft.created_at).toLocaleString('id-ID')}</span>
          {draft.consent_at ? <span>Persetujuan {new Date(draft.consent_at).toLocaleString('id-ID')}</span> : null}
        </div>
      </div>

      <div className="review-layout">
        <DraftPayloadPanel draft={draft} />
        <CustomerProfilePicker token={token} selected={selected} onSelect={setSelected} />
      </div>

      <TicketCreationPanel
        draft={draft}
        selected={selected}
        corrections={corrections}
        onCorrectionsChange={setCorrections}
        identityVerified={identityVerified}
        onIdentityVerifiedChange={(event) => setIdentityVerified(event.target.checked)}
        creating={creating}
        createError={createError}
        onCreate={handleCreate}
      />
    </div>
  )
}
