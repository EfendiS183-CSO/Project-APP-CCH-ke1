import { useContext, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { AlertCircle, ArrowLeft, Inbox, Loader2, Send } from 'lucide-react'
import { AuthContext } from '../../auth/auth-context.js'
import { complaintApi } from '../api/complaintApi.js'
import { Section } from '../components/Section.jsx'
import { ComplaintTable } from '../components/ComplaintTable.jsx'
import { EmptyState } from '../components/EmptyState.jsx'
import { Pagination } from '../components/Pagination.jsx'

const PAGE_SIZE = 25

export function ReceivedOpenComplaintsPage() {
  const { token } = useContext(AuthContext)
  const [items, setItems] = useState([])
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    complaintApi
      .listReceivedOpen(token, { limit: PAGE_SIZE, offset })
      .then((rows) => {
        if (!cancelled) setItems(rows)
      })
      .catch((loadError) => {
        if (!cancelled) setError(loadError.message || 'Gagal memuat tiket diterima.')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [token, offset])

  function changePage(nextOffset) {
    setLoading(true)
    setError('')
    setOffset(nextOffset)
  }

  return (
    <div className="page-wide workspace-page">
      <Link to="/workspace" className="back-link">
        <ArrowLeft size={14} />
        Kembali ke dasbor
      </Link>

      <div className="page-heading">
        <div>
          <p className="eyebrow">Call Center Workspace</p>
          <h1>Diterima terbuka</h1>
          <p className="page-heading-sub">
            Tiket terbuka yang diteruskan ke kantor Anda, berdasarkan token kantor tujuan.
          </p>
        </div>
      </div>

      {error ? (
        <div className="alert alert-error" role="alert">
          <AlertCircle size={16} />
          <span>{error}</span>
        </div>
      ) : null}

      <Section title="Tiket diterima terbuka">
        {loading ? (
          <div className="queue-state">
            <Loader2 size={22} className="spin" />
            <span>Memuat…</span>
          </div>
        ) : items.length === 0 ? (
          <EmptyState icon={Inbox} title="Tidak ada tiket diterima terbuka" />
        ) : (
          <>
            <ComplaintTable items={items} variant="received" title="diterima" />
            <Pagination offset={offset} count={items.length} perPage={PAGE_SIZE} onPage={changePage} loading={loading} />
          </>
        )}
      </Section>
    </div>
  )
}

export function SentOpenComplaintsPage() {
  const { token } = useContext(AuthContext)
  const [items, setItems] = useState([])
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    complaintApi
      .listSentOpen(token, { limit: PAGE_SIZE, offset })
      .then((rows) => {
        if (!cancelled) setItems(rows)
      })
      .catch((loadError) => {
        if (!cancelled) setError(loadError.message || 'Gagal memuat tiket dikirim.')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [token, offset])

  function changeSentPage(nextOffset) {
    setLoading(true)
    setError('')
    setOffset(nextOffset)
  }

  return (
    <div className="page-wide workspace-page">
      <Link to="/workspace" className="back-link">
        <ArrowLeft size={14} />
        Kembali ke dasbor
      </Link>

      <div className="page-heading">
        <div>
          <p className="eyebrow">Call Center Workspace</p>
          <h1>Dikirim terbuka</h1>
          <p className="page-heading-sub">
            Tiket terbuka berawal di kantor Anda yang masih diproses di tempat lain.
          </p>
        </div>
      </div>

      {error ? (
        <div className="alert alert-error" role="alert">
          <AlertCircle size={16} />
          <span>{error}</span>
        </div>
      ) : null}

      <Section title="Tiket dikirim terbuka">
        {loading ? (
          <div className="queue-state">
            <Loader2 size={22} className="spin" />
            <span>Memuat…</span>
          </div>
        ) : items.length === 0 ? (
          <EmptyState icon={Send} title="Tidak ada tiket dikirim terbuka" />
        ) : (
          <>
            <ComplaintTable items={items} variant="sent" title="dikirim" />
            <Pagination offset={offset} count={items.length} perPage={PAGE_SIZE} onPage={changeSentPage} loading={loading} />
          </>
        )}
      </Section>
    </div>
  )
}