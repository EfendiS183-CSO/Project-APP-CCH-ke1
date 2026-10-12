import { Clock3, Loader2 } from 'lucide-react'
import { StatusBadge } from './StatusBadge.jsx'
import { EmptyState } from './EmptyState.jsx'
import { formatDateTime, formatDuration } from '../format.js'

export function TimelinePanel({ items, loading, error, reload }) {
  if (loading) {
    return (
      <article className="panel timeline-panel">
        <div className="queue-state"><Loader2 size={22} className="spin" /><span>Memuat riwayat…</span></div>
      </article>
    )
  }

  if (error) {
    return (
      <article className="panel timeline-panel">
        <div className="alert alert-error" role="alert"><span>{error}</span></div>
      </article>
    )
  }

  return (
    <article className="panel timeline-panel">
      <div className="panel-head">
        <div>
          <p className="section-eyebrow">Riwayat status</p>
          <h3>Timeline</h3>
        </div>
        <button type="button" className="btn btn-secondary btn-small" onClick={reload}>
          Muat ulang
        </button>
      </div>

      {!items || items.length === 0 ? (
        <EmptyState title="Belum ada riwayat status" description="Perubahan status akan dicatat di sini." />
      ) : (
        <ol className="timeline">
          {items.map((entry) => (
            <li key={`${entry.history_id || entry.updated_at}-${entry.status_code}`} className="timeline-item">
              <span className="timeline-rail" aria-hidden="true" />
              <div className="timeline-head">
                <StatusBadge code={entry.status_code} label={entry.status_description} size="sm" />
                <span className="timeline-date">{formatDateTime(entry.updated_at)}</span>
              </div>
              <div className="timeline-body">
                <p className="timeline-note">{entry.description || 'Tidak ada catatan.'}</p>
                <div className="timeline-meta">
                  <span>Oleh {entry.updated_by || '—'}</span>
                  <span>Kantor {entry.origin_office_code || '—'}</span>
                  {entry.duration_seconds !== null && entry.duration_seconds !== undefined ? (
                    <span className="timeline-duration">
                      <Clock3 size={12} />
                      {formatDuration(entry.duration_seconds)}
                    </span>
                  ) : null}
                </div>
              </div>
            </li>
          ))}
        </ol>
      )}
    </article>
  )
}