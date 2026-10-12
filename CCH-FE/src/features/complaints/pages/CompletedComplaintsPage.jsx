import { useContext, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { AlertCircle, ArrowLeft, CheckCircle2, Loader2 } from 'lucide-react'
import { AuthContext } from '../../auth/auth-context.js'
import { complaintApi } from '../api/complaintApi.js'
import { Section } from '../components/Section.jsx'
import { ComplaintTable } from '../components/ComplaintTable.jsx'
import { EmptyState } from '../components/EmptyState.jsx'
import { Pagination } from '../components/Pagination.jsx'

const PAGE_SIZE = 25

export function CompletedComplaintsPage() {
  const { token } = useContext(AuthContext)
  const [items, setItems] = useState([])
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    complaintApi
      .listCompleted(token, { limit: PAGE_SIZE, offset })
      .then((rows) => {
        if (!cancelled) setItems(rows)
      })
      .catch((loadError) => {
        if (!cancelled) setError(loadError.message || 'Gagal memuat tiket selesai.')
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
          <h1>Tiket selesai</h1>
          <p className="page-heading-sub">
            Riwayat tiket berstatus 109 (SELESAI) yang terlihat oleh kantor Anda.
          </p>
        </div>
      </div>

      {error ? (
        <div className="alert alert-error" role="alert">
          <AlertCircle size={16} />
          <span>{error}</span>
        </div>
      ) : null}

      <Section title="Daftar tiket selesai">
        {loading ? (
          <div className="queue-state">
            <Loader2 size={22} className="spin" />
            <span>Memuat…</span>
          </div>
        ) : items.length === 0 ? (
          <EmptyState
            icon={CheckCircle2}
            title="Belum ada tiket selesai"
            description="Tiket berstatus SELESAI akan tampil di sini."
          />
        ) : (
          <>
            <ComplaintTable items={items} variant="closed" title="selesai" />
            <Pagination offset={offset} count={items.length} perPage={PAGE_SIZE} onPage={changePage} loading={loading} />
          </>
        )}
      </Section>
    </div>
  )
}