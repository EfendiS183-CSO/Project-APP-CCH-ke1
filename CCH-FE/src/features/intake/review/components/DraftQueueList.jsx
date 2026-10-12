import { Link } from 'react-router-dom'
import { ChevronRight, ScanSearch } from 'lucide-react'

export function DraftQueueList({ drafts }) {
  return (
    <div className="queue-list">
      {drafts.map((draft) => (
        <Link key={draft.id} to={`/review/drafts/${draft.id}`} className="queue-row">
          <span className="queue-row-icon"><ScanSearch size={17} /></span>
          <span className="queue-row-body">
            <strong>{draft.summary || 'Draft tanpa ringkasan'}</strong>
            <small>#{draft.id} · dibuat {new Date(draft.created_at).toLocaleString('id-ID')}</small>
          </span>
          {draft.review_reason ? <code className="reason-chip">{draft.review_reason}</code> : null}
          <ChevronRight size={16} className="queue-row-arrow" />
        </Link>
      ))}
    </div>
  )
}

export function QueuePagination({ offset, count, loading, onPage }) {
  return (
    <div className="queue-pagination">
      <button
        type="button"
        className="btn btn-secondary btn-small"
        disabled={offset === 0 || loading}
        onClick={() => onPage(Math.max(0, offset - 25))}
      >
        Sebelumnya
      </button>
      <span>{offset + 1}–{offset + count}</span>
      <button
        type="button"
        className="btn btn-secondary btn-small"
        disabled={count < 25 || loading}
        onClick={() => onPage(offset + 25)}
      >
        Berikutnya
      </button>
    </div>
  )
}
