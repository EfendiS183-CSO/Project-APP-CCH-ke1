import { useContext, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { AlertCircle, ArrowLeft, Inbox, Loader2 } from 'lucide-react'
import { AuthContext } from '../../auth/auth-context.js'
import { complaintApi } from '../api/complaintApi.js'
import { Section } from '../components/Section.jsx'
import { ComplaintTable } from '../components/ComplaintTable.jsx'
import { EmptyState } from '../components/EmptyState.jsx'
import { Pagination } from '../components/Pagination.jsx'

const PAGE_SIZE = 25

export function OpenComplaintsPage() {
  const { token } = useContext(AuthContext)
  const [items, setItems] = useState([])
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    complaintApi
      .listOpen(token, { limit: PAGE_SIZE, offset })
      .then((rows) => {
        if (!cancelled) setItems(rows)
      })
      .catch((loadError) => {
        if (!cancelled) setError(loadError.message || 'Gagal memuat tiket terbuka.')
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
          <h1>Tiket terbuka</h1>
          <p className="page-heading-sub">
            Tiket dengan status aktif dan visible untuk kantor Anda, berdasarkan routing kantor asal/tujuan.
          </p>
        </div>
      </div>

      {error ? (
        <div className="alert alert-error" role="alert">
          <AlertCircle size={16} />
          <span>{error}</span>
        </div>
      ) : null}

      <Section title="Daftar tiket terbuka">
        {loading ? (
          <div className="queue-state">
            <Loader2 size={22} className="spin" />
            <span>Memuat…</span>
          </div>
        ) : items.length === 0 ? (
          <EmptyState
            icon={Inbox}
            title="Tidak ada tiket terbuka"
            description="Tiket baru akan tampil di sini sesuai scope kantor Anda."
          />
        ) : (
          <>
            <ComplaintTable items={items} variant="open" title="terbuka" />
            <Pagination offset={offset} count={items.length} perPage={PAGE_SIZE} onPage={changePage} loading={loading} />
          </>
        )}
      </Section>
    </div>
  )
}