import { useContext, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { AlertCircle, ArrowLeft, CheckCircle2, ClipboardList, Loader2 } from 'lucide-react'
import { AuthContext } from '../../auth/auth-context.js'
import { followUpsApi } from '../api/followUpsApi.js'
import { Section } from '../../complaints/components/Section.jsx'
import { EmptyState } from '../../complaints/components/EmptyState.jsx'
import { Pagination } from '../../complaints/components/Pagination.jsx'
import { StatusBadge } from '../../complaints/components/StatusBadge.jsx'
import { formatDateTime } from '../../complaints/format.js'

const PAGE_SIZE = 25

function renderFollowUpState({ loading, error, empty, children }) {
  if (loading) {
    return (
      <div className="queue-state">
        <Loader2 size={20} className="spin" />
        <span>Memuat…</span>
      </div>
    )
  }
  if (error) {
    return (
      <div className="alert alert-error" role="alert">
        <AlertCircle size={16} />
        <span>{error}</span>
      </div>
    )
  }
  if (empty) return empty
  return children
}

function FollowUpList({ items }) {
  if (!items || items.length === 0) return null
  return (
    <div className="table-scroll">
      <table className="complaint-table">
        <thead>
          <tr>
            <th>ID tiket</th>
            <th>Kantor</th>
            <th>Status</th>
            <th>Rencana</th>
            <th>Realisasi</th>
            <th>Catatan</th>
          </tr>
        </thead>
        <tbody>
          {items.map((entry) => (
            <tr key={`${entry.complaint_id}-${entry.office_code}-${entry.status_code}`}>
              <td>
                <Link to={`/workspace/complaints/open/${entry.complaint_id}`} className="table-ticket-id">
                  <span className="ticket-id-text">{entry.complaint_id}</span>
                </Link>
              </td>
              <td><span className="cell-monospace">{entry.office_code}</span></td>
              <td><StatusBadge code={entry.status_code} size="sm" /></td>
              <td><span className="cell-date">{formatDateTime(entry.planned_at)}</span></td>
              <td>
                <span className="cell-date">
                  {entry.realized_at ? (
                    <>
                      <CheckCircle2 size={13} />
                      {formatDateTime(entry.realized_at)}
                    </>
                  ) : (
                    <span className="followup-pending">Belum</span>
                  )}
                </span>
              </td>
              <td className="cell-note">{entry.plan_notes || '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function FollowUpPageShell({ eyebrow, title, subtitle, loadPage }) {
  const { token } = useContext(AuthContext)
  const [items, setItems] = useState([])
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    Promise.resolve(loadPage(token, offset))
      .then((rows) => {
        if (!cancelled) setItems(rows)
      })
      .catch((loadError) => {
        if (!cancelled) setError(loadError.message || 'Gagal memuat data follow-up.')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [token, offset, loadPage])

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
          <p className="eyebrow">{eyebrow}</p>
          <h1>{title}</h1>
          <p className="page-heading-sub">{subtitle}</p>
        </div>
      </div>

      {error ? (
        <div className="alert alert-error" role="alert">
          <AlertCircle size={16} />
          <span>{error}</span>
        </div>
      ) : null}

      <Section title={title}>
        {renderFollowUpState({
          loading,
          error,
          empty: (
            <EmptyState
              icon={ClipboardList}
              title="Tidak ada follow-up"
              description="Rencana follow-up kantor Anda akan tampil di sini."
            />
          ),
          children: (
            <>
              <FollowUpList items={items} />
              <Pagination offset={offset} count={items.length} perPage={PAGE_SIZE} onPage={changePage} loading={loading} />
            </>
          ),
        })}
      </Section>
    </div>
  )
}

export function OfficeFollowUpsPage() {
  return (
    <FollowUpPageShell
      backTo="/workspace"
      eyebrow="Call Center Workspace"
      title="Follow-up kantor"
      subtitle="Rencana tindak lanjut milik kantor Anda yang belum/telah direalisasikan."
      loadPage={(token, offset) => followUpsApi.listForOffice(token, { limit: PAGE_SIZE, offset })}
    />
  )
}

export function DueTodayFollowUpsPage() {
  return (
    <FollowUpPageShell
      backTo="/workspace/follow-ups"
      eyebrow="Call Center Workspace"
      title="Jatuh tempo hari ini"
      subtitle="Follow-up terjadwal untuk hari ini yang belum direalisasikan, dari view jatuh tempo kantor Anda."
      loadPage={(token, offset) => followUpsApi.listDueToday(token, { limit: PAGE_SIZE, offset })}
    />
  )
}