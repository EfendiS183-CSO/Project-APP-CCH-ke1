import { useContext } from 'react'
import { Link } from 'react-router-dom'
import { AlertCircle, ArrowRight, CalendarCheck, CheckCircle2, ClipboardList, Loader2 } from 'lucide-react'
import { AuthContext } from '../../auth/auth-context.js'
import { followUpsApi } from '../api/followUpsApi.js'
import { Section } from '../../complaints/components/Section.jsx'
import { EmptyState } from '../../complaints/components/EmptyState.jsx'
import { StatusBadge } from '../../complaints/components/StatusBadge.jsx'
import { formatDateTime } from '../../complaints/format.js'
import { useWorkspaceApi } from '../../complaints/useWorkspaceApi.js'

export function FollowUpsWorkspacePage() {
  const { token } = useContext(AuthContext)

  const office = useWorkspaceApi(() => followUpsApi.listForOffice(token, { limit: 8 }), [token])
  const dueToday = useWorkspaceApi(() => followUpsApi.listDueToday(token, { limit: 8 }), [token])

  function renderList(state, emptyTitle) {
    if (state.loading) {
      return (
        <div className="queue-state">
          <Loader2 size={20} className="spin" />
          <span>Memuat…</span>
        </div>
      )
    }
    if (state.error) {
      return (
        <div className="alert alert-error" role="alert">
          <AlertCircle size={16} />
          <span>{state.error}</span>
        </div>
      )
    }
    if (!state.data || state.data.length === 0) {
      return <EmptyState icon={ClipboardList} title={emptyTitle} />
    }
    return (
      <ul className="followup-list">
        {state.data.map((entry) => (
          <li key={`${entry.complaint_id}-${entry.office_code}-${entry.status_code}`} className="followup-item">
            <span className="followup-icon" aria-hidden="true">
              {entry.realized_at ? <CheckCircle2 size={16} /> : <CalendarCheck size={16} />}
            </span>
            <div className="followup-body">
              <div className="followup-head">
                <StatusBadge code={entry.status_code} size="sm" />
                <span className="followup-office">{entry.office_code}</span>
              </div>
              <p className="followup-note">{entry.plan_notes || 'Tidak ada catatan rencana.'}</p>
              <div className="followup-meta">
                <span>Rencana {formatDateTime(entry.planned_at)}</span>
                {entry.realized_at ? (
                  <span>Realisasi {formatDateTime(entry.realized_at)}</span>
                ) : (
                  <span className="followup-pending">Belum direalisasi</span>
                )}
              </div>
            </div>
          </li>
        ))}
      </ul>
    )
  }

  return (
    <div className="page-wide workspace-followups">
      <div className="page-heading">
        <div>
          <p className="eyebrow">Call Center Workspace</p>
          <h1>Follow-up</h1>
          <p className="page-heading-sub">
            Rencana dan realisasi tindak lanjut untuk kantor Anda, termasuk yang jatuh tempo hari ini.
          </p>
        </div>
        <Link to="/workspace/follow-ups/all" className="btn btn-secondary">
          Lihat semua <ArrowRight size={14} />
        </Link>
      </div>

      <div className="followup-grid">
        <Section eyebrow="Rencana" title="Jatuh tempo hari ini" actions={
          <Link to="/workspace/follow-ups/due-today" className="btn btn-secondary btn-small">Lihat semua</Link>
        }>
          {renderList(dueToday, 'Tidak ada follow-up jatuh tempo hari ini.')}
        </Section>
        <Section eyebrow="Kantor Anda" title="Follow-up terbaru" actions={
          <Link to="/workspace/follow-ups/all" className="btn btn-secondary btn-small">Lihat semua</Link>
        }>
          {renderList(office, 'Tidak ada follow-up kantor Anda.')}
        </Section>
      </div>
    </div>
  )
}