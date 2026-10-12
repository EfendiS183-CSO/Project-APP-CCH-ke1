import { useContext, useEffect, useState } from 'react'
import { AlertCircle, Inbox, Loader2, RefreshCw } from 'lucide-react'
import { AuthContext } from '../../../auth/auth-context.js'
import { reviewApi } from '../api/reviewApi.js'
import { DraftQueueList, QueuePagination } from '../components/DraftQueueList.jsx'

const PAGE_SIZE = 25

export default function ReviewQueuePage() {
  const { token } = useContext(AuthContext)
  const [drafts, setDrafts] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [offset, setOffset] = useState(0)

  function loadDrafts(nextOffset = 0) {
    setLoading(true)
    setError('')
    reviewApi.listDrafts(token, { limit: PAGE_SIZE, offset: nextOffset })
      .then((response) => {
        setDrafts(response?.data || [])
        setOffset(nextOffset)
      })
      .catch((loadError) => setError(loadError.message || 'Gagal memuat antrean draft.'))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    let cancelled = false
    reviewApi.listDrafts(token, { limit: PAGE_SIZE, offset: 0 })
      .then((response) => {
        if (!cancelled) setDrafts(response?.data || [])
      })
      .catch((loadError) => {
        if (!cancelled) setError(loadError.message || 'Gagal memuat antrean draft.')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [token])

  return (
    <div className="page-wide">
      <div className="page-heading">
        <div>
          <p className="eyebrow">Call Center Review</p>
          <h1>Antrean draft</h1>
          <p className="page-heading-sub">Draft yang belum dapat dicocokkan profil pelanggan, menunggu verifikasi staf.</p>
        </div>
        <button type="button" className="btn btn-secondary" onClick={() => loadDrafts(0)} disabled={loading}>
          <RefreshCw size={15} className={loading ? 'spin' : ''} />
          Muat ulang
        </button>
      </div>

      {error ? <div className="alert alert-error" role="alert"><AlertCircle size={16} /><span>{error}</span></div> : null}

      {loading ? (
        <div className="queue-state"><Loader2 size={22} className="spin" /><span>Memuat antrean…</span></div>
      ) : drafts.length === 0 ? (
        <div className="queue-state"><Inbox size={26} /><span>Tidak ada draft menunggu review.</span></div>
      ) : (
        <>
          <DraftQueueList drafts={drafts} />
          <QueuePagination offset={offset} count={drafts.length} loading={loading} onPage={loadDrafts} />
        </>
      )}
    </div>
  )
}
