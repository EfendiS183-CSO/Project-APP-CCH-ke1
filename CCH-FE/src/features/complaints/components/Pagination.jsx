import { formatCount } from '../format.js'

export function Pagination({ offset, count, perPage = 25, loading, onPage, label = 'baris' }) {
  if (loading || !count) return null
  const page = Math.floor(offset / perPage) + 1
  const hasPrevious = offset > 0
  const hasNext = count === perPage

  return (
    <div className="pagination">
      <span className="pagination-meta">
        Menampilkan {formatCount(offset + 1)}–{formatCount(offset + count)} · {formatCount(count)} {label}
      </span>
      <div className="pagination-actions">
        <button
          type="button"
          className="btn btn-secondary btn-small"
          disabled={!hasPrevious}
          onClick={() => onPage(offset - perPage)}
        >
          Sebelumnya
        </button>
        <span className="pagination-page">Halaman {page}</span>
        <button
          type="button"
          className="btn btn-secondary btn-small"
          disabled={!hasNext}
          onClick={() => onPage(offset + perPage)}
        >
          Berikutnya
        </button>
      </div>
    </div>
  )
}