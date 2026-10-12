import { CalendarClock, CheckCircle2, ClipboardList, Loader2 } from 'lucide-react'
import { StatusBadge } from './StatusBadge.jsx'
import { EmptyState } from './EmptyState.jsx'
import { formatDateTime } from '../format.js'

export function FollowUpPanel({ items, loading, error, reload }) {
  return (
    <article className="panel followup-panel">
      <div className="panel-head">
        <div>
          <p className="section-eyebrow">Kegiatan lanjutan</p>
          <h3>Follow-up</h3>
        </div>
        <button type="button" className="btn btn-secondary btn-small" onClick={reload}>
          Muat ulang
        </button>
      </div>

      {loading ? (
        <div className="queue-state"><Loader2 size={22} className="spin" /><span>Memuat follow-up…</span></div>
      ) : error ? (
        <div className="alert alert-error" role="alert"><span>{error}</span></div>
      ) : !items || items.length === 0 ? (
        <EmptyState
          icon={ClipboardList}
          title="Tidak ada rencana follow-up"
          description="Rencana tindak lanjut kantor-kantor terkait tampil di sini."
        />
      ) : (
        <ul className="followup-list">
          {items.map((entry) => (
            <li key={`${entry.complaint_id}-${entry.office_code}-${entry.status_code}`} className="followup-item">
              <span className="followup-icon" aria-hidden="true">
                {entry.realized_at ? <CheckCircle2 size={16} /> : <CalendarClock size={16} />}
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
                    <>
                      <span>Realisasi {formatDateTime(entry.realized_at)}</span>
                      {entry.outcome ? <span className="followup-outcome">{entry.outcome}</span> : null}
                    </>
                  ) : (
                    <span className="followup-pending">Belum direalisasi</span>
                  )}
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}
    </article>
  )
}